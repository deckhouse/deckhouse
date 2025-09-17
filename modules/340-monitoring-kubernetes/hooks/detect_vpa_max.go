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

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

func nodeNameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	return obj.GetName(), nil
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/monitoring-kubernetes/detect_vpa_max",
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 20.0,
	},
	Schedule: []go_hook.ScheduleConfig{
		{
			Name:    "detect_vpa_max",
			Crontab: "*/10 * * * *",
		},
	},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "nodes",
			ApiVersion:                   "v1",
			Kind:                         "Node",
			ExecuteHookOnEvents:          ptr.To(false),
			ExecuteHookOnSynchronization: ptr.To(false),
			FilterFunc:                   nodeNameFilter,
		},
	},
}, delectVPAMax)

const (
	memoryPerNode = int64(30)
	cpuPerNode    = int64(15)
)

func delectVPAMax(_ context.Context, input *go_hook.HookInput) error {
	nodeSnap := input.Snapshots.Get("nodes")

	// TODO use node CAPACITY in calculationsinput
	nodeCount := int64(len(nodeSnap)) //nolint:gosec
	maxMemory := fmt.Sprintf("%dMi", 150+memoryPerNode*nodeCount)
	maxCPU := fmt.Sprintf("%dm", 100+cpuPerNode*nodeCount)

	input.Values.Set("monitoringKubernetes.internal.vpa.kubeStateMetricsMaxMemory", maxMemory)
	input.Values.Set("monitoringKubernetes.internal.vpa.kubeStateMetricsMaxCPU", maxCPU)

	return nil
}
