/*
Copyright 2024 Flant JSC

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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	sdkpkg "github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/modules/040-node-manager/hooks/internal/capi/v1beta1"
)

const (
	capsMachineDeploymentMetricsGroup          = "caps_md"
	capsMachineDeploymentMetricReplicasName    = "d8_caps_md_replicas"
	capsMachineDeploymentMetricDesiredName     = "d8_caps_md_desired"
	capsMachineDeploymentMetricReadyName       = "d8_caps_md_ready"
	capsMachineDeploymentMetricUnavailableName = "d8_caps_md_unavailable"
	capsMachineDeploymentMetricPhaseName       = "d8_caps_md_phase"
)

type machineDeploymentStatus struct {
	Name        string
	Replicas    float64
	Desired     float64
	Ready       float64
	Unavailable float64
	Phase       float64
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/metrics",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                   "machinedeployment_status",
			ApiVersion:             "cluster.x-k8s.io/v1beta2",
			Kind:                   "MachineDeployment",
			WaitForSynchronization: ptr.To(false),
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-cloud-instance-manager"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchExpressions: []v1.LabelSelectorRequirement{
					{
						Key:      "app",
						Operator: v1.LabelSelectorOpIn,
						Values: []string{
							"caps-controller",
						},
					},
				},
			},
			FilterFunc: filterMachineDeploymentStatus,
		},
	},
}, handleMachineDeploymentStatus)

func filterMachineDeploymentStatus(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var md v1beta1.MachineDeployment

	err := sdk.FromUnstructured(obj, &md)
	if err != nil {
		return nil, err
	}

	var replicas int32
	if md.Spec.Replicas != nil {
		replicas = *md.Spec.Replicas
	}

	var Phase float64
	switch md.Status.Phase {
	case "Running":
		Phase = 1
	case "ScalingUp":
		Phase = 2
	case "ScalingDown":
		Phase = 3
	case "Failed":
		Phase = 4
	default:
		Phase = 5
	}

	return machineDeploymentStatus{
		Name:        md.Name,
		Replicas:    float64(md.Status.Replicas),
		Desired:     float64(replicas),
		Ready:       float64(md.Status.ReadyReplicas),
		Unavailable: float64(md.Status.UnavailableReplicas),
		Phase:       Phase,
	}, nil
}

func handleMachineDeploymentStatus(_ context.Context, input *go_hook.HookInput) error {
	mdStatusSnapshots := input.Snapshots.Get("machinedeployment_status")

	input.MetricsCollector.Expire(capsMachineDeploymentMetricsGroup)

	options := []sdkpkg.MetricCollectorOption{
		metrics.WithGroup(capsMachineDeploymentMetricsGroup),
	}
	for mdStatus, err := range sdkobjectpatch.SnapshotIter[machineDeploymentStatus](mdStatusSnapshots) {
		if err != nil {
			return fmt.Errorf("failed to iterate over 'machinedeployment_status' snapshots: %w", err)
		}

		labels := map[string]string{"machine_deployment_name": mdStatus.Name}

		input.MetricsCollector.Set(capsMachineDeploymentMetricReplicasName, mdStatus.Replicas, labels, options...)

		input.MetricsCollector.Set(capsMachineDeploymentMetricDesiredName, mdStatus.Desired, labels, options...)

		input.MetricsCollector.Set(capsMachineDeploymentMetricReadyName, mdStatus.Ready, labels, options...)

		input.MetricsCollector.Set(capsMachineDeploymentMetricUnavailableName, mdStatus.Unavailable, labels, options...)

		input.MetricsCollector.Set(capsMachineDeploymentMetricPhaseName, mdStatus.Phase, labels, options...)
	}

	return nil
}
