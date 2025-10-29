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

func ListInputDevices() {
	devs, err := inputdev.ListInputDevicesWithAnyButton(inputdev.KEY_POWER, inputdev.KEY_POWER2)
	if err != nil {
		dlog.Error("list power key devices failed", dlog.Err(err))
		os.Exit(1)
	}

	for _, dev := range devs {
		dlog.Info("input device", slog.String("name", dev.Name), slog.String("path", dev.DevPath))
	}
}
