/*
Copyright 2025 Flant JSC

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

package updateapproval

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/internal/controller"
)

var (
	nodeGroupNodeStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "node_group_node_status",
			Help: "Status of a node within a node group for update approval",
		},
		[]string{"node", "node_group", "status"},
	)

	metricStatuses = []string{
		"WaitingForApproval", "Approved", "DrainingForDisruption", "Draining", "Drained",
		"WaitingForDisruptionApproval", "WaitingForManualDisruptionApproval", "DisruptionApproved",
		"ToBeUpdated", "UpToDate", "UpdateFailedNoConfigChecksum",
	}
)

func init() {
	ctrlmetrics.Registry.MustRegister(nodeGroupNodeStatus)
	controller.Register("UpdateApproval", Setup)
}

const (
	// NodeGroupLabel is the label on Node that indicates which NodeGroup it belongs to
	NodeGroupLabel = "node.deckhouse.io/group"

	// ConfigurationChecksumAnnotation is the annotation on Node with configuration checksum
	ConfigurationChecksumAnnotation = "node.deckhouse.io/configuration-checksum"

	// MachineNamespace is the namespace where configuration checksums secret is stored
	MachineNamespace = "d8-cloud-instance-manager"

	// ConfigurationChecksumsSecretName is the name of the secret with configuration checksums
	ConfigurationChecksumsSecretName = "configuration-checksums"

	// Update approval annotations
	ApprovedAnnotation           = "update.node.deckhouse.io/approved"
	WaitingForApprovalAnnotation = "update.node.deckhouse.io/waiting-for-approval"
	DisruptionRequiredAnnotation = "update.node.deckhouse.io/disruption-required"
	DisruptionApprovedAnnotation = "update.node.deckhouse.io/disruption-approved"
	RollingUpdateAnnotation      = "update.node.deckhouse.io/rolling-update"
	DrainingAnnotation           = "update.node.deckhouse.io/draining"
	DrainedAnnotation            = "update.node.deckhouse.io/drained"
)

// Reconciler handles node update approvals.
//
// It watches NodeGroup and Node resources and manages the update approval workflow:
// 1. processUpdatedNodes - marks nodes as UpToDate when checksum matches
// 2. approveDisruptions - approves disruptions for nodes in Automatic mode
// 3. approveUpdates - approves updates respecting concurrency limits
type Reconciler struct {
	Client   client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder

	// deckhouseNodeName is the name of the node running deckhouse
	deckhouseNodeName string
}

// Setup registers the UpdateApproval controller with the manager.
func Setup(mgr ctrl.Manager) error {
	return (&Reconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		Recorder:          mgr.GetEventRecorderFor("node-controller"),
		deckhouseNodeName: os.Getenv("DECKHOUSE_NODE_NAME"),
	}).SetupWithManager(mgr)
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Only process Node events for nodes that belong to a NodeGroup
	nodeHasGroupLabel := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, exists := obj.GetLabels()[NodeGroupLabel]
		return exists
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.NodeGroup{}).
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.nodeToNodeGroup),
			builder.WithPredicates(nodeHasGroupLabel),
		).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.secretToAllNodeGroups),
			builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
				return obj.GetNamespace() == MachineNamespace && obj.GetName() == ConfigurationChecksumsSecretName
			})),
		).
		Named("update-approval").
		Complete(r)
}

// nodeToNodeGroup maps Node events to NodeGroup reconcile requests.
func (r *Reconciler) nodeToNodeGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	node, ok := obj.(*corev1.Node)
	if !ok {
		return nil
	}

	ngName, exists := node.Labels[NodeGroupLabel]
	if !exists {
		return nil
	}

	return []reconcile.Request{
		{NamespacedName: types.NamespacedName{Name: ngName}},
	}
}

// secretToAllNodeGroups maps Secret events to all NodeGroup reconcile requests.
func (r *Reconciler) secretToAllNodeGroups(ctx context.Context, obj client.Object) []reconcile.Request {
	// When configuration checksums secret changes, reconcile all NodeGroups
	ngList := &v1.NodeGroupList{}
	if err := r.Client.List(ctx, ngList); err != nil {
		return nil
	}

	requests := make([]reconcile.Request, 0, len(ngList.Items))
	for _, ng := range ngList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: ng.Name},
		})
	}
	return requests
}

// Reconcile handles update approval for a NodeGroup.
//
// The original hook processes one action at a time (sets finished=true after first mutation).
// This ensures that each reconcile loop performs at most one state change, matching the
// step-by-step behavior of the addon-operator hook.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.V(1).Info("reconciling update approval", "nodegroup", req.Name)

	// Get NodeGroup
	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Get configuration checksums
	checksums := r.getConfigurationChecksums(ctx)
	if len(checksums) == 0 {
		logger.V(1).Info("no configuration checksums secret found, skipping")
		return ctrl.Result{}, nil
	}
	ngChecksum := checksums[ng.Name]

	// Get nodes for this NodeGroup
	nodes, err := r.getNodesForNodeGroup(ctx, ng.Name)
	if err != nil {
		logger.Error(err, "failed to get nodes")
		return ctrl.Result{}, err
	}

	// Build node info list
	nodeInfos := make([]nodeInfo, 0, len(nodes))
	for _, node := range nodes {
		nodeInfos = append(nodeInfos, r.buildNodeInfo(&node))
	}

	// Set metrics for all nodes
	for _, node := range nodeInfos {
		r.setNodeMetrics(node, ng, ngChecksum)
	}

	// Step 1: Process updated nodes (mark as UpToDate)
	finished, err := r.processUpdatedNodes(ctx, ng, nodeInfos, ngChecksum)
	if err != nil {
		return ctrl.Result{}, err
	}
	if finished {
		return ctrl.Result{}, nil
	}

	// Step 2: Approve disruptions
	finished, err = r.approveDisruptions(ctx, ng, nodeInfos)
	if err != nil {
		return ctrl.Result{}, err
	}
	if finished {
		return ctrl.Result{}, nil
	}

	// Step 3: Approve updates
	_, err = r.approveUpdates(ctx, ng, nodeInfos)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// nodeInfo contains processed node information for update approval logic.
type nodeInfo struct {
	Name      string
	NodeGroup string

	ConfigurationChecksum string

	IsReady              bool
	IsApproved           bool
	IsDisruptionApproved bool
	IsWaitingForApproval bool
	IsDisruptionRequired bool
	IsUnschedulable      bool
	IsDraining           bool
	IsDrained            bool
	IsRollingUpdate      bool
}

// buildNodeInfo extracts relevant information from a Node for update approval.
func (r *Reconciler) buildNodeInfo(node *corev1.Node) nodeInfo {
	annotations := node.Annotations
	if annotations == nil {
		annotations = make(map[string]string)
	}

	info := nodeInfo{
		Name:                  node.Name,
		NodeGroup:             node.Labels[NodeGroupLabel],
		ConfigurationChecksum: annotations[ConfigurationChecksumAnnotation],
		IsUnschedulable:       node.Spec.Unschedulable,
	}

	// Check annotations
	_, info.IsApproved = annotations[ApprovedAnnotation]
	_, info.IsWaitingForApproval = annotations[WaitingForApprovalAnnotation]
	_, info.IsDisruptionRequired = annotations[DisruptionRequiredAnnotation]
	_, info.IsDisruptionApproved = annotations[DisruptionApprovedAnnotation]
	_, info.IsRollingUpdate = annotations[RollingUpdateAnnotation]

	// Draining/Drained annotations are set by bashible
	if v, ok := annotations[DrainingAnnotation]; ok && v == "bashible" {
		info.IsDraining = true
	}
	if v, ok := annotations[DrainedAnnotation]; ok && v == "bashible" {
		info.IsDrained = true
	}

	// Check Ready condition
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady && cond.Status == corev1.ConditionTrue {
			info.IsReady = true
			break
		}
	}

	return info
}

// processUpdatedNodes removes update annotations from nodes that are up to date.
// Returns finished=true after first mutation, matching original hook behavior.
func (r *Reconciler) processUpdatedNodes(ctx context.Context, ng *v1.NodeGroup, nodes []nodeInfo, ngChecksum string) (bool, error) {
	logger := log.FromContext(ctx)

	for _, node := range nodes {
		if !node.IsApproved {
			continue
		}
		if node.ConfigurationChecksum == "" || ngChecksum == "" {
			continue
		}
		if node.ConfigurationChecksum != ngChecksum {
			continue
		}
		if !node.IsReady {
			continue
		}

		// Node is up to date - remove all update annotations
		logger.Info("node is up to date", "node", node.Name, "nodegroup", ng.Name)

		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					ApprovedAnnotation:           nil,
					WaitingForApprovalAnnotation: nil,
					DisruptionRequiredAnnotation: nil,
					DisruptionApprovedAnnotation: nil,
					DrainedAnnotation:            nil,
				},
			},
		}

		if node.IsDrained {
			patch["spec"] = map[string]interface{}{
				"unschedulable": nil,
			}
		}

		if err := r.patchNode(ctx, node.Name, patch); err != nil {
			return false, err
		}

		r.Recorder.Event(ng, corev1.EventTypeNormal, "NodeUpToDate",
			"Node "+node.Name+" is now up to date")
		return true, nil
	}

	return false, nil
}

// approveDisruptions approves disruptions for nodes in Automatic/RollingUpdate mode.
// Returns finished=true after first mutation, matching original hook behavior.
func (r *Reconciler) approveDisruptions(ctx context.Context, ng *v1.NodeGroup, nodes []nodeInfo) (bool, error) {
	logger := log.FromContext(ctx)

	approvalMode := getApprovalMode(ng)
	now := time.Now()

	for _, node := range nodes {
		if !node.IsApproved {
			continue
		}
		if node.IsDraining || (!node.IsDisruptionRequired && !node.IsRollingUpdate) || node.IsDisruptionApproved {
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
			if !isInAllowedWindow(windows, now) {
				continue
			}

		case "RollingUpdate":
			var windows []v1.DisruptionWindow
			if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.RollingUpdate != nil {
				windows = ng.Spec.Disruptions.RollingUpdate.Windows
			}
			if !isInAllowedWindow(windows, now) {
				continue
			}
		}

		switch {
		case approvalMode == "RollingUpdate":
			// For RollingUpdate mode, delete the Instance
			logger.Info("deleting instance for rolling update", "node", node.Name, "nodegroup", ng.Name)
			if err := r.deleteInstance(ctx, node.Name); err != nil {
				return false, err
			}
			r.Recorder.Event(ng, corev1.EventTypeNormal, "RollingUpdate",
				"Deleting instance "+node.Name+" for rolling update")
			return true, nil

		case !r.needDrainNode(&node, ng) || node.IsDrained:
			// Approve disruption
			logger.Info("approving disruption", "node", node.Name, "nodegroup", ng.Name)
			patch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						DisruptionApprovedAnnotation: "",
						DisruptionRequiredAnnotation: nil,
					},
				},
			}
			if err := r.patchNode(ctx, node.Name, patch); err != nil {
				return false, err
			}
			r.Recorder.Event(ng, corev1.EventTypeNormal, "DisruptionApproved",
				"Disruption approved for node "+node.Name)
			return true, nil

		case !node.IsUnschedulable:
			// Start draining
			logger.Info("starting drain for disruption", "node", node.Name, "nodegroup", ng.Name)
			patch := map[string]interface{}{
				"metadata": map[string]interface{}{
					"annotations": map[string]interface{}{
						DrainingAnnotation: "bashible",
					},
				},
			}
			if err := r.patchNode(ctx, node.Name, patch); err != nil {
				return false, err
			}
			r.Recorder.Event(ng, corev1.EventTypeNormal, "DrainingForDisruption",
				"Draining node "+node.Name+" for disruption")
			return true, nil
		}
	}

	return false, nil
}

// approveUpdates approves updates for nodes respecting concurrency limits.
// Returns finished=true after first mutation, matching original hook behavior.
//
// Original hook logic:
//   - Allow if (ng.Status.Desired <= ng.Status.Ready) OR (ng.NodeType != CloudEphemeral),
//     AND all nodes in the group are ready.
//   - If quota not filled, also approve not-ready waiting nodes.
func (r *Reconciler) approveUpdates(ctx context.Context, ng *v1.NodeGroup, nodes []nodeInfo) (bool, error) {
	logger := log.FromContext(ctx)

	// Calculate concurrency
	var maxConcurrent *intstr.IntOrString
	if ng.Spec.Update != nil {
		maxConcurrent = ng.Spec.Update.MaxConcurrent
	}
	concurrency := calculateConcurrency(maxConcurrent, len(nodes))

	// Count already approved nodes and check if any are waiting
	currentUpdates := 0
	hasWaitingForApproval := false

	for _, node := range nodes {
		if node.IsApproved {
			currentUpdates++
		}
		if node.IsWaitingForApproval {
			hasWaitingForApproval = true
		}
	}

	// Skip if max concurrent reached or no waiting nodes
	if currentUpdates >= concurrency || !hasWaitingForApproval {
		return false, nil
	}

	countToApprove := concurrency - currentUpdates
	approvedNodes := make([]nodeInfo, 0, countToApprove)

	// Match original logic:
	//   if ng.Status.Desired <= ng.Status.Ready || ng.NodeType != ngv1.NodeTypeCloudEphemeral {
	//       allReady := true ... if allReady { approve waiting nodes }
	//   }
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

	// If we haven't filled quota, approve not-ready waiting nodes
	if len(approvedNodes) < countToApprove {
		for _, node := range nodes {
			if !node.IsReady && node.IsWaitingForApproval {
				// Check if already added
				alreadyAdded := false
				for _, an := range approvedNodes {
					if an.Name == node.Name {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					approvedNodes = append(approvedNodes, node)
					if len(approvedNodes) >= countToApprove {
						break
					}
				}
			}
		}
	}

	if len(approvedNodes) == 0 {
		return false, nil
	}

	// Approve selected nodes
	for _, node := range approvedNodes {
		logger.Info("approving node update", "node", node.Name, "nodegroup", ng.Name)
		patch := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					ApprovedAnnotation:           "",
					WaitingForApprovalAnnotation: nil,
				},
			},
		}
		if err := r.patchNode(ctx, node.Name, patch); err != nil {
			return false, err
		}
		r.Recorder.Event(ng, corev1.EventTypeNormal, "NodeApproved",
			"Update approved for node "+node.Name)
	}

	return true, nil
}

// needDrainNode determines if a node needs to be drained before disruption.
func (r *Reconciler) needDrainNode(node *nodeInfo, ng *v1.NodeGroup) bool {
	// Can't drain single control-plane node because deckhouse webhook will evict
	// and deckhouse will malfunction; draining single node does not matter, we always
	// reboot single control plane node without problem
	if ng.Name == "master" && ng.Status.Nodes == 1 {
		return false
	}

	// Can't drain node with deckhouse if it's the only ready node
	if node.Name == r.deckhouseNodeName && ng.Status.Ready < 2 {
		return false
	}

	// Check DrainBeforeApproval setting (default true, matching original hook)
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.Automatic != nil &&
		ng.Spec.Disruptions.Automatic.DrainBeforeApproval != nil {
		return *ng.Spec.Disruptions.Automatic.DrainBeforeApproval
	}
	return true
}

// Helper functions

func (r *Reconciler) getNodesForNodeGroup(ctx context.Context, ngName string) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabels{NodeGroupLabel: ngName}); err != nil {
		return nil, err
	}
	return nodeList.Items, nil
}

func (r *Reconciler) getConfigurationChecksums(ctx context.Context) map[string]string {
	secret := &corev1.Secret{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: MachineNamespace,
		Name:      ConfigurationChecksumsSecretName,
	}, secret)
	if err != nil {
		return make(map[string]string)
	}

	checksums := make(map[string]string)
	for k, v := range secret.Data {
		checksums[k] = string(v)
	}
	return checksums
}

func (r *Reconciler) patchNode(ctx context.Context, nodeName string, patch map[string]interface{}) error {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	return r.Client.Patch(ctx, node, client.RawPatch(types.MergePatchType, patchBytes))
}

func (r *Reconciler) deleteInstance(ctx context.Context, instanceName string) error {
	// Delete Instance resource (deckhouse.io/v1alpha1)
	// Using unstructured to avoid importing the Instance type
	instance := &unstructured.Unstructured{}
	instance.SetAPIVersion("deckhouse.io/v1alpha1")
	instance.SetKind("Instance")
	instance.SetName(instanceName)

	return client.IgnoreNotFound(r.Client.Delete(ctx, instance))
}

func getApprovalMode(ng *v1.NodeGroup) string {
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.ApprovalMode != "" {
		return string(ng.Spec.Disruptions.ApprovalMode)
	}
	return "Automatic"
}

func calculateConcurrency(maxConcurrent *intstr.IntOrString, totalNodes int) int {
	if maxConcurrent == nil {
		return 1
	}

	switch maxConcurrent.Type {
	case intstr.Int:
		return maxConcurrent.IntValue()

	case intstr.String:
		s := maxConcurrent.String()
		if strings.HasSuffix(s, "%") {
			percentStr := strings.TrimSuffix(s, "%")
			percent, _ := strconv.Atoi(percentStr)
			concurrency := totalNodes * percent / 100
			if concurrency == 0 {
				concurrency = 1
			}
			return concurrency
		}
		return maxConcurrent.IntValue()
	}

	return 1
}

// isInAllowedWindow checks if current time is within any of the allowed disruption windows.
func isInAllowedWindow(windows []v1.DisruptionWindow, now time.Time) bool {
	if len(windows) == 0 {
		return true // No windows = always allowed
	}

	for _, w := range windows {
		if isWindowAllowed(w, now) {
			return true
		}
	}
	return false
}

// isWindowAllowed checks if the given time falls within a single disruption window.
func isWindowAllowed(w v1.DisruptionWindow, now time.Time) bool {
	// Check day of week if days are specified
	if len(w.Days) > 0 {
		currentDay := now.Weekday().String()
		dayMatch := false
		for _, d := range w.Days {
			if strings.EqualFold(d, currentDay) {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			return false
		}
	}

	// Parse From and To times (expected format "HH:MM")
	fromParts := strings.Split(w.From, ":")
	toParts := strings.Split(w.To, ":")
	if len(fromParts) != 2 || len(toParts) != 2 {
		return false
	}

	fromHour, err1 := strconv.Atoi(fromParts[0])
	fromMin, err2 := strconv.Atoi(fromParts[1])
	toHour, err3 := strconv.Atoi(toParts[0])
	toMin, err4 := strconv.Atoi(toParts[1])
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return false
	}

	nowMinutes := now.Hour()*60 + now.Minute()
	fromMinutes := fromHour*60 + fromMin
	toMinutes := toHour*60 + toMin

	if fromMinutes <= toMinutes {
		return nowMinutes >= fromMinutes && nowMinutes < toMinutes
	}
	// Window crosses midnight
	return nowMinutes >= fromMinutes || nowMinutes < toMinutes
}

// setNodeMetrics sets prometheus metrics for node status, matching original hook behavior.
func (r *Reconciler) setNodeMetrics(node nodeInfo, ng *v1.NodeGroup, desiredChecksum string) {
	nodeStatus := calculateNodeStatus(node, ng, desiredChecksum)
	for _, status := range metricStatuses {
		var value float64
		if status == nodeStatus {
			value = 1
		}
		nodeGroupNodeStatus.WithLabelValues(node.Name, node.NodeGroup, status).Set(value)
	}
}

// calculateNodeStatus determines node status for metrics, matching original hook logic exactly.
func calculateNodeStatus(node nodeInfo, ng *v1.NodeGroup, desiredChecksum string) string {
	approvalMode := getApprovalMode(ng)

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

	case node.ConfigurationChecksum == "":
		return "UpdateFailedNoConfigChecksum"

	case node.ConfigurationChecksum != desiredChecksum:
		return "ToBeUpdated"

	case node.ConfigurationChecksum == desiredChecksum:
		return "UpToDate"

	case node.IsRollingUpdate:
		return "RollingUpdate"

	default:
		return "Unknown"
	}
}
