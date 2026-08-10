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

// Package nodeconfig renders a NodeConfig object for every node of an
// an olcedar NodeGroup. Such nodes carry no bashible: the on-node agent
// watches its own NodeConfig, reconciles the node towards it and reports the
// outcome back through the object's status. This controller is the writer of
// that desired state, built from the NodeGroup the node belongs to plus the
// cluster's own state (API server endpoints, DNS, image digests, proxy token).
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

// MaxConcurrentReconciles is one, and this controller is only correct at one.
//
// The rollout gate decides how many nodes of a group may take a new spec by
// counting the group's unapplied NodeConfigs and then writing one — a
// read-then-write over the whole group with nothing shared to serialize on. Two
// workers reconciling two nodes of one group both read "none updating" and both
// write, so the group takes the change at once however small maxConcurrent is.
// With one worker the queue hands one key to one worker and an all-nodes pass
// walks the group in order, which is what makes the count mean what it says.
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

// ForPredicates drops Node updates that cannot change what this controller
// renders. Every node in an immutable group refreshes its status on a timer,
// and each of those updates used to start a full pass for that node: the
// cluster-wide inputs are read live (see readClusterInputs) and the group's
// Kubernetes version is derived from scratch, so a thousand-node fleet paid
// nine uncached round trips per node per heartbeat to render a spec that was
// identical every time.
//
// What the render actually reads off a Node is the whole of what is compared
// here:
//
//   - metadata.name — the node name, its hostname and the object's own name
//     (renderSpec, renderNetwork, newNodeConfig); it cannot change.
//   - metadata.uid — the ownerReference that has the API server collect the
//     NodeConfig with its Node (newNodeConfig); a new UID is a new object, which
//     arrives as a create.
//   - metadata.labels — all of them, not just the two this package names: the
//     group label chooses the NodeGroup (reconcileNode), the control-plane label
//     decides whether the node publishes its own config (isControlPlaneNode),
//     and a NodeExtensionRequest can select on any label at all (nerMatchesNode).
//   - metadata.creationTimestamp — whether the node has joined yet, which is
//     what decides if registration taints are rendered (renderKubelet).
//
// status.addresses is read too, but never from this object: the manager's cache
// strips it, so it is fetched from the API server when it is needed
// (reportedNodeIPs) and cannot be compared here in any case.
//
// A kubelet heartbeat touches none of that — it writes status conditions and
// timestamps — so it is dropped. Creates and deletes are not filtered, and this
// predicate applies only to the Node watch: a NodeGroup, NodeConfig or
// NodeExtensionRequest event still enqueues the all-nodes pass, which is how a
// change to the cluster's own state reaches every node.
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

// nodeConfigRolloutInputsChanged reports whether an update to a NodeConfig
// touched anything a pass reads off it. An event missing either side is passed
// through, for the same reason nodeRenderInputsChanged does it.
//
// Generation stands in for the whole spec: the resource has a status
// subresource, so only a spec write moves it. Everything else here is what the
// rollout gate reads (rollout.go: applied) or what decides whether the object is
// still ours (labels, owner references, deletion).
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
	for _, condition := range []string{configurationAppliedCondition, disruptionRequiredCondition} {
		if !conditionEqual(oldConfig.Status.Conditions, newConfig.Status.Conditions, condition) {
			return true
		}
	}
	return false
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
	// A node reporting the spec it was given frees its rollout slot, which is
	// what lets the next node of the group be updated.
	//
	// Predicated, and it has to be: the agent republishes its status on every
	// pass of its own — units, extensions, the maintenance token, the last
	// reconcile time — and this mapper enqueues the ALL-nodes key, so without a
	// filter every heartbeat of every node re-renders the whole fleet. With one
	// worker (MaxConcurrentReconciles) a fleet large enough to heartbeat faster
	// than a pass completes never leaves that loop.
	w.Watches(&internalv1alpha1.NodeConfig{}, allMapper, builder.WithPredicates(predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return nodeConfigRolloutInputsChanged(e.ObjectOld, e.ObjectNew)
		},
	}))
	// A NodeExtensionRequest change alters which extensions a node merges, so it
	// re-renders every node — the broad mapper is enough until per-request
	// selection is tracked.
	//
	// The CRD is a hard dependency rather than something whose absence is
	// tolerated anywhere: a watch on a kind the cluster does not have never
	// syncs, and the manager gives up on it after two minutes and exits, taking
	// every other controller in this binary — and the NodeGroup webhook — with
	// it. Tolerating it in the *read* would not help, because the render never
	// runs. It is safe to depend on because this is an embedded module:
	// addon-operator applies every enabled module's crds/ before any of them runs
	// Helm (EnsureCRDs tasks are queued ahead of every ModuleRun on the single
	// main queue), and an upgrade replaces this pod, whose first converge ensures
	// the CRDs of every enabled module before the chart that carries this
	// deployment is applied.
	w.Watches(&deckhousev1alpha1.NodeExtensionRequest{}, allMapper)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if req.Name == allRequestName {
		return r.reconcileAllNodes(ctx, logger)
	}
	return r.reconcileNode(ctx, req.Name, logger, newPass())
}

