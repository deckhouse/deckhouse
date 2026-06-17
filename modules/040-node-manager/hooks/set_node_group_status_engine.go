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
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
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
	OnStartup: &go_hook.OrderedConfig{Order: 2},
}, dependency.WithExternalDependencies(setNodeGroupStatusEngine))

func setNodeGroupStatusEngine(ctx context.Context, input *go_hook.HookInput, dc dependency.Container) error {
	kubeClient, err := dc.GetK8sClient()
	if err != nil {
		return fmt.Errorf("cannot init Kubernetes client: %w", err)
	}

	_, err = kubeClient.CoreV1().ConfigMaps("kube-system").Get(ctx, nodeGroupEngineMigrationConfigMap, metav1.GetOptions{})
	if err == nil {
		input.Logger.Debug("NodeGroup engine migration marker already exists")
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("cannot get migration marker ConfigMap: %w", err)
	}

	defaultEngine, err := defaultCloudEphemeralNodeGroupEngine(kubeClient.CoreV1().Secrets("kube-system").Get(ctx, "d8-node-manager-cloud-provider", metav1.GetOptions{}))
	if err != nil {
		return fmt.Errorf("cannot determine default NodeGroup engine from registration secret: %w", err)
	}
	nodeGroupGVR := schema.GroupVersionResource{
		Group:    "deckhouse.io",
		Version:  "v1",
		Resource: "nodegroups",
	}

	nodeGroups, err := kubeClient.Dynamic().Resource(nodeGroupGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("cannot list NodeGroups: %w", err)
	}

	for _, item := range nodeGroups.Items {
		nodeGroup, err := applyNodeGroupEngineMigrationFilter(&item)
		if err != nil {
			return fmt.Errorf("cannot decode NodeGroup %q: %w", item.GetName(), err)
		}
		info := nodeGroup.(nodeGroupEngineMigrationInfo)

		if info.Engine != "" {
			input.Logger.Debug("NodeGroup engine is already set", slog.String("node_group", info.Name), slog.String("engine", string(info.Engine)))
			continue
		}

		engine := calculateMigratedNodeGroupEngine(info.Spec, defaultEngine)
		input.Logger.Info("Set NodeGroup engine", slog.String("node_group", info.Name), slog.String("engine", string(engine)))

		patchData, err := json.Marshal(map[string]any{
			"status": map[string]any{
				"engine": engine,
			},
		})
		if err != nil {
			return fmt.Errorf("cannot marshal NodeGroup %q status patch: %w", info.Name, err)
		}

		_, err = kubeClient.Dynamic().Resource(nodeGroupGVR).Patch(ctx, info.Name, types.MergePatchType, patchData, metav1.PatchOptions{}, "status")
		if err != nil {
			return fmt.Errorf("cannot patch NodeGroup %q status.engine: %w", info.Name, err)
		}
	}

	_, err = kubeClient.CoreV1().ConfigMaps("kube-system").Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nodeGroupEngineMigrationConfigMap,
			Namespace: "kube-system",
		},
	}, metav1.CreateOptions{})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("cannot create migration marker ConfigMap: %w", err)
	}

	return nil
}

func defaultCloudEphemeralNodeGroupEngine(secret *corev1.Secret, err error) (ngv1.NodeGroupEngine, error) {
	if apierrors.IsNotFound(err) {
		return ngv1.NodeGroupEngineNone, nil
	}
	if err != nil {
		return "", err
	}

	hasMCM := false
	if value, ok := secret.Data["machineClassKind"]; ok && len(value) > 0 {
		hasMCM = true
	}
	hasCAPI := false
	if value, ok := secret.Data["capiClusterKind"]; ok && len(value) > 0 {
		hasCAPI = true
	}

	switch {
	case hasMCM:
		return ngv1.NodeGroupEngineMCM, nil
	case hasCAPI:
		return ngv1.NodeGroupEngineCAPI, nil
	default:
		return ngv1.NodeGroupEngineNone, nil
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
