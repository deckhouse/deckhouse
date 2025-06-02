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
	"encoding/json"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	registry_const "github.com/deckhouse/deckhouse/go_lib/system-registry-manager/const"
)

type Config struct {
	Mode           registry_const.ModeType `json:"mode" yaml:"mode"`
	Version        string                  `json:"version" yaml:"version"`
	ImagesBase     string                  `json:"imagesBase" yaml:"imagesBase"`
	ProxyEndpoints []string                `json:"proxyEndpoints,omitempty" yaml:"proxyEndpoints,omitempty"`
	Hosts          map[string]Hosts        `json:"hosts" yaml:"hosts"`
}

type Hosts struct {
	CA      []string     `json:"ca,omitempty" yaml:"ca,omitempty"`
	Mirrors []MirrorHost `json:"mirrors" yaml:"mirrors"`
}

type MirrorHost struct {
	Host     string    `json:"host" yaml:"host"`
	Scheme   string    `json:"scheme" yaml:"scheme"`
	Auth     Auth      `json:"auth,omitempty" yaml:"auth,omitempty"`
	Rewrites []Rewrite `json:"rewrites,omitempty" yaml:"rewrites,omitempty"`
}

type Auth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
}

type Rewrite struct {
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

func (h *Hosts) Validate() error {
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

func (m *MirrorHost) Validate() error {
	return validation.ValidateStruct(m,
		validation.Field(&m.Host, validation.Required),
		validation.Field(&m.Scheme, validation.Required),
	)
}

func (m *MirrorHost) IsEqual(other MirrorHost) bool {
	if m.Host != other.Host || m.Scheme != other.Scheme {
		return false
	}
	if !m.Auth.IsEqual(other.Auth) {
		return false
	}
	if len(m.Rewrites) != len(other.Rewrites) {
		return false
	}
	for i := range m.Rewrites {
		if m.Rewrites[i] != other.Rewrites[i] {
			return false
		}
	}
	return true
}

func (a *Auth) IsEqual(b Auth) bool {
	return a.Username == b.Username &&
		a.Password == b.Password &&
		a.Auth == b.Auth
}

func ToMap(s interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(jsonData, &result)
	return result, err
}
