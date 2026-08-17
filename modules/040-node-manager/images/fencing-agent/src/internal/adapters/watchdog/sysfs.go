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
	"os"
	"path/filepath"
	"strings"
)

const (
	// miscMajor and legacyWatchdogMinor identify /dev/watchdog: the kernel creates
	// this misc device for the first registered watchdog only, which is watchdog0.
	miscMajor           = 10
	legacyWatchdogMinor = 130
)

// nowayoutIn reads the nowayout attribute of the watchdog identified by its
// device number. Any failure is an error and never a false: the agent must not
// arm a watchdog whose nowayout setting it could not verify, because with
// nowayout enabled Magic Close is silently ignored and maintenance would panic
// the Node.
func nowayoutIn(root string, major, minor uint32) (bool, error) {
	dir, err := sysfsDirIn(root, major, minor)
	if err != nil {
		return false, err
	}

	path := filepath.Join(dir, "nowayout")

	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	switch value := strings.TrimSpace(string(raw)); value {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("unexpected value %q in %s", value, path)
	}
}

// sysfsDirIn maps a watchdog device number onto its /sys/class/watchdog entry.
func sysfsDirIn(root string, major, minor uint32) (string, error) {
	// The legacy alias carries the misc device number instead of the cdev number
	// of the watchdog itself, so it cannot be matched against sysfs.
	if major == miscMajor && minor == legacyWatchdogMinor {
		dir := filepath.Join(root, "watchdog0")
		if _, err := os.Stat(dir); err != nil {
			return "", fmt.Errorf("resolve the legacy watchdog device through %s: %w", dir, err)
		}

		return dir, nil
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", root, err)
	}

	want := fmt.Sprintf("%d:%d", major, minor)

	for _, entry := range entries {
		dir := filepath.Join(root, entry.Name())

		raw, err := os.ReadFile(filepath.Join(dir, "dev"))
		if err != nil {
			continue
		}

		if strings.TrimSpace(string(raw)) == want {
			return dir, nil
		}
	}

	return "", fmt.Errorf("no entry in %s matches device number %s", root, want)
}
