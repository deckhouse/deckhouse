// Copyright 2024 Flant JSC
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

package config

import (
	"path/filepath"
	"sync"

	"github.com/cloudflare/cfssl/csr"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/certificate"
)

var (
	registryCache = registryCacheData{}
)

type registryCacheData struct {
	once      sync.Once
	initError error
	cache     *cache.StateCache
}
type RegistryPkiData struct {
	CaCert string `json:"caCert"`
	CaKey  string `json:"caKey"`
}

func (d *RegistryPkiData) ConvertToMap() map[string]interface{} {
	return map[string]interface{}{
		"caCert": d.CaCert,
		"caKey":  d.CaKey,
	}
}

func getRegistryCache() (*cache.StateCache, error) {
	registryCache.once.Do(func() {
		registryCache.cache, registryCache.initError = cache.NewStateCache(filepath.Join(app.CacheDir, "system-registry"))
	})
	return registryCache.cache, registryCache.initError
}

func getRegistryPkiData() (*RegistryPkiData, error) {
	registryPkiDataCacheName := "pki-data"

	registryCache, err := getRegistryCache()
	if err != nil {
		return nil, err
	}

	inCache, err := registryCache.InCache(registryPkiDataCacheName)
	if err != nil {
		return nil, err
	}
	if inCache {
		var registryPkiData RegistryPkiData
		err := registryCache.LoadStruct(registryPkiDataCacheName, &registryPkiData)
		return &registryPkiData, err
	} else {
		authority, err := newRegistryAuthority()
		if err != nil {
			return nil, err
		}

		registryPkiData := RegistryPkiData{CaCert: authority.Cert, CaKey: authority.Key}
		err = registryCache.SaveStruct(registryPkiDataCacheName, registryPkiData)
		return &registryPkiData, err
	}
}

func newRegistryAuthority() (certificate.Authority, error) {
	return certificate.GenerateCA(
		"registry-selfsigned-ca",
		certificate.WithNames(
			csr.Name{
				C:  "RU",
				ST: "MO",
				L:  "Moscow",
				O:  "Flant",
				OU: "Deckhouse Registry",
			},
		),
	)
}
