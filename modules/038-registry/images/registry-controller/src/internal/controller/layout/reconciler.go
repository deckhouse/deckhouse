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

// Package layout reconciles the configuration into the objects the data plane
// reads: one RegistryStorage and one RegistryNode per node.
//
// Both are produced by a single reconciler on purpose. The air-gap transition
// removes the upstream from the storage and from every node layout, and it must
// happen as one decision: split across two reconcilers, a cluster could end up
// with nodes pointing at an upstream the storage has already forgotten.
//
// Every input maps to the same request key — the RegistryConfig singleton — so
// reconciliations serialize and the layout is always computed from one coherent
// snapshot.
package layout

import (
	"context"
	"fmt"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	registryv1alpha1 "github.com/deckhouse/deckhouse/go_lib/registry/apis/deckhouse.io/v1alpha1"
	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"

	"github.com/deckhouse/registry-controller/internal/controller/config"
	"github.com/deckhouse/registry-controller/internal/layout"
	"github.com/deckhouse/registry-controller/internal/metrics"
	"github.com/deckhouse/registry-controller/internal/probe"
	"github.com/deckhouse/registry-controller/internal/register"
	"github.com/deckhouse/registry-controller/internal/upstreams"
)

const (
	// ControllerName identifies this controller in logs, events and the
	// --disable-controllers flag.
	ControllerName = "registry-layout"

	// Namespace holds the module's in-cluster components.
	Namespace = "d8-system"

	// StorageAccessSecretName holds what a client needs to reach the storage: the
	// certificate authority of its serving certificate and the read credentials.
	//
	// An interim contract with the storage PKI work: the controller only consumes
	// it, and does not care whether a hook or a controller produced it.
	StorageAccessSecretName = "registry-storage-access"

	storageAccessCAKey        = "ca.crt"
	storageAccessUsernameKey  = "username"
	storageAccessPasswordKey  = "password"
	storageAccessAddressesKey = "addresses"

	// masterNodeGroup is the node group whose maintenance windows place the garbage
	// collection, being the one the storage replicas run on.
	masterNodeGroup = "master"

	// StorageLeaseName is the election lease the storage replicas take to decide which
	// of them fills the cache, and the authority on which one leads.
	//
	// Read here rather than trusting the Role each replica writes about itself, because a
	// replica that has gone away cannot retract its claim. It must be the same name the
	// syncer's --lease-name defaults to.
	StorageLeaseName = "registry-storage"

	// probeRetryInterval is how often a rejected upstream is retried.
	//
	// A failed probe means "unreachable OR wrong", and the two cannot be told
	// apart from one attempt. Retrying means a registry that was merely down gets
	// picked up on its own, without the operator having to touch the object.
	probeRetryInterval = time.Minute
)

func init() {
	register.RegisterController(ControllerName, &registryv1alpha1.RegistryConfig{}, &Reconciler{})
}

// Reconciler compiles the configuration into the per-node and storage layout.
type Reconciler struct {
	register.Base

	// Prober verifies a changed primary upstream before the cluster is switched
	// over to it. Injectable so the reconciliation is testable without a registry;
	// left unset it defaults to the real one.
	Prober probe.Prober
}

// prober defaults at the point of use rather than in a setup hook, so the
// reconciler is correct however it was constructed.
func (r *Reconciler) prober() probe.Prober {
	if r.Prober != nil {
		return r.Prober
	}
	return &probe.Registry{}
}

var _ register.Reconciler = (*Reconciler)(nil)

// SetupWatches funnels every input into the singleton request key.
func (r *Reconciler) SetupWatches(w register.Watcher) {
	toSingleton := handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, _ client.Object) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{Name: registryv1alpha1.SingletonName},
			}}
		},
	)

	// Nodes drive the set of RegistryNode objects.
	w.Watches(&corev1.Node{}, toSingleton)
	// The storage status carries the fill progress that gates the air-gap
	// transition, and its spec carries the last applied upstream.
	w.Watches(&registryv1alpha1.RegistryStorage{}, toSingleton)
	// Someone editing a node layout by hand must be corrected.
	w.Watches(&registryv1alpha1.RegistryNode{}, toSingleton)
	// Any writer may declare an additional upstream at any time.
	w.Watches(&registryv1alpha1.RegistryUpstream{}, toSingleton)
}

