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

// Package machinedrain evicts the workload of a deleted Cluster API Machine
// through a Drain operation. CAPI waits on node-manager's pre-drain hook and
// skips its own drain, so the eviction is seen in the NodeOperation. The
// nodeoperation controller carries the Drain out, this one only creates it and
// removes the hook once it ended.
package machinedrain

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api/controllers/noderefutil"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	deckhousev1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	v1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController(controllerName, &capiv1beta2.Machine{}, &Reconciler{})
}

var (
	_ register.NeedsSetup         = (*Reconciler)(nil)
	_ register.NeedsForPredicates = (*Reconciler)(nil)
)

type Reconciler struct {
	register.Base

	// apiReader reads the node and the Drain past the cache: a Drain the cache
	// has not seen yet would be created twice, and a node it has not caught up
	// with would let the machine go without a drain.
	apiReader client.Reader
}

func (r *Reconciler) Setup(_ context.Context, mgr ctrl.Manager) error {
	r.apiReader = mgr.GetAPIReader()
	return nil
}

// ForPredicates passes only the Machines this controller acts on: deleted ones
// still held by the hook. CAPI rewrites Machine status all the time.
func (r *Reconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.NewPredicateFuncs(func(obj client.Object) bool {
		_, hooked := obj.GetAnnotations()[nodecommon.MachineDrainHookAnnotation]
		return hooked && obj.GetDeletionTimestamp() != nil
	})}
}

// SetupWatches follows what the hook waits on: the Drain, which ends or is
// deleted, and the node, which goes away or becomes unreachable.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	w.Watches(&v1alpha1.NodeOperation{}, handler.EnqueueRequestsFromMapFunc(r.machineOfDrain),
		builder.WithPredicates(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			_, ofMachine := obj.GetLabels()[v1alpha1.NodeOperationMachineUIDLabel]
			return ofMachine
		})))

	// Predicated: every kubelet heartbeat is a Node update.
	w.Watches(&corev1.Node{}, handler.EnqueueRequestsFromMapFunc(machineOfNode),
		builder.WithPredicates(predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				return reachabilityChanged(e.ObjectOld, e.ObjectNew)
			},
		}))
}

func reachabilityChanged(before, after client.Object) bool {
	oldNode, okOld := before.(*corev1.Node)
	newNode, okNew := after.(*corev1.Node)
	if !okOld || !okNew {
		return true
	}
	return noderefutil.IsNodeUnreachable(oldNode) != noderefutil.IsNodeUnreachable(newNode)
}

// machineOfDrain finds the Machine a Drain evicts for by the UID on its label.
func (r *Reconciler) machineOfDrain(ctx context.Context, obj client.Object) []reconcile.Request {
	uid := types.UID(obj.GetLabels()[v1alpha1.NodeOperationMachineUIDLabel])
	machines := &capiv1beta2.MachineList{}
	if err := r.Client.List(ctx, machines, client.InNamespace(nodecommon.MachineNamespace)); err != nil {
		log.FromContext(ctx).Error(err, "list the Machines to find the one of a Drain", "operation", obj.GetName())
		return nil
	}
	for i := range machines.Items {
		if machines.Items[i].UID == uid {
			return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(&machines.Items[i])}}
		}
	}
	return nil
}

// machineOfNode reads the Machine off the annotation CAPI writes onto its node.
func machineOfNode(_ context.Context, obj client.Object) []reconcile.Request {
	name := obj.GetAnnotations()[nodecommon.CAPIMachineAnnotation]
	if name == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: nodecommon.MachineNamespace, Name: name}}}
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	machine := &capiv1beta2.Machine{}
	if err := r.Client.Get(ctx, req.NamespacedName, machine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// A Machine deleted before the hook was set is drained by CAPI, as before.
	if _, hooked := machine.Annotations[nodecommon.MachineDrainHookAnnotation]; !hooked || machine.DeletionTimestamp == nil {
		return ctrl.Result{}, nil
	}

	node, err := r.nodeOf(ctx, machine)
	if err != nil {
		return ctrl.Result{}, err
	}
	// A node that is gone or unreachable has no kubelet to end its pods, and
	// waiting for the Drain deadline would only hold the replacement of a
	// broken machine. Ready=False comes from a kubelet that is up: it drains.
	if node == nil || noderefutil.IsNodeUnreachable(node) {
		return ctrl.Result{}, r.releaseWithoutDrain(ctx, machine, node)
	}

	drain, err := r.drainOf(ctx, machine)
	if err != nil {
		return ctrl.Result{}, err
	}
	if drain == nil {
		return ctrl.Result{}, r.createDrain(ctx, machine, node)
	}
	if terminal(drain) {
		return ctrl.Result{}, r.removeHook(ctx, machine, fmt.Sprintf("the Drain %s is %s", drain.Name, drain.Status.Phase))
	}
	return ctrl.Result{}, nil
}

// nodeOf returns the node of the machine, nil when it has none or it is gone.
func (r *Reconciler) nodeOf(ctx context.Context, machine *capiv1beta2.Machine) (*corev1.Node, error) {
	if machine.Status.NodeRef.Name == "" {
		return nil, nil
	}
	node := &corev1.Node{}
	if err := r.apiReader.Get(ctx, types.NamespacedName{Name: machine.Status.NodeRef.Name}, node); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get node %s of machine %s: %w", machine.Status.NodeRef.Name, machine.Name, err)
	}
	return node, nil
}

