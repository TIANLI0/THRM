package rtss

import (
	"testing"
	"time"
)

type fakeSink struct {
	succeed    bool
	updates    []uint16
	closeCount int
}

func (s *fakeSink) Update(rpm uint16) bool {
	s.updates = append(s.updates, rpm)
	return s.succeed
}

func (s *fakeSink) Close() { s.closeCount++ }

func TestSupportedUpdateInterval(t *testing.T) {
	for _, interval := range []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second} {
		if got := SupportedUpdateInterval(interval); got != interval {
			t.Fatalf("SupportedUpdateInterval(%s) = %s", interval, got)
		}
	}
	if got := SupportedUpdateInterval(10 * time.Millisecond); got != DefaultUpdateInterval {
		t.Fatalf("invalid interval = %s, want %s", got, DefaultUpdateInterval)
	}
}

func TestPublisherDisabledByDefault(t *testing.T) {
	sink := &fakeSink{succeed: true}
	publisher := newPublisher(sink, time.Now)
	if publisher.Publish(1500) {
		t.Fatal("disabled publisher unexpectedly wrote to RTSS")
	}
	if len(sink.updates) != 0 {
		t.Fatalf("sink received %d updates while disabled", len(sink.updates))
	}
}

func TestPublisherThrottlesAndRefreshesUnchangedRPM(t *testing.T) {
	now := time.Unix(100, 0)
	sink := &fakeSink{succeed: true}
	publisher := newPublisher(sink, func() time.Time { return now })
	publisher.Configure(true, 500*time.Millisecond)

	if !publisher.Publish(1500) {
		t.Fatal("first update was not written")
	}
	now = now.Add(100 * time.Millisecond)
	if publisher.Publish(1600) {
		t.Fatal("update bypassed configured interval")
	}
	now = now.Add(400 * time.Millisecond)
	if !publisher.Publish(1600) {
		t.Fatal("changed RPM was not written after interval")
	}
	now = now.Add(500 * time.Millisecond)
	if publisher.Publish(1600) {
		t.Fatal("unchanged RPM was written before refresh interval")
	}
	now = now.Add(4500 * time.Millisecond)
	if !publisher.Publish(1600) {
		t.Fatal("unchanged RPM heartbeat was not written")
	}

	want := []uint16{1500, 1600, 1600}
	if len(sink.updates) != len(want) {
		t.Fatalf("updates = %v, want %v", sink.updates, want)
	}
	for i := range want {
		if sink.updates[i] != want[i] {
			t.Fatalf("updates = %v, want %v", sink.updates, want)
		}
	}
}

func TestPublisherBacksOffWhenRTSSIsUnavailable(t *testing.T) {
	now := time.Unix(100, 0)
	sink := &fakeSink{succeed: false}
	publisher := newPublisher(sink, func() time.Time { return now })
	publisher.Configure(true, 250*time.Millisecond)

	publisher.Publish(1500)
	now = now.Add(time.Second)
	publisher.Publish(1600)
	if len(sink.updates) != 1 {
		t.Fatalf("retried too early: %d attempts", len(sink.updates))
	}
	now = now.Add(time.Second)
	publisher.Publish(1600)
	if len(sink.updates) != 2 {
		t.Fatalf("retry count = %d, want 2", len(sink.updates))
	}
}

func TestPublisherDisableAndCloseReleaseSink(t *testing.T) {
	sink := &fakeSink{succeed: true}
	publisher := newPublisher(sink, time.Now)
	publisher.Configure(true, DefaultUpdateInterval)
	publisher.Publish(1500)
	publisher.Configure(false, DefaultUpdateInterval)
	if sink.closeCount != 1 {
		t.Fatalf("close count after disable = %d, want 1", sink.closeCount)
	}

	publisher.Configure(true, DefaultUpdateInterval)
	publisher.Close()
	if sink.closeCount != 2 {
		t.Fatalf("close count after Close = %d, want 2", sink.closeCount)
	}
}
