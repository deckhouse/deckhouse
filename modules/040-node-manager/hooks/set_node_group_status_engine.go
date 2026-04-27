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

package hooks

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

const nodeGroupEngineMigrationConfigMap = "d8-node-group-engine-migration"

type nodeGroupEngineMigrationInfo struct {
	Name   string
	Spec   ngv1.NodeGroupSpec
	Engine ngv1.NodeGroupEngine
}

func applyNodeGroupEngineMigrationFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var nodeGroup ngv1.NodeGroup
	if err := sdk.FromUnstructured(obj, &nodeGroup); err != nil {
		return nil, err
	}

	return nodeGroupEngineMigrationInfo{
		Name:   nodeGroup.GetName(),
		Spec:   nodeGroup.Spec,
		Engine: nodeGroup.Status.Engine,
	}, nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue:        "/modules/node-manager",
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 2},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_groups",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: applyNodeGroupEngineMigrationFilter,
		},
		{
			Name:       "migration_config_map",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{MatchNames: []string{"kube-system"}},
			},
			NameSelector: &types.NameSelector{MatchNames: []string{nodeGroupEngineMigrationConfigMap}},
			FilterFunc:   nameFilter,
			// The marker is only used as a one-shot migration guard. Deleting it should not
			// trigger this hook to recalculate engines for already existing NodeGroups.
			ExecuteHookOnEvents: ptr.To(false),
		},
	},
}, setNodeGroupStatusEngine)

func setNodeGroupStatusEngine(_ context.Context, input *go_hook.HookInput) error {
	if len(input.Snapshots.Get("migration_config_map")) > 0 {
		input.Logger.Debug("NodeGroup engine migration marker already exists")
		return nil
	}

	defaultEngine := defaultCloudEphemeralNodeGroupEngine(input)

	for nodeGroup, err := range sdkobjectpatch.SnapshotIter[nodeGroupEngineMigrationInfo](input.Snapshots.Get("node_groups")) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'node_groups' snapshots: %w", err)
		}

		if nodeGroup.Engine != "" {
			input.Logger.Debug("NodeGroup engine is already set", slog.String("node_group", nodeGroup.Name), slog.String("engine", string(nodeGroup.Engine)))
			continue
		}

		engine := calculateMigratedNodeGroupEngine(nodeGroup.Spec, defaultEngine)
		input.Logger.Info("Set NodeGroup engine", slog.String("node_group", nodeGroup.Name), slog.String("engine", string(engine)))
		setNodeGroupStatus(input.PatchCollector, nodeGroup.Name, "engine", engine)
	}

	input.PatchCollector.CreateIfNotExists(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeGroupEngineMigrationConfigMap,
			Namespace: "kube-system",
		},
	})

	return nil
}

func defaultCloudEphemeralNodeGroupEngine(input *go_hook.HookInput) ngv1.NodeGroupEngine {
	hasMCM := valueExistsAndNotEmpty(input, "nodeManager.internal.cloudProvider.machineClassKind")
	hasCAPI := valueExistsAndNotEmpty(input, "nodeManager.internal.cloudProvider.capiClusterKind")

	switch {
	case hasMCM:
		return ngv1.NodeGroupEngineMCM
	case hasCAPI:
		return ngv1.NodeGroupEngineCAPI
	default:
		return ngv1.NodeGroupEngineNone
	}
}

func calculateMigratedNodeGroupEngine(spec ngv1.NodeGroupSpec, defaultCloudEphemeralEngine ngv1.NodeGroupEngine) ngv1.NodeGroupEngine {
	switch spec.NodeType {
	case ngv1.NodeTypeCloudEphemeral:
		return defaultCloudEphemeralEngine
	case ngv1.NodeTypeStatic:
		if spec.StaticInstances != nil {
			return ngv1.NodeGroupEngineCAPI
		}
		return ngv1.NodeGroupEngineNone
	default:
		return ngv1.NodeGroupEngineNone
	}
}

func valueExistsAndNotEmpty(input *go_hook.HookInput, path string) bool {
	value := input.Values.Get(path)
	return value.Exists() && value.String() != ""
}
