// Copyright 2023 Flant JSC
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
	"log/slog"
	"slices"
	"sort"
	"sync"

	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"

	"registry-modules-watcher/internal"
	"registry-modules-watcher/internal/backends"
	"registry-modules-watcher/internal/metrics"
)

type (
	registryName       string
	moduleName         string
	versionNum         string
	releaseChannelName string
)

type moduleData struct {
	releaseChecksum map[releaseChannelName]string
	versions        map[versionNum]versionData
}

type versionData struct {
	releaseChannels map[string]struct{}
	tarFile         []byte
}

type Cache struct {
	m   sync.RWMutex
	val map[registryName]map[moduleName]moduleData
	ms  *metricsstorage.MetricStorage
}

func New(ms *metricsstorage.MetricStorage) *Cache {
	c := &Cache{
		val: make(map[registryName]map[moduleName]moduleData),
		ms:  ms,
	}

	// function that will be triggered on metrics handler
	ms.AddCollectorFunc(func(s metricsstorage.Storage) {
		log.Debug(
			"collector func triggered",
			slog.Int("registry_len", len(c.val)),
		)

		for registry, modules := range c.val {
			s.GaugeSet(
				metrics.RegistryScannerCacheLengthMetric,
				float64(len(modules)),
				map[string]string{
					"registry": string(registry),
				},
			)
		}
	})

	return c
}

func (c *Cache) GetState() []backends.DocumentationTask {
	c.m.RLock()
	defer c.m.RUnlock()

	return RemapFromMapToVersions(c.val, backends.TaskCreate)
}

// GetGetReleaseVersionData searches for cached version data by checksum across all release channels
// Returns version, tarFile if found, empty values otherwise
func (c *Cache) GetGetReleaseVersionData(version *internal.VersionData) (string, []byte) {
	c.m.RLock()
	defer c.m.RUnlock()

	r, ok := c.val[registryName(version.Registry)]
	if !ok {
		return "", nil
	}

	m, ok := r[moduleName(version.ModuleName)]
	if !ok {
		return "", nil
	}

	// Search across all release channels for matching checksum
	for channelName, checksum := range m.releaseChecksum {
		if checksum == version.Checksum {
			// Found matching checksum, now find the version that contains this channel
			for ver, verData := range m.versions {
				// Check if this version contains the channel with matching checksum
				if _, hasChannel := verData.releaseChannels[string(channelName)]; hasChannel {
					return string(ver), verData.tarFile
				}
			}
		}
	}

	return "", nil
}

// SyncWithRegistryVersions compares cache with registry versions and returns
// documentation tasks that need to be performed (create new, delete old).
//
// It identifies which versions:
// - Are new and need to be created
// - No longer exist and need to be deleted
// - Remain unchanged
//
// The cache is updated to match registry state after comparison.
func (c *Cache) SyncWithRegistryVersions(registryVersions []internal.VersionData) []backends.DocumentationTask {
	c.m.Lock()
	defer c.m.Unlock()

	// Find versions needing creation (not matching in cache)
	versionsToCreate := make([]internal.VersionData, 0)

	// Make a copy of current cache to track versions for deletion
	cacheCopy := copyCache(c.val)

	// Compare registry versions against cache
	for _, version := range registryVersions {
		reg := registryName(version.Registry)
		mod := moduleName(version.ModuleName)
		ver := versionNum(version.Version)
		relChannel := releaseChannelName(version.ReleaseChannel)

		// Check if version matches in cache
		moduleMap, regExists := cacheCopy[reg]
		if !regExists {
			versionsToCreate = append(versionsToCreate, version)
			continue
		}

		moduleData, modExists := moduleMap[mod]
		if !modExists {
			versionsToCreate = append(versionsToCreate, version)
			continue
		}

		vData, verExists := moduleData.versions[ver]
		if !verExists {
			versionsToCreate = append(versionsToCreate, version)
			continue
		}

		// Check if release channel exists with matching checksum
		_, channelExists := vData.releaseChannels[version.ReleaseChannel]
		checksumMatches := moduleData.releaseChecksum[relChannel] == version.Checksum

		if !channelExists || !checksumMatches {
			versionsToCreate = append(versionsToCreate, version)
			continue
		}

		// Remove from deletion tracking if it matches
		delete(vData.releaseChannels, version.ReleaseChannel)
		delete(moduleData.releaseChecksum, relChannel)

		// Clean up empty maps
		cleanupEmptyMaps(cacheCopy, reg, mod, ver)
	}

	// Get versions to delete from what remains in the copy
	versionsToDelete := RemapFromMapToVersions(cacheCopy, backends.TaskDelete)

	// Get versions to create
	createTasks := RemapFromMapToVersions(RemapFromVersionData(versionsToCreate), backends.TaskCreate)

	// Combine and sort all tasks
	createTasks = append(createTasks, versionsToDelete...)
	sortDocumentationTasks(createTasks)

	// Update cache with registry versions
	c.val = RemapFromVersionData(registryVersions)

	return createTasks
}