// pass memoises what one reconcile pass reads. An all-nodes pass renders every
// node from the same cluster state, so without this each node repeats the reads
// the node before it just made — and every one of them goes straight to the API
// server, because both are decisions the manager's cache is too old to make.
type pass struct {
	// inputs is what reading the cluster-wide render inputs produced, keyed by
	// Kubernetes version: the ~6 uncached reads readClusterInputs performs are
	// done once per distinct version rather than once per node.
	inputs map[string]clusterInputsResult
	// versions is the Kubernetes version each group runs, keyed by group name.
	// It is the key the inputs above are memoised under, and producing it is not
	// free: derived_status lists MachineDeployments and InstanceClasses and
	// unmarshals the whole instance-type catalogue. Without this the memo below
	// was paid for once per node anyway.
	versions map[string]versionResult
	// rollouts is each group's remaining rollout budget, keyed by group name.
	// One listing of the group's NodeConfigs per pass, rather than one per node
	// of the group — a NodeGroup edit drifts every node at once, so listing per
	// node made a group of N cost N listings of N multi-kilobyte objects.
	rollouts map[string]*rolloutBudget
	// created holds the nodes this pass gave a first config to, per group. They
	// are updating from that moment and the group's next listing counts them, so
	// a budget built later in the same pass has to count them too — otherwise the
	// answer depends on the order the pass happened to walk the nodes in.
	created map[string]map[string]struct{}
}

// recordCreated remembers that this pass gave a node of a group its first
// config.
func (p *pass) recordCreated(ngName, nodeName string) {
	nodes, ok := p.created[ngName]
	if !ok {
		nodes = map[string]struct{}{}
		p.created[ngName] = nodes
	}
	nodes[nodeName] = struct{}{}
	if budget, ok := p.rollouts[ngName]; ok {
		budget.spend(nodeName)
	}
}

// clusterInputsResult is one attempt at reading the cluster-wide inputs, kept
// whether it succeeded or not. The failure is worth keeping as much as the
// answer: these inputs are cluster-wide by construction, so an absent DNS
// service or an unreadable cluster configuration fails identically for every
// node, and re-reading per node turns one missing object into four live reads
// per node on every pass of a fleet that keeps triggering passes.
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
		created:  map[string]map[string]struct{}{},
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

// reconcileAllNodes re-renders every node that belongs to an immutable group.
// One node failing does not stop the others, but the failures are counted rather
// than logged one by one: the usual reason a node cannot be rendered is that the
// cluster's own state cannot be read, which is the same reason for every node in
// the fleet, and a line each buries the cause under its own repetition. The
// first error is returned, wrapped in how many nodes shared its fate, so the
// pass is retried with the controller's backoff.
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
	r.reconcileNERStatuses(ctx, logger)

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

	ngName := node.Labels[nodeGroupNameLabel]
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
	// The node may be asking to interrupt itself to apply what it was given, and
	// the answer has to be about the object as it now stands: re-reading it from
	// the cache here returned the pre-patch revision, and the operation minted
	// from it named a generation the node had already left behind — a drain the
	// node could never report done.
	current, err := r.apply(ctx, ng, node, desired, logger, p)
	if err != nil {
		return ctrl.Result{}, err
	}
	if current == nil {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.reconcileDisruption(ctx, ng, node, current, logger)
}

