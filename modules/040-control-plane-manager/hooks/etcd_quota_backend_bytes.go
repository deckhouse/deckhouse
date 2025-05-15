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
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strconv"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook/metrics"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/deckhouse/deckhouse/go_lib/filter"
)

type etcdNode struct {
	Memory int64
	// isDedicated - indicate that node has taint
	//   - effect: NoSchedule
	//    key: node-role.kubernetes.io/control-plane
	// it means that on node can be scheduled only control-plane components
	IsDedicated bool
}

const (
	etcdBackendBytesGroup = "etcd_quota_backend_should_decrease"
)

var (
	maxDbSizeRegExp    = regexp.MustCompile(`(^|\s+)--quota-backend-bytes=(\d+)$`)
	defaultEtcdMaxSize = gb(2)
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: moduleQueue + "/etcd_maintenance",
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "master_nodes",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.kubernetes.io/control-plane": "",
				},
			},
			FilterFunc: etcdQuotaFilterNode,
		},
		{
			Name:       "etcd_endpoints",
			ApiVersion: "v1",
			Kind:       "Pod",
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"kube-system"},
				},
			},
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"component": "etcd",
					"tier":      "control-plane",
				},
			},
			FieldSelector: &types.FieldSelector{
				MatchExpressions: []types.FieldSelectorRequirement{
					{
						Field:    "status.phase",
						Operator: "Equals",
						Value:    "Running",
					},
				},
			},
			FilterFunc: maintenanceEtcdFilter,
		},
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
		if taint.Key == "node-role.kubernetes.io/control-plane" && taint.Effect == corev1.TaintEffectNoSchedule {
			isDedicated = true
			break
		}
	}

	return &etcdNode{
		Memory:      memory,
		IsDedicated: isDedicated,
	}, nil
}

func maintenanceEtcdFilter(unstructured *unstructured.Unstructured) (go_hook.FilterResult, error) {
	var pod corev1.Pod

	err := sdk.FromUnstructured(unstructured, &pod)
	if err != nil {
		return nil, err
	}

	var ip string
	if pod.Spec.HostNetwork {
		ip = pod.Status.HostIP
	} else {
		ip = pod.Status.PodIP
	}

	curMaxDbSize := defaultEtcdMaxSize
	maxBytesStr := filter.GetArgPodWithRegexp(&pod, maxDbSizeRegExp, 1, "")
	if maxBytesStr != "" {
		curMaxDbSize, err = strconv.ParseInt(maxBytesStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot get quota-backend-bytes from etcd argument, got %s: %v", maxBytesStr, err)
		}
	}

	return &etcdInstance{
		Endpoint:  fmt.Sprintf("https://%s:2379", ip),
		MaxDbSize: curMaxDbSize,
		PodName:   pod.GetName(),
		Node:      pod.Spec.NodeName,
	}, nil
}

func getCurrentEtcdQuotaBytes(input *go_hook.HookInput) (int64, string) {
	var currentQuotaBytes int64
	var nodeWithMaxQuota string
	for _, endpointRaw := range input.Snapshots["etcd_endpoints"] {
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

func getNodeWithMinimalMemory(snapshots []go_hook.FilterResult) *etcdNode {
	if len(snapshots) == 0 {
		return nil
	}

	node := snapshots[0].(*etcdNode)
	for i := 1; i < len(snapshots); i++ {
		n := snapshots[i].(*etcdNode)
		// for not dedicated nodes we will not set new quota
		if !n.IsDedicated {
			return n
		}

		if n.Memory < node.Memory {
			node = n
		}
	}

	return node
}

func calcNewQuotaForMemory(minimalMemoryNodeBytes int64) int64 {
	var (
		minimalNodeSizeForCalc = gb(16)
		nodeSizeStepForAdd     = gb(8) // every 8 GB memory
		quotaStep              = gb(1) // add 1 GB etcd memory every nodeSizeStepForAdd
		maxQuota               = gb(8)
	)

	if minimalMemoryNodeBytes <= minimalNodeSizeForCalc {
		return defaultEtcdMaxSize
	}

	// node capacity often less than set size
	// for example for 24GB node size capacity can be 23.48GB
	// for there cases we should round step value
	stepsFloat := float64(minimalMemoryNodeBytes-minimalNodeSizeForCalc) / float64(nodeSizeStepForAdd)
	steps := int64(math.Round(stepsFloat))

	newQuota := steps*quotaStep + defaultEtcdMaxSize

	if newQuota > maxQuota {
		newQuota = maxQuota
	}

	return newQuota
}

func calcEtcdQuotaBackendBytes(input *go_hook.HookInput) int64 {
	currentQuotaBytes, nodeWithMaxQuota := getCurrentEtcdQuotaBytes(input)

	input.Logger.Debug("Current etcd quota. Getting from node with max quota", slog.Int64("quota", currentQuotaBytes), slog.String("from", nodeWithMaxQuota))

	snaps := input.Snapshots["master_nodes"]
	node := getNodeWithMinimalMemory(snaps)
	if node == nil {
		input.Logger.Warn("Cannot get node with minimal memory")
		return currentQuotaBytes
	}

	newQuotaBytes := currentQuotaBytes

	if node.IsDedicated {
		newQuotaBytes = calcNewQuotaForMemory(node.Memory)
		if newQuotaBytes < currentQuotaBytes {
			newQuotaBytes = currentQuotaBytes

			input.Logger.Warn("Cannot decrease quota backend bytes. Use current", slog.Int64("current", currentQuotaBytes), slog.Int64("calculated", newQuotaBytes))

			input.MetricsCollector.Set(
				"d8_etcd_quota_backend_should_decrease",
				1.0,
				map[string]string{},
				metrics.WithGroup(etcdBackendBytesGroup))
		}

		input.Logger.Debug("New backend quota bytes calculated", slog.Int64("calculated", newQuotaBytes))
	} else {
		input.Logger.Debug("Found not dedicated control-plane node. Skip calculate backend quota. Use current.", slog.Int64("current", newQuotaBytes))
	}

	return newQuotaBytes
}

func etcdQuotaBackendBytesHandler(input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(etcdBackendBytesGroup)

	var newQuotaBytes int64

	userQuotaBytes := input.Values.Get("controlPlaneManager.etcd.maxDbSize")
	if userQuotaBytes.Exists() {
		newQuotaBytes = userQuotaBytes.Int()
	} else {
		newQuotaBytes = calcEtcdQuotaBackendBytes(input)
	}

	// use string because helm render big number in scientific format
	input.Values.Set("controlPlaneManager.internal.etcdQuotaBackendBytes", strconv.FormatInt(newQuotaBytes, 10))

	input.MetricsCollector.Set(
		"d8_etcd_quota_backend_total",
		float64(newQuotaBytes),
		map[string]string{})

	return nil
}
