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
	"fmt"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"github.com/google/uuid"
	v1core "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/filter"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "cluster_uuid",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-cluster-uuid"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			FilterFunc: filter.KeyFromConfigMap("cluster-uuid"),
		},
	},
}, discoveryClusterUUID)

func createConfigMapWithUUID(patch go_hook.PatchCollector, clusterUUID string) {
	cm := &v1core.ConfigMap{
		TypeMeta: v1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},

		ObjectMeta: v1.ObjectMeta{
			Name:      "d8-cluster-uuid",
			Namespace: "kube-system",
			Labels: map[string]string{
				"heritage": "deckhouse",
			},
		},
	}

	cm.Data = map[string]string{
		"cluster-uuid": clusterUUID,
	}

	patch.CreateIfNotExists(cm)
}

// discoveryClusterUUID
// There is CM kube-system/d8-cluster-uuid with cluster uuid. Hook must store it to `global.discovery.clusterUUID`.
// Or generate uuid and create CM
func discoveryClusterUUID(_ context.Context, input *go_hook.HookInput) error {
	const valPath = "global.discovery.clusterUUID"

	uuidSnap, err := sdkobjectpatch.UnmarshalToStruct[string](input.Snapshots, "cluster_uuid")
	if err != nil {
		return fmt.Errorf("failed to unmarshal cluster_uuid snapshot: %w", err)
	}

	var clusterUUID string
	if len(uuidSnap) > 0 {
		clusterUUID = uuidSnap[0]
	} else {
		if uuidFromVals, ok := input.Values.GetOk(valPath); ok {
			clusterUUID = uuidFromVals.String()
		} else {
			clusterUUID = uuid.New().String()
		}

		createConfigMapWithUUID(input.PatchCollector, clusterUUID)
	}

	input.Values.Set(valPath, clusterUUID)

	return nil
}
