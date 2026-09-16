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
// installed ClusterLicense keys, samples the consumption metrics into a durable
// journal, publishes the aggregated EffectiveLicense and keeps a signed
// registration request ready for the customer to hand to the license server.
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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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
	resyncPeriod = 10 * time.Minute

	// minSampleAge is the age at which the last observation is due for a
	// successor. It is short of the nominal hour so that a resync landing a few
	// minutes early still samples, instead of pushing every sample to 70 minutes.
	minSampleAge = 55 * time.Minute
	// retention is the length of the observation window behind avg_7d.
	retention = 7 * 24 * time.Hour
	// defaultHorizon is how far ahead consumption is extrapolated when no active
	// record expires.
	defaultHorizon = 30 * 24 * time.Hour

	// keySecretName holds the cluster Ed25519 identity as a raw 32 byte seed.
	keySecretName  = "cluster-key"
	keySecretField = "seed"

	// journalConfigMapName holds the observation window and the registration
	// counter, both of which must survive a restart.
	journalConfigMapName = "consumption-journal"
	journalField         = "journal"
	// journalCorruptField parks a journal payload that could not be decoded, so
	// that restarting the window does not destroy the evidence.
	journalCorruptField = "journal.corrupt"

	metricVCPU  = "vCPU"
	metricNodes = "nodes"
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
	// journal ConfigMap fall outside the label selectors the manager caches, and
	// a stale miss on the key would mint a second cluster identity.
	apiReader client.Reader

	metricStorage metricsstorage.Storage
	logger        *log.Logger

	// vendorKeys defaults to licensing.VendorPublicKeys. A test issues its own
	// packages and overrides the field instead of the package variable.
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
		logger:        logger,
		vendorKeys:    licensing.VendorPublicKeys,
		// ponytail: the thresholds are compiled in. Sales has not asked for a
		// knob, and §7.2 forbids any path that could also feed the metrics.
		thresholds: licensing.DefaultThresholds(),
		dkpVersion: app.Version,
		build:      editionName(),
		now:        time.Now,
	}

	// A cluster with no ClusterLicense at all still needs its policy computed
	// and its registration request published, and nothing would ever enqueue a
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
		WatchesRawSource(source.Channel(kick, &handler.EnqueueRequestForObject{})).
		Complete(r)
}

// editionName returns the edition of the running build, or an empty string when
// it cannot be read: the registration request carries it as an optional hint,
// it is never a reason to fail a reconcile.
func editionName() string {
	ed, err := edition.Parse(app.Version)
	if err != nil {
		return ""
	}
	return ed.Name
}

// Reconcile recomputes the whole policy. Every event lands on the same
// computation, so the request is ignored: the policy is a property of the key
// set, not of the key that happened to change.
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

	licenses, keys, failures, packages, err := r.verifyKeys(ctx, licensing.VerifyContext{
		VendorKeys:           r.vendorKeys,
		ClusterID:            clusterID,
		ClusterKeyThumbprint: licensing.Thumbprint(pub),
		Revoked:              licensing.RevokedRecords,
		Now:                  now,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	journal, corrupt, err := r.loadJournal(ctx)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("load journal: %w", err)
	}

	// The extrapolation horizon is the nearest expiry, which is only known once
	// the records have been resolved; the records do not depend on the metrics,
	// so a first pass without metrics settles the horizon and the second pass
	// produces the policy that is published.
	horizon := horizonOf(licensing.Compute(keys, nil, now, r.thresholds), now)

	due := sampleDue(journal, now)
	if due {
		// A failed observation aborts the reconcile: nothing is appended, the
		// counter does not move, and controller-runtime retries with backoff.
		values, err := r.sampleConsumption(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		journal.Add(licensing.Sample{At: now, Values: values}, retention)
	}

	values := map[string]licensing.MetricValue{
		metricVCPU:  stats(journal, metricVCPU, now, horizon),
		metricNodes: stats(journal, metricNodes, now, horizon),
	}
	res := licensing.Compute(keys, values, now, r.thresholds)

	effective, err := r.getEffectiveLicense(ctx)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Every request that is issued carries its own seq, whether it was a
	// sampling tick that produced it or a lost status that had to be rebuilt:
	// the license server reads seq as a strictly growing counter.
	request := effective.Status.RegistrationRequest
	issue := due || request == ""
	if issue {
		journal.Seq++
		request, err = r.buildRegistrationRequest(priv, clusterID, res, values, journal.Seq, now)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("build registration request: %w", err)
		}
		if err := r.saveJournal(ctx, journal, corrupt); err != nil {
			return ctrl.Result{}, fmt.Errorf("save journal: %w", err)
		}
	}

	owners, err := r.updateKeyStatuses(ctx, licenses, keys, failures, packages, res)
	if err != nil {
		return ctrl.Result{}, err
	}

	if err := r.updateEffectiveLicense(ctx, effective, res, values, request, now); err != nil {
		return ctrl.Result{}, err
	}

	r.publishMetrics(res, values, owners, now)

	return ctrl.Result{RequeueAfter: requeueAfter(res, journal, now)}, nil
}

// verifyKeys lists the installed keys and verifies each of them. The returned
// slices are index aligned with licenses.Items: failures[i] is the package level
// error of key i, packages[i] its parsed envelope, and keys holds the record
// sets of the keys that survived, in the same order.
func (r *reconciler) verifyKeys(ctx context.Context, vc licensing.VerifyContext) (
	[]v1alpha1.ClusterLicense, []licensing.KeyRecords, []error, []*licensing.Package, error,
) {
	var list v1alpha1.ClusterLicenseList
	if err := r.List(ctx, &list); err != nil {
		return nil, nil, nil, nil, fmt.Errorf("list cluster licenses: %w", err)
	}
	// Order decides which of two identical records is the duplicate, so it must
	// not depend on what the cache happened to return.
	items := list.Items
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	keys := make([]licensing.KeyRecords, 0, len(items))
	failures := make([]error, len(items))
	packages := make([]*licensing.Package, len(items))

	for i, item := range items {
		pkg, records, err := licensing.ParsePackage(item.Spec.LicenseKey, vc)
		if err != nil {
			// The token itself never reaches the log, only the verdict on it.
			r.logger.Warn("license key rejected", slog.String("license", item.Name), log.Err(err))
			failures[i] = err
			continue
		}
		packages[i] = pkg
		keys = append(keys, licensing.KeyRecords{Key: item.Name, Records: records})
	}

	return items, keys, failures, packages, nil
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
