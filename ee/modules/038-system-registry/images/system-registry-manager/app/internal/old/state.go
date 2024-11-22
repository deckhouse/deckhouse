/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package registry_controller

import (
	"crypto/x509"

	util_time "embeded-registry-manager/internal/utils/time"
)

type state struct {
	ModuleConfig ModuleConfig
	CAPKI        x509.Certificate
	AuthTokenPKI x509.Certificate
	RWUser       RegistryUser
	ROUser       RegistryUser
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
