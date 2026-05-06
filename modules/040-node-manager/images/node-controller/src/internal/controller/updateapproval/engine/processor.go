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

package engine

import (
	"context"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	ua "github.com/deckhouse/node-controller/internal/controller/updateapproval/common"
	"github.com/deckhouse/node-controller/internal/controller/updateapproval/kubeclient"
	uametrics "github.com/deckhouse/node-controller/internal/controller/updateapproval/metrics"
)

type Processor struct {
	Kube              kubeclient.Client
	Recorder          record.EventRecorder
	DeckhouseNodeName string
}

func (p Processor) ProcessUpdatedNodes(ctx context.Context, ng *v1.NodeGroup, nodes []ua.NodeInfo, ngChecksum string) (bool, error) {
	logger := log.FromContext(ctx)
	for _, node := range nodes {
		// Match original hook: only process approved nodes.
		if !node.IsApproved {
			continue
		}

		checksumMatch := node.ConfigurationChecksum != "" && ngChecksum != "" && node.ConfigurationChecksum == ngChecksum
		canRunCleanup := checksumMatch && node.IsReady
		if node.IsApproved || node.IsDisruptionApproved || node.IsDrained || node.IsUnschedulable {
			logger.Info(
				"evaluate up-to-date candidate",
				"node", node.Name,
				"nodegroup", ng.Name,
				"isApproved", node.IsApproved,
				"isReady", node.IsReady,
				"isDrained", node.IsDrained,
				"isUnschedulable", node.IsUnschedulable,
				"nodeChecksumEmpty", node.ConfigurationChecksum == "",
				"ngChecksumEmpty", ngChecksum == "",
				"checksumMatch", checksumMatch,
				"canRunCleanup", canRunCleanup,
			)
		}
		if !canRunCleanup {
			logger.V(1).Info(
				"skip up-to-date processing for node",
				"node", node.Name,
				"nodegroup", ng.Name,
				"isApproved", node.IsApproved,
				"isReady", node.IsReady,
				"isDrained", node.IsDrained,
				"isUnschedulable", node.IsUnschedulable,
				"nodeChecksumEmpty", node.ConfigurationChecksum == "",
				"ngChecksumEmpty", ngChecksum == "",
				"checksumMatch", checksumMatch,
				"canRunCleanup", canRunCleanup,
			)
			continue
		}

		logger.Info("node is up to date", "node", node.Name, "nodegroup", ng.Name)
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					ua.ApprovedAnnotation:           nil,
					ua.WaitingForApprovalAnnotation: nil,
					ua.DisruptionRequiredAnnotation: nil,
					ua.DisruptionApprovedAnnotation: nil,
					ua.DrainedAnnotation:            nil,
				},
			},
		}
		if node.IsDrained {
			logger.V(1).Info("up-to-date node is drained, removing unschedulable", "node", node.Name, "nodegroup", ng.Name)
			patch["spec"] = map[string]interface{}{"unschedulable": nil}
		}
		logger.Info(
			"applying up-to-date cleanup patch",
			"node", node.Name,
			"nodegroup", ng.Name,
			"removeUnschedulable", node.IsDrained,
		)
		if err := p.Kube.PatchNode(ctx, node.Name, patch); err != nil {
			return false, err
		}
		uametrics.SetNodeStatusMetrics(node.Name, node.NodeGroup, "UpToDate")
		p.Recorder.Event(ng, corev1.EventTypeNormal, "NodeUpToDate", "Node "+node.Name+" is now up to date")
		return true, nil
	}
	return false, nil
}