// drainOf finds the Drain of this machine by the UID label.
func (r *Reconciler) drainOf(ctx context.Context, machine *capiv1beta2.Machine) (*v1alpha1.NodeOperation, error) {
	list := &v1alpha1.NodeOperationList{}
	if err := r.apiReader.List(ctx, list, client.MatchingLabels{v1alpha1.NodeOperationMachineUIDLabel: string(machine.UID)}); err != nil {
		return nil, fmt.Errorf("list the Drain of machine %s: %w", machine.Name, err)
	}
	if len(list.Items) == 0 {
		return nil, nil
	}
	return &list.Items[0], nil
}

// createDrain asks for the eviction the way a Disruption is asked for in
// internal/controller/nodeconfig/disruption.go: owned by the node, so it goes
// with the node.
func (r *Reconciler) createDrain(ctx context.Context, machine *capiv1beta2.Machine, node *corev1.Node) error {
	group := machine.Labels[machineNodeGroupLabel]
	unlimited, err := r.drainUnlimited(ctx, machine)
	if err != nil {
		return err
	}
	op := &v1alpha1.NodeOperation{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("drain-%s-", node.Name),
			Labels: map[string]string{
				nodecommon.NodeGroupLabel:             group,
				managedByLabel:                        managedByValue,
				v1alpha1.NodeOperationMachineUIDLabel: string(machine.UID),
			},
			Annotations: map[string]string{v1alpha1.NodeOperationMachineAnnotation: machine.Name},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Node",
				Name:       node.Name,
				UID:        node.UID,
			}},
		},
		Spec: v1alpha1.NodeOperationSpec{
			Type:     v1alpha1.NodeOperationTypeDrain,
			NodeName: node.Name,
			Cause:    fmt.Sprintf("Machine %s of NodeGroup %s is deleted", machine.Name, group),
		},
	}
	if unlimited {
		op.Annotations[v1alpha1.NodeOperationUnlimitedDrainAnnotation] = "true"
	}
	if err := r.Client.Create(ctx, op); err != nil {
		return fmt.Errorf("create the Drain of machine %s: %w", machine.Name, err)
	}
	log.FromContext(ctx).Info("draining the node of a deleted machine",
		"machine", machine.Name, "node", node.Name, "operation", op.Name)
	return nil
}

// drainUnlimited keeps the time limit Cluster API had: none for a Static or
// CloudStatic group, whose MachineDeployment sets no drain timeout, unless the
// group sets nodeDrainTimeoutSecond. A group that is gone leaves spec.deletion.
func (r *Reconciler) drainUnlimited(ctx context.Context, machine *capiv1beta2.Machine) (bool, error) {
	group := machine.Labels[machineNodeGroupLabel]
	if group == "" {
		return false, nil
	}
	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: group}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			return ptr.Deref(machine.Spec.Deletion.NodeDrainTimeoutSeconds, 0) <= 0, nil
		}
		return false, fmt.Errorf("get NodeGroup %s: %w", group, err)
	}
	if ng.Spec.NodeDrainTimeoutSecond != nil {
		return false, nil
	}
	return ng.Spec.NodeType == deckhousev1.NodeTypeStatic || ng.Spec.NodeType == deckhousev1.NodeTypeCloudStatic, nil
}

// releaseWithoutDrain lets the machine go at once and says so on its group.
func (r *Reconciler) releaseWithoutDrain(ctx context.Context, machine *capiv1beta2.Machine, node *corev1.Node) error {
	why := "it has no node"
	if node != nil {
		why = fmt.Sprintf("node %s is unreachable: Ready is Unknown", node.Name)
	}
	message := fmt.Sprintf("Machine %s is deleted without a drain: %s", machine.Name, why)
	if err := r.removeHook(ctx, machine, message); err != nil {
		return err
	}
	// Announced once the hook is gone, so a patch that failed and is retried
	// does not announce it twice.
	ng := &deckhousev1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: machine.Labels[machineNodeGroupLabel]}, ng); err != nil {
		log.FromContext(ctx).Info("no NodeGroup to report a machine deleted without a drain on",
			"machine", machine.Name, "error", err.Error())
		return nil
	}
	r.Recorder.Event(ng, corev1.EventTypeWarning, eventDeletedWithoutDrain, message)
	return nil
}

// removeHook patches out the hook and nothing else, so CAPI goes on with the
// deletion.
func (r *Reconciler) removeHook(ctx context.Context, machine *capiv1beta2.Machine, why string) error {
	patch := client.MergeFrom(machine.DeepCopy())
	delete(machine.Annotations, nodecommon.MachineDrainHookAnnotation)
	if err := r.Client.Patch(ctx, machine, patch); err != nil {
		return fmt.Errorf("remove the hook of machine %s: %w", machine.Name, err)
	}
	log.FromContext(ctx).Info("removed the pre-drain hook of a deleted machine", "machine", machine.Name, "reason", why)
	return nil
}

func terminal(op *v1alpha1.NodeOperation) bool {
	return op.Status.Phase == v1alpha1.NodeOperationPhaseCompleted || op.Status.Phase == v1alpha1.NodeOperationPhaseFailed
}
