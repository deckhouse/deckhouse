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
	"slices"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

const (
	bashibleConfigSecretName = "registry-bashible-config"
)

type bashibleConfigSecret bashible.Config

func (c *bashibleConfigSecret) decode(secret *corev1.Secret) error {
	if err := yaml.Unmarshal(secret.Data["config"], c); err != nil {
		return fmt.Errorf("failed to parse registry bashible config: %w", err)
	}
	return nil
}

func (c *bashibleConfigSecret) validate() error {
	if c == nil {
		return fmt.Errorf("failed: is empty")
	}
	cfg := bashible.Config(*c)
	return cfg.Validate()
}

func (c bashibleConfigSecret) toRegistryData() *RegistryData {
	ret := &RegistryData{
		RegistryModuleEnable: true,
		Mode:                 c.Mode,
		Version:              c.Version,
		ImagesBase:           c.ImagesBase,
		ProxyEndpoints:       slices.Clone(c.ProxyEndpoints),
		Hosts:                make(map[string]bashible.ContextHosts, len(c.Hosts)),
	}

	for key, hosts := range c.Hosts {
		rh := bashible.ContextHosts{
			Mirrors: make([]bashible.ContextMirrorHost, 0, len(hosts.Mirrors)),
		}

		for _, m := range hosts.Mirrors {
			mh := bashible.ContextMirrorHost{
				Host:   m.Host,
				Scheme: m.Scheme,
				CA:     m.CA,
				Auth: bashible.ContextAuth{
					Username: m.Auth.Username,
					Password: m.Auth.Password,
					Auth:     m.Auth.Auth,
				},
			}
			for _, rw := range m.Rewrites {
				mh.Rewrites = append(mh.Rewrites, bashible.ContextRewrite(rw))
			}

			rh.Mirrors = append(rh.Mirrors, mh)
		}

		ret.Hosts[key] = rh
	}
	return ret
}