func (p Processor) ApproveDisruptions(ctx context.Context, ng *v1.NodeGroup, nodes []ua.NodeInfo) (bool, error) {
	logger := log.FromContext(ctx)
	approvalMode := ua.GetApprovalMode(ng)

	now := time.Now()
	if os.Getenv("D8_IS_TESTS_ENVIRONMENT") != "" {
		now = time.Date(2021, 1, 1, 13, 30, 0, 0, time.UTC)
	}

	for _, node := range nodes {
		if !node.IsApproved || node.IsDraining || (!node.IsDisruptionRequired && !node.IsRollingUpdate) || node.IsDisruptionApproved {
			continue
		}

		switch approvalMode {
		case "Manual":
			continue
		case "Automatic":
			var windows []v1.DisruptionWindow
			if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.Automatic != nil {
				windows = ng.Spec.Disruptions.Automatic.Windows
			}
			if !ua.IsInAllowedWindow(windows, now) {
				continue
			}
		case "RollingUpdate":
			var windows []v1.DisruptionWindow
			if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.RollingUpdate != nil {
				windows = ng.Spec.Disruptions.RollingUpdate.Windows
			}
			if !ua.IsInAllowedWindow(windows, now) {
				continue
			}
		}

		switch {
		case approvalMode == "RollingUpdate":
			logger.Info("deleting instance for rolling update", "node", node.Name, "nodegroup", ng.Name)
			if err := p.Kube.DeleteInstance(ctx, node.Name); err != nil {
				return false, err
			}
			p.Recorder.Event(ng, corev1.EventTypeNormal, "RollingUpdate", "Deleting instance "+node.Name+" for rolling update")
			return true, nil

		case !p.NeedDrainNode(ctx, &node, ng) || node.IsDrained:
			logger.Info("approving disruption", "node", node.Name, "nodegroup", ng.Name)
			patch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						ua.DisruptionApprovedAnnotation: "",
						ua.DisruptionRequiredAnnotation: nil,
					},
				},
			}
			if err := p.Kube.PatchNode(ctx, node.Name, patch); err != nil {
				return false, err
			}
			uametrics.SetNodeStatusMetrics(node.Name, node.NodeGroup, "DisruptionApproved")
			p.Recorder.Event(ng, corev1.EventTypeNormal, "DisruptionApproved", "Disruption approved for node "+node.Name)
			return true, nil

		case !node.IsUnschedulable:
			logger.Info("starting drain for disruption", "node", node.Name, "nodegroup", ng.Name)
			patch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						ua.DrainingAnnotation: "bashible",
					},
				},
			}
			if err := p.Kube.PatchNode(ctx, node.Name, patch); err != nil {
				return false, err
			}
			uametrics.SetNodeStatusMetrics(node.Name, node.NodeGroup, "DrainingForDisruption")
			p.Recorder.Event(ng, corev1.EventTypeNormal, "DrainingForDisruption", "Draining node "+node.Name+" for disruption")
			return true, nil
		}
	}

	return false, nil
}

func (p Processor) ApproveUpdates(ctx context.Context, ng *v1.NodeGroup, nodes []ua.NodeInfo) (bool, error) {
	logger := log.FromContext(ctx)
	var maxConcurrent = ng.Spec.Update
	var max *intstr.IntOrString
	if maxConcurrent != nil {
		max = maxConcurrent.MaxConcurrent
	}
	concurrency := ua.CalculateConcurrency(max, len(nodes))

	currentUpdates := 0
	hasWaiting := false
	for _, node := range nodes {
		if node.IsApproved {
			currentUpdates++
		}
		if node.IsWaitingForApproval {
			hasWaiting = true
		}
	}
	if currentUpdates >= concurrency || !hasWaiting {
		return false, nil
	}

	countToApprove := concurrency - currentUpdates
	approvedNodes := make([]ua.NodeInfo, 0, countToApprove)

	if ng.Status.Desired <= ng.Status.Ready || ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		allReady := true
		for _, node := range nodes {
			if !node.IsReady {
				allReady = false
				break
			}
		}
		if allReady {
			for _, node := range nodes {
				if node.IsWaitingForApproval {
					approvedNodes = append(approvedNodes, node)
					if len(approvedNodes) >= countToApprove {
						break
					}
				}
			}
		}
	}

	if len(approvedNodes) < countToApprove {
		for _, node := range nodes {
			if node.IsReady || !node.IsWaitingForApproval {
				continue
			}
			already := false
			for _, approved := range approvedNodes {
				if approved.Name == node.Name {
					already = true
					break
				}
			}
			if !already {
				approvedNodes = append(approvedNodes, node)
				if len(approvedNodes) >= countToApprove {
					break
				}
			}
		}
	}

	if len(approvedNodes) == 0 {
		return false, nil
	}

	for _, node := range approvedNodes {
		logger.Info("approving node update", "node", node.Name, "nodegroup", ng.Name)
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					ua.ApprovedAnnotation:           "",
					ua.WaitingForApprovalAnnotation: nil,
				},
			},
		}
		if err := p.Kube.PatchNode(ctx, node.Name, patch); err != nil {
			return false, err
		}
		uametrics.SetNodeStatusMetrics(node.Name, node.NodeGroup, "Approved")
		p.Recorder.Event(ng, corev1.EventTypeNormal, "NodeApproved", "Update approved for node "+node.Name)
	}
	return true, nil
}

func (p Processor) NeedDrainNode(ctx context.Context, node *ua.NodeInfo, ng *v1.NodeGroup) bool {
	logger := log.FromContext(ctx)

	if ng.Name == "master" && ng.Status.Nodes == 1 {
		logger.Info("skip drain single control-plane node")
		return false
	}
	if node.Name == p.DeckhouseNodeName && ng.Status.Ready < 2 {
		logger.Info("skip drain node with deckhouse pod because node-group contains single node and deckhouse will not run after drain",
			"node", node.Name, "node_group", ng.Name)
		return false
	}
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.Automatic != nil &&
		ng.Spec.Disruptions.Automatic.DrainBeforeApproval != nil {
		return *ng.Spec.Disruptions.Automatic.DrainBeforeApproval
	}
	return true
}
