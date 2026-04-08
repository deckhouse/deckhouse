package status

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/mcm.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/dynr"
	"github.com/deckhouse/node-controller/internal/rcname"
)

func init() {
	dynr.RegisterReconciler(rcname.NodeGroupStatus, &deckhousev1.NodeGroup{}, &NodeGroupStatusReconciler{})
}

var _ dynr.Reconciler = (*NodeGroupStatusReconciler)(nil)

type NodeGroupStatusReconciler struct {
	dynr.Base
}

func (r *NodeGroupStatusReconciler) SetupWatches(w dynr.Watcher) {
	w.
		Watches(
			&corev1.Node{},
			handler.EnqueueRequestsFromMapFunc(r.nodeToNodeGroup),
		).
		Watches(
			&deckhousev1alpha1.Instance{},
			handler.EnqueueRequestsFromMapFunc(r.instanceToNodeGroup),
		).
		Watches(
			&mcmv1alpha1.MachineDeployment{},
			handler.EnqueueRequestsFromMapFunc(r.machineDeploymentToNodeGroup),
		)
}

func (r *NodeGroupStatusReconciler) nodeToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	ngName, ok := obj.GetLabels()["node.deckhouse.io/group"]
	if !ok || ngName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: ngName}}}
}

func (r *NodeGroupStatusReconciler) instanceToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	ngName := obj.GetLabels()["node.deckhouse.io/group"]
	if ngName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: ngName}}}
}

func (r *NodeGroupStatusReconciler) machineDeploymentToNodeGroup(_ context.Context, obj client.Object) []reconcile.Request {
	ngName, ok := obj.GetLabels()["node-group"]
	if !ok || ngName == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: ngName}}}
}

func (r *NodeGroupStatusReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, ng); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList, client.MatchingLabelsSelector{
		Selector: labels.SelectorFromSet(labels.Set{"node.deckhouse.io/group": ng.Name}),
	}); err != nil {
		return ctrl.Result{}, fmt.Errorf("list nodes for node group %s: %w", ng.Name, err)
	}

	mdList := &mcmv1alpha1.MachineDeploymentList{}
	if err := r.Client.List(ctx, mdList, client.InNamespace("d8-cloud-instance-manager"), client.MatchingLabels{"node-group": ng.Name}); err != nil {
		return ctrl.Result{}, fmt.Errorf("list machine deployments for node group %s: %w", ng.Name, err)
	}

	instanceList := &deckhousev1alpha1.InstanceList{}
	if err := r.Client.List(ctx, instanceList); err != nil {
		return ctrl.Result{}, fmt.Errorf("list instances: %w", err)
	}

	agg := r.aggregate(ng, nodeList.Items, mdList.Items, instanceList.Items)

	if err := r.updateStatus(ctx, ng, agg); err != nil {
		return ctrl.Result{}, fmt.Errorf("update status for node group %s: %w", ng.Name, err)
	}

	log.V(1).Info("reconciled node group status", "nodeGroup", ng.Name, "nodes", agg.nodes, "ready", agg.ready, "upToDate", agg.upToDate)
	return ctrl.Result{}, nil
}

type aggregation struct {
	nodes     int32
	ready     int32
	upToDate  int32
	min       int32
	max       int32
	desired   int32
	instances int32

	lastMachineFailures []deckhousev1.MachineFailure
	failureMessage      string
}

func (r *NodeGroupStatusReconciler) aggregate(
	ng *deckhousev1.NodeGroup,
	nodes []corev1.Node,
	mds []mcmv1alpha1.MachineDeployment,
	instances []deckhousev1alpha1.Instance,
) aggregation {
	var agg aggregation

	configChecksum := ng.Annotations["node.deckhouse.io/configuration-checksum"]

	for i := range nodes {
		node := &nodes[i]
		agg.nodes++

		if isNodeReady(node) {
			agg.ready++
		}

		nodeChecksum := node.Annotations["node.deckhouse.io/configuration-checksum"]
		if nodeChecksum != "" && nodeChecksum == configChecksum {
			agg.upToDate++
		}
	}

	for i := range instances {
		instNG := instances[i].Labels["node.deckhouse.io/group"]
		if instNG == ng.Name {
			agg.instances++
		}
	}

	if ng.Spec.NodeType == deckhousev1.NodeTypeCloudEphemeral && ng.Spec.CloudInstances != nil {
		zonesCount := int32(len(ng.Spec.CloudInstances.Zones))
		if zonesCount == 0 {
			zonesCount = 1
		}

		agg.min = ng.Spec.CloudInstances.MinPerZone * zonesCount
		agg.max = ng.Spec.CloudInstances.MaxPerZone * zonesCount

		for i := range mds {
			md := &mds[i]
			agg.desired += md.Spec.Replicas

			for _, fm := range md.Status.FailedMachines {
				if fm != nil {
					agg.lastMachineFailures = append(agg.lastMachineFailures, convertMachineSummary(fm))
				}
			}
		}

		if agg.min > agg.desired {
			agg.desired = agg.min
		}

		if len(agg.lastMachineFailures) > 0 {
			sort.Slice(agg.lastMachineFailures, func(i, j int) bool {
				return agg.lastMachineFailures[i].LastOperation.LastUpdateTime < agg.lastMachineFailures[j].LastOperation.LastUpdateTime
			})
			last := agg.lastMachineFailures[len(agg.lastMachineFailures)-1]
			if last.LastOperation != nil {
				agg.failureMessage = last.LastOperation.Description
			}
		}
	}

	return agg
}

func (r *NodeGroupStatusReconciler) updateStatus(ctx context.Context, ng *deckhousev1.NodeGroup, agg aggregation) error {
	patch := client.MergeFrom(ng.DeepCopy())

	ng.Status.Nodes = agg.nodes
	ng.Status.Ready = agg.ready
	ng.Status.UpToDate = agg.upToDate

	if ng.Spec.NodeType == deckhousev1.NodeTypeCloudEphemeral {
		ng.Status.Min = agg.min
		ng.Status.Max = agg.max
		ng.Status.Desired = agg.desired
		ng.Status.Instances = agg.instances
		ng.Status.LastMachineFailures = agg.lastMachineFailures
	} else {
		ng.Status.Min = 0
		ng.Status.Max = 0
		ng.Status.Desired = 0
		ng.Status.Instances = 0
		ng.Status.LastMachineFailures = nil
	}

	statusMsg := buildStatusMessage(ng.Status.Error, agg.failureMessage)

	ready := "True"
	if statusMsg != "" {
		ready = "False"
	}

	ng.Status.ConditionSummary = &deckhousev1.ConditionSummary{
		Ready:         ready,
		StatusMessage: statusMsg,
	}

	return r.Client.Status().Patch(ctx, ng, patch)
}

func buildStatusMessage(ngError, failureReason string) string {
	msg := strings.TrimSpace(ngError + " " + failureReason)
	if msg == "" {
		return ""
	}
	if len(msg) > 1024 {
		msg = msg[:1024]
	}
	return "Machine creation failed. Check events for details."
}

func isNodeReady(node *corev1.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

func convertMachineSummary(ms *mcmv1alpha1.MachineSummary) deckhousev1.MachineFailure {
	return deckhousev1.MachineFailure{
		Name:     ms.Name,
		OwnerRef: ms.OwnerRef,
		LastOperation: &deckhousev1.MachineLastOperation{
			Description:    ms.LastOperation.Description,
			LastUpdateTime: ms.LastOperation.LastUpdateTime.Format(metav1.RFC3339Micro),
			State:          string(ms.LastOperation.State),
			Type:           string(ms.LastOperation.Type),
		},
	}
}
