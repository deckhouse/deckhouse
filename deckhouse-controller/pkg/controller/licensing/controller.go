// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package licensing computes the license policy of the cluster: it verifies the
// installed ClusterLicense keys, reads the node set, lays the nodes out over the
// metrics of the key, publishes the aggregated EffectiveLicense and keeps a
// signed cluster data file ready for the customer to hand to the license server.
package licensing

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log/slog"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
	"github.com/deckhouse/deckhouse/pkg/log"
	metricsstorage "github.com/deckhouse/deckhouse/pkg/metrics-storage"
)

const (
	controllerName = "licensing-controller"

	// resyncPeriod bounds how stale the policy can get while nothing happens in
	// the cluster: records expire on a wall clock, not on an event.
	resyncPeriod = time.Hour

	// keySecretName holds the cluster Ed25519 identity as a raw 32 byte seed.
	keySecretName  = "cluster-key"
	keySecretField = "seed"

	// seqConfigMapName holds the registration counter, which has to survive a
	// restart and the loss of the status: the license server reads seq as
	// strictly growing.
	seqConfigMapName = "license-registration"
	seqField         = "seq"

	// eventKeySuperseded is emitted on EffectiveLicense when the controller
	// deletes a key a reissue extinguished.
	eventKeySuperseded = "KeySuperseded"
)

// objectLabels mark the objects the controller owns, the way every other
// deckhouse owned object in d8-system is marked.
var objectLabels = map[string]string{
	"heritage":               "deckhouse",
	"app.kubernetes.io/name": "deckhouse",
}

type reconciler struct {
	client.Client

	// apiReader bypasses the manager cache. The cluster key Secret and the
	// counter ConfigMap fall outside the label selectors the manager caches, and
	// a stale miss on the key would mint a second cluster identity.
	apiReader client.Reader

	metricStorage metricsstorage.Storage
	recorder      record.EventRecorder
	logger        *log.Logger

	// vendorKeys defaults to licensing.VendorPublicKeys. A test issues its own
	// keys and overrides the field instead of the package variable.
	vendorKeys []ed25519.PublicKey
	thresholds licensing.Thresholds

	dkpVersion string
	build      string

	now func() time.Time
}

// RegisterController wires the licensing controller into the manager. The
// controller is built in, not gated by a feature flag: a cluster always has a
// license policy, even if it is "nothing is installed".
func RegisterController(mgr manager.Manager, ms metricsstorage.Storage, logger *log.Logger) error {
	r := &reconciler{
		Client:        mgr.GetClient(),
		apiReader:     mgr.GetAPIReader(),
		metricStorage: ms,
		recorder:      mgr.GetEventRecorderFor(controllerName),
		logger:        logger,
		vendorKeys:    licensing.VendorPublicKeys,
		// ponytail: the thresholds are compiled in. Sales has not asked for a
		// knob, and specification 7.5 forbids any path that could also feed the
		// metrics.
		thresholds: licensing.DefaultThresholds(),
		dkpVersion: app.Version,
		build:      editionName(),
		now:        time.Now,
	}

	// A cluster with no ClusterLicense at all still needs its policy computed
	// and its cluster data file published, and nothing would ever enqueue a
	// request there. One event at startup gets the loop going; RequeueAfter
	// keeps it alive from then on.
	kick := make(chan event.TypedGenericEvent[client.Object], 1)
	kick <- event.TypedGenericEvent[client.Object]{
		Object: &v1alpha1.EffectiveLicense{ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.EffectiveLicenseName}},
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
			CacheSyncTimeout:        3 * time.Minute,
			NeedLeaderElection:      ptr.To(false),
		}).
		For(&v1alpha1.ClusterLicense{}).
		Watches(&corev1.Node{}, enqueueRecompute(), builder.WithPredicates(nodeChanged{})).
		WatchesRawSource(source.Channel(kick, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

// enqueueRecompute funnels every node event onto the one request the reconciler
// recognises. With MaxConcurrentReconciles at 1 the workqueue then collapses a
// rolling node group update into a single recompute instead of one per node.
func enqueueRecompute() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}}}
	})
}

// nodeChanged is the debounce of specification 10.6: only the two things the
// policy reads off a node wake the controller up. Without it every kubelet
// status heartbeat of every node would recompute the whole policy.
type nodeChanged struct{ predicate.Funcs }

