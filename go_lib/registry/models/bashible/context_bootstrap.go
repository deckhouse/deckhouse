/*
Copyright 2026 Flant JSC

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
	validation "github.com/go-ozzo/ozzo-validation/v4"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/initsecret"
)

var (
	_ validation.Validatable = ContextBootstrap{}
	_ validation.Validatable = ContextBootstrapProxy{}
)

type ContextBootstrapProxy struct {
	Host     string `json:"host" yaml:"host"`
	Path     string `json:"path" yaml:"path"`
	Scheme   string `json:"scheme" yaml:"scheme"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	CA       string `json:"ca,omitempty" yaml:"ca,omitempty"`
	TTL      string `json:"ttl,omitempty" yaml:"ttl,omitempty"`
}

func (c ContextBootstrapProxy) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Host, validation.Required),
		validation.Field(&c.Path, validation.Required),
		validation.Field(&c.Scheme, validation.Required),
		validation.Field(&c.Username, validation.When(c.Password != "", validation.Required)),
		validation.Field(&c.Password, validation.When(c.Username != "", validation.Required)),
	)
}

func (c ContextBootstrapProxy) ToMap() map[string]any {
	m := make(map[string]any)

	m["host"] = c.Host
	m["path"] = c.Path
	m["scheme"] = c.Scheme

	if c.CA != "" {
		m["ca"] = c.CA
	}
	if c.Username != "" {
		m["username"] = c.Username
	}
	if c.Password != "" {
		m["password"] = c.Password
	}
	if c.TTL != "" {
		m["ttl"] = c.TTL
	}

	return m
}

type ContextBootstrap struct {
	Init  initsecret.Config      `json:"init" yaml:"init"`
	Proxy *ContextBootstrapProxy `json:"proxy,omitempty" yaml:"proxy,omitempty"`
}

func (c ContextBootstrap) Validate() error {
	return validation.ValidateStruct(&c,
		validation.Field(&c.Init, validation.Required),
		validation.Field(&c.Proxy),
	)
}

func (c ContextBootstrap) ToMap() map[string]any {
	m := make(map[string]any)

	m["init"] = c.Init.ToMap()

	if c.Proxy != nil {
		m["proxy"] = c.Proxy.ToMap()
	}

	return m
}
