// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package docs

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNewChannelMappingEditor(t *testing.T) {
	editor := newChannelMappingEditor("/test/path")
	if editor == nil {
		t.Fatal("expected editor to be non-nil")
	}
	if editor.baseDir != "/test/path" {
		t.Errorf("expected baseDir to be %q, got %q", "/test/path", editor.baseDir)
	}
}

func TestChannelMappingEditor_Edit(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "channel-mapping-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create modules directory
	modulesPath := filepath.Join(tempDir, modulesDir)
	if err := os.MkdirAll(modulesPath, 0755); err != nil {
		t.Fatalf("failed to create modules directory: %v", err)
	}

	editor := newChannelMappingEditor(tempDir)

	t.Run("Edit empty file", func(t *testing.T) {
		err = editor.edit(func(cm channelMapping) {
			cm["console"] = map[string]map[string]versionEntity{
				channelMappingChannels: {
					"stable": {Version: "1.0.0"},
					"alpha":  {Version: "1.1.0"},
				},
			}
		})
		if err != nil {
			t.Fatalf("edit failed: %v", err)
		}

		// Verify the file was created with correct content
		modules, err := editor.get()
		if err != nil {
			t.Fatalf("get failed after edit: %v", err)
		}

		if len(modules) != 1 {
			t.Fatalf("expected 1 module, got %d", len(modules))
		}

		if modules[0].ModuleName != "console" {
			t.Errorf("expected module name to be %q, got %q", "console", modules[0].ModuleName)
		}

		expectedChannels := []Channel{
			{Code: "alpha", Version: "1.1.0"},
			{Code: "stable", Version: "1.0.0"},
		}
		if !reflect.DeepEqual(modules[0].Channels, expectedChannels) {
			t.Errorf("channels do not match expected: got %v, want %v", modules[0].Channels, expectedChannels)
		}
	})

	t.Run("Update existing data", func(t *testing.T) {
		err = editor.edit(func(cm channelMapping) {
			cm["console"][channelMappingChannels]["stable"] = versionEntity{Version: "1.2.0"}
			cm["parca"] = map[string]map[string]versionEntity{
				channelMappingChannels: {
					"beta": {Version: "0.1.0"},
				},
			}
		})
		if err != nil {
			t.Fatalf("edit failed: %v", err)
		}

		// Verify updates
		modules, err := editor.get()
		if err != nil {
			t.Fatalf("get failed after edit: %v", err)
		}

		if len(modules) != 2 {
			t.Fatalf("expected 2 modules, got %d", len(modules))
		}

		// Check console was updated
		for _, module := range modules {
			if module.ModuleName == "console" {
				for _, ch := range module.Channels {
					if ch.Code == "stable" && ch.Version != "1.2.0" {
						t.Errorf("expected version 1.2.0 for stable channel, got %s", ch.Version)
					}
				}
			}
		}
	})
}

func TestChannelMappingEditor_Get_EmptyFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "channel-mapping-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create modules directory
	modulesPath := filepath.Join(tempDir, modulesDir)
	if err := os.MkdirAll(modulesPath, 0755); err != nil {
		t.Fatalf("failed to create modules directory: %v", err)
	}

	// Create empty channels.yaml file
	channelsPath := filepath.Join(modulesPath, "channels.yaml")
	if err := os.WriteFile(channelsPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty channels file: %v", err)
	}

	editor := newChannelMappingEditor(tempDir)
	modules, err := editor.get()
	if err != nil {
		t.Fatalf("get failed on empty file: %v", err)
	}

	if len(modules) != 0 {
		t.Errorf("expected 0 modules from empty file, got %d", len(modules))
	}
}

func TestChannelMappingEditor_Get_NonExistentFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "channel-mapping-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create modules directory
	modulesPath := filepath.Join(tempDir, modulesDir)
	if err := os.MkdirAll(modulesPath, 0755); err != nil {
		t.Fatalf("failed to create modules directory: %v", err)
	}

	editor := newChannelMappingEditor(tempDir)
	_, err = editor.get()
	if err == nil {
		t.Error("expected error when file doesn't exist, got nil")
	}
}
