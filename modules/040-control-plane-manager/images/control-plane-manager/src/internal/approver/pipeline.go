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

package approver

import (
	controlplanev1alpha1 "control-plane-manager/api/v1alpha1"
)

// NormalPipeline is the approval pipeline for a normal (non-virtual) control plane: etcd gates
// everything else (wide-block), then kube-apiserver, then kube-controller-manager/kube-scheduler.
var NormalPipeline = []pipelineStage{
	{
		components: []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentEtcd,
		},
		concurrencyLimitFn: getConcurrencyLimit,
		wideBlock:          true, // etcd affects the whole quorum, not just its own node
	},
	{
		components: []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentKubeAPIServer,
		},
		concurrencyLimitFn: getConcurrencyLimit,
	},
	{
		components: []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentKubeControllerManager,
			controlplanev1alpha1.OperationComponentKubeScheduler,
		},
		concurrencyLimitFn: getConcurrencyLimit,
	},
}

// VirtualPipeline drops the Etcd stage entirely: virtual control-plane has no etcd component, so
// KubeAPIServer becomes the first stage (its own per-node/concurrency limits are sufficient, no
// wide-block stage needed).
var VirtualPipeline = []pipelineStage{
	{
		components: []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentKubeAPIServer,
		},
		concurrencyLimitFn: getConcurrencyLimit,
	},
	{
		components: []controlplanev1alpha1.OperationComponent{
			controlplanev1alpha1.OperationComponentKubeControllerManager,
			controlplanev1alpha1.OperationComponentKubeScheduler,
		},
		concurrencyLimitFn: getConcurrencyLimit,
	},
}

// Arbiters run etcd only; workload components (apiserver, etc.) run on master nodes exclusively.
// For etcd the limit accounts for the full quorum membership (masters + arbiters).
// For all other components only master nodes count.
func getConcurrencyLimit(nodes NodeCounts, c controlplanev1alpha1.OperationComponent) int {
	switch c {
	case controlplanev1alpha1.OperationComponentEtcd:
		return etcdConcurrencyLimit(nodes.Masters + nodes.Arbiters)
	default:
		return controlPlaneWorkloadConcurrencyLimit(nodes.Masters)
	}
}

// TODO: the limit is hardcoded to 1 until we settle on a quorum-safe formula,
// e.g. (n-1)/2 for a cluster of n etcd members (masters + arbiters).
func etcdConcurrencyLimit(nodes int) int {
	_ = nodes
	return 1
}

func controlPlaneWorkloadConcurrencyLimit(nodes int) int {
	return max(1, nodes-1)
}
