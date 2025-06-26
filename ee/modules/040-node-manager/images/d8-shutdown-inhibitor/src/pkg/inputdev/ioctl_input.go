/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inputdev

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	KEY_MAX = 0x2ff
)

// EventType is a type for event type constants.
type EventType uint

const (
	EV_SYN EventType = 0
	EV_KEY EventType = 1
)

// Button is a type for bit numbers of keys and buttons.
type Button uint

// Only power keys.
const (
	KEY_POWER  Button = 116 /* SC System Power Down */
	KEY_POWER2 Button = 0x164
	// test
	KEY_Q     Button = 16
	KEY_W     Button = 17
	KEY_E     Button = 18
	KEY_ENTER Button = 28
)

// EVIOCGNAME is a copy from linux/input.h
// It returns encoded command to get device name.
// #define EVIOCGNAME(len) _IOC(_IOC_READ, 'E', 0x06, len)
func EVIOCGNAME(size int) uint {
	return _IOC(_IOC_READ, 'E', 0x06, size)
}

func GetDeviceName(fd int) (string, error) {
	// Get device name.
	nameBuf := make([]byte, 128)
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(EVIOCGNAME(len(nameBuf))),
		uintptr(unsafe.Pointer(&nameBuf[0])))

	if errno != 0 {
		if errors.Is(errno, syscall.ENOTTY) {
			return "", nil
		}
		return "", fmt.Errorf("ioctl (errno=%x): %v\n", uint(errno), error(errno))
	}

	return string(nameBuf), nil
}

// EVIOCGBIT is a copy from linux/input.h
// It returns info for the event type.
// #define EVIOCGBIT(ev,len)  _IOC(_IOC_READ, 'E', 0x20 + ev, len)
func EVIOCGBIT(ev EventType, size int) uint {
	return _IOC(_IOC_READ, 'E', uint(0x20)+uint(ev), size)
}

func GetEventBits(fd int, evType EventType, bits uintptr, bitsLen int) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(fd),
		uintptr(EVIOCGBIT(evType, bitsLen)),
		bits)

	if errno != 0 {
		return fmt.Errorf("ioctl (errno=%x): %w\n", uint(errno), error(errno))
	}

	return nil
}

func GetSupportedEventTypes(fd int) (uint64, error) {
	var eventTypeBits uint64

	err := GetEventBits(fd,
		EV_SYN,
		uintptr(unsafe.Pointer(&eventTypeBits)),
		int(unsafe.Sizeof(eventTypeBits)),
	)

	if err != nil {
		return 0, err
	}

	return eventTypeBits, nil
}

// IsReportingKeyEvents returns if the device support EV_KEY events reporting.
func IsReportingKeyEvents(fd int) (bool, error) {
	eventTypeBits, err := GetSupportedEventTypes(fd)

	if err != nil {
		if errors.Is(err, syscall.ENOTTY) {
			return false, nil
		}
		return false, err
	}

	return eventTypeBits&(1<<EV_KEY) > 0, nil
}

func GetButtonsBits(fd int) ([]byte, error) {
	buttonsBytes := KEY_MAX/8 + 1
	buttonsBits := make([]byte, buttonsBytes)

	err := GetEventBits(fd,
		EV_KEY,
		uintptr(unsafe.Pointer(&buttonsBits[0])),
		buttonsBytes,
	)

	if err != nil {
		if errors.Is(err, syscall.ENOTTY) {
			return nil, nil
		}
		return nil, err
	}

	return buttonsBits, nil
}

// HasAnyButton returns if device supports events from any of the specified buttons.
func HasAnyButton(fd int, buttons ...Button) (bool, error) {
	buttonsBits, err := GetButtonsBits(fd)

	if err != nil {
		return false, err
	}

	for _, button := range buttons {
		btnIdx := button / 8
		btnBitMask := byte(1 << (button % 8))

		if buttonsBits[btnIdx]&btnBitMask != 0 {
			return true, nil
		}
	}

	return false, nil
}
