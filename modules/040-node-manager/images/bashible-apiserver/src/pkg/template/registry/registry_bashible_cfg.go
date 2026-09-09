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

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/registry/models/bashible"
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

// toRegistryData turns the module's configuration into the node-side context.
//
// Delegated to Config.ToContext rather than copied field by field: this used to be a
// second, hand-written translation of the same structure, and a field added on one side
// silently never reached a node. The only thing that belongs here is the one fact
// ToContext cannot know — that this context came from the registry module at all.
func (c bashibleConfigSecret) toRegistryData() *RegistryData {
	ctx := bashible.Config(c).ToContext()
	ctx.RegistryModuleEnable = true

	ret := RegistryData(ctx)
	return &ret
}