// apply creates the object or patches it when the rendered spec drifted, and
// returns the object as it now stands — nil when the node has none, or when
// another writer got to it first and this pass is leaving it alone. The status
// belongs to the node-local agent and is never touched here.
func (r *Reconciler) apply(ctx context.Context, ng *v1.NodeGroup, node *corev1.Node, desired *internalv1alpha1.NodeConfig, logger logr.Logger, p *pass) (*internalv1alpha1.NodeConfig, error) {
	existing := &internalv1alpha1.NodeConfig{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: desired.Name}, existing)
	if apierrors.IsNotFound(err) {
		// A CloudPermanent node was provisioned from an installer payload, not
		// from a rendered NodeConfig, and it publishes that payload itself. The
		// object created here would carry none of the bootstrap-only fields
		// (see keepBootstrapOnlyFields) and would win over the file the node
		// holds them in, so it waits for the node to register instead.
		//
		// The group's type, not the control-plane role label: a joining master
		// cannot label itself with the role (NodeRestriction), so between its
		// registration and the node-template controller applying the label the
		// role test reads false — and this create would win the race against
		// the node's own publish, taking the etcd mounts with it. The group
		// label the test relies on instead is in the payload's kubelet labels,
		// so it is there from the node's first appearance.
		if isControlPlaneNode(node) || ng.Spec.NodeType == v1.NodeTypeCloudPermanent {
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
		// A node given its first config is updating like any other, and the next
		// listing of the group will say so. Recording it here keeps this pass's
		// answer the same as that listing's, instead of depending on whether the
		// group happened to be read before or after the node was created.
		p.recordCreated(ng.Name, desired.Name)
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

	// Ownership is compared along with the rest because the patch below writes
	// it: left out, a NodeConfig whose ownerReference is missing or names a
	// replaced Node is never corrected, and the API server never collects it
	// with the node it belongs to.
	if apiequality.Semantic.DeepEqual(existing.Spec, desired.Spec) &&
		apiequality.Semantic.DeepEqual(existing.Labels, desired.Labels) &&
		apiequality.Semantic.DeepEqual(existing.OwnerReferences, desired.OwnerReferences) {
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
	// passes rendering the same node at once — its own and an all-nodes one —
	// would otherwise both write, the second one over a slot the first had
	// already spent.
	patch := client.MergeFromWithOptions(existing.DeepCopy(), client.MergeFromWithOptimisticLock{})
	existing.Spec = desired.Spec
	existing.Labels = desired.Labels
	existing.OwnerReferences = desired.OwnerReferences
	if err := r.Client.Patch(ctx, existing, patch); err != nil {
		// Someone wrote the object between the read this decision was made
		// against and this patch. The node is left to the next pass — the write
		// that won is a NodeConfig change, and the watch on those enqueues one —
		// rather than failing the whole walk over a race that resolves itself.
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

// reportedNodeIPs returns the internal addresses the node itself reports, read
// from the API server: the manager's cache strips Node.status.addresses to keep
// its memory down, so a cached Node claims every node in the cluster has no
// address at all. Nothing is read unless the node's config pins an address,
// which is the bootstrapped first master and nothing else.
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
// did not carry over as written. They are clamped rather than passed through
// because the agent's schema refuses the value and a refused config leaves the
// node with none at all, but silently narrowing what an operator asked for is
// how a group ends up running something nobody configured.
func (r *Reconciler) recordClampedSettings(ng *v1.NodeGroup) {
	if ng.Spec.Kubelet != nil && ng.Spec.Kubelet.MaxPods != nil && int(*ng.Spec.Kubelet.MaxPods) > maxPodsCeiling {
		r.Recorder.Event(ng, corev1.EventTypeWarning, "SettingClamped",
			fmt.Sprintf("kubelet.maxPods %d exceeds the %d an immutable node accepts; the nodes of this group are configured with %d",
				*ng.Spec.Kubelet.MaxPods, maxPodsCeiling, maxPodsCeiling))
	}
	if ng.Spec.Disruptions != nil && ng.Spec.Disruptions.ApprovalMode == v1.DisruptionApprovalModeRollingUpdate {
		r.Recorder.Event(ng, corev1.EventTypeWarning, "SettingClamped",
			fmt.Sprintf("disruptions.approvalMode %s has no counterpart on an immutable node; the nodes of this group are configured with %s",
				v1.DisruptionApprovalModeRollingUpdate, v1.DisruptionApprovalModeAutomatic))
	}
	if ng.Spec.Kubelet != nil && ng.Spec.Kubelet.ResourceReservation != nil &&
		ng.Spec.Kubelet.ResourceReservation.Mode == resourceReservationModeStatic {
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
