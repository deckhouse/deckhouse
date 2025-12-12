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

	validation "github.com/go-ozzo/ozzo-validation/v4"

	init_secret "github.com/deckhouse/deckhouse/go_lib/registry/models/init-secret"
)

var (
	_ validation.Validatable = Context{}
	_ validation.Validatable = ContextHosts{}
	_ validation.Validatable = ContextMirrorHost{}
)

type Context struct {
	Init                 init_secret.Config      `json:"init,omitempty" yaml:"init,omitempty"`
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

func (c Context) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Mode, validation.Required),
		validation.Field(&c.Version, validation.Required),
		validation.Field(&c.ImagesBase, validation.Required),
		validation.Field(&c.ProxyEndpoints, validation.Each(validation.Required)),
		// Hosts key must not be empty
		validation.Field(&c.Hosts, validation.Required),
		// Validate each host
		validation.Field(&c.Hosts, validation.Each(validation.Required)),
	)
}

func (h ContextHosts) Validate() error {
	if err := validation.ValidateStruct(&h,
		// Mirrors must not be empty
		validation.Field(&h.Mirrors, validation.Required),
		// Validate each mirror
		validation.Field(&h.Mirrors, validation.Each(validation.Required)),
	); err != nil {
		return err
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

func (m ContextMirrorHost) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.Host, validation.Required),
		validation.Field(&m.Scheme, validation.Required),
	)
}

func (m ContextMirrorHost) UniqueKey() string {
	return m.Host + "|" + m.Scheme
}

func (c Context) ToMap() map[string]any {
	proxies := make([]any, 0, len(c.ProxyEndpoints))
	for _, ep := range c.ProxyEndpoints {
		proxies = append(proxies, ep)
	}

	hosts := make(map[string]any, len(c.Hosts))
	for hostName, host := range c.Hosts {
		mirrors := make([]any, 0, len(host.Mirrors))
		for _, mirror := range host.Mirrors {
			auth := map[string]any{
				"username": mirror.Auth.Username,
				"password": mirror.Auth.Password,
				"auth":     mirror.Auth.Auth,
			}

			rewrites := make([]any, 0, len(mirror.Rewrites))
			for _, rw := range mirror.Rewrites {
				rewrites = append(rewrites, map[string]any{
					"from": rw.From,
					"to":   rw.To,
				})
			}

			mirrors = append(mirrors, map[string]any{
				"host":     mirror.Host,
				"scheme":   mirror.Scheme,
				"ca":       mirror.CA,
				"auth":     auth,
				"rewrites": rewrites,
			})
		}
		hosts[hostName] = map[string]any{
			"mirrors": mirrors,
		}
	}

	ret := map[string]any{
		"registryModuleEnable": c.RegistryModuleEnable,
		"mode":                 c.Mode,
		"version":              c.Version,
		"imagesBase":           c.ImagesBase,
		"proxyEndpoints":       proxies,
		"hosts":                hosts,
	}

	init := c.Init.ToMap()
	if len(init) > 0 {
		ret["init"] = init
	}
	return ret
}
