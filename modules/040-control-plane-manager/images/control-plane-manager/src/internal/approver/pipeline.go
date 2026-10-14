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

// etcdStage wide-blocks everything else: one in-flight etcd operation affects the whole quorum,
// not just its own node.
var etcdStage = pipelineStage{
	components: []controlplanev1alpha1.OperationComponent{
		controlplanev1alpha1.OperationComponentEtcd,
	},
	clusterWide: true, // etcd affects the whole quorum, not just its own node
}

// apiServerStage wide-blocks kube-controller-manager/kube-scheduler: they reach any kube-apiserver
// through kubernetes-api-proxy and must not be newer than any of them (Kubernetes version skew
// policy), so none of them restarts until every kube-apiserver operation has finished.
var apiServerStage = pipelineStage{
	components: []controlplanev1alpha1.OperationComponent{
		controlplanev1alpha1.OperationComponentKubeAPIServer,
	},
	clusterWide: true,
}

var workloadStage = pipelineStage{
	components: []controlplanev1alpha1.OperationComponent{
		controlplanev1alpha1.OperationComponentKubeControllerManager,
		controlplanev1alpha1.OperationComponentKubeScheduler,
	},
}

// StaticPodPipeline is the approval pipeline for a static-pod control plane: etcd gates
// everything else (cluster-wide), then kube-apiserver (cluster-wide), then
// kube-controller-manager/kube-scheduler.
var StaticPodPipeline = []pipelineStage{etcdStage, apiServerStage, workloadStage}

// Arbiters run etcd only; workload components (apiserver, etc.) run on master nodes exclusively.
// For etcd the limit accounts for the full quorum membership (masters + arbiters).
// For all other components only master nodes count.
func getConcurrencyLimit(nodes Nodes, c controlplanev1alpha1.OperationComponent) int {
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

// controlPlaneWorkloadConcurrencyLimit keeps a majority of kube-apiservers (and of
// kube-controller-manager/kube-scheduler instances) serving while the rest restart:
// 3 masters -> 1, 5 masters -> 2. With nodes-1 two of three kube-apiservers restarted at once
// and the whole API load landed on a single instance.
func controlPlaneWorkloadConcurrencyLimit(nodes int) int {
	return max(1, (nodes-1)/2)
}
