//go:build linux

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

package watchdog

import (
	"fmt"

	"golang.org/x/sys/unix"
)

const sysfsClassWatchdog = "/sys/class/watchdog"

// Nowayout reports whether the kernel refuses to stop this watchdog.
//
// There is no ioctl for it. WDIOF_MAGICCLOSE is a driver capability, while
// nowayout is a build or module setting that makes the kernel ignore Magic Close
// (watchdog_stop returns EBUSY). Reading it through sysfs avoids opening the
// device, so a refusal never leaves an armed watchdog behind.
func Nowayout(devicePath string) (bool, error) {
	var stat unix.Stat_t
	if err := unix.Stat(devicePath, &stat); err != nil {
		return false, fmt.Errorf("stat %q: %w", devicePath, err)
	}

	return nowayoutIn(sysfsClassWatchdog, unix.Major(stat.Rdev), unix.Minor(stat.Rdev))
}
