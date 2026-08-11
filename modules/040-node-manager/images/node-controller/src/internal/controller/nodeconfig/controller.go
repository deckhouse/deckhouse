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

// Package nodeconfig renders a NodeConfig object for every node of an olcedar
// NodeGroup: the on-node agent reconciles the node towards it. This controller
// writes that desired state from the NodeGroup plus live cluster state.
package nodeconfig

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	deckhousev1alpha1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	internalv1alpha1 "github.com/deckhouse/node-controller/api/internal.deckhouse.io/v1alpha1"
	nodecommon "github.com/deckhouse/node-controller/internal/common"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/derived_status"
	"github.com/deckhouse/node-controller/internal/register"
)

func init() {
	register.RegisterController(controllerName, &corev1.Node{}, &Reconciler{})
}

type Reconciler struct {
	register.Base

	sources *sourceReader
	// derived answers which Kubernetes version this group runs — the same
	// answer bashible gets, so both kinds of node follow one cluster decision.
	derived *derived_status.Service
}

// MaxConcurrentReconciles is one, and this controller is only correct at one:
// the rollout gate is a read-then-write over the whole group with nothing to
// serialize on, so two workers would both see "none updating" and both write.
func (r *Reconciler) MaxConcurrentReconciles() int {
	return 1
}

// Setup wires an uncached reader: the secrets and config maps a NodeConfig is
// rendered from live outside the manager's cache scope.
func (r *Reconciler) Setup(_ context.Context, mgr ctrl.Manager) error {
	r.sources = &sourceReader{Client: r.Client, Reader: mgr.GetAPIReader()}
	r.derived = &derived_status.Service{Client: r.Client, Reader: mgr.GetAPIReader()}
	return nil
}

// ForPredicates drops Node updates that cannot change the render: it reads only
// name, uid, all labels (NERs select on any) and creationTimestamp, so kubelet
// heartbeats are filtered. Applies to the Node watch only; creates/deletes pass.
func (r *Reconciler) ForPredicates() []predicate.Predicate {
	return []predicate.Predicate{predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return nodeRenderInputsChanged(e.ObjectOld, e.ObjectNew)
		},
	}}
}

// nodeRenderInputsChanged reports whether an update touched anything the render
// reads. An event missing either side is passed through: a pass that renders the
// same spec costs a read, and one that never runs costs a node its config.
func nodeRenderInputsChanged(before, after client.Object) bool {
	if before == nil || after == nil {
		return true
	}
	if before.GetUID() != after.GetUID() {
		return true
	}
	createdBefore := before.GetCreationTimestamp()
	createdAfter := after.GetCreationTimestamp()
	if !createdBefore.Equal(&createdAfter) {
		return true
	}
	return !maps.Equal(before.GetLabels(), after.GetLabels())
}

// nodeConfigRolloutInputsChanged reports whether a NodeConfig update touched
// anything a pass reads: Generation stands in for the spec (status subresource),
// the rest is what the rollout gate reads or what decides ownership.
func nodeConfigRolloutInputsChanged(before, after client.Object) bool {
	oldConfig, okOld := before.(*internalv1alpha1.NodeConfig)
	newConfig, okNew := after.(*internalv1alpha1.NodeConfig)
	if !okOld || !okNew {
		return true
	}
	if oldConfig.Generation != newConfig.Generation ||
		oldConfig.Status.AppliedGeneration != newConfig.Status.AppliedGeneration {
		return true
	}
	if (oldConfig.DeletionTimestamp == nil) != (newConfig.DeletionTimestamp == nil) {
		return true
	}
	if !maps.Equal(oldConfig.Labels, newConfig.Labels) ||
		!slices.EqualFunc(oldConfig.OwnerReferences, newConfig.OwnerReferences, ownerRefEqual) {
		return true
	}
	return !conditionEqual(oldConfig.Status.Conditions, newConfig.Status.Conditions, configurationAppliedCondition) ||
		!conditionEqual(oldConfig.Status.Conditions, newConfig.Status.Conditions, disruptionRequiredCondition)
}

