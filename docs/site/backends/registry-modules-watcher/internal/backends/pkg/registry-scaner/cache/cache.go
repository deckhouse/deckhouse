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
	"sync"

	"registry-modules-watcher/internal"
	"registry-modules-watcher/internal/backends"
)

type (
	RegistryName       string
	ModuleName         string
	VersionNum         string
	ReleaseChannelName string
)

type ModuleData struct {
	ReleaseChecksum map[ReleaseChannelName]string
	Versions        map[VersionNum]Data
}

type Data struct {
	ReleaseChannels map[string]struct{}
	TarFile         []byte
	TarLen          int
}

type Cache struct {
	m   sync.RWMutex
	val map[RegistryName]map[ModuleName]ModuleData
}

func New() *Cache {
	return &Cache{
		val: make(map[RegistryName]map[ModuleName]ModuleData),
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

	r, ok := c.val[RegistryName(version.Registry)]
	if !ok {
		return "", false
	}

	m, ok := r[ModuleName(version.ModuleName)]
	if !ok {
		return "", false
	}

	rc, ok := m.ReleaseChecksum[ReleaseChannelName(version.ReleaseChannel)]
	if !ok {
		return "", false
	}

	return rc, true
}

func (c *Cache) GetReleaseVersionData(version *internal.VersionData) (string, []byte, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	r, ok := c.val[RegistryName(version.Registry)]
	if !ok {
		return "", nil, false
	}

	m, ok := r[ModuleName(version.ModuleName)]
	if !ok {
		return "", nil, false
	}

	for ver, verData := range m.Versions {
		_, ok := verData.ReleaseChannels[version.ReleaseChannel]
		if ok {
			return string(ver), verData.TarFile, true
		}
	}

	return "", nil, false
}

// cache is populated
// 1.2.3 was not removed and remains in cache, form a list for deletion
// 1.2.4 was removed from cache and taken into the list (already existed)
// 1.2.5 was removed from cache and taken into the list (already existed)
// versions arrived
// 1.2.4
// 1.2.5
// 1.2.6 was not found and added to the list (needs to be added)
func (c *Cache) SyncWithRegistryVersions(versions []internal.VersionData) []backends.DocumentationTask {
	c.m.RLock()
	defer c.m.RUnlock()

	// Create a slice to hold unique versions
	newVersions := make([]internal.VersionData, 0, len(versions))

	// Iterate through all input versions
	for _, version := range versions {
		// Check if this version already exists in the cache
		found := false

		if modules, ok := c.val[RegistryName(version.Registry)]; ok {
			if module, ok := modules[ModuleName(version.ModuleName)]; ok {
				if versionData, ok := module.Versions[VersionNum(version.Version)]; ok {
					delete(versionData.ReleaseChannels, version.ReleaseChannel)
					delete(module.ReleaseChecksum, ReleaseChannelName(version.ReleaseChannel))

					if len(versionData.ReleaseChannels) == 0 {
						delete(module.Versions, VersionNum(version.Version))
					}

					found = true
				}

				if len(module.Versions) == 0 {
					delete(modules, ModuleName(version.ModuleName))
				}
			}

			if len(modules) == 0 {
				delete(c.val, RegistryName(version.Registry))
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

	// Update the cache with the registry versions
	c.val = RemapFromVersionData(versions)

	return result
}

func RemapFromMapToVersions(m map[RegistryName]map[ModuleName]ModuleData, task backends.Task) []backends.DocumentationTask {
	versions := make([]backends.DocumentationTask, 0, 1)
	for registry, modules := range m {
		for module, moduleData := range modules {
			for version, data := range moduleData.Versions {
				releaseChannels := make([]string, 0, len(data.ReleaseChannels))
				for releaseChannel := range data.ReleaseChannels {
					releaseChannels = append(releaseChannels, releaseChannel)
				}

				versions = append(versions, backends.DocumentationTask{
					Registry:        string(registry),
					Module:          string(module),
					Version:         string(version),
					ReleaseChannels: releaseChannels,
					TarFile:         data.TarFile,
					Task:            task,
				})
			}
		}
	}

	return versions
}

func RemapFromVersionData(input []internal.VersionData) map[RegistryName]map[ModuleName]ModuleData {
	result := make(map[RegistryName]map[ModuleName]ModuleData)

	for _, ver := range input {
		registry := RegistryName(ver.Registry)
		module := ModuleName(ver.ModuleName)
		version := VersionNum(ver.Version)

		// Initialize registry map if it doesn't exist
		if _, exists := result[registry]; !exists {
			result[registry] = make(map[ModuleName]ModuleData)
		}

		// Initialize module data if it doesn't exist
		if _, exists := result[registry][module]; !exists {
			result[registry][module] = ModuleData{
				ReleaseChecksum: make(map[ReleaseChannelName]string),
				Versions:        make(map[VersionNum]Data),
			}
		}

		// Add or update version data
		moduleData := result[registry][module]

		// Initialize version data if it doesn't exist
		if _, exists := moduleData.Versions[version]; !exists {
			moduleData.Versions[version] = Data{
				ReleaseChannels: make(map[string]struct{}),
				TarFile:         ver.TarFile,
				TarLen:          len(ver.TarFile),
			}
		}

		// Add release channel to the version
		if ver.ReleaseChannel != "" {
			moduleData.Versions[version].ReleaseChannels[ver.ReleaseChannel] = struct{}{}

			// Update release checksum if provided
			if ver.Checksum != "" {
				moduleData.ReleaseChecksum[ReleaseChannelName(ver.ReleaseChannel)] = ver.Checksum
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
	r, ok := c.val[RegistryName(registry)]
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

	r, ok := c.val[RegistryName(registry)]
	if ok {
		delete(r, ModuleName(module))
	}
}

func (c *Cache) GetReleaseChannels(registry, module string) []string {
	c.m.RLock()
	defer c.m.RUnlock()

	var releaseChannels []string
	r, ok := c.val[RegistryName(registry)]
	if ok {
		m, ok := r[ModuleName(module)]
		if ok {
			for _, m := range m.Versions {
				for releaseChannel := range m.ReleaseChannels {
					releaseChannels = append(releaseChannels, releaseChannel)
				}
			}
		}
	}

	return slices.Compact(releaseChannels)
}

func (c *Cache) DeleteReleaseChannel(registry, module, releaseChannel string) {
	c.m.Lock()
	defer c.m.Unlock()

	r, ok := c.val[RegistryName(registry)]
	if ok {
		m, ok := r[ModuleName(module)]
		if ok {
			for _, m := range m.Versions {
				delete(m.ReleaseChannels, releaseChannel)
			}
		}
	}
}
