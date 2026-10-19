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

// Package nodeconfig renders a NodeConfig object for every node of a NodeGroup:
// the whole node for a Deckhouse Engine group, the agent's own part for a group
// bashible manages. The on-node agent reconciles the node towards it.
package nodeconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	"github.com/deckhouse/node-controller/internal/network"
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
	r.sources = &sourceReader{Reader: mgr.GetAPIReader()}
	r.derived = &derived_status.Service{Client: r.Client}
	// The OS image root hash is resolved here rather than while rendering: the
	// cluster publishes one digest at a time, so it is work per release, and doing
	// it in the render would put a registry round trip — and a registry outage —
	// in front of every node's configuration.
	return mgr.Add(&rootHashWatcher{sources: r.sources, resolver: r.sources.rootHashes()})
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
	// A NER change re-renders every node, and only a spec change can: the render
	// reads spec, name and creationTimestamp. The predicate is what breaks the
	// loop of this controller's own status writes re-entering its queue.
	// The CRD is a hard dependency: a watch on a missing kind kills the whole
	// manager after two minutes. Safe because addon-operator applies every
	// enabled module's crds/ before any Helm run.
	w.Watches(&deckhousev1alpha1.NodeExtensionRequest{}, allMapper,
		builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	// A NodeStaticPodRequest change re-renders every node, and only a spec change
	// can: the render reads the manifest and the selector. The predicate is what
	// breaks the loop of this controller's own status writes re-entering its
	// queue. The CRD is a hard dependency for the same reason as the one above.
	w.Watches(&deckhousev1alpha1.NodeStaticPodRequest{}, allMapper,
		builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	// A disruption approval that ends moves nothing on the node when it failed before the
	// node was let go (a drain that timed out), yet the request statuses name it. One that is
	// collected frees an attempt. No other type of operation is read here.
	w.Watches(&deckhousev1alpha1.NodeOperation{}, allMapper, builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isDisruption(e.ObjectNew) && operationPhaseChanged(e.ObjectOld, e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool { return isDisruption(e.Object) },
	}))
	// The registry module's answer to who writes containerd's registry.d. Without
	// this watch the switch reaches no node until something else enqueues a pass,
	// and no resync period is set. The predicate is mandatory: the Secret informer
	// also covers kube-system, and this mapper enqueues the ALL-nodes key, so an
	// unfiltered watch would re-render the whole fleet on every kube-system secret
	// write. Scoped to this one object in common.CacheOptions too.
	w.Watches(&corev1.Secret{}, allMapper, builder.WithPredicates(predicate.NewPredicateFuncs(
		func(obj client.Object) bool {
			return obj.GetNamespace() == d8SystemNS && obj.GetName() == registryBashibleConfigSecret
		},
	)))
	// The image digests and the platform document of the release. Without this
	// watch a new release re-renders nothing until some unrelated input moves:
	// nothing else enqueues a pass, and no resync period is set.
	w.Watches(&corev1.ConfigMap{}, allMapper, builder.WithPredicates(predicate.NewPredicateFuncs(
		func(obj client.Object) bool {
			if obj.GetNamespace() != cloudInstanceManagerNS {
				return false
			}
			return obj.GetName() == imagesDigestsConfigMapName || obj.GetName() == platformTemplatesConfigMapName
		},
	)))
	// Without this watch, an operator editing the network group has nothing enqueuing a pass: it
	// touches neither a NodeGroup nor a Node.
	moduleConfigGVK := network.ModuleConfigGVK()
	watchedModuleConfig := &unstructured.Unstructured{}
	watchedModuleConfig.SetGroupVersionKind(moduleConfigGVK)
	w.Watches(watchedModuleConfig, allMapper, builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return e.Object.GetName() == network.ModuleConfigName
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return e.Object.GetName() == network.ModuleConfigName
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectNew.GetName() == network.ModuleConfigName && network.NetworkGroupChanged(e.ObjectOld, e.ObjectNew)
		},
	}))
}

// operationPhaseChanged reports whether a NodeOperation update moved its phase, the one
// field of it the request statuses follow.
func operationPhaseChanged(before, after client.Object) bool {
	oldOp, okOld := before.(*deckhousev1alpha1.NodeOperation)
	newOp, okNew := after.(*deckhousev1alpha1.NodeOperation)
	if !okOld || !okNew {
		return true
	}
	return oldOp.Status.Phase != newOp.Status.Phase
}

func isDisruption(obj client.Object) bool {
	op, ok := obj.(*deckhousev1alpha1.NodeOperation)
	return !ok || op.Spec.Type == deckhousev1alpha1.NodeOperationTypeDisruption
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if req.Name == allRequestName {
		return r.reconcileAllNodes(ctx, logger)
	}
	return r.reconcileNode(ctx, req.Name, logger, newPass())
}

