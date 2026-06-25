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
	"testing"

	dto "github.com/prometheus/client_model/go"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
)

func TestCalculateNodeStatus(t *testing.T) {
	automaticNG := &v1.NodeGroup{Spec: v1.NodeGroupSpec{
		Disruptions: &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalModeAutomatic},
	}}
	manualNG := &v1.NodeGroup{Spec: v1.NodeGroupSpec{
		Disruptions: &v1.DisruptionsSpec{ApprovalMode: v1.DisruptionApprovalModeManual},
	}}

	tests := []struct {
		name            string
		node            ua.NodeInfo
		ng              *v1.NodeGroup
		desiredChecksum string
		want            string
	}{
		{
			name: "waiting for approval has highest priority",
			node: ua.NodeInfo{IsWaitingForApproval: true, IsApproved: true},
			ng:   automaticNG,
			want: "WaitingForApproval",
		},
		{
			name: "approved, disruption required and draining",
			node: ua.NodeInfo{IsApproved: true, IsDisruptionRequired: true, IsDraining: true},
			ng:   automaticNG,
			want: "DrainingForDisruption",
		},
		{
			name: "draining without disruption required",
			node: ua.NodeInfo{IsDraining: true},
			ng:   automaticNG,
			want: "Draining",
		},
		{
			name: "drained",
			node: ua.NodeInfo{IsDrained: true},
			ng:   automaticNG,
			want: "Drained",
		},
		{
			name: "approved, disruption required, automatic mode",
			node: ua.NodeInfo{IsApproved: true, IsDisruptionRequired: true},
			ng:   automaticNG,
			want: "WaitingForDisruptionApproval",
		},
		{
			name: "approved, disruption required, manual mode",
			node: ua.NodeInfo{IsApproved: true, IsDisruptionRequired: true},
			ng:   manualNG,
			want: "WaitingForManualDisruptionApproval",
		},
		{
			name: "approved and disruption approved",
			node: ua.NodeInfo{IsApproved: true, IsDisruptionApproved: true},
			ng:   automaticNG,
			want: "DisruptionApproved",
		},
		{
			name: "approved only",
			node: ua.NodeInfo{IsApproved: true},
			ng:   automaticNG,
			want: "Approved",
		},
		{
			name: "rolling update",
			node: ua.NodeInfo{IsRollingUpdate: true},
			ng:   automaticNG,
			want: "RollingUpdate",
		},
		{
			name: "empty checksum",
			node: ua.NodeInfo{ConfigurationChecksum: ""},
			ng:   automaticNG,
			want: "UpdateFailedNoConfigChecksum",
		},
		{
			name:            "checksum differs from desired",
			node:            ua.NodeInfo{ConfigurationChecksum: "old"},
			ng:              automaticNG,
			desiredChecksum: "new",
			want:            "ToBeUpdated",
		},
		{
			name:            "checksum equals desired",
			node:            ua.NodeInfo{ConfigurationChecksum: "same"},
			ng:              automaticNG,
			desiredChecksum: "same",
			want:            "UpToDate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateNodeStatus(tt.node, tt.ng, tt.desiredChecksum); got != tt.want {
				t.Fatalf("CalculateNodeStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

func gaugeValue(t *testing.T, node, nodeGroup, status string) float64 {
	t.Helper()
	var m dto.Metric
	g, err := updateApprovalNodeGroupNodeStatus.GetMetricWithLabelValues(node, nodeGroup, status)
	if err != nil {
		t.Fatalf("get metric: %v", err)
	}
	if err := g.Write(&m); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	return m.GetGauge().GetValue()
}

func TestSetNodeStatusMetrics_OnlyMatchingStatusIsOne(t *testing.T) {
	updateApprovalNodeGroupNodeStatus.Reset()

	SetNodeStatusMetrics("n1", "worker", "Approved")

	if got := gaugeValue(t, "n1", "worker", "Approved"); got != 1 {
		t.Fatalf("expected Approved gauge = 1, got %v", got)
	}
	if got := gaugeValue(t, "n1", "worker", "WaitingForApproval"); got != 0 {
		t.Fatalf("expected WaitingForApproval gauge = 0, got %v", got)
	}
	if got := gaugeValue(t, "n1", "worker", "UpToDate"); got != 0 {
		t.Fatalf("expected UpToDate gauge = 0, got %v", got)
	}
}

func TestSetNodeMetrics_DerivesStatusFromNode(t *testing.T) {
	updateApprovalNodeGroupNodeStatus.Reset()

	node := ua.NodeInfo{Name: "n2", NodeGroup: "worker", IsWaitingForApproval: true}
	ng := &v1.NodeGroup{}

	SetNodeMetrics(node, ng, "checksum")

	if got := gaugeValue(t, "n2", "worker", "WaitingForApproval"); got != 1 {
		t.Fatalf("expected WaitingForApproval gauge = 1, got %v", got)
	}
	if got := gaugeValue(t, "n2", "worker", "Approved"); got != 0 {
		t.Fatalf("expected Approved gauge = 0, got %v", got)
	}
}

func TestRegister_AddsCollectorToControllerRegistry(t *testing.T) {
	// Clean state in case another test in this binary already registered it.
	ctrlmetrics.Registry.Unregister(updateApprovalNodeGroupNodeStatus)

	Register()
	t.Cleanup(func() { ctrlmetrics.Registry.Unregister(updateApprovalNodeGroupNodeStatus) })

	updateApprovalNodeGroupNodeStatus.Reset()
	SetNodeStatusMetrics("reg-node", "worker", "Approved")

	families, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("gather registry: %v", err)
	}
	found := false
	for _, fam := range families {
		if fam.GetName() == "node_group_node_status" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected node_group_node_status metric family to be registered")
	}
}
