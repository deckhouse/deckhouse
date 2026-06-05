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
		// Binding 0: PCC secret in kube-system
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
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterPCCSecret,
		},
		// Binding 1: ModuleConfig (read-only snapshot)
		{
			Name:       "module_config",
			ApiVersion: dvpModuleConfigAPIVersion,
			Kind:       "ModuleConfig",
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpModuleConfigName},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterModuleConfig,
		},
		// Binding 2: d8-credentials Secret (read-only snapshot)
		{
			Name:       "credential_secret_d8",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dvpNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpCredentialSecretName},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterCredentialSecret,
		},
		// Binding 3: NodeGroup CRs (read-only snapshot)
		{
			Name:                "node_groups",
			ApiVersion:          "deckhouse.io/v1",
			Kind:                "NodeGroup",
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterNamedResource,
		},
		// Binding 4: DVPInstanceClass CRs (read-only snapshot)
		{
			Name:                "dvp_instance_classes",
			ApiVersion:          dvpModuleConfigAPIVersion,
			Kind:                dvpInstanceClassKind,
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterNamedResource,
		},
		// Binding 5: d8-migration-resources Secret (read-only snapshot)
		{
			Name:       "migration_resources_secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dvpNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpMigrationResourcesName},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterNamedResource,
		},
		// Binding 6: d8-module-is-migrating ConfigMap (read-only snapshot)
		{
			Name:       "migration_configmap",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{dvpNamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{dvpMigrationConfigMapName},
			},
			ExecuteHookOnEvents: ptr.To(false),
			FilterFunc:          filterNamedResource,
		},
	},
}, handleDVPMigrationResources)

func handleDVPMigrationResources(_ context.Context, input *go_hook.HookInput) error {
	pccSnaps := input.Snapshots.Get("provider_cluster_configuration")
	if len(pccSnaps) == 0 {
		// State A: no PCC — clean up migration artifacts.
		deleteMigrationArtifacts(input)
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

	if isNewResourcesComplete(input, &pcc) {
		// State C: migration done — clean up artifacts.
		deleteMigrationArtifacts(input)
		return nil
	}

	// State B: PCC present, migration in progress — create artifacts in namespace (which now exists after Helm).
	var moduleConfiguration v1.DvpModuleConfiguration
	if err := json.Unmarshal([]byte(input.Values.Get("cloudProviderDvp").String()), &moduleConfiguration); err != nil {
		return fmt.Errorf("parse module configuration: %w", err)
	}
	if err := overrideValues(&pcc, &moduleConfiguration); err != nil {
		return fmt.Errorf("override values: %w", err)
	}

	if err := createProviderClusterConfigurationResources(input, &pcc); err != nil {
		return fmt.Errorf("create migration resources: %w", err)
	}

	createMigrationConfigMap(input)
	return nil
}