func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	if req.Name != registryv1alpha1.SingletonName {
		return ctrl.Result{}, nil
	}

	cfg := &registryv1alpha1.RegistryConfig{}
	if err := r.Client.Get(ctx, req.NamespacedName, cfg); err != nil {
		if apierrors.IsNotFound(err) {
			// No configuration means nothing to compile. Existing objects are left
			// alone: the components keep serving images from what they already have,
			// which is what makes the data plane survive a dead control plane.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// A spec that does not validate is not projected. Compiling a broken
	// configuration into the node layouts would push the mistake all the way to
	// containerd, where it is far more expensive to undo.
	if errs := config.Validate(cfg); len(errs) > 0 {
		log.Info("skipping layout: the configuration is invalid", "errors", errs.ToAggregate().Error())
		return ctrl.Result{}, nil
	}

	inputs, err := r.collectInputs(ctx, cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	// The additional upstreams are arbitrated before the layout is computed, so a
	// rejected one never reaches a node.
	verdicts, err := r.arbitrateUpstreams(ctx, cfg, &inputs)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("arbitrating RegistryUpstream objects: %w", err)
	}

	probeFailure := r.probeUpstream(ctx, cfg, &inputs)

	desired := layout.Compute(inputs)

	if desired.DropUpstream {
		log.Info("the storage leader holds the expected set; dropping the upstream and going air-gap")
	}

	// The one transition in the design that can cut every node off from images, and the
	// one place a wait leaves no trace in any object: the configuration says air-gap, the
	// layout still holds an upstream, and both look settled. Recorded so the wait itself
	// can be alerted on.
	observeAirGapWait(cfg.Spec.Primary.Upstream == nil, desired.HeldUpstream != nil)
	observeRejections(verdicts)

	// Before anything that references them. The layouts below name keys in this Secret
	// instead of carrying credentials, so an agent that reads a layout whose key is not
	// there yet authenticates with nothing and gets a 401 that explains none of this.
	if err := r.applyAuthSecret(ctx, desired.Credentials); err != nil {
		return ctrl.Result{}, fmt.Errorf("applying the resolved auth Secret: %w", err)
	}
	if err := r.applyStorage(ctx, desired.Storage); err != nil {
		return ctrl.Result{}, fmt.Errorf("applying RegistryStorage: %w", err)
	}
	if err := r.applyNodes(ctx, desired.Nodes); err != nil {
		return ctrl.Result{}, fmt.Errorf("applying RegistryNode objects: %w", err)
	}

	// The status is written last, so it describes a layout that is already in
	// place rather than one that was merely intended.
	// The digest of the credentials that went into the Secret alongside this upstream, so
	// the record says which credentials are in effect without being able to disclose them.
	heldAuth := desired.Credentials[constant.AuthKeyUpstream]
	if err := r.patchConfigStatus(
		ctx, cfg, desired.HeldUpstream, heldAuth.AuthDigest(), probeFailure,
	); err != nil {
		return ctrl.Result{}, fmt.Errorf("patching status of RegistryConfig: %w", err)
	}
	if err := r.patchUpstreamVerdicts(ctx, verdicts); err != nil {
		return ctrl.Result{}, fmt.Errorf("patching status of RegistryUpstream objects: %w", err)
	}

	if probeFailure != nil {
		// Retry: the upstream may be down rather than wrong, and the operator should
		// not have to touch anything for a recovered registry to be picked up.
		return ctrl.Result{RequeueAfter: probeRetryInterval}, nil
	}
	return ctrl.Result{}, nil
}

// arbitrateUpstreams resolves the additional upstreams and puts the accepted ones
// into the layout inputs.
func (r *Reconciler) arbitrateUpstreams(
	ctx context.Context, cfg *registryv1alpha1.RegistryConfig, inputs *layout.Inputs,
) (map[string]upstreams.Verdict, error) {
	list := &registryv1alpha1.RegistryUpstreamList{}
	if err := r.Client.List(ctx, list); err != nil {
		return nil, err
	}

	result := upstreams.Arbitrate(cfg.Spec.Primary.Upstream, list.Items)
	inputs.Routes = result.Routes
	return result.Verdicts, nil
}

// unchangedUpstream answers whether the configured upstream is the one already in effect.
//
// Compared in two parts because the record holds only one of them. The addresses are
// compared directly; the credentials are compared through the digest that was recorded
// beside them, since recording the credentials themselves would put a license key in a
// cluster-scoped resource. Both halves matter: a rotated license under an unchanged address
// is exactly the change that has to be probed before the cluster is switched onto it.
func unchangedUpstream(
	desired, effective *registryv1alpha1.Upstream, effectiveDigest string,
) bool {
	if desired == nil || effective == nil {
		return false
	}

	// Compared without credentials on either side: the recorded copy has none, so leaving
	// them in would make every comparison unequal.
	withoutAuth := func(upstream *registryv1alpha1.Upstream) *registryv1alpha1.Upstream {
		stripped := upstream.DeepCopy()
		stripped.Endpoint.Auth = nil
		for i := range stripped.Mirrors {
			stripped.Mirrors[i].Auth = nil
		}
		return stripped
	}

	if !equality.Semantic.DeepEqual(withoutAuth(desired), withoutAuth(effective)) {
		return false
	}
	return desired.Endpoint.Auth.AuthDigest() == effectiveDigest
}

// probeUpstream verifies a CHANGED primary upstream before the cluster is switched
// to it, and returns the failure that must hold the switch back.
//
// Only a change is probed. Probing an unchanged upstream on every reconciliation
// would turn a transient outage of the registry into a configuration rollback,
// which is the opposite of what this is for.
func (r *Reconciler) probeUpstream(
	ctx context.Context, cfg *registryv1alpha1.RegistryConfig, inputs *layout.Inputs,
) error {
	log := ctrl.LoggerFrom(ctx)

	desired := cfg.Spec.Primary.Upstream
	if desired == nil {
		// Air-gap is not probed: it is gated on cache completeness instead.
		return nil
	}
	if unchangedUpstream(desired, cfg.Status.EffectiveUpstream, cfg.Status.EffectiveUpstreamAuthDigest) {
		return nil
	}

	if err := r.prober().Probe(ctx, desired); err != nil {
		// Deliberately not returned as a reconcile error: the layout still has to be
		// applied, holding the previous upstream. Failing the whole reconciliation
		// would leave the rest of the cluster unconverged over an unreachable
		// registry.
		log.Info("the configured primary upstream did not pass its preflight probe; holding the last known good one",
			"host", desired.Host, "error", err.Error())
		inputs.UpstreamProbeFailed = true
		metrics.UpstreamProbes.WithLabelValues(probeOutcome(err)).Inc()
		return err
	}

	log.Info("the configured primary upstream passed its preflight probe", "host", desired.Host)
	metrics.UpstreamProbes.WithLabelValues(metrics.OutcomeAccepted).Inc()
	return nil
}

// observeAirGapWait records whether the cluster is waiting to become air-gapped.
func observeAirGapWait(wantAirGap, stillHoldingUpstream bool) {
	if wantAirGap && stillHoldingUpstream {
		metrics.AirGapHeld.Set(1)
		return
	}
	metrics.AirGapHeld.Set(0)
}

// observeRejections records how many additional registries arbitration refused.
//
// Reset each pass rather than accumulated: a reason that no longer applies has to go away,
// or the gauge would keep reporting a conflict an operator has already resolved.
func observeRejections(verdicts map[string]upstreams.Verdict) {
	counts := map[string]float64{}
	for _, verdict := range verdicts {
		if verdict.Accepted {
			continue
		}
		counts[verdict.Reason]++
	}

	metrics.UpstreamsRejected.Reset()
	for reason, count := range counts {
		metrics.UpstreamsRejected.WithLabelValues(reason).Set(count)
	}
}

// probeOutcome names why a probe refused, since the reconciler treats the three the same
// and an operator does not: unreachable is a network or a registry, rejected credentials
// are a license, and a missing sentinel is the wrong repository path.
func probeOutcome(err error) string {
	failure, ok := probe.AsFailure(err)
	if !ok {
		return metrics.OutcomeUnreachable
	}

	switch failure.Kind {
	case probe.FailureAuth:
		return metrics.OutcomeAuth
	case probe.FailureSentinel:
		return metrics.OutcomeSentinel
	default:
		return metrics.OutcomeUnreachable
	}
}

// patchConfigStatus records which upstream is in effect and why, which is what
// makes a held upstream explainable instead of looking like the change was lost.
func (r *Reconciler) patchConfigStatus(
	ctx context.Context, cfg *registryv1alpha1.RegistryConfig,
	heldUpstream *registryv1alpha1.Upstream, heldAuthDigest string, probeFailure error,
) error {
	patch := client.MergeFrom(cfg.DeepCopy())

	condition := metav1.Condition{
		Type:               registryv1alpha1.ConditionUpstreamValid,
		ObservedGeneration: cfg.Generation,
	}
	switch {
	case probeFailure != nil:
		condition.Status = metav1.ConditionFalse
		condition.Reason = registryv1alpha1.ReasonProbeFailed
		condition.Message = probeFailure.Error()
		if failure, ok := probe.AsFailure(probeFailure); ok {
			condition.Reason = failure.Reason()
		}
	case cfg.Spec.Primary.Upstream == nil:
		condition.Status = metav1.ConditionTrue
		condition.Reason = registryv1alpha1.ReasonAirGap
		condition.Message = "No upstream is configured; the cache is the source of images."
	default:
		condition.Status = metav1.ConditionTrue
		condition.Reason = registryv1alpha1.ReasonProbed
		condition.Message = "The primary upstream is reachable, accepts the credentials and serves Deckhouse content."
	}

	previous := cfg.Status.EffectiveUpstream
	previousDigest := cfg.Status.EffectiveUpstreamAuthDigest
	cfg.Status.EffectiveUpstream = heldUpstream.DeepCopy()
	cfg.Status.EffectiveUpstreamAuthDigest = heldAuthDigest

	changed := apimeta.SetStatusCondition(&cfg.Status.Conditions, condition)
	if !changed &&
		equality.Semantic.DeepEqual(previous, cfg.Status.EffectiveUpstream) &&
		previousDigest == cfg.Status.EffectiveUpstreamAuthDigest {
		return nil
	}

	return r.Client.Status().Patch(ctx, cfg, patch)
}

// patchUpstreamVerdicts tells each writer whether their upstream took effect.
func (r *Reconciler) patchUpstreamVerdicts(
	ctx context.Context, verdicts map[string]upstreams.Verdict,
) error {
	for name, verdict := range verdicts {
		upstream := &registryv1alpha1.RegistryUpstream{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: name}, upstream); err != nil {
			if apierrors.IsNotFound(err) {
				// Deleted between the arbitration and now; nothing to tell anyone.
				continue
			}
			return err
		}

		patch := client.MergeFrom(upstream.DeepCopy())

		condition := metav1.Condition{
			Type:               registryv1alpha1.ConditionAccepted,
			ObservedGeneration: upstream.Generation,
			Reason:             verdict.Reason,
		}
		if verdict.Accepted {
			condition.Status = metav1.ConditionTrue
			condition.Message = "The upstream is routed on every node."
		} else {
			condition.Status = metav1.ConditionFalse
			condition.Message = verdict.Conflict
		}

		unchanged := upstream.Status.Accepted == verdict.Accepted &&
			upstream.Status.Conflict == verdict.Conflict &&
			upstream.Status.ObservedGeneration == upstream.Generation

		upstream.Status.Accepted = verdict.Accepted
		upstream.Status.Conflict = verdict.Conflict
		upstream.Status.ObservedGeneration = upstream.Generation
		changed := apimeta.SetStatusCondition(&upstream.Status.Conditions, condition)

		if unchanged && !changed {
			continue
		}
		if err := r.Client.Status().Patch(ctx, upstream, patch); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	return nil
}

func (r *Reconciler) collectInputs(
	ctx context.Context, cfg *registryv1alpha1.RegistryConfig,
) (layout.Inputs, error) {
	inputs := layout.Inputs{
		Config: cfg.Spec,
		// The controller's own record of what is in effect. Taken from the status
		// rather than from RegistryStorage.spec so that the hold works identically
		// with the cache off, where no storage object exists to remember anything.
		AppliedUpstream: cfg.Status.EffectiveUpstream,
	}

	// Where an operator has already said disruption is acceptable, which is where the
	// garbage collection goes unless they said otherwise. Read best-effort: a cluster
	// without those windows, or without permission to read them, gets the night default
	// rather than no collection at all.
	inputs.MaintenanceWindows = r.maintenanceWindows(ctx)

	storage := &registryv1alpha1.RegistryStorage{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: registryv1alpha1.SingletonName}, storage)
	switch {
	case err == nil:
		inputs.LeaderFull = layout.LeaderFull(storage.Status.Replicas, r.storageLeaseHolder(ctx))
		if inputs.AppliedUpstream == nil {
			// Nothing recorded yet, which is what an upgrade from a release without
			// the status field looks like. The upstream the storage is running on is
			// the truth in that case.
			inputs.AppliedUpstream = storage.Spec.Upstream
		}
	case !apierrors.IsNotFound(err):
		return inputs, fmt.Errorf("reading RegistryStorage: %w", err)
	}

	// What this module wrote last time, for the upstream it may now be holding without any
	// credentials of its own. See layout.Inputs.PersistedCredentials.
	inputs.PersistedCredentials = r.persistedCredentials(ctx)

	nodes := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodes); err != nil {
		return inputs, fmt.Errorf("listing nodes: %w", err)
	}
	inputs.Nodes = make([]string, 0, len(nodes.Items))
	for i := range nodes.Items {
		inputs.Nodes = append(inputs.Nodes, nodes.Items[i].Name)
	}

	if cfg.Spec.Storage.Cache {
		access, err := r.storageAccess(ctx)
		if err != nil {
			return inputs, err
		}
		inputs.StorageAccess = access
	}

	return inputs, nil
}