func ownerRefEqual(a, b metav1.OwnerReference) bool {
	return a.UID == b.UID && a.Name == b.Name && a.Kind == b.Kind
}

// conditionEqual compares one condition across two status lists by the two
// fields the rollout gate tests it on.
func conditionEqual(before, after []metav1.Condition, name string) bool {
	oldCondition := meta.FindStatusCondition(before, name)
	newCondition := meta.FindStatusCondition(after, name)
	if oldCondition == nil || newCondition == nil {
		return oldCondition == newCondition
	}
	return oldCondition.Status == newCondition.Status &&
		oldCondition.ObservedGeneration == newCondition.ObservedGeneration
}

func (r *Reconciler) SetupWatches(w register.Watcher) {
	// A NodeGroup change affects every node of every group, so it is funnelled
	// into one pass instead of a request per node.
	allMapper := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: allRequestName}}}
	})
	w.Watches(&v1.NodeGroup{}, allMapper)
	// A node reporting its applied spec frees its rollout slot. The predicate is
	// mandatory: the agent republishes status constantly and this mapper enqueues
	// the ALL-nodes key, so unfiltered heartbeats would re-render the whole fleet.
	w.Watches(&internalv1alpha1.NodeConfig{}, allMapper, builder.WithPredicates(predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return nodeConfigRolloutInputsChanged(e.ObjectOld, e.ObjectNew)
		},
	}))
	// A NER change re-renders every node. The CRD is a hard dependency: a watch
	// on a missing kind kills the whole manager after two minutes. Safe because
	// addon-operator applies every enabled module's crds/ before any Helm run.
	w.Watches(&deckhousev1alpha1.NodeExtensionRequest{}, allMapper)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if req.Name == allRequestName {
		return r.reconcileAllNodes(ctx, logger)
	}
	return r.reconcileNode(ctx, req.Name, logger, newPass())
}

// pass memoises what one reconcile pass reads: an all-nodes pass renders every
// node from the same cluster state, and every read goes straight to the API
// server because the manager's cache is too old for these decisions.
type pass struct {
	// inputs is what reading the cluster-wide render inputs produced, keyed by
	// Kubernetes version: the ~6 uncached reads readClusterInputs performs are
	// done once per distinct version rather than once per node.
	inputs map[string]clusterInputsResult
	// versions is the Kubernetes version each group runs, keyed by group name —
	// the memo key for inputs above, and expensive to produce (derived_status
	// lists MachineDeployments/InstanceClasses and unmarshals the catalogue).
	versions map[string]versionResult
	// rollouts is each group's remaining rollout budget, keyed by group name:
	// one NodeConfig listing per group per pass instead of one per node.
	rollouts map[string]*rolloutBudget
}

// clusterInputsResult is one attempt at reading the cluster-wide inputs, kept
// whether it succeeded or not: these inputs fail identically for every node,
// so the failure is memoised as much as the answer.
type clusterInputsResult struct {
	inputs clusterInputs
	err    error
}

// versionResult is one attempt at answering which Kubernetes version a group
// runs, kept whether it succeeded or not, for the same reason as the inputs
// above: the answer is a property of the group, not of the node being rendered.
type versionResult struct {
	version string
	err     error
}

func newPass() *pass {
	return &pass{
		inputs:   map[string]clusterInputsResult{},
		versions: map[string]versionResult{},
		rollouts: map[string]*rolloutBudget{},
	}
}

