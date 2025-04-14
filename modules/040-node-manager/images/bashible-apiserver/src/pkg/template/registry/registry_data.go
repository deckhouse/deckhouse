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

package registry

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"

	validation "github.com/go-ozzo/ozzo-validation"
)

var (
	_ validation.Validatable = &RegistryData{}
	_ validation.Validatable = &RegistryDataHostsObject{}
	_ validation.Validatable = &RegistryDataMirrorHostObject{}
)

type RegistryData struct {
	Mode           string                    `json:"mode" yaml:"mode"`
	ImagesBase     string                    `json:"imagesBase" yaml:"imagesBase"`
	Version        string                    `json:"version" yaml:"version"`
	ProxyEndpoints []string                  `json:"proxyEndpoints" yaml:"proxyEndpoints"`
	Hosts          []RegistryDataHostsObject `json:"hosts" yaml:"hosts"`
	PrepullHosts   []RegistryDataHostsObject `json:"prepullHosts" yaml:"prepullHosts"`
}

type RegistryDataHostsObject struct {
	Host    string                         `json:"host" yaml:"host"`
	CA      []string                       `json:"ca" yaml:"ca"`
	Mirrors []RegistryDataMirrorHostObject `json:"mirrors" yaml:"mirrors"`
}

type RegistryDataMirrorHostObject struct {
	Host     string `json:"host" yaml:"host"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
	Scheme   string `json:"scheme" yaml:"scheme"`
}

func (d *RegistryData) FromInputData(deckhouseRegistry deckhouseRegistry, registryBashibleConfig *registryBashibleConfig) error {
	registryDataFromDeckhouseRegistry, err := deckhouseRegistry.ConvertToRegistryData()
	if err != nil {
		return err
	}

	if registryBashibleConfig != nil {
		d.copyFrom(registryBashibleConfig.ConvertToRegistryData())
		d.Hosts = appendUniqueHosts(d.Hosts, registryDataFromDeckhouseRegistry.Hosts)
		d.PrepullHosts = appendUniqueHosts(d.PrepullHosts, registryDataFromDeckhouseRegistry.PrepullHosts)
		return nil
	}

	d.copyFrom(registryDataFromDeckhouseRegistry)
	return nil
}

func (d *RegistryData) copyFrom(other *RegistryData) {
	*d = *other
}

func (d *RegistryData) hashSum() (string, error) {
	rawData, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("error marshalling data: %w", err)
	}

	hash := sha256.New()
	_, err = hash.Write(rawData)
	if err != nil {
		return "", fmt.Errorf("error generating hash: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (d *RegistryData) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.Mode, validation.Required),
		validation.Field(&d.Version, validation.Required),
		validation.Field(&d.ImagesBase, validation.Required),
		validation.Field(&d.ProxyEndpoints, validation.Each(validation.Required)),
		validation.Field(&d.Hosts, validation.Each(validation.By(func(value interface{}) error {
			if v, ok := value.(RegistryDataHostsObject); ok {
				return v.Validate()
			}
			return nil
		}))),
		validation.Field(&d.PrepullHosts, validation.Each(validation.By(func(value interface{}) error {
			if v, ok := value.(RegistryDataHostsObject); ok {
				return v.Validate()
			}
			return nil
		}))),
	)
}

func (d *RegistryDataHostsObject) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.Host, validation.Required),
		validation.Field(&d.CA, validation.Each(validation.Required)),
		validation.Field(&d.Mirrors, validation.Each(validation.By(func(value interface{}) error {
			if v, ok := value.(RegistryDataMirrorHostObject); ok {
				return v.Validate()
			}
			return nil
		}))),
	)
}

func (d *RegistryDataMirrorHostObject) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.Host, validation.Required),
		validation.Field(&d.Scheme, validation.Required),
	)
}

func appendUniqueHosts(existingHosts, newHosts []RegistryDataHostsObject) []RegistryDataHostsObject {
	for _, newHost := range newHosts {
		if !slices.ContainsFunc(existingHosts, func(host RegistryDataHostsObject) bool {
			return host.Host == newHost.Host
		}) {
			existingHosts = append(existingHosts, newHost)
		}
	}
	return existingHosts
}
