package config

import (
	"path/filepath"
	"sync"

	// "github.com/cloudflare/cfssl/csr"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	// "github.com/deckhouse/deckhouse/go_lib/certificate"
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
		// authority, err := newRegistryAuthority()
		// if err != nil {
		// 	return nil, err
		// }

		// registryPkiData := RegistryPkiData{CaCert: authority.Cert, CaKey: authority.Key}
		registryPkiData := RegistryPkiData{CaCert: "", CaKey: ""}
		err = registryCache.SaveStruct(registryPkiDataCacheName, registryPkiData)
		return &registryPkiData, err
	}
}

// func newRegistryAuthority() (certificate.Authority, error) {
// 	return certificate.GenerateCA(
// 		nil,
// 		"registry-selfsigned-ca",
// 		certificate.WithNames(
// 			csr.Name{
// 				C:  "RU",
// 				ST: "MO",
// 				L:  "Moscow",
// 				O:  "Flant",
// 				OU: "Deckhouse Registry",
// 			},
// 		),
// 	)
// }
