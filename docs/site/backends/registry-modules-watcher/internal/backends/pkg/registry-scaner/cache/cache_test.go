// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"registry-modules-watcher/internal"
	"registry-modules-watcher/internal/backends"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCache(t *testing.T) {
	// Test data
	testVersion := internal.VersionData{
		Registry:       "TestReg",
		ModuleName:     "TestModule",
		ReleaseChannel: "alpha",
		Version:        "1.0.0",
		TarFile:        []byte("test"),
		Checksum:       "checksum",
	}

	t.Run("EmptyCache", func(t *testing.T) {
		cache := New()
		state := cache.GetState()
		assert.Empty(t, state, "GetState should return empty state")
	})

	t.Run("SyncWithRegistryVersions", func(t *testing.T) {
		cache := New()
		cache.SyncWithRegistryVersions([]internal.VersionData{testVersion})

		expectedState := []backends.DocumentationTask{
			{
				Registry:        "TestReg",
				Module:          "TestModule",
				Version:         "1.0.0",
				ReleaseChannels: []string{"alpha"},
				TarFile:         []byte("test"),
				Task:            backends.TaskCreate,
			},
		}
		state := cache.GetState()
		assert.Equal(t, expectedState, state, "State should match expected after sync")
	})

	t.Run("ReleaseData", func(t *testing.T) {
		cache := New()
		cache.SyncWithRegistryVersions([]internal.VersionData{testVersion})

		t.Run("GetReleaseChecksum", func(t *testing.T) {
			checksum, found := cache.GetReleaseChecksum(&testVersion)
			assert.True(t, found, "Checksum should be found")
			assert.Equal(t, "checksum", checksum, "Checksum should match")
		})

		t.Run("GetReleaseVersionData", func(t *testing.T) {
			version, tarFile, found := cache.GetReleaseVersionData(&testVersion)
			assert.True(t, found, "Version data should be found")
			assert.Equal(t, "1.0.0", version, "Version should match")
			assert.Equal(t, []byte("test"), tarFile, "TarFile should match")
		})
	})

	t.Run("ModuleOperations", func(t *testing.T) {
		cache := New()
		cache.SyncWithRegistryVersions([]internal.VersionData{testVersion})

		t.Run("GetModules", func(t *testing.T) {
			modules := cache.GetModules("TestReg")
			assert.Equal(t, []string{"TestModule"}, modules, "Modules should match")
		})

		t.Run("DeleteModule", func(t *testing.T) {
			cache.DeleteModule("TestReg", "TestModule")
			modules := cache.GetModules("TestReg")
			assert.Empty(t, modules, "Modules should be empty after deletion")
		})
	})

	t.Run("ReleaseChannelOperations", func(t *testing.T) {
		cache := New()
		cache.SyncWithRegistryVersions([]internal.VersionData{testVersion})

		t.Run("GetReleaseChannels", func(t *testing.T) {
			releaseChannels := cache.GetReleaseChannels("TestReg", "TestModule")
			assert.Equal(t, []string{"alpha"}, releaseChannels, "ReleaseChannels should match")
		})

		t.Run("DeleteReleaseChannel", func(t *testing.T) {
			cache.DeleteReleaseChannel("TestReg", "TestModule", "alpha")
			releaseChannels := cache.GetReleaseChannels("TestReg", "TestModule")
			assert.Empty(t, releaseChannels, "ReleaseChannels should be empty after deletion")
		})
	})
}
