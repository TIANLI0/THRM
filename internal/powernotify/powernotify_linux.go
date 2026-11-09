//go:build linux

package powernotify

import (
	"fmt"
	"sync"

	"github.com/godbus/dbus/v5"
)

type notifier struct {
	conn      *dbus.Conn
	onSuspend func()
	onResume  func(userPresent bool)
	stopCh    chan struct{}
	stopOnce  sync.Once
}

// logind 的 PrepareForSleep(false) 不区分是否有人值守，一律按用户在场处理。
func RegisterSuspendResumeNotifications(onSuspend func(), onResume func(userPresent bool)) (func(), error) {
	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to D-Bus system bus: %w", err)
	}

	err = conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.login1.Manager"),
		dbus.WithMatchMember("PrepareForSleep"),
		dbus.WithMatchObjectPath("/org/freedesktop/login1"),
	)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to add D-Bus signal match: %w", err)
	}

	pn := &notifier{
		conn:      conn,
		onSuspend: onSuspend,
		onResume:  onResume,
		stopCh:    make(chan struct{}),
	}

	go pn.listen()

	return pn.stop, nil
}

func (pn *notifier) listen() {
	ch := make(chan *dbus.Signal, 10)
	pn.conn.Signal(ch)

	for {
		select {
		case sig, ok := <-ch:
			if !ok {
				return
			}
			if sig != nil && len(sig.Body) >= 1 {
				if starting, ok := sig.Body[0].(bool); ok {
					if starting {
						if pn.onSuspend != nil {
							pn.onSuspend()
						}
					} else {
						if pn.onResume != nil {
							pn.onResume(true)
						}
					}
				}
			}
		case <-pn.stopCh:
			return
		}
	}
}

func (pn *notifier) stop() {
	pn.stopOnce.Do(func() {
		close(pn.stopCh)
		pn.conn.Close()
	})
}