// clusterInputs returns the render inputs for the group's Kubernetes version,
// reading them once per version per pass and serving the rest from the pass.
func (r *Reconciler) clusterInputs(ctx context.Context, ng *v1.NodeGroup, p *pass) (clusterInputs, error) {
	resolved, ok := p.versions[ng.Name]
	if !ok {
		version, err := resolveKubernetesVersion(ctx, r.derived, ng)
		resolved = versionResult{version: version, err: err}
		p.versions[ng.Name] = resolved
	}
	if resolved.err != nil {
		return clusterInputs{}, resolved.err
	}
	if result, ok := p.inputs[resolved.version]; ok {
		return result.inputs, result.err
	}
	in, err := r.sources.readClusterInputs(ctx, resolved.version)
	p.inputs[resolved.version] = clusterInputsResult{inputs: in, err: err}
	return in, err
}

// reconcileAllNodes re-renders every node of every immutable group. One failure
// does not stop the others; failures are counted, not logged one by one, and
// the first error is returned so the pass is retried with backoff.
func (r *Reconciler) reconcileAllNodes(ctx context.Context, logger logr.Logger) (ctrl.Result, error) {
	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes); err != nil {
		return ctrl.Result{}, fmt.Errorf("list nodes: %w", err)
	}

	var firstErr error
	failed := 0
	p := newPass()
	for i := range nodes.Items {
		// The listing already carries the Node, so it is rendered from that
		// rather than fetched again once per node.
		if _, err := r.reconcileNodeObject(ctx, &nodes.Items[i], logger, p); err != nil {
			logger.V(1).Info("cannot render the NodeConfig of a node", "node", nodes.Items[i].Name, "error", err.Error())
			failed++
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr != nil {
		firstErr = fmt.Errorf("render the NodeConfig of %d of %d nodes: %w", failed, len(nodes.Items), firstErr)
	}

	// Report each request's resolution back on its own status. This runs on the
	// same all-nodes pass a NER change triggers, so editing a request refreshes
	// both the nodes it targets and its status.
	if err := r.reconcileNERStatuses(ctx, logger); err != nil && firstErr == nil {
		firstErr = err
	}

	return ctrl.Result{}, firstErr
}

// reconcileNode brings one node's NodeConfig in line with its NodeGroup. A node
// that is gone, ungrouped, or in a bashible-managed group has no NodeConfig of
// ours; any leftover object is removed.
func (r *Reconciler) reconcileNode(ctx context.Context, nodeName string, logger logr.Logger, p *pass) (ctrl.Result, error) {
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		if apierrors.IsNotFound(err) {
			// The NodeConfig is owned by the Node, so the API server collects
			// it; nothing to do here.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get node %s: %w", nodeName, err)
	}

	return r.reconcileNodeObject(ctx, node, logger, p)
}

// reconcileNodeObject is reconcileNode for a Node the caller already holds.
func (r *Reconciler) reconcileNodeObject(ctx context.Context, node *corev1.Node, logger logr.Logger, p *pass) (ctrl.Result, error) {
	nodeName := node.Name

	ngName := node.Labels[nodecommon.NodeGroupLabel]
	if ngName == "" {
		return ctrl.Result{}, r.deleteOrphaned(ctx, nodeName, logger)
	}

	ng := &v1.NodeGroup{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: ngName}, ng); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.deleteOrphaned(ctx, nodeName, logger)
		}
		return ctrl.Result{}, fmt.Errorf("get NodeGroup %s: %w", ngName, err)
	}

	if ng.Spec.SystemType != v1.SystemTypeImmutable {
		return ctrl.Result{}, r.deleteOrphaned(ctx, nodeName, logger)
	}

	inputs, err := r.clusterInputs(ctx, ng, p)
	if err != nil {
		return ctrl.Result{}, err
	}

	desired := newNodeConfig(ng, node, inputs)
	// The disruption answer must be about the object as it now stands: a cache
	// re-read here returned the pre-patch revision, minting an operation for a
	// generation the node had already left behind.
	current, err := r.apply(ctx, ng, node, desired, logger, p)
	if err != nil {
		return ctrl.Result{}, err
	}
	if current == nil {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.reconcileDisruption(ctx, ng, node, current, logger)
}

