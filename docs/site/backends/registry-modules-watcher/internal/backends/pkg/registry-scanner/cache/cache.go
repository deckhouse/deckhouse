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
}

func New() *Cache {
	return &Cache{
		val: make(map[registryName]map[moduleName]moduleData),
	}
}

func (c *Cache) GetState() []backends.DocumentationTask {
	c.m.RLock()
	defer c.m.RUnlock()

	return RemapFromMapToVersions(c.val, backends.TaskCreate)
}

func (c *Cache) GetReleaseChecksum(version *internal.VersionData) (string, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	r, ok := c.val[registryName(version.Registry)]
	if !ok {
		return "", false
	}

	m, ok := r[moduleName(version.ModuleName)]
	if !ok {
		return "", false
	}

	rc, ok := m.releaseChecksum[releaseChannelName(version.ReleaseChannel)]
	if !ok {
		return "", false
	}

	return rc, true
}

func (c *Cache) GetReleaseVersionData(version *internal.VersionData) (string, []byte, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	r, ok := c.val[registryName(version.Registry)]
	if !ok {
		return "", nil, false
	}

	m, ok := r[moduleName(version.ModuleName)]
	if !ok {
		return "", nil, false
	}

	verData, ok := m.versions[versionNum(version.Version)]
	if !ok {
		return "", nil, false
	}

	if _, ok := verData.releaseChannels[version.ReleaseChannel]; ok {
		return string(version.Version), verData.tarFile, true
	}

	return "", nil, false
}

// SyncWithRegistryVersions compares the current cache with versions from the registry
// and returns documentation tasks that need to be performed.
//
// Flow:
// 1. For each version from the registry:
//
//   - If it exists in the cache with matching registry, module, version, release channel, and checksum:
//     Remove that release channel from the cache comparison
//
//   - If it doesn't match completely: Mark it for creation
//
//     2. After processing all registry versions, any versions/release channels remaining
//     in the cache are marked for deletion
//     3. Update the cache with all versions from the registry
//     4. Return sorted tasks for both creation and deletion
//
// Example scenario:
// - Initial cache contains versions: 1.2.3, 1.2.4, 1.2.5
// - Registry provides versions: 1.2.4, 1.2.5, 1.2.6
// - Result:
//   - 1.2.3 remains in cache temporarily and is marked for deletion
//   - 1.2.4 and 1.2.5 are temporarily removed from cache comparison
//   - 1.2.6 is identified as new and marked for creation
//   - Final tasks: Delete 1.2.3, Create 1.2.6
//   - Cache is updated to match registry: 1.2.4, 1.2.5, 1.2.6
func (c *Cache) SyncWithRegistryVersions(versions []internal.VersionData) []backends.DocumentationTask {
	c.m.Lock()
	defer c.m.Unlock()

	// Create a slice to hold unique versions
	newVersions := make([]internal.VersionData, 0)

	// Iterate through all input versions
	for _, version := range versions {
		// Check if this version already exists in the cache
		found := false

		if modules, ok := c.val[registryName(version.Registry)]; ok {
			if module, ok := modules[moduleName(version.ModuleName)]; ok {
				if versionData, ok := module.versions[versionNum(version.Version)]; ok {
					if _, ok := versionData.releaseChannels[version.ReleaseChannel]; ok {
						if module.releaseChecksum[releaseChannelName(version.ReleaseChannel)] == version.Checksum {
							delete(versionData.releaseChannels, version.ReleaseChannel)
							delete(module.releaseChecksum, releaseChannelName(version.ReleaseChannel))

							found = true
						}
					}

					if len(versionData.releaseChannels) == 0 {
						delete(module.versions, versionNum(version.Version))
					}
				}

				if len(module.versions) == 0 {
					delete(modules, moduleName(version.ModuleName))
				}
			}

			if len(modules) == 0 {
				delete(c.val, registryName(version.Registry))
			}
		}

		// If not found in cache, keep it in the result
		if !found {
			newVersions = append(newVersions, version)
		}
	}

	versionsToDelete := RemapFromMapToVersions(c.val, backends.TaskDelete)

	result := RemapFromMapToVersions(RemapFromVersionData(newVersions), backends.TaskCreate)
	result = append(result, versionsToDelete...)

	sortDocumentationTasks(result)

	// Update the cache with the registry versions
	c.val = RemapFromVersionData(versions)

	return result
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

func RemapFromVersionData(input []internal.VersionData) map[registryName]map[moduleName]moduleData {
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

func (c *Cache) GetModules(registry string) []string {
	c.m.RLock()
	defer c.m.RUnlock()

	var modules []string
	r, ok := c.val[registryName(registry)]
	if ok {
		for m := range r {
			modules = append(modules, string(m))
		}
	}

	return modules
}

func (c *Cache) DeleteModule(registry string, module string) {
	c.m.Lock()
	defer c.m.Unlock()

	r, ok := c.val[registryName(registry)]
	if ok {
		delete(r, moduleName(module))
	}
}

func sortDocumentationTasks(input []backends.DocumentationTask) {
	for i := range input {
		slices.Sort(input[i].ReleaseChannels)
	}

	sort.Slice(input, func(i, j int) bool {
		if input[i].Registry != input[j].Registry {
			return input[i].Registry < input[j].Registry
		}

		if input[i].Module != input[j].Module {
			return input[i].Module < input[j].Module
		}

		if input[i].Version != input[j].Version {
			return input[i].Version < input[j].Version
		}

		return input[i].Task > input[j].Task
	})
}
