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

type Context struct {
	RegistryModuleEnable bool                    `json:"registryModuleEnable" yaml:"registryModuleEnable"`
	Mode                 string                  `json:"mode" yaml:"mode"`
	Version              string                  `json:"version" yaml:"version"`
	ImagesBase           string                  `json:"imagesBase" yaml:"imagesBase"`
	ProxyEndpoints       []string                `json:"proxyEndpoints,omitempty" yaml:"proxyEndpoints,omitempty"`
	Hosts                map[string]ContextHosts `json:"hosts" yaml:"hosts"`
}

type ContextHosts struct {
	Mirrors []ContextMirrorHost `json:"mirrors" yaml:"mirrors"`
}

type ContextMirrorHost struct {
	Host     string           `json:"host" yaml:"host"`
	Scheme   string           `json:"scheme" yaml:"scheme"`
	CA       string           `json:"ca,omitempty" yaml:"ca,omitempty"`
	Auth     ContextAuth      `json:"auth,omitempty" yaml:"auth,omitempty"`
	Rewrites []ContextRewrite `json:"rewrites,omitempty" yaml:"rewrites,omitempty"`
}

type ContextAuth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
}

type ContextRewrite struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
}

func (b *Context) Validate() error {
	if err := validation.ValidateStruct(b,
		validation.Field(&b.Mode, validation.Required),
		validation.Field(&b.Version, validation.Required),
		validation.Field(&b.ImagesBase, validation.Required),
		validation.Field(&b.ProxyEndpoints, validation.Each(validation.Required)),
		validation.Field(&b.Hosts, validation.Required),
	); err != nil {
		return err
	}

	for name, host := range b.Hosts {
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("hosts map contains empty key")
		}
		if err := host.Validate(); err != nil {
			return fmt.Errorf("hosts[%q] validation failed: %w", name, err)
		}
	}
	return nil
}

func (h *ContextHosts) Validate() error {
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

func (m *ContextMirrorHost) Validate() error {
	return validation.ValidateStruct(m,
		validation.Field(&m.Host, validation.Required),
		validation.Field(&m.Scheme, validation.Required),
	)
}

func (m *ContextMirrorHost) UniqueKey() string {
	return m.Host + "|" + m.Scheme
}

func (c Context) ToMap() (map[string]interface{}, error) {
	proxies := make([]interface{}, 0, len(c.ProxyEndpoints))
	for _, ep := range c.ProxyEndpoints {
		proxies = append(proxies, ep)
	}

	hosts := make(map[string]interface{}, len(c.Hosts))
	for hostName, host := range c.Hosts {
		mirrors := make([]interface{}, 0, len(host.Mirrors))
		for _, mirror := range host.Mirrors {
			auth := map[string]interface{}{
				"username": mirror.Auth.Username,
				"password": mirror.Auth.Password,
				"auth":     mirror.Auth.Auth,
			}

			rewrites := make([]interface{}, 0, len(mirror.Rewrites))
			for _, rw := range mirror.Rewrites {
				rewrites = append(rewrites, map[string]interface{}{
					"from": rw.From,
					"to":   rw.To,
				})
			}

			mirrors = append(mirrors, map[string]interface{}{
				"host":     mirror.Host,
				"scheme":   mirror.Scheme,
				"ca":       mirror.CA,
				"auth":     auth,
				"rewrites": rewrites,
			})
		}
		hosts[hostName] = map[string]interface{}{
			"mirrors": mirrors,
		}
	}

	ret := map[string]interface{}{
		"registryModuleEnable": c.RegistryModuleEnable,
		"mode":                 c.Mode,
		"version":              c.Version,
		"imagesBase":           c.ImagesBase,
		"proxyEndpoints":       proxies,
		"hosts":                hosts,
	}
	return ret, nil
}