// apply creates or patches the object and returns it as it now stands — nil
// when the node has none or another writer got there first. The status belongs
// to the node-local agent and is never touched here.
func (r *Reconciler) apply(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, desired *internalv1alpha1.NodeConfig, logger logr.Logger, p *pass) (*internalv1alpha1.NodeConfig, error) {
	existing := &internalv1alpha1.NodeConfig{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)
	if apierrors.IsNotFound(err) {
		// A payload-provisioned node publishes its own NodeConfig; creating one
		// here would lose the bootstrap-only fields. Only a CloudEphemeral payload
		// is rendered by this controller, so only it can be reproduced.
		if isControlPlaneNode(node) || ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
			logger.V(1).Info("waiting for the payload-provisioned node to publish its own NodeConfig", "node", desired.Name)
			return nil, nil
		}
		if err := r.Client.Create(ctx, desired); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("create NodeConfig %s: %w", desired.Name, err)
		}
		logger.Info("NodeConfig created", "node", desired.Name)
		// A node given its first config is updating. A budget this pass has
		// already read predates the object and has to be told; one read later
		// lists it itself, still at generation 1 with nothing applied.
		if budget, ok := p.rollouts[ng.Name]; ok {
			budget.spend(desired.Name)
		}
		r.recordClampedSettings(ng)
		return desired, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get NodeConfig %s: %w", desired.Name, err)
	}

	reported, err := r.reportedNodeIPs(ctx, node.Name, existing.Spec.Kubelet.NodeIP)
	if err != nil {
		return nil, err
	}
	keepBootstrapOnlyFields(&desired.Spec, &existing.Spec, reported)

	if upToDate(existing, desired) {
		logger.V(1).Info("NodeConfig unchanged", "node", desired.Name)
		return existing, nil
	}

	// A node that just joined is configured immediately; only changes to a node
	// that already has its config wait for a rollout slot.
	budget, err := r.rolloutBudget(ctx, ng, p)
	if err != nil {
		return nil, err
	}
	if !budget.hasSlot(desired.Name) {
		logger.Info("holding the NodeConfig back until the group has a free rollout slot", "node", desired.Name, "nodeGroup", ng.Name)
		return existing, nil
	}

	// Conditional on the revision the decision above was made against: two
	// passes rendering the same node at once would otherwise both write, the
	// second over a slot the first had already spent.
	patch := client.MergeFromWithOptions(existing.DeepCopy(), client.MergeFromWithOptimisticLock{})
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	existing.OwnerReferences = desired.OwnerReferences
	if err := r.Client.Patch(ctx, existing, patch); err != nil {
		// Someone wrote the object between the read and this patch. The node is
		// left to the next pass — the winning write is a NodeConfig change and
		// the watch enqueues one — rather than failing the walk over a race.
		if apierrors.IsConflict(err) {
			logger.V(1).Info("NodeConfig changed while it was being rendered; leaving it to the next pass", "node", desired.Name)
			return nil, nil
		}
		return nil, fmt.Errorf("patch NodeConfig %s: %w", desired.Name, err)
	}
	logger.Info("NodeConfig updated", "node", desired.Name)
	budget.spend(desired.Name)
	r.recordClampedSettings(ng)
	return existing, nil
}

// upToDate reports whether the object already carries everything the patch
// writes. Ownership is compared because the patch writes it: left out, a stale
// or missing ownerReference is never corrected and the object outlives its node.
func upToDate(existing, desired *internalv1alpha1.NodeConfig) bool {
	if !apiequality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		return false
	}
	if !apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) {
		return false
	}
	return apiequality.Semantic.DeepEqual(existing.OwnerReferences, desired.OwnerReferences)
}

