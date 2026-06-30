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

package hooks

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/utils/ptr"

	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		// Binding 0: PCC secret - read-only snapshot; no events (deletion is handled by dvp_cluster_configuration.go).
		{
			Name:       "provider_cluster_configuration",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-provider-cluster-configuration"},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterPCCSecret,
		},
		// Binding 1: ModuleConfig - read-only snapshot for State B value override.
		// ExecuteHookOnSynchronization=false: hook must not fire before the namespace exists (created by Helm).
		{
			Name:       "module_config",
			ApiVersion: moduleConfigAPIVersion,
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpModuleName},
			},
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   filterModuleConfig,
		},
	},
}, handleDVPMigrationResources)

func handleDVPMigrationResources(_ context.Context, input *go_hook.HookInput) error {
	pccSnaps := input.Snapshots.Get("provider_cluster_configuration")
	if len(pccSnaps) == 0 {
		// State A: no PCC - nothing to create; deletion is handled by dvp_cluster_configuration.go.
		return nil
	}

	var pccResult pccSecretFilterResult
	if err := pccSnaps[0].UnmarshalTo(&pccResult); err != nil {
		return fmt.Errorf("unmarshal PCC snapshot: %w", err)
	}

	var pcc v1.DvpProviderClusterConfiguration
	if len(pccResult.ProviderClusterConfig) > 0 {
		if err := convertJSONRawMessageToStruct(pccResult.ProviderClusterConfig, &pcc); err != nil {
			return fmt.Errorf("parse PCC: %w", err)
		}
	}

	// State B: PCC present, migration in progress - create artifacts in namespace (which now exists after Helm).
	// State C (migration complete) is detected and handled by dvp_cluster_configuration.go (OnBeforeHelm),
	// which fires on NodeGroup/DVPInstanceClass/ModuleConfig/Secret events and calls deleteMigrationArtifacts.
	// Running createProviderClusterConfigurationResources in State C is safe: CreateOrUpdate is idempotent
	// and dvp_cluster_configuration.go will delete the secret on the next (or concurrent) cycle.
	var moduleConfiguration v1.DvpModuleConfiguration
	if err := json.Unmarshal([]byte(input.Values.Get("cloudProviderDvp").String()), &moduleConfiguration); err != nil {
		return fmt.Errorf("parse module configuration: %w", err)
	}

	overrideProviderClusterConfigValues(&pcc, &moduleConfiguration)

	if err := validateProviderClusterConfig(pcc); err != nil {
		return fmt.Errorf("validate provider cluster config: %w", err)
	}

	if err := createProviderClusterConfigurationResources(input, &pcc); err != nil {
		return fmt.Errorf("create migration resources: %w", err)
	}

	createMigrationConfigMap(input)
	return nil
}
