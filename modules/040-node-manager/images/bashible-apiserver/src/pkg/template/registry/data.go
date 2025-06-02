/*
Copyright 2025 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package registry

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
)

type RegistryData struct {
	Mode           string                   `json:"mode" yaml:"mode"`
	Version        string                   `json:"version" yaml:"version"`
	ImagesBase     string                   `json:"imagesBase" yaml:"imagesBase"`
	ProxyEndpoints []string                 `json:"proxyEndpoints,omitempty" yaml:"proxyEndpoints,omitempty"`
	Hosts          map[string]registryHosts `json:"hosts" yaml:"hosts"`
}

type registryHosts struct {
	CA      []string             `json:"ca,omitempty" yaml:"ca,omitempty"`
	Mirrors []registryMirrorHost `json:"mirrors" yaml:"mirrors"`
}

type registryMirrorHost struct {
	Host     string            `json:"host" yaml:"host"`
	Scheme   string            `json:"scheme" yaml:"scheme"`
	Auth     registryAuth      `json:"auth,omitempty" yaml:"auth,omitempty"`
	Rewrites []registryRewrite `json:"rewrites,omitempty" yaml:"rewrites,omitempty"`
}

type registryAuth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
}

type registryRewrite struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
}

func (rd *RegistryData) loadFromInput(deckhouseRegistrySecret deckhouseRegistrySecret, bashibleCfgSecret *bashibleConfigSecret) error {
	if bashibleCfgSecret != nil {
		rData := bashibleCfgSecret.toRegistryData()
		*rd = *rData
		return nil
	}

	rData, err := deckhouseRegistrySecret.toRegistryData()
	if err != nil {
		return err
	}
	*rd = *rData
	return nil
}

func (rd *RegistryData) hashSum() (string, error) {
	data, err := json.Marshal(rd)
	if err != nil {
		return "", fmt.Errorf("error marshalling data: %w", err)
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), nil
}

func (rd *RegistryData) Validate() error {
	if err := validation.ValidateStruct(rd,
		validation.Field(&rd.Mode, validation.Required),
		validation.Field(&rd.Version, validation.Required),
		validation.Field(&rd.ImagesBase, validation.Required),
		validation.Field(&rd.ProxyEndpoints, validation.Each(validation.Required)),
		validation.Field(&rd.Hosts, validation.Required),
	); err != nil {
		return err
	}

	for name, host := range rd.Hosts {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("hosts map contains empty key")
		}
		if err := host.Validate(); err != nil {
			return fmt.Errorf("hosts[%q] validation failed: %w", name, err)
		}
	}
	return nil
}

func (h *registryHosts) Validate() error {
	if err := validation.ValidateStruct(h,
		validation.Field(&h.CA, validation.Each(validation.Required)),
		validation.Field(&h.Mirrors, validation.Required),
	); err != nil {
		return err
	}

	for i, mirror := range h.Mirrors {
		if err := mirror.Validate(); err != nil {
			return fmt.Errorf("mirror[%d] validation failed: %w", i, err)
		}
	}
	return nil
}

func (m *registryMirrorHost) Validate() error {
	return validation.ValidateStruct(m,
		validation.Field(&m.Host, validation.Required),
		validation.Field(&m.Scheme, validation.Required),
	)
}
