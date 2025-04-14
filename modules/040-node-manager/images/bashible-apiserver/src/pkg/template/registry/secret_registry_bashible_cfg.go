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
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	RegistryBashibleConfigSecretName = "registry-bashible-config"
)

var (
	_ validation.Validatable = &RegistryData{}
	_ validation.Validatable = &RegistryDataHostsObject{}
	_ validation.Validatable = &RegistryDataMirrorHostObject{}
)

type registryBashibleConfig struct {
	Mode           string                              `json:"mode" yaml:"mode"`
	ImagesBase     string                              `json:"imagesBase" yaml:"imagesBase"`
	Version        string                              `json:"version" yaml:"version"`
	ProxyEndpoints []string                            `json:"proxyEndpoints" yaml:"proxyEndpoints"`
	Hosts          []registryBashibleConfigHostsObject `json:"hosts" yaml:"hosts"`
	PrepullHosts   []registryBashibleConfigHostsObject `json:"prepullHosts" yaml:"prepullHosts"`
}

type registryBashibleConfigHostsObject struct {
	Host    string                                   `json:"host" yaml:"host"`
	CA      []string                                 `json:"ca" yaml:"ca"`
	Mirrors []registryBashibleConfigMirrorHostObject `json:"mirrors" yaml:"mirrors"`
}

type registryBashibleConfigMirrorHostObject struct {
	Host     string `json:"host" yaml:"host"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
	Scheme   string `json:"scheme" yaml:"scheme"`
}

func (d *registryBashibleConfig) DecodeSecret(secret *corev1.Secret) error {
	if err := yaml.Unmarshal(secret.Data["config"], d); err != nil {
		return fmt.Errorf("failed to parse registry bashible config: %w", err)
	}
	return nil
}

func (d registryBashibleConfig) ConvertToRegistryData() *RegistryData {
	ret := &RegistryData{
		Mode:           d.Mode,
		ImagesBase:     d.ImagesBase,
		Version:        d.Version,
		ProxyEndpoints: d.ProxyEndpoints,
		Hosts:          make([]RegistryDataHostsObject, 0, len(d.Hosts)),
		PrepullHosts:   make([]RegistryDataHostsObject, 0, len(d.PrepullHosts)),
	}

	for _, host := range d.Hosts {
		ret.Hosts = append(ret.Hosts, host.ConvertToRegistryDataHostsObject())
	}

	for _, host := range d.PrepullHosts {
		ret.PrepullHosts = append(ret.PrepullHosts, host.ConvertToRegistryDataHostsObject())
	}
	return ret
}

func (host registryBashibleConfigHostsObject) ConvertToRegistryDataHostsObject() RegistryDataHostsObject {
	out := RegistryDataHostsObject{
		Host:    host.Host,
		CA:      host.CA,
		Mirrors: make([]RegistryDataMirrorHostObject, 0, len(host.Mirrors)),
	}
	for _, mirror := range host.Mirrors {
		out.Mirrors = append(out.Mirrors, mirror.ConvertToRegistryDataMirrorHostObject())
	}
	return out
}

func (mirror registryBashibleConfigMirrorHostObject) ConvertToRegistryDataMirrorHostObject() RegistryDataMirrorHostObject {
	return RegistryDataMirrorHostObject(mirror)
}

func (d *registryBashibleConfig) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.Mode, validation.Required),
		validation.Field(&d.Version, validation.Required),
		validation.Field(&d.ImagesBase, validation.Required),
		validation.Field(&d.ProxyEndpoints, validation.Each(validation.Required)),
		validation.Field(&d.Hosts, validation.Each(validation.By(func(value interface{}) error {
			if v, ok := value.(registryBashibleConfigHostsObject); ok {
				return v.Validate()
			}
			return nil
		}))),
		validation.Field(&d.PrepullHosts, validation.Each(validation.By(func(value interface{}) error {
			if v, ok := value.(registryBashibleConfigHostsObject); ok {
				return v.Validate()
			}
			return nil
		}))),
	)
}

func (d *registryBashibleConfigHostsObject) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.Host, validation.Required),
		validation.Field(&d.CA, validation.Each(validation.Required)),
		validation.Field(&d.Mirrors, validation.Each(validation.By(func(value interface{}) error {
			if v, ok := value.(registryBashibleConfigMirrorHostObject); ok {
				return v.Validate()
			}
			return nil
		}))),
	)
}

func (d *registryBashibleConfigMirrorHostObject) Validate() error {
	return validation.ValidateStruct(d,
		validation.Field(&d.Host, validation.Required),
		validation.Field(&d.Scheme, validation.Required),
	)
}
