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
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_node_names",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			FilterFunc: applyMasterNodeFilter,
		},
		{
			Name:              "converge_state",
			ApiVersion:        "v1",
			Kind:              "Secret",
			NamespaceSelector: &types.NamespaceSelector{NameSelector: &types.NameSelector{MatchNames: []string{"d8-system"}}},
			NameSelector:      &types.NameSelector{MatchNames: []string{"d8-dhctl-converge-state"}},
			FilterFunc:        applyConvergeStateFilter,
		},
	},
}, isHighAvailabilityCluster)

func applyMasterNodeFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

type convergeState struct {
	PreserveExistingHAMode bool `json:"preserveExistingHAMode"`
}

func applyConvergeStateFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	secret := &corev1.Secret{}
	if err := sdk.FromUnstructured(obj, secret); err != nil {
		slog.Info("Failed to parse converge state secret", slog.String("error", err.Error()))
		return nil, fmt.Errorf("from unstructured: %w", err)
	}

	stateBytes, ok := secret.Data["state.json"]
	if !ok || len(stateBytes) == 0 {
		return false, nil
	}

	var st convergeState
	if err := json.Unmarshal(stateBytes, &st); err != nil {
		slog.Warn("Failed to unmarshal converge state, falling back to autodetection", slog.String("error", err.Error()))
		return false, nil
	}

	return st.PreserveExistingHAMode, nil
}

func isHighAvailabilityCluster(_ context.Context, input *go_hook.HookInput) error {
	masterNodesSnap := input.Snapshots.Get("master_node_names")
	convergeStateSnap := input.Snapshots.Get("converge_state")

	mastersCount := len(masterNodesSnap)
	preserveExistingHAMode := false
	for v, err := range sdkobjectpatch.SnapshotIter[bool](convergeStateSnap) {
		if err != nil {
			input.Logger.Info("Failed to parse converge_state snapshot item, skipping", slog.String("error", err.Error()))
			continue
		}
		if v {
			preserveExistingHAMode = true
			break
		}
	}

	input.Values.Set("global.discovery.clusterMasterCount", mastersCount)

	haPath := "global.discovery.clusterControlPlaneIsHighlyAvailable"
	prevHAValue, haAlreadySet := input.Values.GetOk(haPath)

	if preserveExistingHAMode && haAlreadySet {
		currentHA := prevHAValue.Bool()
		input.Logger.Info(
			"HA mode autodetection preserved",
			slog.Int("master_count", mastersCount),
			slog.Bool("preserveExistingHAMode", preserveExistingHAMode),
			slog.Bool("isHA", currentHA),
		)
		return nil
	}

	isHA := mastersCount > 1
	input.Logger.Info(
		"HA mode autodetection recalculated",
		slog.Int("master_count", mastersCount),
		slog.Bool("preserveExistingHAMode", preserveExistingHAMode),
		slog.Bool("isHA", isHA),
	)
	input.Values.Set(haPath, isHA)

	return nil
}
