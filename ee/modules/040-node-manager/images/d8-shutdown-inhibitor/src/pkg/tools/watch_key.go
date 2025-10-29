/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package tools

import (
	"os"

	"log/slog"

	"d8_shutdown_inhibitor/pkg/inputdev"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

func WatchForKey(buttons ...inputdev.Button) {
	devs, err := inputdev.ListInputDevicesWithAnyButton(buttons...)
	if err != nil {
		dlog.Fatal("watch key: failed to list devices", dlog.Err(err))
	}

	for _, dev := range devs {
		dlog.Info("watch key: device found", slog.String("name", dev.Name), slog.String("path", dev.DevPath))
	}

	watcher := inputdev.NewWatcher(devs, buttons...)
	watcher.Start()
	dlog.Info("watch key: waiting for button press")
	<-watcher.Pressed()
	dlog.Info("watch key: button pressed")
	os.Exit(0)
}
