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

package bashible

import (
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
)

type Config struct {
	Mode           string                 `json:"mode" yaml:"mode"`
	Version        string                 `json:"version" yaml:"version"`
	ImagesBase     string                 `json:"imagesBase" yaml:"imagesBase"`
	ProxyEndpoints []string               `json:"proxyEndpoints,omitempty" yaml:"proxyEndpoints,omitempty"`
	Hosts          map[string]ConfigHosts `json:"hosts" yaml:"hosts"`
}

type ConfigHosts struct {
	Mirrors []ConfigMirrorHost `json:"mirrors" yaml:"mirrors"`
}

type ConfigMirrorHost struct {
	Host     string          `json:"host" yaml:"host"`
	Scheme   string          `json:"scheme" yaml:"scheme"`
	CA       string          `json:"ca,omitempty" yaml:"ca,omitempty"`
	Auth     ConfigAuth      `json:"auth,omitempty" yaml:"auth,omitempty"`
	Rewrites []ConfigRewrite `json:"rewrites,omitempty" yaml:"rewrites,omitempty"`
}

type ConfigAuth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
}

type ConfigRewrite struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
}

func (c *Config) Validate() error {
	if err := validation.ValidateStruct(c,
		validation.Field(&c.Mode, validation.Required),
		validation.Field(&c.Version, validation.Required),
		validation.Field(&c.ImagesBase, validation.Required),
		validation.Field(&c.ProxyEndpoints, validation.Each(validation.Required)),
		validation.Field(&c.Hosts, validation.Required),
	); err != nil {
		return err
	}

	for name, host := range c.Hosts {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("hosts map contains empty key")
		}
		if err := host.Validate(); err != nil {
			return fmt.Errorf("hosts[%q] validation failed: %w", name, err)
		}
	}
	return nil
}

func (h *ConfigHosts) Validate() error {
	if err := validation.ValidateStruct(h,
		validation.Field(&h.Mirrors, validation.Required),
	); err != nil {
		return err
	}

	for i, mirror := range h.Mirrors {
		if err := mirror.Validate(); err != nil {
			return fmt.Errorf("mirror[%d] validation failed: %w", i, err)
		}
	}

	seen := make(map[string]struct{})
	for i, mirror := range h.Mirrors {
		key := mirror.UniqueKey()
		if _, ok := seen[key]; ok {
			return fmt.Errorf("mirror[%d] validation failed: has duplicate", i)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (m *ConfigMirrorHost) Validate() error {
	return validation.ValidateStruct(m,
		validation.Field(&m.Host, validation.Required),
		validation.Field(&m.Scheme, validation.Required),
	)
}

func (m *ConfigMirrorHost) UniqueKey() string {
	return m.Host + "|" + m.Scheme
}
