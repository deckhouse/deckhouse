/*
Copyright 2022 Flant JSC

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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

type etcdNode struct {
	memory int64
	// isDedicated - indicate that node has taint
	//   - effect: NoSchedule
	//    key: node-role.kubernetes.io/master
	// it means that on node can be scheduled only control-plane components
	isDedicated bool
}

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: etcdMaintenanceQueue,
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/master": "",
				},
			},
			FilterFunc: etcdQuotaFilterNode,
		},
		etcdMaintenanceConfig,
	},
}, etcdQuotaBackendBytesHandler)

func etcdQuotaFilterNode(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var node corev1.Node

	err := sdk.FromUnstructured(unstructured, &node)
	if err != nil {
		return nil, err
	}

	memory := node.Status.Capacity.Memory().Value()

	isDedicated := false
	for _, taint := range node.Spec.Taints {
		if taint.Key == "node-role.kubernetes.io/master" && taint.Effect == corev1.TaintEffectNoSchedule {
			isDedicated = true
			break
		}
	}

	return &etcdNode{
		memory:      memory,
		isDedicated: isDedicated,
	}, nil
}

func getCurrentEtcdQuotaBytes(input *go_hook.HookInput) (int64, string) {
	var currentQuotaBytes int64
	var nodeWithMaxQuota string
	for _, endpointRaw := range input.Snapshots[etcdEndpointsSnapshotName] {
		endpoint := endpointRaw.(*etcdInstance)
		quotaForInstance := endpoint.MaxDbSize
		if quotaForInstance > currentQuotaBytes {
			currentQuotaBytes = quotaForInstance
			nodeWithMaxQuota = endpoint.Node
		}
	}

	if currentQuotaBytes == 0 {
		currentQuotaBytes = defaultEtcdMaxSize
		nodeWithMaxQuota = "default"
	}

	return currentQuotaBytes, nodeWithMaxQuota
}

func getNodeWithMinimalMemory(input *go_hook.HookInput) *etcdNode {
	snapshots := input.Snapshots["master_nodes"]
	if len(snapshots) == 0 {
		return nil
	}

	node := snapshots[0].(*etcdNode)
	for i := 1; i < len(snapshots); i++ {
		n := snapshots[i].(*etcdNode)
		// for not dedicated nodes we will not set new quota
		if !n.isDedicated {
			return n
		}

		if n.memory < node.memory {
			node = n
		}
	}

	return node
}

func calcNewQuota(minimalMemoryNodeBytes int64, currentQuotaBytes int64) int64 {
	const minimalNodeSizeForCalc = 16 * 1024 * 1024 * 1024 // 16 GB
	const nodeSizeStepForAdd = 8 * 1024 * 1024 * 1024      // every 8 GB memory
	const quotaStep = 1 * 1024 * 1024 * 1024               // every 1 GB etcd memory every nodeSizeStepForAdd
	const maxEtcdQuota = 8 * 1024 * 1024 * 1024            // 8 GB memory, if quota > 8Gb etcd will start with warning

	if minimalMemoryNodeBytes < minimalNodeSizeForCalc {
		return currentQuotaBytes
	}

	if currentQuotaBytes == maxEtcdQuota {
		return maxEtcdQuota
	}

	increaseSteps := []int{
		16 * 1024 * 1024 * 1024, // 2 + 1 = 3GB
		24 * 1024 * 1024 * 1024, // 4GB
		32 * 1024 * 1024 * 1024, // 5GB
		48 * 1024 * 1024 * 1024, // 6GB
		64 * 1024 * 1024 * 1024, // 7GB
		96 * 1024 * 1024 * 1024, // 8GB
	}
	// for 16 gb add one gb
	newQuota := defaultEtcdMaxSize + quotaStep

}

func etcdQuotaBackendBytesHandler(input *go_hook.HookInput) error {
	currentQuotaBytes, nodeWithMaxQuota := getCurrentEtcdQuotaBytes(input)

	input.LogEntry.Infof("Current etcd quota: %d. Getting from %s", currentQuotaBytes, nodeWithMaxQuota)

	node := getNodeWithMinimalMemory(input)

	newQuotaBytes := currentQuotaBytes

	if node.isDedicated {
		newQuotaBytes = calcNewQuota(node.memory, currentQuotaBytes)
	}

	input.Values.Set("controlPlaneManager.internal.etcdQuotaBackendBytes", newQuotaBytes)

	return nil
}