// reportedNodeIPs returns the node's reported internal addresses, read from the
// API server (the manager's cache strips Node.status.addresses). Nothing is
// read unless the config pins an address — only the bootstrapped first master.
func (r *Reconciler) reportedNodeIPs(ctx context.Context, nodeName, pinned string) ([]string, error) {
	if pinned == "" {
		return nil, nil
	}
	node := &corev1.Node{}
	if err := r.sources.Reader.Get(ctx, types.NamespacedName{Name: nodeName}, node); err != nil {
		return nil, fmt.Errorf("read the addresses of node %s: %w", nodeName, err)
	}
	var addresses []string
	for _, address := range node.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			addresses = append(addresses, address.Address)
		}
	}
	return addresses, nil
}

// recordClampedSettings tells the operator which NodeGroup settings the render
// did not carry over as written: the agent's schema refuses the value, and
// silent narrowing would leave the group running something nobody configured.
func (r *Reconciler) recordClampedSettings(ng *v1.NodeGroup) {
	if maxPodsClamped(ng) {
		r.Recorder.Event(ng, corev1.EventTypeWarning, "SettingClamped",
			fmt.Sprintf("kubelet.maxPods %d exceeds the %d an immutable node accepts; the nodes of this group are configured with %d",
				*ng.Spec.Kubelet.MaxPods, maxPodsCeiling, maxPodsCeiling))
	}
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.ApprovalMode == v1.DisruptionApprovalModeRollingUpdate {
		r.Recorder.Event(ng, corev1.EventTypeWarning, "SettingClamped",
			fmt.Sprintf("disruptions.approvalMode %s has no counterpart on an immutable node; the nodes of this group are configured with %s",
				v1.DisruptionApprovalModeRollingUpdate, v1.DisruptionApprovalModeAutomatic))
	}
	if staticReservationClamped(ng) {
		r.Recorder.Event(ng, corev1.EventTypeWarning, "SettingClamped",
			fmt.Sprintf("kubelet.resourceReservation.mode %s cannot be honoured on an immutable node, which reserves by capacity or not at all; the nodes of this group are configured with %s",
				resourceReservationModeStatic, resourceReservationModeAuto))
	}
	if ng.Spec.Disruptions != nil {
		var windows []v1.DisruptionWindow
		if ng.Spec.Disruptions.Automatic != nil {
			windows = ng.Spec.Disruptions.Automatic.Windows
		} else if ng.Spec.Disruptions.RollingUpdate != nil {
			windows = ng.Spec.Disruptions.RollingUpdate.Windows
		}
		if len(windows) > 1 {
			r.Recorder.Event(ng, corev1.EventTypeWarning, "SettingClamped",
				fmt.Sprintf("disruptions windows: an immutable node holds a single update window; the nodes of this group are configured with the first of the %d given",
					len(windows)))
		}
	}
}

// maxPodsClamped reports whether the group asks for more pods per node than an
// immutable node accepts.
func maxPodsClamped(ng *v1.NodeGroup) bool {
	if ng.Spec.Kubelet == nil || ng.Spec.Kubelet.MaxPods == nil {
		return false
	}
	return int(*ng.Spec.Kubelet.MaxPods) > maxPodsCeiling
}

// staticReservationClamped reports whether the group asks for a static resource
// reservation, which an immutable node cannot honour.
func staticReservationClamped(ng *v1.NodeGroup) bool {
	if ng.Spec.Kubelet == nil || ng.Spec.Kubelet.ResourceReservation == nil {
		return false
	}
	return ng.Spec.Kubelet.ResourceReservation.Mode == resourceReservationModeStatic
}

// deleteOrphaned removes a NodeConfig this controller no longer owns, for
// instance after a node left an immutable group.
func (r *Reconciler) deleteOrphaned(ctx context.Context, name string, logger logr.Logger) error {
	existing := &internalv1alpha1.NodeConfig{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, existing); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("get NodeConfig %s: %w", name, err)
	}
	if existing.Labels[managedByLabel] != managedByValue {
		return nil
	}
	if err := r.Client.Delete(ctx, existing); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete NodeConfig %s: %w", name, err)
	}
	logger.Info("NodeConfig removed", "node", name)
	return nil
}