// Helper function to clean up empty maps
func cleanupEmptyMaps(cache map[registryName]map[moduleName]moduleData, reg registryName, mod moduleName, ver versionNum) {
	moduleMap := cache[reg]
	moduleData := moduleMap[mod]

	if len(moduleData.versions[ver].releaseChannels) == 0 {
		delete(moduleData.versions, ver)
	}

	if len(moduleData.versions) == 0 {
		delete(moduleMap, mod)
	}

	if len(moduleMap) == 0 {
		delete(cache, reg)
	}
}

// Helper function to deep copy the cache map
func copyCache(original map[registryName]map[moduleName]moduleData) map[registryName]map[moduleName]moduleData {
	cacheCopy := make(map[registryName]map[moduleName]moduleData)

	for reg, moduleMap := range original {
		cacheCopy[reg] = make(map[moduleName]moduleData)

		for mod, data := range moduleMap {
			newData := moduleData{
				releaseChecksum: make(map[releaseChannelName]string),
				versions:        make(map[versionNum]versionData),
			}

			for channel, checksum := range data.releaseChecksum {
				newData.releaseChecksum[channel] = checksum
			}

			for ver, vData := range data.versions {
				newVersionData := versionData{
					releaseChannels: make(map[string]struct{}),
					tarFile:         vData.tarFile,
				}

				for channel := range vData.releaseChannels {
					newVersionData.releaseChannels[channel] = struct{}{}
				}

				newData.versions[ver] = newVersionData
			}

			cacheCopy[reg][mod] = newData
		}
	}

	return cacheCopy
}

func RemapFromMapToVersions(m map[registryName]map[moduleName]moduleData, task backends.Task) []backends.DocumentationTask {
	versions := make([]backends.DocumentationTask, 0, 1)
	for registry, modules := range m {
		for module, moduleData := range modules {
			for version, data := range moduleData.versions {
				releaseChannels := make([]string, 0, len(data.releaseChannels))
				for releaseChannel := range data.releaseChannels {
					releaseChannels = append(releaseChannels, releaseChannel)
				}

				versions = append(versions, backends.DocumentationTask{
					Registry:        string(registry),
					Module:          string(module),
					Version:         string(version),
					ReleaseChannels: releaseChannels,
					TarFile:         data.tarFile,
					Task:            task,
				})
			}
		}
	}

	sortDocumentationTasks(versions)

	return versions
}

// nolint: revive
func RemapFromVersionData(input []internal.VersionData) map[registryName]map[moduleName]moduleData {
	sort.Slice(input, func(i, j int) bool {
		if input[i].Registry != input[j].Registry {
			return input[i].Registry < input[j].Registry
		}

		if input[i].ModuleName != input[j].ModuleName {
			return input[i].ModuleName < input[j].ModuleName
		}

		return input[i].Version < input[j].Version
	})

	result := make(map[registryName]map[moduleName]moduleData)

	for _, ver := range input {
		registry := registryName(ver.Registry)
		module := moduleName(ver.ModuleName)
		version := versionNum(ver.Version)

		// Initialize registry map if it doesn't exist
		if _, exists := result[registry]; !exists {
			result[registry] = make(map[moduleName]moduleData)
		}

		// Initialize module data if it doesn't exist
		if _, exists := result[registry][module]; !exists {
			result[registry][module] = moduleData{
				releaseChecksum: make(map[releaseChannelName]string),
				versions:        make(map[versionNum]versionData),
			}
		}

		// Add or update version data
		moduleData := result[registry][module]

		// Initialize version data if it doesn't exist
		if _, exists := moduleData.versions[version]; !exists {
			moduleData.versions[version] = versionData{
				releaseChannels: make(map[string]struct{}),
				tarFile:         ver.TarFile,
			}
		}

		// remove all module versions containing the same release channel
		for _, existedVer := range moduleData.versions {
			delete(existedVer.releaseChannels, ver.ReleaseChannel)
		}

		// Add release channel to the version
		if ver.ReleaseChannel != "" {
			moduleData.versions[version].releaseChannels[ver.ReleaseChannel] = struct{}{}

			// Update release checksum if provided
			if ver.Checksum != "" {
				moduleData.releaseChecksum[releaseChannelName(ver.ReleaseChannel)] = ver.Checksum
			}
		}

		// Store the updated module data back in the result map
		result[registry][module] = moduleData
	}

	return result
}

func sortDocumentationTasks(input []backends.DocumentationTask) {
	for i := range input {
		slices.Sort(input[i].ReleaseChannels)
	}

	sort.Slice(input, func(i, j int) bool {
		if input[i].Task != input[j].Task {
			return input[i].Task > input[j].Task
		}

		if input[i].Registry != input[j].Registry {
			return input[i].Registry < input[j].Registry
		}

		if input[i].Module != input[j].Module {
			return input[i].Module < input[j].Module
		}

		return input[i].Version < input[j].Version
	})
}
