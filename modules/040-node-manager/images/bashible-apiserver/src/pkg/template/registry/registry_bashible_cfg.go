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
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	bashibleConfigSecretName = "registry-bashible-config"
)

type bashibleConfigSecret struct {
	Mode           string                         `json:"mode" yaml:"mode"`
	Version        string                         `json:"version" yaml:"version"`
	ImagesBase     string                         `json:"imagesBase" yaml:"imagesBase"`
	ProxyEndpoints []string                       `json:"proxyEndpoints,omitempty" yaml:"proxyEndpoints,omitempty"`
	Hosts          map[string]bashibleConfigHosts `json:"hosts" yaml:"hosts"`
}

type bashibleConfigHosts struct {
	Mirrors []bashibleConfigMirrorHost `json:"mirrors" yaml:"mirrors"`
}

type bashibleConfigMirrorHost struct {
	Host     string                  `json:"host" yaml:"host"`
	Scheme   string                  `json:"scheme" yaml:"scheme"`
	CA       string                  `json:"ca,omitempty" yaml:"ca,omitempty"`
	Auth     bashibleConfigAuth      `json:"auth,omitempty" yaml:"auth,omitempty"`
	Rewrites []bashibleConfigRewrite `json:"rewrites,omitempty" yaml:"rewrites,omitempty"`
}

type bashibleConfigAuth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	Auth     string `json:"auth" yaml:"auth"`
}

type bashibleConfigRewrite struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to" yaml:"to"`
}

func (c *bashibleConfigSecret) decode(secret *corev1.Secret) error {
	if err := yaml.Unmarshal(secret.Data["config"], c); err != nil {
		return fmt.Errorf("failed to parse registry bashible config: %w", err)
	}
	return nil
}

func (c *bashibleConfigSecret) Validate() error {
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

func (h *bashibleConfigHosts) Validate() error {
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
	return nil
}

func (m *bashibleConfigMirrorHost) Validate() error {
	return validation.ValidateStruct(m,
		validation.Field(&m.Host, validation.Required),
		validation.Field(&m.Scheme, validation.Required),
	)
}

func (c bashibleConfigSecret) toRegistryData() *RegistryData {
	ret := &RegistryData{
		Mode:           c.Mode,
		Version:        c.Version,
		ImagesBase:     c.ImagesBase,
		ProxyEndpoints: append([]string(nil), c.ProxyEndpoints...),
		Hosts:          make(map[string]registryHosts, len(c.Hosts)),
	}

	for key, hosts := range c.Hosts {
		ret.Hosts[key] = hosts.toRegistryHosts()
	}
	return ret
}

func (h bashibleConfigHosts) toRegistryHosts() registryHosts {
	ret := registryHosts{
		Mirrors: make([]registryMirrorHost, 0, len(h.Mirrors)),
	}
	for _, m := range h.Mirrors {
		ret.Mirrors = append(ret.Mirrors, m.toRegistryMirrorHost())
	}
	return ret
}

func (m bashibleConfigMirrorHost) toRegistryMirrorHost() registryMirrorHost {
	ret := registryMirrorHost{
		Host:   m.Host,
		Scheme: m.Scheme,
		CA:     m.CA,
		Auth: registryAuth{
			Username: m.Auth.Username,
			Password: m.Auth.Password,
			Auth:     m.Auth.Auth,
		},
	}

	for _, rw := range m.Rewrites {
		ret.Rewrites = append(ret.Rewrites, registryRewrite(rw))
	}
	return ret
}
