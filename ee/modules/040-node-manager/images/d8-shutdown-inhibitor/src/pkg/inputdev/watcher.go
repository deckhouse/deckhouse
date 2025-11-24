/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inputdev

import (
	"errors"
	"fmt"
	"log/slog"
	"syscall"
	"unsafe"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

type Watcher struct {
	devs      []Device
	buttons   []Button
	stopCh    chan struct{}
	pressedCh chan struct{}
}

var errDevicesNeedRefresh = errors.New("input devices need refresh")

func NewWatcher(devs []Device, buttons ...Button) *Watcher {
	return &Watcher{
		devs:      devs,
		buttons:   buttons,
		stopCh:    make(chan struct{}),
		pressedCh: make(chan struct{}),
	}
}

func (w *Watcher) Start() {
	go w.watch()
}

func (w *Watcher) Pressed() <-chan struct{} {
	return w.pressedCh
}

func (w *Watcher) Stop() {
	close(w.stopCh)
}

func (w *Watcher) watch() {
	evCh := make(chan *inputEvent)
	go w.readEvents(evCh)
	for {
		select {
		case event := <-evCh:
			if event.isAnyButtonPressed(w.buttons) {
				w.pressedCh <- struct{}{}
			}
		case <-w.stopCh:
			return
		}
	}
}

func (w *Watcher) readEvents(evCh chan *inputEvent) {
	var fds []int

refreshDevices:
	fds = fds[:0]

	// Open each device.
	for _, dev := range w.devs {
		dlog.Info("power button watcher device:", slog.Any("DevPath", dev.DevPath))
		fd, err := syscall.Open(dev.DevPath, syscall.O_RDONLY, 0)
		if err != nil {
			continue
		}
		fds = append(fds, fd)
	}

	if len(fds) == 0 {
		dlog.Warn("power button watcher: no file descriptors to watch")
		return
	}

	// Create FdSet bitmask
	fdSet := syscall.FdSet{}
	// Max fd to check for Select.
	fdMax := fds[len(fds)-1] + 1

	// Read events until stopped via channel.
	for {
		// Return if watcher was stopped.
		select {
		case <-w.stopCh:
			closeFDs(fds)
			return
		default:
		}

		InitFdSet(&fdSet, fds...)
		// Wait when read is available on any of the fds.
		// TODO add timeout to check stopCh more frequently?
		_, err := syscall.Select(fdMax, &fdSet, nil, nil, nil)
		if err != nil {
			dlog.Warn("power button watcher: select failed", dlog.Err(err))
			continue
		}

		// Check if fd is set and read event.
		for _, fd := range fds {
			if !FD_ISSET(fd, &fdSet) {
				continue
			}

			event, err := w.readEvent(fd)
			if err != nil {
				if errors.Is(err, errDevicesNeedRefresh) {
					closeFDs(fds)
					goto refreshDevices
				}
				dlog.Warn("power button watcher: read event failed", slog.Int("fd", fd), dlog.Err(err))
				continue
			}

			evCh <- event
		}
	}
}


func closeFDs(fds []int) {
	for _, fd := range fds {
		_ = syscall.Close(fd)
	}
}

func (w *Watcher) readEvent(fd int) (*inputEvent, error) {
	var event inputEvent
	err := w.binaryRead(fd, unsafe.Pointer(&event), unsafe.Sizeof(event))
	if err != nil {
		if errors.Is(err, syscall.ENODEV) {
			dlog.Error("power button watcher: device disappeared", slog.Int("fd", fd))
			w.refreshDevsOnError()
			return nil, fmt.Errorf("%w: %v", errDevicesNeedRefresh, err)
		}

		return nil, fmt.Errorf("read event error: %v\n", err)
	}

	return &event, nil
}

func (w *Watcher) refreshDevsOnError() {
	dlog.Info("power button watcher: refresh devs list")
	var err error

	w.devs, err = ListInputDevicesWithAnyButton(KEY_POWER, KEY_POWER2)
	if err != nil {
		dlog.Error("power button watcher: refresh devs list", dlog.Err(err))
	}
}

type inputEvent struct {
	Time  [16]byte
	Type  uint16
	Code  uint16
	Value int32
}

func (ev *inputEvent) isAnyButtonPressed(buttons []Button) bool {
	if ev.Type == uint16(EV_KEY) && ev.Value == 1 {
		for _, button := range buttons {
			if ev.Code == uint16(button) {
				return true
			}
		}
	}

	return false
}

func (w *Watcher) binaryRead(fd int, data unsafe.Pointer, size uintptr) error {
	buf := make([]byte, size)
	n, err := syscall.Read(fd, buf)
	if err != nil {
		return err
	}
	if n != int(size) {
		return fmt.Errorf("got %d bytes, expected %d", n, size)
	}
	copy(unsafe.Slice((*byte)(data), size), buf)
	return nil
}
