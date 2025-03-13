/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	// Получаем список устройств в /dev/input
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
