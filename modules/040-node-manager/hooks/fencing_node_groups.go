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
	"sort"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

// Default mirrors `dig "fencing" "watchdog" "timeout" 60` in templates/fencing-agent.
const defaultFencingWatchdogTimeout = int64(60)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/fencing",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "fencing_ngs",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
			FilterFunc: fencingFilterNG,
		},
	},
}, handleFencingNodeGroups)

type fencingNodeGroup struct {
	Name            string `json:"name"`
	Mode            string `json:"mode"`
	WatchdogTimeout int64  `json:"watchdogTimeout"`
}

func fencingFilterNG(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	if ng.Spec.Fencing.Mode == "" {
		return nil, nil
	}

	timeout := defaultFencingWatchdogTimeout
	if ng.Spec.Fencing.Watchdog != nil && ng.Spec.Fencing.Watchdog.Timeout != 0 {
		timeout = ng.Spec.Fencing.Watchdog.Timeout
	}

	return fencingNodeGroup{
		Name:            ng.Name,
		Mode:            ng.Spec.Fencing.Mode,
		WatchdogTimeout: timeout,
	}, nil
}

func handleFencingNodeGroups(_ context.Context, input *go_hook.HookInput) error {
	snaps := input.Snapshots.Get("fencing_ngs")

	fencingNGs := make([]fencingNodeGroup, 0, len(snaps))
	for ng, err := range sdkobjectpatch.SnapshotIter[fencingNodeGroup](snaps) {
		if err != nil {
			return fmt.Errorf("iterate over 'fencing_ngs' snapshots: %w", err)
		}

		if ng.Mode == "" {
			continue
		}

		fencingNGs = append(fencingNGs, ng)
	}

	sort.Slice(fencingNGs, func(i, j int) bool { return fencingNGs[i].Name < fencingNGs[j].Name })

	input.Values.Set("nodeManager.internal.fencingNodeGroups", fencingNGs)

	return nil
}