// maintenanceWindows reads the disruption windows of the master node group.
//
// Read as unstructured, so that this controller does not take a dependency on the node
// manager's Go types for four strings. Failures are swallowed on purpose: this only decides a
// default hour, and a cluster where it cannot be read should collect at night rather than
// never.
// persistedCredentials reads back the credentials this module wrote last time.
//
// Best-effort, and nil on any failure. The one thing they are used for is giving a held
// upstream back the credentials it was working with, and a hold arriving without them is the
// state this repairs — not a reason to abandon the whole reconciliation. If the Secret is not
// there yet, there is nothing to hold over anyway.
//
// A plain Get, which reaches the API rather than a cache: the manager excludes Secrets from
// its cache precisely because the role granting this one is scoped by resourceNames, and a
// rule with resourceNames cannot authorize the list an informer would need.
func (r *Reconciler) persistedCredentials(ctx context.Context) map[string]registryv1alpha1.Auth {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: Namespace, Name: constant.AuthSecretName}
	if err := r.Client.Get(ctx, key, secret); err != nil {
		return nil
	}

	out := make(map[string]registryv1alpha1.Auth, len(secret.Data))
	for name := range secret.Data {
		// The pre-encoded form, which is the only form applyAuthSecret writes.
		if value := string(secret.Data[name]); value != "" {
			out[name] = registryv1alpha1.Auth{Auth: value}
		}
	}
	return out
}

