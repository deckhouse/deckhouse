/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package legacy_controller

import (
	"sync"

	"embeded-registry-manager/internal/staticpod"
	k8s "embeded-registry-manager/internal/utils/k8s_legacy"
	util_time "embeded-registry-manager/internal/utils/time"
)

type embeddedRegistry struct {
	mutex          sync.Mutex
	mc             ModuleConfig
	caPKI          k8s.Certificate
	authTokenPKI   k8s.Certificate
	registryRwUser k8s.RegistryUser
	registryRoUser k8s.RegistryUser
	masterNodes    map[string]k8s.MasterNode
	images         staticpod.Images
}

type RegistryUser struct {
	UserName       string
	Password       string
	HashedPassword string
}

type ModuleConfig struct {
	Enabled  bool           `json:"enabled"`
	Settings RegistryConfig `json:"settings"`
}

type RegistryConfig struct {
	Mode     string          `json:"mode"` // enum: Direct, Proxy, Detached
	Proxy    *ProxyConfig    `json:"proxy,omitempty"`
	Detached *DetachedConfig `json:"detached,omitempty"`
}

type StorageMode string

type DetachedConfig struct {
	StorageMode StorageMode `json:"storageMode"` // enum: S3, Fs
}

type ProxyConfig struct {
	Host        string              `json:"host"`
	Scheme      string              `json:"scheme"`
	CA          string              `json:"ca"`
	Path        string              `json:"path"`
	User        string              `json:"user"`
	Password    string              `json:"password"`
	StorageMode StorageMode         `json:"storageMode"` // enum: S3, Fs
	TTL         *util_time.Duration `json:"ttl"`
}
