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
	testVersions := []internal.VersionData{
		{
			Registry:       "TestReg",
			ModuleName:     "TestModule",
			ReleaseChannel: "alpha",
			Version:        "1.0.0",
			TarFile:        []byte("test"),
			Checksum:       "checksum",
		},
		{
			Registry:       "TestReg",
			ModuleName:     "TestModule",
			ReleaseChannel: "beta",
			Version:        "1.0.0",
			TarFile:        []byte("test"),
			Checksum:       "checksum",
		},
	}

	t.Run("EmptyCache", func(t *testing.T) {
		cache := New()
		state := cache.GetState()
		assert.Empty(t, state, "GetState should return empty state")
	})

	t.Run("SyncWithRegistryVersions", func(t *testing.T) {
		t.Run("AddNewVersion", func(t *testing.T) {
			cache := New()

			tasks := cache.SyncWithRegistryVersions(testVersions)

			expectedTasks := []backends.DocumentationTask{
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "1.0.0",
					ReleaseChannels: []string{"alpha", "beta"},
					TarFile:         []byte("test"),
					Task:            backends.TaskCreate,
				},
			}

			assert.Equal(t, expectedTasks, tasks, "Tasks returned from SyncWithRegistryVersions should match expected")
			state := cache.GetState()
			assert.Equal(t, expectedTasks, state, "Tasks from GetState should match expected")
		})

		t.Run("AddAdditionalVersion", func(t *testing.T) {
			cache := New()

			initialTasks := cache.SyncWithRegistryVersions(testVersions)
			assert.NotEmpty(t, initialTasks, "Initial sync should return tasks")

			newVersion := internal.VersionData{
				Registry:       "TestReg",
				ModuleName:     "TestModule",
				ReleaseChannel: "stable",
				Version:        "2.0.0",
				TarFile:        []byte("new version"),
				Checksum:       "newchecksum",
			}

			tasks := cache.SyncWithRegistryVersions(append(testVersions, newVersion))

			expectedTasks := []backends.DocumentationTask{
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "2.0.0",
					ReleaseChannels: []string{"stable"},
					TarFile:         []byte("new version"),
					Task:            backends.TaskCreate,
				},
			}

			assert.Equal(t, expectedTasks, tasks, "Only new version should be returned as task")

			expectedCachedTasks := []backends.DocumentationTask{
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "2.0.0",
					ReleaseChannels: []string{"stable"},
					TarFile:         []byte("new version"),
					Task:            backends.TaskCreate,
				},
			}
			state := cache.GetState()
			assert.Equal(t, expectedCachedTasks, state, "All versions should be in state")
		})

		t.Run("UpdateExistingVersion", func(t *testing.T) {
			cache := New()

			initialTasks := cache.SyncWithRegistryVersions(testVersions)
			assert.NotEmpty(t, initialTasks, "Initial sync should return tasks")

			// Consume the initial tasks state
			cache.GetState()

			updatedVersions := []internal.VersionData{
				{
					Registry:       "TestReg",
					ModuleName:     "TestModule",
					ReleaseChannel: "alpha",
					Version:        "1.0.0",
					TarFile:        []byte("updated content"),
					Checksum:       "newchecksum",
				},
				{
					Registry:       "TestReg",
					ModuleName:     "TestModule",
					ReleaseChannel: "beta",
					Version:        "1.0.0",
					TarFile:        []byte("updated content"),
					Checksum:       "newchecksum",
				},
			}

			tasks := cache.SyncWithRegistryVersions(updatedVersions)

			expectedTasks := []backends.DocumentationTask{
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "1.0.0",
					ReleaseChannels: []string{"beta"},
					TarFile:         []byte("updated content"),
					Task:            backends.TaskCreate,
				},
			}

			assert.Equal(t, expectedTasks, tasks, "Should generate update task")
			state := cache.GetState()
			assert.Equal(t, expectedTasks, state, "Should have update task in state")
		})

		t.Run("NoChangeNoTask", func(t *testing.T) {
			cache := New()

			initialTasks := cache.SyncWithRegistryVersions(testVersions)
			assert.NotEmpty(t, initialTasks, "Initial sync should return tasks")

			// Consume the initial tasks state
			cache.GetState()

			tasks := cache.SyncWithRegistryVersions(testVersions)

			assert.Empty(t, tasks, "No tasks should be generated when nothing changes")
			state := cache.GetState()
			assert.Empty(t, state, "No tasks should be in state when nothing changes")
		})

		t.Run("AddNewReleaseChannel", func(t *testing.T) {
			cache := New()

			initialTasks := cache.SyncWithRegistryVersions(testVersions)
			assert.NotEmpty(t, initialTasks, "Initial sync should return tasks")

			// Consume the initial tasks state
			cache.GetState()

			// Same version but new release channel
			newChannelVersions := []internal.VersionData{
				{
					Registry:       "TestReg",
					ModuleName:     "TestModule",
					ReleaseChannel: "alpha",
					Version:        "1.0.0",
					TarFile:        []byte("test"),
					Checksum:       "checksum",
				},
				{
					Registry:       "TestReg",
					ModuleName:     "TestModule",
					ReleaseChannel: "beta",
					Version:        "1.0.0",
					TarFile:        []byte("test"),
					Checksum:       "checksum",
				},
				{
					Registry:       "TestReg",
					ModuleName:     "TestModule",
					ReleaseChannel: "stable",
					Version:        "1.0.0", // Same version
					TarFile:        []byte("test"),
					Checksum:       "checksum",
				},
			}

			tasks := cache.SyncWithRegistryVersions(append(testVersions, newChannelVersions...))

			expectedTasks := []backends.DocumentationTask{
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "1.0.0",
					ReleaseChannels: []string{"alpha", "beta", "stable"},
					TarFile:         []byte("test"),
					Task:            backends.TaskCreate,
				},
			}

			assert.Equal(t, expectedTasks, tasks, "Should return task with new release channel")
			state := cache.GetState()
			assert.Equal(t, expectedTasks, state, "Should have task with new release channel in state")
		})

		t.Run("RemoveVersion", func(t *testing.T) {
			cache := New()

			initialTasks := cache.SyncWithRegistryVersions(testVersions)
			assert.NotEmpty(t, initialTasks, "Initial sync should return tasks")

			// Consume the initial tasks state
			cache.GetState()

			tasks := cache.SyncWithRegistryVersions([]internal.VersionData{})

			expectedTasks := []backends.DocumentationTask{
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "1.0.0",
					ReleaseChannels: []string{"alpha"},
					Task:            backends.TaskDelete,
				},
			}

			assert.Equal(t, expectedTasks, tasks, "Should generate delete task")
			state := cache.GetState()
			assert.Equal(t, expectedTasks, state, "Should have delete task in state")
		})
	})

	t.Run("ReleaseData", func(t *testing.T) {
		cache := New()
		cache.SyncWithRegistryVersions(testVersions)

		t.Run("GetReleaseChecksum", func(t *testing.T) {
			checksum, found := cache.GetReleaseChecksum(&testVersions[0])
			assert.True(t, found, "Checksum should be found")
			assert.Equal(t, "checksum", checksum, "Checksum should match")
		})

		t.Run("GetReleaseVersionData", func(t *testing.T) {
			version, tarFile, found := cache.GetReleaseVersionData(&testVersions[0])
			assert.True(t, found, "Version data should be found")
			assert.Equal(t, "1.0.0", version, "Version should match")
			assert.Equal(t, []byte("test"), tarFile, "TarFile should match")
		})
	})

	t.Run("ModuleOperations", func(t *testing.T) {
		cache := New()
		cache.SyncWithRegistryVersions(testVersions)

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
		cache.SyncWithRegistryVersions(testVersions)

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
