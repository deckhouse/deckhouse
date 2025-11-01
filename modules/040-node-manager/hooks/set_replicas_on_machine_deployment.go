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
	"fmt"
	"log/slog"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/capi/v1beta1"
	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/mcm/v1alpha1"
	ngv1 "github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/v1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/set_replicas_on_machine_deployment",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "mds",
			ApiVersion:             "machine.sapcloud.io/v1alpha1",
			Kind:                   "MachineDeployment",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: setReplicasFilterMD,
		},

		{
			Name:                   "capi_mds",
			ApiVersion:             "cluster.x-k8s.io/v1beta2",
			Kind:                   "MachineDeployment",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			FilterFunc: capiSetReplicasFilterMD,
		},

		{
			Name:                   "ngs",
			ApiVersion:             "deckhouse.io/v1",
			Kind:                   "NodeGroup",
			WaitForSynchronization: ptr.To(false),
			FilterFunc:             setReplicasFilterNG,
		},
	},
}, handleSetReplicas)

type setReplicasNodeGroup struct {
	Name string
	Min  int32
	Max  int32
}
type setReplicasMachineDeployment struct {
	Name      string
	NodeGroup string
	Replicas  int32
}

func setReplicasFilterNG(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var ng ngv1.NodeGroup

	err := sdk.FromUnstructured(obj, &ng)
	if err != nil {
		return nil, err
	}

	var minr, maxr int32

	if ng.Spec.StaticInstances != nil {
		count := ng.Spec.StaticInstances.Count
		minr, maxr = count, count
	}

	if ng.Spec.CloudInstances.MinPerZone != nil {
		minr = *ng.Spec.CloudInstances.MinPerZone
	}

	if ng.Spec.CloudInstances.MaxPerZone != nil {
		maxr = *ng.Spec.CloudInstances.MaxPerZone
	}

	return setReplicasNodeGroup{
		Name: ng.Name,
		Min:  minr,
		Max:  maxr,
	}, nil
}

func setReplicasFilterMD(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var md v1alpha1.MachineDeployment

	err := sdk.FromUnstructured(obj, &md)
	if err != nil {
		return nil, err
	}

	return setReplicasMachineDeployment{
		Name:      md.Name,
		NodeGroup: md.Labels["node-group"],
		Replicas:  md.Spec.Replicas,
	}, nil
}

func capiSetReplicasFilterMD(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var md v1beta1.MachineDeployment

	err := sdk.FromUnstructured(obj, &md)
	if err != nil {
		return nil, err
	}

	var replicas int32
	if md.Spec.Replicas != nil {
		replicas = *md.Spec.Replicas
	}

	return setReplicasMachineDeployment{
		Name:      md.Name,
		NodeGroup: md.Labels["node-group"],
		Replicas:  replicas,
	}, nil
}

func calculateReplicasAndPatchMachineDeployment(
	input *go_hook.HookInput, snaps []pkg.Snapshot, nodeGroups map[string]setReplicasNodeGroup, apiGroup string) error {
	for md, err := range sdkobjectpatch.SnapshotIter[setReplicasMachineDeployment](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over snapshot: %w", err)
		}
		ng, ok := nodeGroups[md.NodeGroup]
		if !ok {
			input.Logger.Warn("can't find NodeGroup to get min and max instances per zone", slog.String("name", md.NodeGroup))
			continue
		}

		var desiredReplicas = md.Replicas

		switch {
		case ng.Min >= ng.Max:
			desiredReplicas = ng.Max

		case md.Replicas == 0:
			desiredReplicas = ng.Min

		case md.Replicas <= ng.Min:
			desiredReplicas = ng.Min

		case md.Replicas > ng.Max:
			desiredReplicas = ng.Max
		}

		if desiredReplicas == md.Replicas {
			// replicas not changed, we don't need to patch deployment
			continue
		}

		patch := map[string]interface{}{
			"spec": map[string]interface{}{
				"replicas": desiredReplicas,
			},
		}

		input.PatchCollector.PatchWithMerge(patch, apiGroup, "MachineDeployment", "d8-cloud-instance-manager", md.Name)
	}
	return nil
}

func handleSetReplicas(_ context.Context, input *go_hook.HookInput) error {
	nodeGroups := make(map[string]setReplicasNodeGroup)

	snaps := input.Snapshots.Get("ngs")
	for ng, err := range sdkobjectpatch.SnapshotIter[setReplicasNodeGroup](snaps) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'ngs' snapshots: %w", err)
		}

		nodeGroups[ng.Name] = ng
	}

	err := calculateReplicasAndPatchMachineDeployment(input, input.Snapshots.Get("mds"), nodeGroups, "machine.sapcloud.io/v1alpha1")
	if err != nil {
		return err
	}

	err = calculateReplicasAndPatchMachineDeployment(input, input.Snapshots.Get("capi_mds"), nodeGroups, "cluster.x-k8s.io/v1beta2")
	if err != nil {
		return err
	}

	return nil
}