// storageLeaseHolder is the node whose replica currently fills the cache, or empty when
// nobody holds the lease and when it cannot be read.
//
// Empty on failure rather than an error, and empty is the safe answer: without a known leader
// nothing reports as complete, so the cluster keeps its upstream. The alternative — falling
// back to the Role a replica wrote about itself — is what this exists to avoid, since a replica
// that has gone away leaves that claim behind.
func (r *Reconciler) storageLeaseHolder(ctx context.Context) string {
	lease := &coordinationv1.Lease{}
	key := types.NamespacedName{Namespace: Namespace, Name: StorageLeaseName}
	if err := r.Client.Get(ctx, key, lease); err != nil {
		return ""
	}
	if lease.Spec.HolderIdentity == nil {
		return ""
	}
	return *lease.Spec.HolderIdentity
}

func (r *Reconciler) maintenanceWindows(ctx context.Context) []layout.MaintenanceWindow {
	group := &unstructured.Unstructured{}
	group.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup",
	})

	if err := r.Client.Get(ctx, types.NamespacedName{Name: masterNodeGroup}, group); err != nil {
		return nil
	}

	raw, found, err := unstructured.NestedSlice(group.Object, "spec", "disruptions", "automatic", "windows")
	if err != nil || !found {
		return nil
	}

	windows := make([]layout.MaintenanceWindow, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}

		window := layout.MaintenanceWindow{}
		window.From, _, _ = unstructured.NestedString(entry, "from")
		window.To, _, _ = unstructured.NestedString(entry, "to")
		if days, ok, _ := unstructured.NestedStringSlice(entry, "days"); ok {
			window.Days = days
		}
		windows = append(windows, window)
	}
	return windows
}

