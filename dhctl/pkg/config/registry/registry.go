// Copyright 2025 Flant JSC
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

package registry

import (
	"encoding/base64"
	"errors"

	registry_docker "github.com/deckhouse/deckhouse/go_lib/registry/docker"
	"github.com/deckhouse/deckhouse/go_lib/registry/models/users"
	"github.com/deckhouse/deckhouse/go_lib/registry/pki"

	"github.com/deckhouse/deckhouse/dhctl/pkg/state/cache"
)

const (
	roUser       = "ro"
	rwUser       = "rw"
	caCommonName = "registry-ca"
	pkiCacheName = "registry-pki"
)

var (
	ErrorUnknownRegistryMode = errors.New("Unknown registry mode")
)

type Registry struct {
	spec Spec
	pki  PKI
}
type Data struct {
	ImagesRepo string `json:"imagesRepo" yaml:"imagesRepo"`
	Scheme     Scheme `json:"scheme" yaml:"scheme"`
	CA         string `json:"ca,omitempty" yaml:"ca,omitempty"`
	Username   string `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string `json:"password,omitempty" yaml:"password,omitempty"`
}

type PKI struct {
	CA     CertKey    `json:"ca" yaml:"ca"`
	UserRW users.User `json:"userRW" yaml:"userRW"`
	UserRO users.User `json:"userRO" yaml:"userRO"`
}

type CertKey struct {
	Cert string `json:"cert" yaml:"cert"`
	Key  string `json:"key" yaml:"key"`
}

func FromDeckhouseSettings(rawJson string) (Registry, error) {
	spec := Spec{}
	err := spec.fromDeckhouseSettings(rawJson)
	return Registry{spec: spec}, err
}

func FromDefault() (Registry, error) {
	spec := Spec{}
	err := spec.fromDefault()
	return Registry{spec: spec}, err
}

func FromInitConfig(initConfig InitConfigSpec) (Registry, error) {
	spec := Spec{}
	err := spec.fromInitConfig(initConfig)
	return Registry{spec: spec}, err
}

func (r *Registry) InitWithGlobalCache() error {
	var ret PKI

	inCache, err := cache.Global().InCache(pkiCacheName)
	if err != nil {
		return err
	}
	if inCache {
		if err := cache.Global().LoadStruct(pkiCacheName, &ret); err != nil {
			return err
		}
	} else {
		certKey, err := pki.GenerateCACertificate(caCommonName)
		if err != nil {
			return err
		}
		cert, key, err := pki.EncodeCertKey(certKey)
		if err != nil {
			return err
		}

		userRW, err := users.New(rwUser)
		if err != nil {
			return err
		}

		userRO, err := users.New(roUser)
		if err != nil {
			return err
		}

		ret = PKI{
			CA:     CertKey{Cert: string(cert), Key: string(key)},
			UserRW: userRW,
			UserRO: userRO,
		}
		if err = cache.Global().SaveStruct(pkiCacheName, ret); err != nil {
			return err
		}
	}

	r.pki = ret
	return nil
}

func (r Registry) ConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{registry: r}
}

func (r *Registry) UpstreamData() (Data, error) {
	switch {
	case r.spec.Unmanaged != nil:
		unmanaged := r.spec.Unmanaged
		return Data{
			ImagesRepo: unmanaged.ImagesRepo,
			Scheme:     unmanaged.Scheme,
			CA:         unmanaged.CA,
			Username:   unmanaged.Username,
			Password:   unmanaged.Password,
		}, nil
	case r.spec.Direct != nil:
		direct := r.spec.Direct
		return Data{
			ImagesRepo: direct.ImagesRepo,
			Scheme:     direct.Scheme,
			CA:         direct.CA,
			Username:   direct.Username,
			Password:   direct.Password,
		}, nil
	default:
		return Data{}, ErrorUnknownRegistryMode
	}
}

func (d *Data) AuthBase64() string {
	if d.Username != "" {
		return base64.StdEncoding.EncodeToString([]byte(d.Username + ":" + d.Password))
	}
	return ""
}

func (d *Data) DockerCfgBase64() (string, error) {
	address, _ := addressAndPathFromImagesRepo(d.ImagesRepo)
	dockerCfg, err := registry_docker.DockerCfgFromCreds(d.Username, d.Password, address)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(dockerCfg), nil
}
