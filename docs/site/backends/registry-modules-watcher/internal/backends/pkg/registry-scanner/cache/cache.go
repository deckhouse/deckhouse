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
	"slices"
	"sort"
	"sync"

	"registry-modules-watcher/internal"
	"registry-modules-watcher/internal/backends"
)

type CacheKey struct {
	Registry string
	Module   string
	Channel  string
}

type ReleaseInfo struct {
	Digest  string
	Version string
}

type VersionInfo struct {
	Version  string
	TarFile  []byte
	Channels []string // which channels use this version
}

type Cache struct {
	m        sync.RWMutex
	releases map[CacheKey]ReleaseInfo // channel -> release info
	versions map[string]VersionInfo   // digest -> version + tar
}

func New() *Cache {
	return &Cache{
		releases: make(map[CacheKey]ReleaseInfo),
		versions: make(map[string]VersionInfo),
	}
}

// GetState returns all documentation tasks that need to be performed (create new, delete old)
func (c *Cache) GetState() []backends.DocumentationTask {
	c.m.RLock()
	defer c.m.RUnlock()

	return c.convertToDocumentationTasks(backends.TaskCreate)
}

// GetReleaseChecksum returns release checksum for given version data if it exists in cache
// if it exists in cache, it returns release checksum
// if it does not exist, it returns empty string
func (c *Cache) GetReleaseChecksum(version *internal.VersionData) (string, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	key := CacheKey{
		Registry: version.Registry,
		Module:   version.ModuleName,
		Channel:  version.ReleaseChannel,
	}

	release, ok := c.releases[key]
	if !ok {
		return "", false
	}

	return release.Digest, true
}