// storageAccess reads how agents reach the storage.
//
// A missing secret is an error rather than a reason to proceed: writing a layout
// without the certificate authority and credentials would leave every agent
// unable to pull from the cache, and the node layout is exactly the thing that is
// expensive to get wrong.
func (r *Reconciler) storageAccess(ctx context.Context) (layout.Access, error) {
	secret := &corev1.Secret{}
	key := types.NamespacedName{Namespace: Namespace, Name: StorageAccessSecretName}

	if err := r.Client.Get(ctx, key, secret); err != nil {
		return layout.Access{}, fmt.Errorf("reading secret %s: %w", key, err)
	}

	access := layout.Access{CA: string(secret.Data[storageAccessCAKey])}
	if username := string(secret.Data[storageAccessUsernameKey]); username != "" {
		access.Auth = &registryv1alpha1.Auth{
			Username: username,
			Password: string(secret.Data[storageAccessPasswordKey]),
		}
	}

	for _, address := range strings.Split(string(secret.Data[storageAccessAddressesKey]), ",") {
		if trimmed := strings.TrimSpace(address); trimmed != "" {
			access.Addresses = append(access.Addresses, addressWithPort(trimmed))
		}
	}

	if access.CA == "" {
		return layout.Access{}, fmt.Errorf("secret %s has no %s", key, storageAccessCAKey)
	}
	if len(access.Addresses) == 0 {
		// Refused rather than worked around. Without an address the cache cannot be
		// reached, and a layout that named the Service instead would look right and fail
		// on every node: an agent runs in the host network, where a cluster DNS name does
		// not resolve.
		return layout.Access{}, fmt.Errorf("secret %s has no %s", key, storageAccessAddressesKey)
	}
	return access, nil
}

