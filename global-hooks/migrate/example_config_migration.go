// Copyright 2021 Flant JSC
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

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 10},
}, dependency.WithExternalDependencies(flantIntegrationKubeallTeamRemovalMigration))

// THIS IS AN EXAMPLE MIGRATION, NOT USEFUL BY ITSELF
func flantIntegrationKubeallTeamRemovalMigration(input *go_hook.HookInput, dc dependency.Container) error {
	// Setup
	configMigrator, err := newModuleConfigMigrator(dc, input)
	if err != nil {
		return err
	}
	const cmKey = "flantIntegration"

	// Get config
	config, err := configMigrator.getConfig(cmKey)
	if config == nil || err != nil {
		return err
	}

	// Migrate
	if config["kubeall"] == nil {
		return nil
	}
	kubeall := config["kubeall"].(map[string]interface{})
	delete(kubeall, "team")
	config["kubeall"] = kubeall

	// Save config
	return configMigrator.setConfig(cmKey, config)
}
