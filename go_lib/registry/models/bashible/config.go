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
	"slices"
)

var (
	_ validation.Validatable = Config{}
	_ validation.Validatable = ConfigHosts{}
	_ validation.Validatable = ConfigMirrorHost{}
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

func (c Config) Validate() error {
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

func (h ConfigHosts) Validate() error {
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

func (m ConfigMirrorHost) Validate() error {
	return validation.ValidateStruct(&m,
		validation.Field(&m.Host, validation.Required),
		validation.Field(&m.Scheme, validation.Required),
	)
}

func (m ConfigMirrorHost) UniqueKey() string {
	return m.Host + "|" + m.Scheme
}

func (c Config) ToContext() Context {
	ret := Context{
		Mode:           c.Mode,
		Version:        c.Version,
		ImagesBase:     c.ImagesBase,
		ProxyEndpoints: slices.Clone(c.ProxyEndpoints),
		Hosts:          make(map[string]ContextHosts, len(c.Hosts)),
	}

	for key, hosts := range c.Hosts {
		rh := ContextHosts{
			Mirrors: make([]ContextMirrorHost, 0, len(hosts.Mirrors)),
		}
		for _, m := range hosts.Mirrors {
			mh := ContextMirrorHost{
				Host:   m.Host,
				Scheme: m.Scheme,
				CA:     m.CA,
				Auth: ContextAuth{
					Username: m.Auth.Username,
					Password: m.Auth.Password,
					Auth:     m.Auth.Auth,
				},
			}
			for _, rw := range m.Rewrites {
				mh.Rewrites = append(mh.Rewrites, ContextRewrite(rw))
			}
			rh.Mirrors = append(rh.Mirrors, mh)
		}
		ret.Hosts[key] = rh
	}
	return ret
}
