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
	"os"
	"path/filepath"
	"testing"
)

// The device number of a hypothetical /dev/watchdog1: a cdev number, unlike the
// misc number of the legacy /dev/watchdog alias.
const (
	testMajor = 251
	testMinor = 1
)

func fakeSysfsEntry(t *testing.T, root, name, deviceNumber, nowayoutValue string) {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create sysfs entry: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "dev"), []byte(deviceNumber+"\n"), 0o644); err != nil {
		t.Fatalf("write dev: %v", err)
	}

	if nowayoutValue == "" {
		return
	}

	if err := os.WriteFile(filepath.Join(dir, "nowayout"), []byte(nowayoutValue+"\n"), 0o644); err != nil {
		t.Fatalf("write nowayout: %v", err)
	}
}

func TestNowayoutReadsTheEntryMatchingTheDeviceNumber(t *testing.T) {
	root := t.TempDir()

	// The index in the name means nothing: only the device number identifies the
	// entry, and a decoy with another number must not be read.
	fakeSysfsEntry(t, root, "watchdog7", "251:1", "0")
	fakeSysfsEntry(t, root, "watchdog3", "251:0", "1")

	blocked, err := nowayoutIn(root, testMajor, testMinor)
	if err != nil {
		t.Fatalf("nowayoutIn returned an error: %v", err)
	}

	if blocked {
		t.Error("nowayout is 0 for this device, want false")
	}
}

func TestNowayoutReportsAnEnabledSetting(t *testing.T) {
	root := t.TempDir()
	fakeSysfsEntry(t, root, "watchdog1", "251:1", "1")

	blocked, err := nowayoutIn(root, testMajor, testMinor)
	if err != nil {
		t.Fatalf("nowayoutIn returned an error: %v", err)
	}

	if !blocked {
		t.Error("nowayout is 1 for this device, want true")
	}
}

// /dev/watchdog is the misc alias of the first registered watchdog, so its own
// device number never appears in sysfs and the mapping must be hardcoded.
func TestNowayoutMapsTheLegacyDeviceToWatchdogZero(t *testing.T) {
	root := t.TempDir()
	fakeSysfsEntry(t, root, "watchdog0", "251:0", "1")

	blocked, err := nowayoutIn(root, miscMajor, legacyWatchdogMinor)
	if err != nil {
		t.Fatalf("nowayoutIn returned an error: %v", err)
	}

	if !blocked {
		t.Error("the legacy device must resolve to watchdog0, which has nowayout 1")
	}
}

func TestNowayoutFailsWhenTheLegacyEntryIsMissing(t *testing.T) {
	root := t.TempDir()
	fakeSysfsEntry(t, root, "watchdog1", "251:1", "0")

	if _, err := nowayoutIn(root, miscMajor, legacyWatchdogMinor); err == nil {
		t.Error("a legacy device without a watchdog0 entry must be an error")
	}
}

// Every failure below must be an error, never a false: the agent refuses to arm a
// watchdog whose nowayout setting it could not verify.
func TestNowayoutFailsWhenItCannotBeVerified(t *testing.T) {
	t.Run("unexpected value", func(t *testing.T) {
		root := t.TempDir()
		fakeSysfsEntry(t, root, "watchdog1", "251:1", "maybe")

		if _, err := nowayoutIn(root, testMajor, testMinor); err == nil {
			t.Error("an unparsable nowayout value must be an error")
		}
	})

	t.Run("missing attribute", func(t *testing.T) {
		root := t.TempDir()
		fakeSysfsEntry(t, root, "watchdog1", "251:1", "")

		if _, err := nowayoutIn(root, testMajor, testMinor); err == nil {
			t.Error("a missing nowayout attribute must be an error")
		}
	})

	t.Run("no matching entry", func(t *testing.T) {
		root := t.TempDir()
		fakeSysfsEntry(t, root, "watchdog0", "251:0", "0")

		if _, err := nowayoutIn(root, testMajor, testMinor); err == nil {
			t.Error("a device without a sysfs entry must be an error")
		}
	})

	t.Run("missing sysfs root", func(t *testing.T) {
		if _, err := nowayoutIn(filepath.Join(t.TempDir(), "absent"), testMajor, testMinor); err == nil {
			t.Error("an unreadable sysfs root must be an error")
		}
	})
}