// GetReleaseVersionData returns version and tar file for given version data if it exists in cache
// if it exists in cache, it returns version and tar file
// if it does not exist, it returns empty version and nil tar file
// if it exists in cache, but version is not found, it returns empty version and nil tar file
// if it exists in cache, but tar file is not found, it returns empty version and nil tar file
func (c *Cache) GetReleaseVersionData(version *internal.VersionData) (string, []byte, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	key := CacheKey{
		Registry: version.Registry,
		Module:   version.ModuleName,
		Channel:  version.ReleaseChannel,
	}

	release, ok := c.releases[key]
	if !ok {
		return "", nil, false
	}

	versionInfo, ok := c.versions[release.Digest]
	if !ok {
		return "", nil, false
	}

	return versionInfo.Version, versionInfo.TarFile, true
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

	// Create sets for comparison
	registryReleases := make(map[CacheKey]ReleaseInfo)
	registryVersionsMap := make(map[string]VersionInfo)

	// Build registry data structures
	for _, version := range registryVersions {
		key := CacheKey{
			Registry: version.Registry,
			Module:   version.ModuleName,
			Channel:  version.ReleaseChannel,
		}

		registryReleases[key] = ReleaseInfo{
			Digest:  version.Checksum,
			Version: version.Version,
		}

		// Update version info, collecting all channels that point to this digest
		if existing, ok := registryVersionsMap[version.Checksum]; ok {
			// Add channel if not already present
			if !slices.Contains(existing.Channels, version.ReleaseChannel) {
				existing.Channels = append(existing.Channels, version.ReleaseChannel)
				registryVersionsMap[version.Checksum] = existing
			}
		} else {
			registryVersionsMap[version.Checksum] = VersionInfo{
				Version:  version.Version,
				TarFile:  version.TarFile,
				Channels: []string{version.ReleaseChannel},
			}
		}
	}

	// Find tasks to create (new or changed) and delete (changed digests) - group by digest
	createTasksByDigest := make(map[string]*backends.DocumentationTask)
	deleteTasksByDigest := make(map[string]*backends.DocumentationTask)
	
	for key, registryRelease := range registryReleases {
		cacheRelease, exists := c.releases[key]

		// Create task if release doesn't exist or digest changed
		if !exists || cacheRelease.Digest != registryRelease.Digest {
			// If digest changed, we need to delete the old version first
			if exists && cacheRelease.Digest != registryRelease.Digest {
				if versionInfo, ok := c.versions[cacheRelease.Digest]; ok {
					if existingTask, ok := deleteTasksByDigest[cacheRelease.Digest]; ok {
						// Add channel to existing delete task
						existingTask.ReleaseChannels = append(existingTask.ReleaseChannels, key.Channel)
					} else {
						// Create new delete task
						deleteTasksByDigest[cacheRelease.Digest] = &backends.DocumentationTask{
							Registry:        key.Registry,
							Module:          key.Module,
							Version:         versionInfo.Version,
							ReleaseChannels: []string{key.Channel},
							TarFile:         versionInfo.TarFile,
							Task:            backends.TaskDelete,
						}
					}
				}
			}
			
			// Create task for new version
			versionInfo := registryVersionsMap[registryRelease.Digest]
			if existingTask, ok := createTasksByDigest[registryRelease.Digest]; ok {
				// Add channel to existing task
				existingTask.ReleaseChannels = append(existingTask.ReleaseChannels, key.Channel)
			} else {
				// Create new task
				createTasksByDigest[registryRelease.Digest] = &backends.DocumentationTask{
					Registry:        key.Registry,
					Module:          key.Module,
					Version:         versionInfo.Version,
					ReleaseChannels: []string{key.Channel},
					TarFile:         versionInfo.TarFile,
					Task:            backends.TaskCreate,
				}
			}
		}
	}

	// Find tasks to delete (releases that no longer exist in registry) - group by digest
	for key, cacheRelease := range c.releases {
		if _, exists := registryReleases[key]; !exists {
			if versionInfo, ok := c.versions[cacheRelease.Digest]; ok {
				if existingTask, ok := deleteTasksByDigest[cacheRelease.Digest]; ok {
					// Add channel to existing task
					existingTask.ReleaseChannels = append(existingTask.ReleaseChannels, key.Channel)
				} else {
					// Create new task
					deleteTasksByDigest[cacheRelease.Digest] = &backends.DocumentationTask{
						Registry:        key.Registry,
						Module:          key.Module,
						Version:         versionInfo.Version,
						ReleaseChannels: []string{key.Channel},
						TarFile:         versionInfo.TarFile,
						Task:            backends.TaskDelete,
					}
				}
			}
		}
	}

	// Convert maps to slices
	var createTasks []backends.DocumentationTask
	for _, task := range createTasksByDigest {
		createTasks = append(createTasks, *task)
	}
	
	var deleteTasks []backends.DocumentationTask
	for _, task := range deleteTasksByDigest {
		deleteTasks = append(deleteTasks, *task)
	}

	// Update cache with registry data
	c.releases = registryReleases
	c.versions = registryVersionsMap

	// Combine and sort all tasks
	result := append(createTasks, deleteTasks...)
	sortDocumentationTasks(result)

	return result
}

// GetTarFileByDigest returns tar file for given digest if it exists in cache
func (c *Cache) GetVersionInfoByDigest(digest string) (VersionInfo, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	versionInfo, ok := c.versions[digest]
	if !ok {
		return VersionInfo{}, false
	}

	return versionInfo, true
}

// convertToDocumentationTasks converts cache data to documentation tasks
func (c *Cache) convertToDocumentationTasks(task backends.Task) []backends.DocumentationTask {
	tasks := []backends.DocumentationTask{}

	// Group by version (digest) to collect all channels per version
	versionTasks := make(map[string]*backends.DocumentationTask)

	for key, release := range c.releases {
		versionInfo, ok := c.versions[release.Digest]
		if !ok {
			continue
		}

		taskKey := release.Digest
		if existingTask, exists := versionTasks[taskKey]; exists {
			// Add channel to existing task
			existingTask.ReleaseChannels = append(existingTask.ReleaseChannels, key.Channel)
		} else {
			// Create new task
			versionTasks[taskKey] = &backends.DocumentationTask{
				Registry:        key.Registry,
				Module:          key.Module,
				Version:         versionInfo.Version,
				ReleaseChannels: []string{key.Channel},
				TarFile:         versionInfo.TarFile,
				Task:            task,
			}
		}
	}

	// Convert map to slice
	for _, task := range versionTasks {
		tasks = append(tasks, *task)
	}

	sortDocumentationTasks(tasks)
	return tasks
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