// addressWithPort adds the registry port to a bare address.
//
// The addresses are discovered as node addresses, and the port they serve on is a
// constant of the design rather than something discovered with them.
func addressWithPort(address string) string {
	if strings.Contains(address, ":") && !strings.HasPrefix(address, "[") {
		// Already has a port, or is a bare IPv6 address — which has to be bracketed
		// before a port can be appended, and is left alone rather than guessed at.
		return address
	}
	return fmt.Sprintf("%s:%d", address, constant.Port)
}

// applyAuthSecret persists the credentials the layouts reference.
//
// One Secret for the whole module, keyed by what each credential belongs to. One object
// rather than one per registry because the node agent has to read it, and a single name can
// be granted by resourceNames — so a node's kubelet reaches exactly this and nothing else,
// instead of reading credentials out of every RegistryNode in the cluster.
//
// Replaced rather than merged: a key that is gone from the layout is a credential nothing
// references any more, and leaving it behind would keep a rotated license readable for as
// long as the cluster lives.
func (r *Reconciler) applyAuthSecret(
	ctx context.Context, credentials map[string]registryv1alpha1.Auth,
) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constant.AuthSecretName,
			Namespace: Namespace,
		},
	}

	if len(credentials) == 0 {
		if err := r.Client.Delete(ctx, secret); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	data := make(map[string][]byte, len(credentials))
	for key := range credentials {
		auth := credentials[key]
		// The pre-encoded form, which is what a containerd hosts.toml wants and what
		// every reader of this ultimately produces. Storing the pair separately would
		// mean each reader re-encoding it, and a credential containing a colon would
		// then decode differently in different readers.
		data[key] = []byte(auth.Encoded())
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = data
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		secret.Labels["heritage"] = "deckhouse"
		secret.Labels["module"] = "registry"
		return nil
	})
	return err
}

