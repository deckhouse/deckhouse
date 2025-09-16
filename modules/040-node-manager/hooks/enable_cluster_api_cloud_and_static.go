/*
Copyright 2023 Flant JSC

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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

type hookParam struct {
	serviceAccount string
	cluster        string
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 10},
	Queue:        "/modules/node-manager",
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "capi_static_kubeconfig_secret",
			Crontab: "0 1 * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "node_group",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: staticInstancesNodeGroupFilter,
		},
		{
			Name:       "config_map",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{clusterAPINamespace},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"capi-controller-manager"},
			},
			FilterFunc: capsConfigMapFilter,
		},
	},
}, handleClusterAPIDeploymentRequired)

func staticInstancesNodeGroupFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	return ng.Spec.StaticInstances != nil, nil
}

func capsConfigMapFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var configMap corev1.ConfigMap

	err := sdk.FromUnstructured(obj, &configMap)
	if err != nil {
		return nil, err
	}

	enable, ok := configMap.Data["enable"]
	if !ok {
		return nil, nil
	}

	return enable == "true", nil
}

func handleClusterAPIDeploymentRequired(_ context.Context, input *go_hook.HookInput) error {
	input.Logger.Info("Starting hook that set flags for rendering CAPI and CAPS managers")
	defer input.Logger.Info("Finish hook that set flags for rendering CAPI and CAPS managers")

	capiControllerManagerEnabledBeforeExecuting := input.Values.Get("nodeManager.internal.capiControllerManagerEnabled")
	capsControllerManagerEnabledBeforeExecuting := input.Values.Get("nodeManager.internal.capsControllerManagerEnabled")

	input.Logger.Info("Flags before executing.",
		"capiControllerManagerEnabled_exists",
		capiControllerManagerEnabledBeforeExecuting.Exists(),
		"capiControllerManagerEnabled",
		capiControllerManagerEnabledBeforeExecuting.Bool(),
		"capsControllerManagerEnabled_exists",
		capsControllerManagerEnabledBeforeExecuting.Exists(),
		"capsControllerManagerEnabled",
		capsControllerManagerEnabledBeforeExecuting.Bool(),
	)

	var hasStaticInstancesField bool

	nodeGroupSnapshots := input.Snapshots.Get("node_group")
	for hasStaticInstancesFieldSnapshot, err := range sdkobjectpatch.SnapshotIter[bool](nodeGroupSnapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'node_group' snapshots: %w", err)
		}

		hasStaticInstancesField = hasStaticInstancesFieldSnapshot
		if hasStaticInstancesField {
			input.Logger.Info("Found staticInstances field in node group")
			break // we need at least one NodeGroup with staticInstances field
		}
	}

	input.Logger.Info("hasStaticInstancesField", "value", hasStaticInstancesField)

	capiClusterName := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterName").String()
	hasCapiProvider := capiClusterName != ""

	input.Logger.Info("capiClusterName discovered", "capiClusterName", capiClusterName, "hasCapiProvider", hasCapiProvider)

	var capiEnabled bool
	var capsEnabled bool

	configMapSnapshots := input.Snapshots.Get("config_map")

	if len(configMapSnapshots) > 0 {
		var capsFromStartSnap bool
		err := configMapSnapshots[0].UnmarshalTo(&capsFromStartSnap)
		if err != nil {
			return fmt.Errorf("failed to unmarshal start 'config_map' snapshot: %w", err)
		}
		input.Logger.Info("Found ConfigMap d8-cloud-instance-manager/capi-controller-manager that indicated that CAPI should deployed", "enabled", capsFromStartSnap)

		capiEnabled = hasCapiProvider || capsFromStartSnap
		capsEnabled = capsFromStartSnap

		input.Logger.Info("Calculated flags", "capiEnabled", capiEnabled, "capsEnabled", capsEnabled)
	} else {
		input.Logger.Info("ConfigMap d8-cloud-instance-manager/capi-controller-manager that indicated that CAPI should deployed not found")

		capiEnabled = hasCapiProvider || hasStaticInstancesField

		input.Logger.Info("Calculated flags (capsEnabled not set)", "capiEnabled", capiEnabled)
	}

	if capiEnabled {
		input.Logger.Info("nodeManager.internal.capiControllerManagerEnabled set to true")

		input.Values.Set("nodeManager.internal.capiControllerManagerEnabled", true)
	} else {
		input.Logger.Info("nodeManager.internal.capiControllerManagerEnabled removed from values")

		input.Values.Remove("nodeManager.internal.capiControllerManagerEnabled")
	}

	if capsEnabled || hasStaticInstancesField {
		input.Logger.Info("nodeManager.internal.capsControllerManagerEnabled set to true")

		input.Values.Set("nodeManager.internal.capsControllerManagerEnabled", true)
	} else {
		input.Logger.Info("nodeManager.internal.capsControllerManagerEnable removed from values")

		input.Values.Remove("nodeManager.internal.capsControllerManagerEnabled")
	}

	capiControllerManagerEnabledAfterExecuting := input.Values.Get("nodeManager.internal.capiControllerManagerEnabled")
	capsControllerManagerEnabledAfterExecuting := input.Values.Get("nodeManager.internal.capsControllerManagerEnabled")

	input.Logger.Info("Flags after executing",
		"capiControllerManagerEnabled_exists",
		capiControllerManagerEnabledAfterExecuting.Exists(),
		"capiControllerManagerEnabled",
		capiControllerManagerEnabledAfterExecuting.Bool(),
		"capsControllerManagerEnabled_exists",
		capsControllerManagerEnabledAfterExecuting.Exists(),
		"capsControllerManagerEnabled",
		capsControllerManagerEnabledAfterExecuting.Bool(),
	)

	return nil
}
