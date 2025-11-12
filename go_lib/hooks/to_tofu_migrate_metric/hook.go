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
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

const (
	secretsNs          = "d8-system"
	clusterStateSecret = "d8-cluster-terraform-state"
	metricName         = "d8_need_migrate_to_tofu"
	metricGroup        = "D8MigrateToTofu"
	terraformVersion   = "0.14.8"
)

type StateNodeResult struct {
	IsBackup         bool
	TerraformVersion string
	SecretName       string
}

type StateClusterResult struct {
	TerraformVersion string
	ClusterState     bool
	SecretName       string
}

type State struct {
	TerraformVersion string     `json:"terraform_version"`
	Resources        []Resource `json:"resources"`
}

type Resource struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
}

func RegisterHook() bool {
	return sdk.RegisterFunc(&go_hook.HookConfig{
		OnBeforeHelm: &go_hook.OrderedConfig{Order: 1},
		Kubernetes: []go_hook.KubernetesConfig{
			{
				Name:       "cluster_state",
				ApiVersion: "v1",
				Kind:       "Secret",
				NameSelector: &types.NameSelector{
					MatchNames: []string{clusterStateSecret},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{secretsNs},
					},
				},
				FilterFunc: clusterStateSecretFilter,
			},
			{
				Name:       "nodes_state",
				ApiVersion: "v1",
				Kind:       "Secret",
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"node.deckhouse.io/terraform-state": ""},
				},
				NamespaceSelector: &types.NamespaceSelector{
					NameSelector: &types.NameSelector{
						MatchNames: []string{secretsNs},
					},
				},
				FilterFunc: nodeStateSecretFilter,
			},
		},
	}, fireNeedMigrateToOpenTofuMetric)
}

func clusterStateSecretFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret

	err := sdk.FromUnstructured(unstructured, &secret)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	if _, ok := secret.Labels["dhctl.deckhouse.io/state-backup"]; ok {
		return &StateClusterResult{
			ClusterState: false,
			SecretName:   secret.Name,
		}, nil
	}

	stateRaw, ok := secret.Data["cluster-tf-state.json"]
	if !ok {
		// hack for tests,  because tests not supported label and name selector for some bindings object types
		return &StateClusterResult{
			ClusterState: false,
			SecretName:   secret.Name,
		}, nil
	}

	var state State

	err = json.Unmarshal(stateRaw, &state)
	if err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return &StateClusterResult{
		TerraformVersion: state.TerraformVersion,
		ClusterState:     true,
	}, nil
}

func nodeStateSecretFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret corev1.Secret

	err := sdk.FromUnstructured(unstructured, &secret)
	if err != nil {
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	if _, ok := secret.Labels["dhctl.deckhouse.io/state-backup"]; ok {
		return &StateNodeResult{
			IsBackup:   true,
			SecretName: secret.Name,
		}, nil
	}

	stateRaw, ok := secret.Data["node-tf-state.json"]
	if !ok {
		return nil, fmt.Errorf("node-tf-state.json not found in secret")
	}

	var state State

	err = json.Unmarshal(stateRaw, &state)
	if err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	return &StateNodeResult{
		IsBackup:         false,
		SecretName:       secret.Name,
		TerraformVersion: state.TerraformVersion,
	}, nil
}

func fireNeedMigrateToOpenTofuMetric(_ context.Context, input *go_hook.HookInput) error {
	clusterStates, err := sdkobjectpatch.UnmarshalToStruct[StateClusterResult](input.Snapshots, "cluster_state")
	if err != nil {
		return err
	}

	input.MetricsCollector.Expire(metricGroup)

	needMigrate := false

	if len(clusterStates) != 0 {
		for _, clusterState := range clusterStates {
			if !clusterState.ClusterState {
				input.Logger.Warn("Secret is not terraform state. Probably you located in test env", slog.String("secret_name", clusterState.SecretName))
				continue
			}

			if clusterState.TerraformVersion == terraformVersion {
				needMigrate = true
				input.Logger.Info("Cluster state has terraform state. Needing to migrate to tofu")
			}

			// cluster secret always one. hack for test envs see above
			break
		}
	} else {
		input.Logger.Info("Cluster state not found. Probably you have hybrid cluster")
	}

	nodeStates, err := sdkobjectpatch.UnmarshalToStruct[StateNodeResult](input.Snapshots, "nodes_state")
	if err != nil {
		return fmt.Errorf("cannot unmarshal nodes_state snapshot: %w", err)
	}

	if !needMigrate {
		for _, nodeState := range nodeStates {
			if nodeState.IsBackup {
				input.Logger.Info("Node state is backup state. Skip", slog.String("name", nodeState.SecretName))
				continue
			}

			if nodeState.TerraformVersion == terraformVersion {
				input.Logger.Info("Node state has terraform state. Needing to migrate to tofu", slog.String("name", nodeState.SecretName))
				needMigrate = true
				break
			}
		}
	}

	val := 0.0
	if needMigrate {
		val = 1.0
	}

	input.MetricsCollector.Set(metricName, val, nil, metrics.WithGroup(metricGroup))

	return nil
}
