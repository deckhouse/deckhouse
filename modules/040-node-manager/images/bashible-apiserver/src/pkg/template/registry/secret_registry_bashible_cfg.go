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
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

const (
	RegistryBashibleConfigSecretName = "registry-bashible-config"
)

type registryBashibleConfig struct {
	Version        string                `json:"version" yaml:"version"`
	ProxyEndpoints []string              `json:"proxyEndpoints" yaml:"proxyEndpoints"`
	Hosts          []RegistryHostsObject `json:"hosts" yaml:"hosts"`
	PrepullHosts   []RegistryHostsObject `json:"prepullHosts" yaml:"prepullHosts"`
}

func (d *registryBashibleConfig) DecodeSecret(secret *corev1.Secret) error {
	if err := json.Unmarshal(secret.Data["version"], &d.Version); err != nil {
		return fmt.Errorf("failed to parse version: %w", err)
	}
	if err := json.Unmarshal(secret.Data["proxyEndpoints"], &d.ProxyEndpoints); err != nil {
		return fmt.Errorf("failed to parse proxyEndpoints: %w", err)
	}
	if err := json.Unmarshal(secret.Data["hosts"], &d.Hosts); err != nil {
		return fmt.Errorf("failed to parse hosts: %w", err)
	}
	if err := json.Unmarshal(secret.Data["prepullHosts"], &d.PrepullHosts); err != nil {
		return fmt.Errorf("failed to parse prepullHosts: %w", err)
	}
	return nil
}

func (d *registryBashibleConfig) Validate() error {
	return nil
}
