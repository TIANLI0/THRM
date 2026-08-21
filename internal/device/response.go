package device

import (
	"fmt"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/deviceproto"
)

const deviceResponseTimeout = 900 * time.Millisecond

type responseWaiter struct {
	id      uint64
	command byte
	result  chan deviceproto.Frame
}

// responseBroker routes monitor/notification frames to the command that is
// waiting for that exact response. It replaces timing-based sleeps, which lose
// responses whenever the HID polling interval is longer than the sleep.
type responseBroker struct {
	mu      sync.Mutex
	nextID  uint64
	waiters map[byte][]*responseWaiter
}

func newResponseBroker() *responseBroker {
	return &responseBroker{waiters: make(map[byte][]*responseWaiter)}
}

func (b *responseBroker) register(command byte) *responseWaiter {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.nextID++
	waiter := &responseWaiter{id: b.nextID, command: command, result: make(chan deviceproto.Frame, 1)}
	b.waiters[command] = append(b.waiters[command], waiter)
	return waiter
}

func (b *responseBroker) cancel(waiter *responseWaiter) {
	if b == nil || waiter == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	queue := b.waiters[waiter.command]
	for i, candidate := range queue {
		if candidate.id == waiter.id {
			queue = append(queue[:i], queue[i+1:]...)
			break
		}
	}
	if len(queue) == 0 {
		delete(b.waiters, waiter.command)
	} else {
		b.waiters[waiter.command] = queue
	}
}

func (b *responseBroker) deliver(frame deviceproto.Frame) bool {
	if b == nil || !frame.ChecksumOK {
		return false
	}
	b.mu.Lock()
	queue := b.waiters[frame.Command]
	if len(queue) == 0 {
		b.mu.Unlock()
		return false
	}
	waiter := queue[0]
	if len(queue) == 1 {
		delete(b.waiters, frame.Command)
	} else {
		b.waiters[frame.Command] = queue[1:]
	}
	b.mu.Unlock()
	waiter.result <- frame
	return true
}

func waitForResponse(broker *responseBroker, waiter *responseWaiter, timeout time.Duration) (deviceproto.Frame, error) {
	if timeout <= 0 {
		timeout = deviceResponseTimeout
	}
	defer broker.cancel(waiter)
	select {
	case frame := <-waiter.result:
		return frame, nil
	case <-time.After(timeout):
		return deviceproto.Frame{}, fmt.Errorf("等待命令 0x%02X 响应超时", waiter.command)
	}
}

func (m *Manager) sendHIDCommandAndWaitLocked(command byte, payload []byte, reportLen int, timeout time.Duration) (deviceproto.Frame, error) {
	if m.responses == nil {
		m.responses = newResponseBroker()
	}
	waiter := m.responses.register(command)
	if err := m.writeHIDFrameLocked(command, payload, reportLen); err != nil {
		m.responses.cancel(waiter)
		return deviceproto.Frame{}, err
	}
	return waitForResponse(m.responses, waiter, timeout)
}

func validateACK(frame deviceproto.Frame, accepted ...byte) error {
	if len(frame.Payload) < 1 {
		return fmt.Errorf("命令 0x%02X 返回了空 ACK", frame.Command)
	}
	status := frame.Payload[0]
	for _, allowed := range accepted {
		if status == allowed {
			return nil
		}
	}
	return fmt.Errorf("命令 0x%02X 被固件拒绝: 状态 %d (%s)", frame.Command, status, deviceproto.AckStatusName(frame.Command, status))
}

func (m *Manager) sendHIDAckLocked(command byte, payload []byte, accepted ...byte) error {
	frame, err := m.sendHIDCommandAndWaitLocked(command, payload, hidControlReportLen, deviceResponseTimeout)
	if err != nil {
		return err
	}
	return validateACK(frame, accepted...)
}

func (b *BLEManager) sendBLECommandAndWait(command byte, payload []byte, timeout time.Duration) (deviceproto.Frame, error) {
	if b.responses == nil {
		b.responses = newResponseBroker()
	}
	waiter := b.responses.register(command)
	if err := b.WriteCommand(deviceproto.BuildFrame(command, payload...)); err != nil {
		b.responses.cancel(waiter)
		return deviceproto.Frame{}, err
	}
	return waitForResponse(b.responses, waiter, timeout)
}
