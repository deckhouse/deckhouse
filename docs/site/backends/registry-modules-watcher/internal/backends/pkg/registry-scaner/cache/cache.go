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
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"

	"registry-modules-watcher/internal/backends"

	"github.com/google/go-cmp/cmp"
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
	m         sync.RWMutex
	val       map[RegistryName]map[ModuleName]ModuleData
	stateSnap []backends.Version

	stateSnapMap map[RegistryName]map[ModuleName]ModuleData
}

func New() *Cache {
	return &Cache{
		val: make(map[RegistryName]map[ModuleName]ModuleData),
	}
}

// ResetRange sets stateSnap to State
func (c *Cache) ResetRange() {
	state := c.GetState()
	c.m.Lock()
	defer c.m.Unlock()

	c.stateSnap = make([]backends.Version, len(state))
	copy(c.stateSnap, state)

	c.stateSnapMap = make(map[RegistryName]map[ModuleName]ModuleData, len(c.val))
	maps.Copy(c.stateSnapMap, c.val)
}

// GetRange returns a list of module versions from the current State
func (c *Cache) GetRange() []backends.Version {
	c.m.RLock()
	defer c.m.RUnlock()

	versions := []backends.Version{}

	for registryName, modules := range c.val {
		snapModules, ok := c.stateSnapMap[registryName]
		if !ok {
			// TODO: remove all registry data
			delete(c.stateSnapMap, registryName)
			continue
		}

		for moduleName, moduleData := range modules {
			snapModuleData, ok := snapModules[moduleName]
			if !ok {
				// TODO: remove all module data
				delete(c.stateSnapMap[registryName], moduleName)
				continue
			}

			for version, data := range moduleData.Versions {
				snapVersionData, ok := snapModuleData.Versions[version]
				if !ok {
					// TODO: remove version
					delete(c.stateSnapMap[registryName][moduleName].Versions, version)
					continue
				}

				cmp.Equal(data.ReleaseChannels, snapVersionData.ReleaseChannels)
			}
		}
	}

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
			for version, data := range moduleData.Versions {
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

func (c *Cache) GetCache() map[RegistryName]map[ModuleName]ModuleData {
	c.m.RLock()
	defer c.m.RUnlock()

	cacheCopy := CopyMap(c.val)

	return cacheCopy
}

func CopyMap(m map[RegistryName]map[ModuleName]ModuleData) map[RegistryName]map[ModuleName]ModuleData {
	cp := make(map[RegistryName]map[ModuleName]ModuleData)
	for k, v := range m {
		cp[k] = make(map[ModuleName]ModuleData)
		for kk, vv := range v {
			cp[k][kk] = ModuleData{
				ReleaseChecksum: make(map[ReleaseChannelName]string, len(vv.ReleaseChecksum)),
				Versions:        make(map[VersionNum]Data, len(vv.Versions)),
			}

			for kkk, vvv := range vv.Versions {
				cp[k][kk].Versions[kkk] = Data{
					TarLen:          len(vvv.TarFile),
					ReleaseChannels: make(map[string]struct{}, len(vvv.ReleaseChannels)),
				}

				for kkkk, vvvv := range vvv.ReleaseChannels {
					cp[k][kk].Versions[kkk].ReleaseChannels[kkkk] = vvvv
				}
			}

			for kkk, vvv := range vv.ReleaseChecksum {
				cp[k][kk].ReleaseChecksum[kkk] = vvv
			}
		}
	}

	return cp
}

func (c *Cache) GetReleaseChecksum(registry, module, releaseChannel string) (string, bool) {
	c.m.RLock()
	defer c.m.RUnlock()

	r, ok := c.val[RegistryName(registry)]
	if !ok {
		return "", false
	}

	m, ok := r[ModuleName(module)]
	if !ok {
		return "", false
	}

	rc, ok := m.ReleaseChecksum[ReleaseChannelName(releaseChannel)]
	if !ok {
		return "", false
	}

	return rc, true
}

func (c *Cache) SetReleaseChecksum(registry, module, releaseChannel, releaseChecksum string) {
	c.m.Lock()
	defer c.m.Unlock()

	if _, ok := c.val[RegistryName(registry)]; !ok {
		c.val[RegistryName(registry)] = make(map[ModuleName]ModuleData)
	}

	if _, ok := c.val[RegistryName(registry)][ModuleName(module)]; !ok {
		c.val[RegistryName(registry)][ModuleName(module)] = ModuleData{
			Versions:        make(map[VersionNum]Data),
			ReleaseChecksum: make(map[ReleaseChannelName]string),
		}
	}

	c.val[RegistryName(registry)][ModuleName(module)].ReleaseChecksum[ReleaseChannelName(releaseChannel)] = releaseChecksum
}

func (c *Cache) SetTar(registry, module, version, releaseChannel string, tarFile []byte) {
	c.m.Lock()
	defer c.m.Unlock()

	var releaseChannels = make(map[string]struct{})

	r, ok := c.val[RegistryName(registry)]
	if ok {
		m, ok := r[ModuleName(module)]
		if ok {
			v, ok := m.Versions[VersionNum(version)]
			if ok {
				releaseChannels = v.ReleaseChannels
			}
		}
	}

	if _, ok := c.val[RegistryName(registry)]; !ok {
		c.val[RegistryName(registry)] = make(map[ModuleName]ModuleData)
	}
	if _, ok := c.val[RegistryName(registry)][ModuleName(module)]; !ok {
		c.val[RegistryName(registry)][ModuleName(module)] = ModuleData{
			Versions:        make(map[VersionNum]Data),
			ReleaseChecksum: make(map[ReleaseChannelName]string),
		}
	}

	// remove releaseChannel from all another versions
	c.syncReleaseChannels(registry, module, releaseChannel)

	releaseChannels[releaseChannel] = struct{}{}
	c.val[RegistryName(registry)][ModuleName(module)].Versions[VersionNum(version)] = Data{
		TarFile:         tarFile,
		ReleaseChannels: releaseChannels,
	}
}

func (c *Cache) syncReleaseChannels(registry, module, releaseChannel string) {
	r, ok := c.val[RegistryName(registry)]
	if ok {
		m, ok := r[ModuleName(module)]
		if ok {
			for versionKey, version := range m.Versions {
				delete(version.ReleaseChannels, releaseChannel)
				if len(version.ReleaseChannels) == 0 {
					delete(m.Versions, versionKey)
				}
			}
		}
	}
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
