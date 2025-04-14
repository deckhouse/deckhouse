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
	"github.com/hashicorp/go-multierror"
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
	var result *multierror.Error

	err := validation.ValidateStruct(d,
		validation.Field(&d.Mode, validation.Required),
		validation.Field(&d.Version, validation.Required),
		validation.Field(&d.ImagesBase, validation.Required),
	)
	if err != nil {
		result = multierror.Append(result, err)
	}

	for _, host := range d.Hosts {
		if err := host.Validate(); err != nil {
			result = multierror.Append(result, err)
		}
	}

	for _, host := range d.PrepullHosts {
		if err := host.Validate(); err != nil {
			result = multierror.Append(result, err)
		}
	}

	for _, endpoint := range d.ProxyEndpoints {
		if err := validation.Validate(endpoint, validation.Required); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

func (d *RegistryDataHostsObject) Validate() error {
	var result *multierror.Error

	err := validation.ValidateStruct(d,
		validation.Field(&d.Host, validation.Required),
	)
	if err != nil {
		result = multierror.Append(result, err)
	}

	for _, ca := range d.CA {
		if err := validation.Validate(ca, validation.Required); err != nil {
			result = multierror.Append(result, err)
		}
	}

	for _, mirrorHost := range d.Mirrors {
		if err := mirrorHost.Validate(); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}

func (d *RegistryDataMirrorHostObject) Validate() error {
	var result *multierror.Error

	err := validation.ValidateStruct(d,
		validation.Field(&d.Host, validation.Required),
		validation.Field(&d.Scheme, validation.Required),
	)
	if err != nil {
		result = multierror.Append(result, err)
	}

	return result.ErrorOrNil()
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
