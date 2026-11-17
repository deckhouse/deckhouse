/*
Copyright 2026 Flant JSC

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

package verity

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/containerd/go-dmverity/pkg/dm"
	"golang.org/x/sys/unix"

	"github.com/deckhouse/deckhouse/pkg/log"
)

// verityUUIDPrefix starts the uuid veritysetup sets on the devices it creates
const verityUUIDPrefix = "CRYPT-VERITY-"

// removeIncompleteMapper removes the device mapper that an interrupted veritysetup open left
// without a live table, as 'dmsetup remove' does. It reports false and keeps any other device.
func removeIncompleteMapper(name string) (bool, error) {
	control, err := dm.Open()
	if err != nil {
		return false, fmt.Errorf("open device mapper control: %w", err)
	}
	defer control.Close()

	status, err := control.DeviceStatus(name)
	if errors.Is(err, unix.ENXIO) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if !isIncompleteMapper(status) {
		return false, nil
	}

	if err = control.RemoveDevice(name); err != nil {
		return false, err
	}

	log.Warn("removed device mapper left without a live table by an interrupted veritysetup open", slog.String("name", name))

	return true, nil
}

// isIncompleteMapper reports whether the device is left by an interrupted veritysetup open.
// A device with a live table is complete, a device in use can't be removed, and a device without
// the veritysetup uuid isn't created by Deckhouse.
func isIncompleteMapper(status dm.DeviceStatus) bool {
	return !status.ActivePresent && status.OpenCount == 0 && strings.HasPrefix(status.UUID, verityUUIDPrefix)
}