// applyStorage converges RegistryStorage on the desired spec and refreshes the
// aggregate half of its status, or removes the object when no storage is needed.
//
// The per-replica entries in the status are NOT touched: each syncer owns its
// own, and reports what it actually wrote. Only the cluster-wide summary is
// derived here, because no single replica can know whether the others are done.
func (r *Reconciler) applyStorage(ctx context.Context, spec *registryv1alpha1.RegistryStorageSpec) error {
	storage := &registryv1alpha1.RegistryStorage{
		ObjectMeta: metav1.ObjectMeta{Name: registryv1alpha1.SingletonName},
	}

	if spec == nil {
		// With the cache off there is no storage to describe. The blobs on disk are
		// untouched, so turning the cache back on refills from what is already
		// there instead of from scratch.
		if err := r.Client.Delete(ctx, storage); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, storage, func() error {
		storage.Spec = *spec
		return nil
	}); err != nil {
		return err
	}

	return r.patchStorageStatus(ctx, storage)
}

func (r *Reconciler) patchStorageStatus(
	ctx context.Context, storage *registryv1alpha1.RegistryStorage,
) error {
	patch := client.MergeFrom(storage.DeepCopy())

	aggregate := layout.Aggregate(&storage.Spec, storage.Status.Replicas, r.storageLeaseHolder(ctx))
	aggregate.ObservedGeneration = storage.Generation
	// Conditions are merged rather than replaced, so an entry another writer owns
	// is not dropped.
	aggregate.Conditions = storage.Status.Conditions

	converged := metav1.Condition{
		Type:               registryv1alpha1.ConditionStorageConverged,
		ObservedGeneration: storage.Generation,
	}
	switch aggregate.Phase {
	case registryv1alpha1.StoragePhaseReady:
		converged.Status = metav1.ConditionTrue
		converged.Reason = registryv1alpha1.ReasonConverged
		converged.Message = "The storage leader holds the expected set."
	case registryv1alpha1.StoragePhaseFailed:
		converged.Status = metav1.ConditionFalse
		converged.Reason = registryv1alpha1.ReasonFillFailed
		converged.Message = "A fill or verification task failed, so the upstream is kept."
	default:
		converged.Status = metav1.ConditionFalse
		converged.Reason = registryv1alpha1.ReasonFillInProgress
		converged.Message = fmt.Sprintf("The storage is %s.", aggregate.Phase)
	}

	previous := storage.Status
	storage.Status = aggregate
	conditionChanged := apimeta.SetStatusCondition(&storage.Status.Conditions, converged)

	if !conditionChanged && equality.Semantic.DeepEqual(previous, storage.Status) {
		// Skip the write when nothing changed: this object is watched by every
		// syncer, so needless churn wakes the whole storage tier.
		return nil
	}

	return r.Client.Status().Patch(ctx, storage, patch)
}

// applyNodes converges one RegistryNode per node and removes the leftovers of
// nodes that are gone.
func (r *Reconciler) applyNodes(
	ctx context.Context, desired map[string]registryv1alpha1.RegistryNodeSpec,
) error {
	existing := &registryv1alpha1.RegistryNodeList{}
	if err := r.Client.List(ctx, existing); err != nil {
		return fmt.Errorf("listing RegistryNode objects: %w", err)
	}

	for name, spec := range desired {
		node := &registryv1alpha1.RegistryNode{ObjectMeta: metav1.ObjectMeta{Name: name}}

		if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, node, func() error {
			node.Spec = spec
			return r.setNodeOwner(ctx, node)
		}); err != nil {
			return fmt.Errorf("node %s: %w", name, err)
		}
	}

	for i := range existing.Items {
		leftover := &existing.Items[i]
		if _, wanted := desired[leftover.Name]; wanted {
			continue
		}
		// The owner reference on the Node collects these too, but only once the
		// Node object is gone. Removing them here also covers the case of a node
		// dropping out of the desired set while its Node still exists.
		if err := r.Client.Delete(ctx, leftover); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting the layout of the removed node %s: %w", leftover.Name, err)
		}
	}

	return nil
}

// setNodeOwner ties a layout to its Node so that a removed node takes its layout
// with it, without the controller having to notice.
func (r *Reconciler) setNodeOwner(ctx context.Context, layoutObj *registryv1alpha1.RegistryNode) error {
	node := &corev1.Node{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: layoutObj.Name}, node); err != nil {
		if apierrors.IsNotFound(err) {
			// The node disappeared between the list and now. Leaving the layout
			// unowned is fine: the next reconciliation deletes it as a leftover.
			return nil
		}
		return err
	}

	return controllerutil.SetControllerReference(node, layoutObj, r.Client.Scheme())
}
