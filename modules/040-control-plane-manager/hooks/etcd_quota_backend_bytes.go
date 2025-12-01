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
	"context"
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

	"github.com/deckhouse/module-sdk/pkg"
	sdkobjectpatch "github.com/deckhouse/module-sdk/pkg/object-patch"

	"github.com/deckhouse/deckhouse/go_lib/filter"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type etcdNode struct {
	Memory int64
	// IsDedicated indicates that node is dedicated for control-plane or etcd workload.
	// Node is considered dedicated if it has taint with effect NoSchedule and key:
	//   - node-role.kubernetes.io/control-plane, or
	//   - node-role.kubernetes.io/etcd-only
	// For dedicated nodes, etcd quota is calculated based on available memory.
	// For non-dedicated nodes, quota calculation is skipped to avoid resource constraints.
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
			Name:       "etcd_only_node",
			ApiVersion: "v1",
			Kind:       "Node",
			LabelSelector: &v1.LabelSelector{
				MatchLabels: map[string]string{
					"node-role.deckhouse.io/etcd-only": "",
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
		if (taint.Key == "node-role.kubernetes.io/control-plane" || taint.Key == "node-role.kubernetes.io/etcd-only") && taint.Effect == corev1.TaintEffectNoSchedule {
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

func getCurrentEtcdQuotaBytes(_ context.Context, input *go_hook.HookInput) (int64, string, error) {
	var currentQuotaBytes int64
	var nodeWithMaxQuota string
	etcdEndpointsSnapshots := input.Snapshots.Get("etcd_endpoints")
	for endpoint, err := range sdkobjectpatch.SnapshotIter[etcdInstance](etcdEndpointsSnapshots) {
		if err != nil {
			return currentQuotaBytes, nodeWithMaxQuota, fmt.Errorf("cannot iterate over 'etcd_endpoints' snapshot: %w", err)
		}
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

	return currentQuotaBytes, nodeWithMaxQuota, nil
}

func getNodeWithMinimalMemory(snapshots []pkg.Snapshot) (*etcdNode, error) {
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("'master_nodes' and 'etcd_only_node' snapshots are empty")
	}
	var nodeWithMinimalMemory *etcdNode
	for node, err := range sdkobjectpatch.SnapshotIter[etcdNode](snapshots) {
		if err != nil {
			return nil, fmt.Errorf("cannot iterate over 'master_nodes' and 'etcd_only_node' snapshots: %w", err)
		}

		if nodeWithMinimalMemory == nil {
			nodeWithMinimalMemory = &node
		}

		// for not dedicated nodes we will not set new quota
		if !node.IsDedicated {
			return &node, nil
		}

		if node.Memory < nodeWithMinimalMemory.Memory {
			*nodeWithMinimalMemory = node
		}
	}

	return nodeWithMinimalMemory, nil
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

func calcEtcdQuotaBackendBytes(ctx context.Context, input *go_hook.HookInput) int64 {
	currentQuotaBytes, nodeWithMaxQuota, err := getCurrentEtcdQuotaBytes(ctx, input)
	if err != nil {
		input.Logger.Warn("Cannot get current etcd quota bytes", log.Err(err))
		return currentQuotaBytes
	}
	input.Logger.Debug("Current etcd quota. Getting from node with max quota", slog.Int64("quota", currentQuotaBytes), slog.String("from", nodeWithMaxQuota))

	masterNodeSnapshots := input.Snapshots.Get("master_nodes")
	etcdOnlyNodeSnapshots := input.Snapshots.Get("etcd_only_node")
	
	allNodesSnapshots := make([]pkg.Snapshot, 0, len(masterNodeSnapshots)+len(etcdOnlyNodeSnapshots))
	allNodesSnapshots = append(allNodesSnapshots, masterNodeSnapshots...)
	allNodesSnapshots = append(allNodesSnapshots, etcdOnlyNodeSnapshots...)
	
	node, err := getNodeWithMinimalMemory(allNodesSnapshots)

	if err != nil {
		input.Logger.Warn("Cannot get node with minimal memory", log.Err(err))
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

func etcdQuotaBackendBytesHandler(ctx context.Context, input *go_hook.HookInput) error {
	input.MetricsCollector.Expire(etcdBackendBytesGroup)

	var newQuotaBytes int64

	userQuotaBytes := input.Values.Get("controlPlaneManager.etcd.maxDbSize")
	if userQuotaBytes.Exists() {
		newQuotaBytes = userQuotaBytes.Int()
	} else {
		newQuotaBytes = calcEtcdQuotaBackendBytes(ctx, input)
	}

	// use string because helm render big number in scientific format
	input.Values.Set("controlPlaneManager.internal.etcdQuotaBackendBytes", strconv.FormatInt(newQuotaBytes, 10))

	input.MetricsCollector.Set(
		"d8_etcd_quota_backend_total",
		float64(newQuotaBytes),
		map[string]string{})

	return nil
}