// reconcileAllNodes re-renders every node of every group. One failure
// does not stop the others; failures are counted, not logged one by one, and
// the first error is returned so the pass is retried with backoff.
func (r *Reconciler) reconcileAllNodes(ctx context.Context, logger logr.Logger) (ctrl.Result, error) {
	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes); err != nil {
		return ctrl.Result{}, fmt.Errorf("list nodes: %w", err)
	}

	// Unhealthy nodes first, the way bashible hands its approvals out: with one
	// slot free, repairing what is broken beats touching what works.
	slices.SortStableFunc(nodes.Items, func(a, b corev1.Node) int {
		switch {
		case nodeIsReady(&a) == nodeIsReady(&b):
			return 0
		case nodeIsReady(&a):
			return 1
		default:
			return -1
		}
	})

	var firstErr error
	failed := 0
	requeue := time.Duration(0)
	p := newPass()
	for i := range nodes.Items {
		// The listing already carries the Node, so it is rendered from that
		// rather than fetched again once per node.
		result, err := r.reconcileNodeObject(ctx, &nodes.Items[i], logger, p)
		if err != nil {
			logger.V(1).Info("cannot render the NodeConfig of a node", "node", nodes.Items[i].Name, "error", err.Error())
			failed++
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		// A node waiting out a disruption backoff has nothing else to wake it,
		// and this pass covers the whole fleet: the soonest wait wins.
		if result.RequeueAfter == 0 {
			continue
		}
		if requeue == 0 || result.RequeueAfter < requeue {
			requeue = result.RequeueAfter
		}
	}
	var errs []error
	if firstErr != nil {
		errs = append(errs, fmt.Errorf("render the NodeConfig of %d of %d nodes: %w", failed, len(nodes.Items), firstErr))
	}

	// Report each request's resolution back on its own status. This runs on the
	// same all-nodes pass a NER change triggers, so editing a request refreshes
	// both the nodes it targets and its status. Run whatever the renders did:
	// these read their own inputs and fail closed on their own read failures, so
	// one unrenderable node must not freeze every request's status.
	if err := r.reconcileNERStatuses(ctx, logger, p); err != nil {
		errs = append(errs, err)
	}
	if err := r.reconcileNSPRStatuses(ctx, logger, p); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return ctrl.Result{}, errors.Join(errs...)
	}

	return ctrl.Result{RequeueAfter: requeue}, nil
}

// nodeIsReady reports the kubelet's own verdict, which is what "broken" means
// here — not whether the configuration converged.
func nodeIsReady(node *corev1.Node) bool {
	for i := range node.Status.Conditions {
		c := &node.Status.Conditions[i]
		if c.Type == corev1.NodeReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

// reconcileNode brings one node's NodeConfig in line with its NodeGroup. A node
// that is gone or ungrouped has no NodeConfig of ours; any leftover object is
// removed.
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

	ng, err := r.nodeGroup(ctx, ngName, p)
	if err != nil {
		return ctrl.Result{}, err
	}
	if ng == nil {
		return ctrl.Result{}, r.deleteOrphaned(ctx, nodeName, logger)
	}

	spec, err := r.renderNode(ctx, ng, node, p)
	if err != nil {
		return ctrl.Result{}, err
	}

	desired := newNodeConfig(ng, node, spec)
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
	// bashible negotiates the disruptions of the nodes it manages itself.
	if IsMutable(ng) {
		return ctrl.Result{}, nil
	}
	return r.reconcileDisruption(ctx, ng, node, current, logger)
}

// renderNode is the spec a node of the group is published: the Engine render for
// an Immutable group, the agent-only one for a group bashible manages.
func (r *Reconciler) renderNode(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, p *pass) (internalv1alpha1.NodeSpec, error) {
	if IsMutable(ng) {
		in, err := r.mutableInputs(ctx, p)
		if err != nil {
			return internalv1alpha1.NodeSpec{}, fmt.Errorf("read the inputs of a bashible node: %w", err)
		}
		return renderMutableSpec(node, ng.Name, in), nil
	}
	in, err := r.clusterInputs(ctx, ng, p)
	if err != nil {
		return internalv1alpha1.NodeSpec{}, fmt.Errorf("read the inputs of an Engine node: %w", err)
	}
	return renderSpec(ng, node, in), nil
}

// apply creates or patches the object and returns it as it now stands — nil
// when the node has none or another writer got there first. The status belongs
// to the node-local agent and is never touched here.
func (r *Reconciler) apply(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, desired *internalv1alpha1.NodeConfig, logger logr.Logger, p *pass) (*internalv1alpha1.NodeConfig, error) {
	if cached := r.unchanged(ctx, desired); cached != nil {
		logger.V(1).Info("NodeConfig spec unchanged", "node", desired.Name)
		return cached, nil
	}

	// Read live: the cache strips managedFields, and takeOver reads them.
	existing := &internalv1alpha1.NodeConfig{}
	err := r.sources.Reader.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)
	if apierrors.IsNotFound(err) {
		return r.create(ctx, ng, node, desired, logger, p)
	}
	if err != nil {
		return nil, fmt.Errorf("get NodeConfig %s: %w", desired.Name, err)
	}

	if existing, err = r.takeOver(ctx, existing, logger); existing == nil {
		return nil, err
	}
	desired.Spec = withoutMachineFields(desired.Spec)
	specChanged := !specUpToDate(existing, desired)

	if !metadataUpToDate(existing, desired) {
		if existing, err = r.patchMetadata(ctx, existing, desired, logger); existing == nil {
			return nil, err
		}
		// Now listed, a node still applying the published spec holds a slot, and a budget this
		// pass read before the write has to be told, as create() does. Another spec is the gate's.
		if !specChanged && !applied(existing) {
			if budget, ok := p.rollouts[ng.Name]; ok && holdsSlot(existing, IsMutable(ng)) {
				budget.spend(desired.Name)
			}
		}
	}
	if !specChanged {
		logger.V(1).Info("NodeConfig spec unchanged", "node", desired.Name)
		return existing, nil
	}

	// A node that just joined is configured immediately; only changes to a node
	// that already has its config wait for a rollout slot.
	budget, err := r.rolloutBudget(ctx, ng, p)
	if err != nil {
		return nil, err
	}
	if !budget.hasSlot(desired.Name) {
		logger.Info("holding the NodeConfig back until the group has a free rollout slot",
			"node", desired.Name, "nodeGroup", ng.Name,
			"waitingFor", budget.holders(), "concurrency", budget.concurrency)
		return existing, nil
	}

	return r.patch(ctx, ng, existing, desired, logger, budget)
}

