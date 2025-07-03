/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tools

import (
	"fmt"
	"os"

	"d8_shutdown_inhibitor/pkg/inputdev"
)

func ListInputDevices() {
	devs, err := inputdev.ListInputDevicesWithAnyButton(inputdev.KEY_POWER, inputdev.KEY_POWER2)
	if err != nil {
		fmt.Printf("list power key devices: %w", err)
		os.Exit(1)
	}

	for _, dev := range devs {
		fmt.Printf("Device: %s, %s\n", dev.Name, dev.DevPath)
	}
}
