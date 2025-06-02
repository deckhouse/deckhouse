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

func WatchForKey(buttons ...inputdev.Button) {
	devs, err := inputdev.ListInputDevicesWithAnyButton(buttons...)
	if err != nil {
		fmt.Printf("list devices with Q W E Enter: %w", err)
		os.Exit(1)
	}

	for _, dev := range devs {
		fmt.Printf("Device: %s, %s\n", dev.Name, dev.DevPath)
	}

	watcher := inputdev.NewWatcher(devs, buttons...)
	watcher.Start()
	fmt.Printf("watch for button press\n")
	<-watcher.Pressed()
	fmt.Printf("button was pressed\n")
	os.Exit(0)
}
