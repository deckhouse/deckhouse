/*
Copyright 2021 Flant JSC

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
	"math"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "secret",
			ApiVersion: "v1",
			Kind:       "Secret",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-control-plane-manager-control-plane-arguments"},
			},
			FilterFunc: updateFreqFilter,
		},
	},
}, handleUpdateFreq)

func handleUpdateFreq(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("secret")

	if len(snaps) == 0 {
		input.Values.Remove("nodeManager.internal.nodeStatusUpdateFrequency")
		input.Values.Remove("nodeManager.internal.allowedKubeletFeatureGates")
		return nil
	}

	var secretData controlPlaneArgumentsSecret
	err := snaps[0].UnmarshalTo(&secretData)
	if err != nil {
		return fmt.Errorf("failed to unmarshal 'secret' snapshots: %w", err)
	}

	if secretData.Arguments.NodeMonitorGracePeriodSeconds == 0 {
		input.Values.Remove("nodeManager.internal.nodeStatusUpdateFrequency")
	} else {
		freq := math.Round(float64(secretData.Arguments.NodeMonitorGracePeriodSeconds) / 4)
		input.Values.Set("nodeManager.internal.nodeStatusUpdateFrequency", freq)
	}

	if secretData.FeatureGates.Kubelet == nil {
		input.Values.Set("nodeManager.internal.allowedKubeletFeatureGates", []string{})
	} else {
		input.Values.Set("nodeManager.internal.allowedKubeletFeatureGates", secretData.FeatureGates.Kubelet)
	}

	return nil
}

type nodeArguments struct {
	NodeMonitorGracePeriodSeconds int64 `json:"nodeMonitorGracePeriod,omitempty"`
}

type featureGatesData struct {
	Kubelet []string `json:"kubelet,omitempty"`
}

type controlPlaneArgumentsSecret struct {
	Arguments    nodeArguments    `json:"arguments"`
	FeatureGates featureGatesData `json:"featureGates"`
}

func updateFreqFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var secret v1.Secret

	err := sdk.FromUnstructured(obj, &secret)
	if err != nil {
		return nil, err
	}

	var result controlPlaneArgumentsSecret

	if argData, ok := secret.Data["arguments.json"]; ok {
		var args nodeArguments
		if err := json.Unmarshal(argData, &args); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arguments.json: %w", err)
		}
		result.Arguments = args
	}

	if fgData, ok := secret.Data["featureGates.json"]; ok {
		var featureGates featureGatesData
		if err := json.Unmarshal(fgData, &featureGates); err != nil {
			return nil, fmt.Errorf("failed to unmarshal featureGates.json: %w", err)
		}
		result.FeatureGates = featureGates
	}

	return result, nil
}
