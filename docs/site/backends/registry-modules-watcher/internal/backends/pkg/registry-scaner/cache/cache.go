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
	"strings"
	"sync"

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
	versions        map[versionNum]data
}

type data struct {
	ReleaseChannels map[string]struct{}
	TarFile         []byte
}

type Cache struct {
	m         sync.RWMutex
	val       map[registryName]map[moduleName]moduleData
	stateSnap []backends.Version
}

func New() *Cache {
	return &Cache{
		val: make(map[registryName]map[moduleName]moduleData),
	}
}

// ResetRange sets stateSnap to State
func (c *Cache) ResetRange() {
	state := c.GetState()
	c.m.Lock()
	defer c.m.Unlock()

	c.stateSnap = make([]backends.Version, len(state))
	copy(c.stateSnap, state)
}

// GetRange returns a list of module versions from the current State
func (c *Cache) GetRange() []backends.Version {
	c.m.RLock()
	defer c.m.RUnlock()

	versions := []backends.Version{}
	state := c.GetState()
	for _, version := range c.stateSnap {
		if !contain(state, version) {
			version.ToDelete = true
			versions = append(versions, version)
		}
	}

	for _, version := range state {
		if !contain(c.stateSnap, version) {
			version.ToDelete = false
			versions = append(versions, version)
		}
	}

	return versions
}

func (c *Cache) GetState() []backends.Version {
	c.m.RLock()
	defer c.m.RUnlock()

	versions := []backends.Version{}
	for registry, modules := range c.val {
		for module, moduleData := range modules {
			for version, data := range moduleData.versions {
				releaseChannels := []string{}
				for releaseChannel := range data.ReleaseChannels {
					releaseChannels = append(releaseChannels, releaseChannel)
				}

				versions = append(versions, backends.Version{
					Registry:        string(registry),
					Module:          string(module),
					Version:         string(version),
					TarFile:         data.TarFile,
					ReleaseChannels: releaseChannels,
				})
			}
		}
	}

	return versions
}

func (c *Cache) GetReleaseChecksum(registry, module, releaseChannel string) (string, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	r, ok := c.val[registryName(registry)]
	if !ok {
		return "", false
	}

	m, ok := r[moduleName(module)]
	if !ok {
		return "", false
	}

	rc, ok := m.releaseChecksum[releaseChannelName(releaseChannel)]
	if !ok {
		return "", false
	}

	return rc, true
}

func (c *Cache) SetReleaseChecksum(registry, module, releaseChannel, releaseChecksum string) {
	c.m.Lock()
	defer c.m.Unlock()

	if _, ok := c.val[registryName(registry)]; !ok {
		c.val[registryName(registry)] = make(map[moduleName]moduleData)
	}
	if _, ok := c.val[registryName(registry)][moduleName(module)]; !ok {
		c.val[registryName(registry)][moduleName(module)] = moduleData{
			versions:        make(map[versionNum]data),
			releaseChecksum: make(map[releaseChannelName]string),
		}
	}

	c.val[registryName(registry)][moduleName(module)].releaseChecksum[releaseChannelName(releaseChannel)] = releaseChecksum
}

func (c *Cache) SetTar(registry, module, version, releaseChannel string, tarFile []byte) {
	c.m.Lock()
	defer c.m.Unlock()

	var releaseChannels = make(map[string]struct{})

	r, ok := c.val[registryName(registry)]
	if ok {
		m, ok := r[moduleName(module)]
		if ok {
			v, ok := m.versions[versionNum(version)]
			if ok {
				releaseChannels = v.ReleaseChannels
			}
		}
	}

	if _, ok := c.val[registryName(registry)]; !ok {
		c.val[registryName(registry)] = make(map[moduleName]moduleData)
	}
	if _, ok := c.val[registryName(registry)][moduleName(module)]; !ok {
		c.val[registryName(registry)][moduleName(module)] = moduleData{
			versions:        make(map[versionNum]data),
			releaseChecksum: make(map[releaseChannelName]string),
		}
	}

	// remove releaseChannel from all another versions
	c.syncReleaseChannels(registry, module, releaseChannel)

	releaseChannels[releaseChannel] = struct{}{}
	c.val[registryName(registry)][moduleName(module)].versions[versionNum(version)] = data{
		TarFile:         tarFile,
		ReleaseChannels: releaseChannels,
	}
}

func (c *Cache) syncReleaseChannels(registry, module, releaseChannel string) {
	r, ok := c.val[registryName(registry)]
	if ok {
		m, ok := r[moduleName(module)]
		if ok {
			for versionKey, version := range m.versions {
				delete(version.ReleaseChannels, releaseChannel)
				if len(version.ReleaseChannels) == 0 {
					delete(m.versions, versionKey)
				}
			}
		}
	}
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

func (c *Cache) GetReleaseChannels(registry, module string) []string {
	c.m.RLock()
	defer c.m.RUnlock()

	var releaseChannels []string
	r, ok := c.val[registryName(registry)]
	if ok {
		m, ok := r[moduleName(module)]
		if ok {
			for _, m := range m.versions {
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

	r, ok := c.val[registryName(registry)]
	if ok {
		m, ok := r[moduleName(module)]
		if ok {
			for _, m := range m.versions {
				delete(m.ReleaseChannels, releaseChannel)
			}
		}
	}
}

func contain(versions []backends.Version, version backends.Version) bool {
	for _, val := range versions {
		if val.Registry == version.Registry &&
			val.Module == version.Module &&
			val.Version == version.Version {
			sort.Strings(val.ReleaseChannels)
			sort.Strings(version.ReleaseChannels)
			if strings.Join(val.ReleaseChannels, "") == strings.Join(version.ReleaseChannels, "") {
				return true
			}
		}
	}

	return false
}
