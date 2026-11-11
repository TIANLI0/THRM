package device

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
)

const hidLinkAliveWindow = 3 * time.Second

// Keep a small bounded flight recorder even when GUI debug capture is
// off. Periodic EF reports update liveness without evicting control commands.
// This lock is never held while acquiring Manager.mutex.
type hidLinkHealth struct {
	mu         sync.Mutex
	generation uint64
	lastRX     time.Time
	lastStatus time.Time
	status     hidLinkEvent
	rxCount    uint64
	txCount    uint64
	next       uint64
	recent     [32]hidLinkEvent
}

type hidLinkEvent struct {
	at    time.Time
	tx    bool
	valid bool
	n     int
	raw   [32]byte
}

func (h *hidLinkHealth) reset(generation uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.generation = generation
	h.lastRX, h.lastStatus = time.Time{}, time.Time{}
	h.rxCount, h.txCount, h.next = 0, 0, 0
	h.recent = [32]hidLinkEvent{}
	h.status = hidLinkEvent{}
}

func (h *hidLinkHealth) record(generation uint64, tx bool, raw []byte, at time.Time) {
	frame, ok := deviceproto.ParseFrame(raw)
	valid := ok && frame.ChecksumOK
	h.mu.Lock()
	defer h.mu.Unlock()
	if generation != h.generation {
		return
	}
	if tx {
		h.txCount++
	} else {
		h.rxCount++
		if valid {
			h.lastRX = at
			if frame.Command == deviceproto.CmdStatusNotify {
				h.lastStatus = at
				h.status = hidLinkEvent{at: at, valid: true}
				h.status.n = copy(h.status.raw[:], raw)
				return
			}
		}
	}
	event := hidLinkEvent{at: at, tx: tx, valid: valid}
	event.n = copy(event.raw[:], raw)
	h.recent[h.next%uint64(len(h.recent))] = event
	h.next++
}

func (h *hidLinkHealth) alive(now time.Time) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return !h.lastRX.IsZero() && now.Sub(h.lastRX) <= hidLinkAliveWindow
}

func (h *hidLinkHealth) snapshot(now time.Time) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	age := func(at time.Time) int64 {
		if at.IsZero() {
			return -1
		}
		return now.Sub(at).Milliseconds()
	}
	type event struct {
		At        string `json:"at"`
		Direction string `json:"direction"`
		Valid     bool   `json:"valid"`
		Raw       string `json:"raw"`
	}
	result := struct {
		Generation uint64  `json:"generation"`
		RXAge      int64   `json:"rxAgeMs"`
		StatusAge  int64   `json:"statusAgeMs"`
		RXCount    uint64  `json:"rxCount"`
		TXCount    uint64  `json:"txCount"`
		Recent     []event `json:"recent"`
		LastStatus string  `json:"lastStatus"`
	}{Generation: h.generation, RXAge: age(h.lastRX), StatusAge: age(h.lastStatus), RXCount: h.rxCount, TXCount: h.txCount,
		LastStatus: fmt.Sprintf("% X", h.status.raw[:h.status.n])}
	start := uint64(0)
	if h.next > uint64(len(h.recent)) {
		start = h.next - uint64(len(h.recent))
	}
	for i := start; i < h.next; i++ {
		e := h.recent[i%uint64(len(h.recent))]
		direction := "rx"
		if e.tx {
			direction = "tx"
		}
		result.Recent = append(result.Recent, event{e.at.Format("15:04:05.000"), direction, e.valid, fmt.Sprintf("% X", e.raw[:e.n])})
	}
	data, _ := json.Marshal(result)
	return string(data)
}

// HasRecentHIDResponse is deliberately about validated, live traffic, not the
// cached fan status, which may survive indefinitely after a physical link loss.
func (m *Manager) HasRecentHIDResponse() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.isConnected && m.device != nil && m.linkHealth.alive(time.Now())
}

func (m *Manager) logLinkHealth(reason string) {
	m.logWarn("HID 通信诊断（%s）: %s", reason, m.linkHealth.snapshot(time.Now()))
}
