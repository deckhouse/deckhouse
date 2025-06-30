/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inputdev

import (
	"fmt"
	"syscall"
	"unsafe"
)

type Watcher struct {
	devs      []Device
	buttons   []Button
	stopCh    chan struct{}
	pressedCh chan struct{}
}

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
	fds := []int{}

	// Open each device.
	for _, dev := range w.devs {
		fd, err := syscall.Open(dev.DevPath, syscall.O_RDONLY, 0)
		if err != nil {
			continue
		}
		defer syscall.Close(fd)
		fds = append(fds, fd)
		//devices[fd] = devicePath
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
			return
		default:
		}

		InitFdSet(&fdSet, fds...)
		// Wait when read is available on any of the fds.
		// TODO add timeout to check stopCh more frequently?
		_, err := syscall.Select(fdMax, &fdSet, nil, nil, nil)
		if err != nil {
			fmt.Printf("select: %v\n", err)
			continue
		}

		// Check if fd is set and read event.
		for _, fd := range fds {
			if !FD_ISSET(fd, &fdSet) {
				continue
			}

			event, err := readEvent(fd)
			if err != nil {
				fmt.Printf("readEvent: %v\n", err)
				continue
			}

			evCh <- event
		}
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

func readEvent(fd int) (*inputEvent, error) {
	var event inputEvent
	err := binaryRead(fd, unsafe.Pointer(&event), unsafe.Sizeof(event))
	if err != nil {
		return nil, fmt.Errorf("read event error: %v\n", err)
	}

	return &event, nil
}

func binaryRead(fd int, data unsafe.Pointer, size uintptr) error {
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
