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

package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

var (
	updateApprovalNodeGroupNodeStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_node_status",
			Help: "Status of a node within a node group for update approval",
		},
		[]string{"node", "node_group", "status"},
	)

	updateApprovalMetricStatuses = []string{
		"WaitingForApproval", "Approved", "DrainingForDisruption", "Draining", "Drained",
		"WaitingForDisruptionApproval", "WaitingForManualDisruptionApproval", "DisruptionApproved",
		"RollingUpdate", "ToBeUpdated", "UpToDate", "UpdateFailedNoConfigChecksum",
	}
)

func Register() {
	ctrlmetrics.Registry.MustRegister(updateApprovalNodeGroupNodeStatus)
}

func SetNodeMetrics(node ua.NodeInfo, ng *v1.NodeGroup, desiredChecksum string) {
	nodeStatus := CalculateNodeStatus(node, ng, desiredChecksum)
	SetNodeStatusMetrics(node.Name, node.NodeGroup, nodeStatus)
}

func SetNodeStatusMetrics(nodeName, nodeGroup, nodeStatus string) {
	for _, status := range updateApprovalMetricStatuses {
		var value float64
		if status == nodeStatus {
			value = 1
		}
		updateApprovalNodeGroupNodeStatus.WithLabelValues(nodeName, nodeGroup, status).Set(value)
	}
}

func CalculateNodeStatus(node ua.NodeInfo, ng *v1.NodeGroup, desiredChecksum string) string {
	approvalMode := ua.GetApprovalMode(ng)

	switch {
	case node.IsWaitingForApproval:
		return "WaitingForApproval"
	case node.IsApproved && node.IsDisruptionRequired && node.IsDraining:
		return "DrainingForDisruption"
	case node.IsDraining:
		return "Draining"
	case node.IsDrained:
		return "Drained"
	case node.IsApproved && node.IsDisruptionRequired && approvalMode == "Automatic":
		return "WaitingForDisruptionApproval"
	case node.IsApproved && node.IsDisruptionRequired && approvalMode == "Manual":
		return "WaitingForManualDisruptionApproval"
	case node.IsApproved && node.IsDisruptionApproved:
		return "DisruptionApproved"
	case node.IsApproved:
		return "Approved"
	case node.IsRollingUpdate:
		return "RollingUpdate"
	case node.ConfigurationChecksum == "":
		return "UpdateFailedNoConfigChecksum"
	case node.ConfigurationChecksum != desiredChecksum:
		return "ToBeUpdated"
	case node.ConfigurationChecksum == desiredChecksum:
		return "UpToDate"
	default:
		return "Unknown"
	}
}
