/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package inputdev

import (
	"fmt"
	"os"
	"path"
	"syscall"
)

const DevInputDir = "/dev/input"

type Device struct {
	Name    string
	DevPath string
}

// ListInputDevicesWithAnyButton returns a list of input devices that support any of the specified buttons.
func ListInputDevicesWithAnyButton(buttons ...Button) ([]Device, error) {
	dirEntries, err := os.ReadDir(DevInputDir)
	if err != nil {
		return nil, fmt.Errorf("read input devices directory %s: %w", DevInputDir, err)
	}

	devs := make([]Device, 0)

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() {
			// Ignore directories in input devices directory.
			continue
		}

		devPath := path.Join(DevInputDir, dirEntry.Name())

		// Open device file.
		fd, err := syscall.Open(devPath, syscall.O_RDONLY, 0)
		if err != nil {
			fmt.Printf("Ignore input %s, error: %v\n", dirEntry.Name(), err)
			continue
		}

		devName, err := GetDeviceName(fd)
		if err != nil {
			fmt.Printf("%s: error getting device name: %v\n", devPath, err)
			continue
		}

		hasKeyEvents, err := IsReportingKeyEvents(fd)
		if err != nil {
			fmt.Printf("%s: error getting device event types: %v\n", devPath, err)
			continue
		}

		hasButtons := false
		if hasKeyEvents {
			hasButtons, err = HasAnyButton(fd, buttons...)
			if err != nil {
				fmt.Printf("%s: error getting if power button supported: %v\n", devPath, err)
				continue
			}
		}

		if hasButtons {
			devs = append(devs, Device{
				Name:    devName,
				DevPath: devPath,
			})
		}

		_ = syscall.Close(fd)
	}

	return devs, nil
}