func (nodeChanged) Update(e event.UpdateEvent) bool {
	before, okOld := e.ObjectOld.(*corev1.Node)
	after, okNew := e.ObjectNew.(*corev1.Node)
	if !okOld || !okNew {
		return true
	}
	return !equality.Semantic.DeepEqual(before.Spec.Taints, after.Spec.Taints) ||
		before.Status.Capacity.Cpu().MilliValue() != after.Status.Capacity.Cpu().MilliValue()
}

// editionName returns the edition of the running build, or an empty string when
// it cannot be read: the cluster data file carries it as an optional hint, it is
// never a reason to fail a reconcile.
func editionName() string {
	ed, err := edition.Parse(app.Version)
	if err != nil {
		return ""
	}
	return ed.Name
}

// Reconcile recomputes the whole policy. Every event lands on the same
// computation, so the request is ignored: the policy is a property of the key
// set and the node set, not of the object that happened to change.
func (r *reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	now := r.now()

	priv, err := r.clusterKey(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("cluster key: %w", err)
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return ctrl.Result{}, fmt.Errorf("cluster key is not an Ed25519 key")
	}

	clusterID := r.clusterID(ctx)

	items, keys, failures, err := r.verifyKeys(ctx, licensing.VerifyContext{
		VendorKeys:           r.vendorKeys,
		ClusterID:            clusterID,
		ClusterKeyThumbprint: licensing.Thumbprint(pub),
		Revoked:              licensing.RevokedRecords,
		Now:                  now,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	// A node whose pods cannot be listed aborts the reconcile: the previous
	// status stays as it is, which is the honest answer (vector N12).
	observed, err := r.observeNodes(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	effective, err := r.getEffectiveLicense(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	res := licensing.Compute(licensing.Input{
		Keys:           keys,
		RejectedKeys:   rejectedKeys(items, failures),
		Nodes:          licensable(observed),
		PrevServers:    previousServers(effective.Status),
		OverLimitSince: previousOverLimitSince(effective.Status),
		Now:            now,
		Thresholds:     r.thresholds,
	})

	request, err := r.registrationRequest(ctx, effective.Status.RegistrationRequest, priv, clusterID, res, now)
	if err != nil {
		return ctrl.Result{}, err
	}

	owners, err := r.updateKeyStatuses(ctx, items, keys, failures, res, now)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateEffectiveLicense(ctx, effective, res, observed, request, now); err != nil {
		return ctrl.Result{}, err
	}

	// Deletion comes last: the key has to have carried its Superseded verdict
	// into the status of the record set before it goes away.
	if err := r.deleteSupersededKeys(ctx, effective, items, res); err != nil {
		return ctrl.Result{}, err
	}

	r.publishMetrics(res, owners, now)

	return ctrl.Result{RequeueAfter: requeueAfter(res, r.thresholds, now)}, nil
}

// registrationRequest returns the cluster data file to publish, rebuilding it
// only when the published one no longer describes the cluster (specification
// 10.6). Every rebuild consumes one seq, and the counter is written before the
// request is published so that a crash in between can only skip a number.
func (r *reconciler) registrationRequest(
	ctx context.Context,
	published string,
	priv ed25519.PrivateKey,
	clusterID string,
	res licensing.Result,
	now time.Time,
) (string, error) {
	if published != "" && !requestStale(published, res) {
		return published, nil
	}

	seq, err := r.loadSeq(ctx)
	if err != nil {
		return "", fmt.Errorf("load registration counter: %w", err)
	}
	seq++
	if err := r.saveSeq(ctx, seq); err != nil {
		return "", fmt.Errorf("save registration counter: %w", err)
	}

	request, err := r.buildRegistrationRequest(priv, clusterID, res, seq, now)
	if err != nil {
		return "", fmt.Errorf("build registration request: %w", err)
	}
	return request, nil
}

// deleteSupersededKeys removes the keys a reissue extinguished (specification
// 8.4). It is the one place the controller deletes an object the customer wrote,
// and it is deliberately narrow: only a key whose every record was taken over by
// an accepted successor already in force qualifies. A key that expired without a
// successor, a rejected key and a key carrying an unknown record type all stay.
func (r *reconciler) deleteSupersededKeys(
	ctx context.Context,
	effective *v1alpha1.EffectiveLicense,
	items []v1alpha1.ClusterLicense,
	res licensing.Result,
) error {
	for i := range items {
		item := &items[i]
		if !res.Superseded[item.Name] {
			continue
		}
		if err := r.Delete(ctx, item); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("delete superseded cluster license %s: %w", item.Name, err)
		}
		r.logger.Info("superseded license key deleted",
			slog.String("license", item.Name), slog.String("superseded_by", res.SupersededBy[item.Name]))
		r.recorder.Eventf(effective, corev1.EventTypeNormal, eventKeySuperseded,
			"License key %s (jti %s) was superseded by key %s and has been deleted",
			item.Name, item.Status.PackageJti, res.SupersededBy[item.Name])
	}
	return nil
}

// rejectedKeys names the installed keys that did not verify at all. Such a key
// carries no records, so without this it would leave no trace in the summary.
func rejectedKeys(items []v1alpha1.ClusterLicense, failures []error) []string {
	out := make([]string, 0, len(items))
	for i := range items {
		if failures[i] != nil {
			out = append(out, items[i].Name)
		}
	}
	return out
}

// verifyKeys lists the installed keys and verifies each of them. The returned
// slices are index aligned with the items: failures[i] is the package level
// error of key i, and keys holds the record sets of the keys that survived, in
// the same order.
func (r *reconciler) verifyKeys(ctx context.Context, vc licensing.VerifyContext) (
	[]v1alpha1.ClusterLicense, []licensing.KeyRecords, []error, error,
) {
	var list v1alpha1.ClusterLicenseList
	if err := r.List(ctx, &list); err != nil {
		return nil, nil, nil, fmt.Errorf("list cluster licenses: %w", err)
	}
	// Order decides which of two identical records is the duplicate, so it must
	// not depend on what the cache happened to return.
	items := list.Items
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	keys := make([]licensing.KeyRecords, 0, len(items))
	failures := make([]error, len(items))

	for i, item := range items {
		pkg, records, err := licensing.ParsePackage(item.Spec.LicenseKey, vc)
		if err != nil {
			// The token itself never reaches the log, only the verdict on it.
			r.logger.Warn("license key rejected", slog.String("license", item.Name), log.Err(err))
			failures[i] = err
			continue
		}
		keys = append(keys, licensing.KeyRecords{
			Key: item.Name, JTI: pkg.JTI, CustomerName: pkg.CustomerName, Records: records,
		})
	}

	return items, keys, failures, nil
}

// clusterKey returns the cluster Ed25519 identity, generating it on first use.
// The seed and the private key never reach a log or an object status.
func (r *reconciler) clusterKey(ctx context.Context) (ed25519.PrivateKey, error) {
	key := types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: keySecretName}

	seed, err := r.readSeed(ctx, key)
	switch {
	case err == nil:
		return ed25519.NewKeyFromSeed(seed), nil
	case !apierrors.IsNotFound(err):
		return nil, err
	}

	seed = make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("generate cluster key: %w", err)
	}

	err = r.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Labels: objectLabels},
		Type:       corev1.SecretTypeOpaque,
		Data:       map[string][]byte{keySecretField: seed},
	})
	if apierrors.IsAlreadyExists(err) {
		// Another writer won the race; its key is the cluster identity.
		if seed, err = r.readSeed(ctx, key); err != nil {
			return nil, err
		}
		return ed25519.NewKeyFromSeed(seed), nil
	}
	if err != nil {
		return nil, fmt.Errorf("create cluster key secret: %w", err)
	}

	r.logger.Info("cluster license key generated", slog.String("secret", key.String()))
	return ed25519.NewKeyFromSeed(seed), nil
}

func (r *reconciler) readSeed(ctx context.Context, key types.NamespacedName) ([]byte, error) {
	var secret corev1.Secret
	if err := r.apiReader.Get(ctx, key, &secret); err != nil {
		return nil, err
	}
	seed := secret.Data[keySecretField]
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("secret %s carries a %d byte seed, want %d", key, len(seed), ed25519.SeedSize)
	}
	return seed, nil
}

// clusterID reads the installation UUID Deckhouse already keeps in the
// deckhouse-discovery secret. An empty id is reported, never invented: a random
// one would silently reject every key issued for this cluster.
func (r *reconciler) clusterID(ctx context.Context) string {
	key := types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: app.SecretDiscovery}

	var secret corev1.Secret
	if err := r.apiReader.Get(ctx, key, &secret); err != nil {
		r.logger.Warn("read cluster uuid", slog.String("secret", key.String()), log.Err(err))
		return ""
	}
	id := string(secret.Data["clusterUUID"])
	if id == "" {
		r.logger.Warn("cluster uuid is empty", slog.String("secret", key.String()))
	}
	return id
}
