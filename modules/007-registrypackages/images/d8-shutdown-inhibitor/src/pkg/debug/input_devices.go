package debug

import (
	"fmt"
	"os"

	"graceful_shutdown/pkg/inputdev"
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
