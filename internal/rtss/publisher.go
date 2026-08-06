package rtss

import (
	"sync"
	"time"
)

const (
	DefaultUpdateInterval    = time.Second
	retryInterval            = 2 * time.Second
	unchangedRefreshInterval = 5 * time.Second
)

// SupportedUpdateInterval returns the closest supported OSD refresh interval.
// Invalid values fall back to the default to keep manually edited config safe.
func SupportedUpdateInterval(interval time.Duration) time.Duration {
	switch interval {
	case 250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second:
		return interval
	default:
		return DefaultUpdateInterval
	}
}

type osdSink interface {
	Update(rpm uint16) bool
	Close()
}

// Publisher limits RTSS writes without changing the device polling frequency.
// It is safe to call from the HID/BLE data callback and the application lifecycle.
type Publisher struct {
	mu sync.Mutex

	sink osdSink
	now  func() time.Time

	enabled       bool
	interval      time.Duration
	positionMode  string
	positionX     int
	positionY     int
	positionDirty bool
	nextAttempt   time.Time
	lastSuccess   time.Time
	lastRPM       uint16
	hasLastRPM    bool
}

func New() *Publisher {
	return newPublisher(newSharedMemorySink(), time.Now)
}

func newPublisher(sink osdSink, now func() time.Time) *Publisher {
	return &Publisher{
		sink:         sink,
		now:          now,
		interval:     DefaultUpdateInterval,
		positionMode: "anchor",
	}
}

// SetPosition selects the sole source of RTSS cursor positioning. Anchor mode
// preserves the existing OverlayEditor behavior; custom mode emits an explicit
// <P=x,y> prefix before THRM's text.
func (p *Publisher) SetPosition(mode string, x, y int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if mode != "custom" {
		mode = "anchor"
	}
	if p.positionMode == mode && p.positionX == x && p.positionY == y {
		return
	}
	p.positionMode = mode
	p.positionX = x
	p.positionY = y
	p.positionDirty = true
	p.nextAttempt = time.Time{}
	if sink, ok := p.sink.(interface{ SetPosition(string, int, int) }); ok {
		sink.SetPosition(mode, x, y)
	}
	if !p.enabled || !p.hasLastRPM {
		return
	}

	// Position previews must redraw independently of the user-selected RPM
	// refresh interval, otherwise multiple small moves collapse into one jump.
	now := p.now()
	if !p.sink.Update(p.lastRPM) {
		// A short RTSS lock collision must not freeze an active drag. Keep the
		// last RPM and retry on the next preview or device report.
		p.nextAttempt = time.Time{}
		return
	}
	p.positionDirty = false
	p.lastSuccess = now
	p.nextAttempt = now.Add(p.interval)
}

// Configure applies the user-facing RTSS settings immediately. Disabling the
// publisher also removes THRM's OSD slot from RTSS.
func (p *Publisher) Configure(enabled bool, interval time.Duration) {
	if p == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	interval = SupportedUpdateInterval(interval)
	if p.interval != interval || p.enabled != enabled {
		p.nextAttempt = time.Time{}
	}
	p.interval = interval
	p.enabled = enabled
	if !enabled {
		p.closeLocked()
	}
}

// Publish writes the latest RPM when the configured interval has elapsed.
// Unchanged values are refreshed every few seconds so an RTSS restart can
// recover without continuously rewriting the same OSD text.
func (p *Publisher) Publish(rpm uint16) bool {
	if p == nil {
		return false
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.enabled {
		return false
	}
	now := p.now()
	if !p.nextAttempt.IsZero() && now.Before(p.nextAttempt) {
		return false
	}
	if !p.positionDirty && p.hasLastRPM && rpm == p.lastRPM && now.Sub(p.lastSuccess) < unchangedRefreshInterval {
		p.nextAttempt = now.Add(p.interval)
		return false
	}

	if !p.sink.Update(rpm) {
		p.hasLastRPM = false
		p.nextAttempt = now.Add(retryInterval)
		return false
	}

	p.lastRPM = rpm
	p.hasLastRPM = true
	p.positionDirty = false
	p.lastSuccess = now
	p.nextAttempt = now.Add(p.interval)
	return true
}

// Close releases the OSD slot while keeping the current configuration. A
// later device reconnect can publish again without reconfiguring the object.
func (p *Publisher) Close() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked()
}

func (p *Publisher) closeLocked() {
	p.sink.Close()
	p.nextAttempt = time.Time{}
	p.lastSuccess = time.Time{}
	p.hasLastRPM = false
}
