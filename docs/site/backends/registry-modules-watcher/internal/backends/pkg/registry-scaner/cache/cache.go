package cache

import "registry-modules-watcher/internal/backends"

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

// map[registry]map[moduleName]module
type Cache struct {
	val map[registryName]map[moduleName]moduleData
}

func New() *Cache {
	return &Cache{
		val: make(map[registryName]map[moduleName]moduleData),
	}
}

func (c *Cache) GetState() []backends.Version {
	var versions = []backends.Version{}

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
					delete(m.versions, versionNum(versionKey))
				}
			}
		}
	}
}
