package debug

import (
	"fmt"
	"os"

	"graceful_shutdown/pkg/inputdev"
)

func WatchForKey(buttons ...inputdev.Button) {
	//buttons := []inputdev.Button{
	//	inputdev.KEY_Q, inputdev.KEY_E, inputdev.KEY_W, inputdev.KEY_ENTER,
	//}
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