// unchanged returns the cached object when the pass has nothing to write to it. A node asking
// to be interrupted is read live all the same: the answer creates a NodeOperation, and the
// cache may still hold a request the node has withdrawn.
func (r *Reconciler) unchanged(ctx context.Context, desired *internalv1alpha1.NodeConfig) *internalv1alpha1.NodeConfig {
	cached := &internalv1alpha1.NodeConfig{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name}, cached); err != nil {
		return nil
	}
	if !specUpToDate(cached, desired) || !metadataUpToDate(cached, desired) {
		return nil
	}
	if disruptionRequested(cached) {
		return nil
	}
	return cached
}

// create gives a node its first NodeConfig.
func (r *Reconciler) create(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, desired *internalv1alpha1.NodeConfig, logger logr.Logger, p *pass) (*internalv1alpha1.NodeConfig, error) {
	if nodePublishesFirstObject(ng, node) {
		logger.V(1).Info("waiting for the payload-provisioned node to publish its own NodeConfig", "node", desired.Name)
		return nil, nil
	}
	if err := r.Client.Create(ctx, desired, client.FieldOwner(fieldManager)); err != nil {
		if apierrors.IsAlreadyExists(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("create NodeConfig %s: %w", desired.Name, err)
	}
	logger.Info("NodeConfig created", "node", desired.Name)
	// A node given its first config is updating. A budget this pass has already
	// read predates the object and has to be told; one read later lists it
	// itself, still at generation 1 with nothing applied.
	if budget, ok := p.rollouts[ng.Name]; ok && holdsSlot(desired, IsMutable(ng)) {
		budget.spend(desired.Name)
	}
	r.recordClampedSettings(ng)
	return desired, nil
}

// nodePublishesFirstObject reports whether the first object must come from the node. An
// Engine node other than CloudEphemeral carries bootstrap-only fields nothing here can
// reproduce. A bashible node's document is the render alone, and its agent never registers
// one: this controller is the object's only writer.
func nodePublishesFirstObject(ng *v1.NodeGroup, node *corev1.Node) bool {
	if IsMutable(ng) {
		return false
	}
	return isControlPlaneNode(node) || ng.Spec.NodeType != v1.NodeTypeCloudEphemeral
}

// patch applies the rendered spec onto the object the slot was granted against.
func (r *Reconciler) patch(ctx context.Context, ng *v1.NodeGroup, existing, desired *internalv1alpha1.NodeConfig, logger logr.Logger, budget *rolloutBudget) (*internalv1alpha1.NodeConfig, error) {
	current, err := r.applySpec(ctx, existing, desired.Spec, logger)
	if current == nil {
		return nil, err
	}
	logger.Info("NodeConfig updated", "node", desired.Name)
	if holdsSlot(current, IsMutable(ng)) {
		budget.spend(desired.Name)
	}
	r.recordClampedSettings(ng)
	return current, nil
}

// patchMetadata writes the labels and owner, which need no slot: the agent registers an object
// by name alone, and without them the gate does not list it and nothing removes it with its node.
func (r *Reconciler) patchMetadata(ctx context.Context, existing, desired *internalv1alpha1.NodeConfig, logger logr.Logger) (*internalv1alpha1.NodeConfig, error) {
	updated := existing.DeepCopy()
	updated.Labels = desired.Labels
	updated.OwnerReferences = desired.OwnerReferences
	current, err := r.write(ctx, existing, updated, logger)
	if current == nil {
		return nil, err
	}
	logger.Info("NodeConfig labels and owner updated", "node", desired.Name)
	return current, nil
}

// write patches existing into updated, conditional on the revision read: two passes rendering
// one node at once would otherwise both write, the second over a slot the first had spent.
// Someone writing in between leaves the node to the next pass, which that write enqueues.
func (r *Reconciler) write(ctx context.Context, existing, updated *internalv1alpha1.NodeConfig, logger logr.Logger) (*internalv1alpha1.NodeConfig, error) {
	if err := r.Client.Patch(ctx, updated, client.MergeFromWithOptions(existing, client.MergeFromWithOptimisticLock{})); err != nil {
		if apierrors.IsConflict(err) {
			logger.V(1).Info("NodeConfig changed while it was being rendered; leaving it to the next pass", "node", updated.Name)
			return nil, nil
		}
		return nil, fmt.Errorf("patch NodeConfig %s: %w", updated.Name, err)
	}
	return updated, nil
}

// applySpec writes the spec by server-side apply, every field of it owned by
// fieldManager; a field it applied before and leaves out now is removed. The apply
// is conditional on the revision read, as write() is.
func (r *Reconciler) applySpec(ctx context.Context, existing *internalv1alpha1.NodeConfig, spec internalv1alpha1.NodeSpec, logger logr.Logger) (*internalv1alpha1.NodeConfig, error) {
	applied, err := applyConfiguration(existing, spec)
	if err != nil {
		return nil, err
	}
	if err := r.Client.Patch(ctx, applied, client.Apply, client.FieldOwner(fieldManager), client.ForceOwnership); err != nil {
		if apierrors.IsConflict(err) {
			logger.V(1).Info("NodeConfig changed while it was being rendered; leaving it to the next pass", "node", existing.Name)
			return nil, nil
		}
		return nil, fmt.Errorf("apply NodeConfig %s: %w", existing.Name, err)
	}
	current := &internalv1alpha1.NodeConfig{}
	if err := fromUnstructured(applied, current); err != nil {
		return nil, fmt.Errorf("read the applied NodeConfig %s: %w", existing.Name, err)
	}
	return current, nil
}

// applyConfiguration is the apply request for a spec: the object's name and revision
// and the spec alone. Marshalled through encoding/json, which honours omitzero, so an
// empty section is left out rather than applied as {}.
func applyConfiguration(existing *internalv1alpha1.NodeConfig, spec internalv1alpha1.NodeSpec) (*unstructured.Unstructured, error) {
	raw, err := json.Marshal(spec)
	if err != nil {
		return nil, fmt.Errorf("marshal the spec of NodeConfig %s: %w", existing.Name, err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("unmarshal the spec of NodeConfig %s: %w", existing.Name, err)
	}
	applied := &unstructured.Unstructured{Object: map[string]any{"spec": fields}}
	applied.SetGroupVersionKind(internalv1alpha1.GroupVersion.WithKind("NodeConfig"))
	applied.SetName(existing.Name)
	applied.SetResourceVersion(existing.ResourceVersion)
	return applied, nil
}

// fromUnstructured decodes an object the way the API serves it, []byte fields included.
func fromUnstructured(in *unstructured.Unstructured, out *internalv1alpha1.NodeConfig) error {
	raw, err := json.Marshal(in.Object)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return nil
}

// specUpToDate reports whether the object already carries the rendered spec, the
// machine's own fields aside. A bootstrap token left on the object is a change.
func specUpToDate(existing, desired *internalv1alpha1.NodeConfig) bool {
	if existing.Spec.Kubelet.BootstrapToken != "" {
		return false
	}
	return apiequality.Semantic.DeepEqual(withoutMachineFields(existing.Spec), withoutMachineFields(desired.Spec))
}

// metadataUpToDate reports whether the object carries the labels and owner. Ownership is
// compared because it is written: left out, a stale or missing ownerReference is never
// corrected and the object outlives its node.
func metadataUpToDate(existing, desired *internalv1alpha1.NodeConfig) bool {
	if !apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) {
		return false
	}
	return apiequality.Semantic.DeepEqual(existing.OwnerReferences, desired.OwnerReferences)
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
