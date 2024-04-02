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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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

func handleClusterAPIDeploymentRequired(input *go_hook.HookInput) error {
	var hasStaticInstancesField bool

	nodeGroupSnapshots := input.Snapshots["node_group"]
	for _, nodeGroupSnapshot := range nodeGroupSnapshots {
		hasStaticInstancesField = nodeGroupSnapshot.(bool)
		if hasStaticInstancesField {
			break // we need at least one NodeGroup with staticInstances field
		}
	}

	capiClusterName := input.Values.Get("nodeManager.internal.cloudProvider.capiClusterName").String()
	hasCapiProvider := capiClusterName != ""

	var capiEnabled bool
	var capsEnabled bool

	configMapSnapshots := input.Snapshots["config_map"]
	if len(configMapSnapshots) > 0 {
		capiEnabled = hasCapiProvider || configMapSnapshots[0].(bool)
		capsEnabled = configMapSnapshots[0].(bool)
	} else {
		capiEnabled = hasCapiProvider || hasStaticInstancesField
	}

	if capiEnabled {
		input.Values.Set("nodeManager.internal.capiControllerManagerEnabled", true)
	} else {
		input.Values.Remove("nodeManager.internal.capiControllerManagerEnabled")
	}

	if capsEnabled || hasStaticInstancesField {
		input.Values.Set("nodeManager.internal.capsControllerManagerEnabled", true)
	} else {
		input.Values.Remove("nodeManager.internal.capsControllerManagerEnabled")
	}

	return nil
}
