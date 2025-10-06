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
	"testing"

	"github.com/stretchr/testify/assert"

	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"registry-modules-watcher/internal"
	"registry-modules-watcher/internal/backends"
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
		cache := New(metricsstorage.NewMetricStorage("test"))
		state := cache.GetState()
		assert.Empty(t, state, "GetState should return empty state")
	})

	t.Run("SyncWithRegistryVersions", func(t *testing.T) {
		t.Run("AddNewVersions", func(t *testing.T) {
			cache := New(metricsstorage.NewMetricStorage("test"))

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
			cache := New(metricsstorage.NewMetricStorage("test"))

			initialTasks := cache.SyncWithRegistryVersions(testVersions)
			assert.NotEmpty(t, initialTasks, "Initial sync should return tasks")

			newVersion := internal.VersionData{
				Registry:       "TestReg",
				ModuleName:     "TestModule",
				ReleaseChannel: "beta",
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
					ReleaseChannels: []string{"beta"},
					TarFile:         []byte("new version"),
					Task:            backends.TaskCreate,
				},
			}

			assert.Equal(t, expectedTasks, tasks, "Only new version should be returned as task")

			expectedCachedTasks := []backends.DocumentationTask{
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "1.0.0",
					ReleaseChannels: []string{"alpha"},
					TarFile:         []byte("test"),
				},
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "2.0.0",
					ReleaseChannels: []string{"beta"},
					TarFile:         []byte("new version"),
				},
			}
			state := cache.GetState()
			assert.Equal(t, expectedCachedTasks, state, "All versions should be in state")
		})

		t.Run("UpdateExistingVersion", func(t *testing.T) {
			cache := New(metricsstorage.NewMetricStorage("test"))

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
					ReleaseChannels: []string{"alpha", "beta"},
					TarFile:         []byte("test"),
					Task:            backends.TaskDelete,
				},
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "1.0.0",
					ReleaseChannels: []string{"alpha", "beta"},
					TarFile:         []byte("updated content"),
					Task:            backends.TaskCreate,
				},
			}

			assert.Equal(t, expectedTasks, tasks, "Should generate update task")

			expectedCachedTasks := []backends.DocumentationTask{
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "1.0.0",
					ReleaseChannels: []string{"alpha", "beta"},
					TarFile:         []byte("updated content"),
				},
			}
			state := cache.GetState()
			assert.Equal(t, expectedCachedTasks, state, "Should have update task in state")
		})

		t.Run("NoChangeNoTask", func(t *testing.T) {
			cache := New(metricsstorage.NewMetricStorage("test"))

			initialTasks := cache.SyncWithRegistryVersions(testVersions)
			assert.NotEmpty(t, initialTasks, "Initial sync should return tasks")

			// Consume the initial tasks state
			cache.GetState()

			tasks := cache.SyncWithRegistryVersions(testVersions)

			assert.Empty(t, tasks, "No tasks should be generated when nothing changes")

			expectedCachedTasks := []backends.DocumentationTask{
				{
					Registry:        "TestReg",
					Module:          "TestModule",
					Version:         "1.0.0",
					ReleaseChannels: []string{"alpha", "beta"},
					TarFile:         []byte("test"),
				},
			}
			state := cache.GetState()
			assert.Equal(t, expectedCachedTasks, state, "State should not change")
		})

		t.Run("AddNewReleaseChannel", func(t *testing.T) {
			cache := New(metricsstorage.NewMetricStorage("test"))

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
				},
			}

			assert.Equal(t, expectedTasks, tasks, "Should return task with new release channel")
			state := cache.GetState()
			assert.Equal(t, expectedTasks, state, "Should have task with new release channel in state")
		})

		t.Run("RemoveVersion", func(t *testing.T) {
			cache := New(metricsstorage.NewMetricStorage("test"))

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
					ReleaseChannels: []string{"alpha", "beta"},
					TarFile:         []byte("test"),
					Task:            backends.TaskDelete,
				},
			}

			assert.Equal(t, expectedTasks, tasks, "Should generate delete task")
			state := cache.GetState()
			assert.Empty(t, state, "State should be empty after deletion")
		})
	})

	t.Run("ReleaseData", func(t *testing.T) {
		cache := New(metricsstorage.NewMetricStorage("test"))
		cache.SyncWithRegistryVersions(testVersions)

		t.Run("GetVersionDataByChecksum", func(t *testing.T) {
			// Test finding version data by checksum from a different channel
			newChannelVersion := internal.VersionData{
				Registry:       "TestReg",
				ModuleName:     "TestModule",
				ReleaseChannel: "stable",   // Different channel
				Checksum:       "checksum", // Same checksum as alpha/beta
			}

			version, tarFile := cache.GetGetReleaseVersionData(&newChannelVersion)
			assert.Equal(t, "1.0.0", version, "Version should match")
			assert.Equal(t, []byte("test"), tarFile, "TarFile should match")
		})

		t.Run("GetVersionDataByChecksum_NotFound", func(t *testing.T) {
			// Test with non-existent checksum
			notFoundVersion := internal.VersionData{
				Registry:       "TestReg",
				ModuleName:     "TestModule",
				ReleaseChannel: "stable",
				Checksum:       "nonexistent",
			}

			version, tarFile := cache.GetGetReleaseVersionData(&notFoundVersion)
			assert.Empty(t, version, "Version should be empty")
			assert.Nil(t, tarFile, "TarFile should be nil")
		})

		t.Run("GetVersionDataByChecksum_DifferentModule", func(t *testing.T) {
			// Test with different module - should not find
			differentModuleVersion := internal.VersionData{
				Registry:       "TestReg",
				ModuleName:     "DifferentModule",
				ReleaseChannel: "alpha",
				Checksum:       "checksum",
			}

			version, tarFile := cache.GetGetReleaseVersionData(&differentModuleVersion)
			assert.Empty(t, version, "Version should be empty")
			assert.Nil(t, tarFile, "TarFile should be nil")
		})
	})
}
