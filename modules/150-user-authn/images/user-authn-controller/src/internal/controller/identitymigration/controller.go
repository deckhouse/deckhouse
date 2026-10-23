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

// Package identitymigration copies what older Dex keeps in OfflineSessions (TOTP secret,
// external lock) and Password (local lock) into the UserIdentity of the same (connector, user)
// pair, the object Dex v2.46 keeps that data in, and keeps the copy up to date while the old
// Dex is still writing the old places.
//
// It runs on the leader in two parts. The first pass pages through every OfflineSessions, every
// locked Password, and every UserIdentity not marked yet through the API reader (the cache strips
// the TOTP secret) and enqueues their pairs. Afterwards the controller copies a pair again when its
// OfflineSessions or Password changes, and marks a UserIdentity Dex creates. Both parts reconcile
// one pair at a time through the same workqueue.
//
// A local pair with neither a Password nor a User is what a user deleted before 1.78 left behind.
// It is not copied, and its OfflineSessions is deleted, so that neither the old Dex nor the copy
// hands its TOTP secret to a user created again with the same name.
//
// Every UserIdentity the controller has reconciled carries the annotation
// user-authn.deckhouse.io/migrated, including one Dex created for a pair the old places know
// nothing about: the transitional patch of Dex 1.78 reads the old places for an identity without
// it on every read.
package identitymigration

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

const (
	controllerName = "identitymigration"

	// maxConcurrentReconciles is low on purpose: a pair costs two or three API reads, and the
	// client of this controller has no rate limit of its own.
	maxConcurrentReconciles = 2

	// retryBaseDelay and retryMaxDelay bound the delay before a failed pair is copied again. The
	// default limiter of controller-runtime doubles it up to about 16 minutes, so a copy that failed
	// for a while resumed long after the cause was gone; with this cap it resumes within a minute.
	retryBaseDelay = time.Second
	retryMaxDelay  = time.Minute
)

// newRateLimiter returns the retry limiter of the controller: a delay per pair that doubles from
// retryBaseDelay up to retryMaxDelay.
//
// Unlike the default limiter, it has no token bucket shared by all pairs. At 10 retries a second,
// a thousand pairs failing at once would wait up to a minute and a half for their retry, and ten
// thousand up to 16 minutes, so the cap would not hold. The load stays bounded without the
// bucket: two workers copy one pair each at a time, and the first pass enqueues at most
// firstPassPairsPerSecond.
func newRateLimiter() workqueue.TypedRateLimiter[reconcile.Request] {
	return workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](retryBaseDelay, retryMaxDelay)
}

// Migrator copies the old places of a pair into its UserIdentity.
type Migrator struct {
	client    client.Client
	apiReader client.Reader
	places    oldPlaces
	log       logr.Logger
	now       func() time.Time

	progress *progress
	results  *prometheus.CounterVec
	events   chan event.GenericEvent
}

// New constructs a Migrator with its own metrics. now defaults to time.Now when nil.
// c serves cached reads (the Password index, refresh token claims) and writes; apiReader must
// bypass the cache, which strips the TOTP secret.
func New(c client.Client, apiReader client.Reader, log logr.Logger, now func() time.Time) *Migrator {
	return newMigrator(c, apiReader, log, now, newPendingGauge(), newTotalCounter())
}

func newMigrator(
	c client.Client,
	apiReader client.Reader,
	log logr.Logger,
	now func() time.Time,
	pending prometheus.Gauge,
	results *prometheus.CounterVec,
) *Migrator {
	if now == nil {
		now = time.Now
	}
	return &Migrator{
		client:    c,
		apiReader: apiReader,
		places:    oldPlaces{client: c, apiReader: apiReader, log: log},
		log:       log,
		now:       now,
		progress:  newProgress(pending),
		results:   results,
		events:    make(chan event.GenericEvent, firstPassPageSize),
	}
}

// Register wires the migration controller and its first pass onto the manager and registers
// d8_user_authn_identity_migration_pending and d8_user_authn_identity_migration_total.
//
// Required RBAC: offlinesessionses get, list, watch, delete; passwords get, list, watch; users
// get; refreshtokens get, list, watch; useridentities get, list, watch, create, update.
func Register(mgr manager.Manager) error {
	if err := registerMetrics(); err != nil {
		return err
	}
	m := newMigrator(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetLogger().WithName(controllerName), nil, pendingGauge, totalCounter)
	if err := m.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup %s controller: %w", controllerName, err)
	}
	if err := mgr.Add(m.FirstPass(mgr.GetCache())); err != nil {
		return fmt.Errorf("add %s first pass: %w", controllerName, err)
	}
	return nil
}

// SetupWithManager watches OfflineSessions and Password for changes that matter to the copy,
// UserIdentity objects that are not marked yet, and the first pass through a channel. Objects of
// the informers' initial list are left to the first pass, which reads them page by page instead of
// all at once.
func (m *Migrator) SetupWithManager(mgr manager.Manager) error {
	inDexNS := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == naming.DexNamespace
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named(controllerName).
		WithOptions(ctrlcontroller.Options{
			MaxConcurrentReconciles: maxConcurrentReconciles,
			RateLimiter:             newRateLimiter(),
		}).
		Watches(
			controller.Object(controller.OfflineSessionsGVK),
			handler.EnqueueRequestsFromMapFunc(mapByName),
			builder.WithPredicates(inDexNS, changesOf(sessionFingerprint)),
		).
		Watches(
			controller.Object(controller.PasswordGVK),
			handler.EnqueueRequestsFromMapFunc(mapPassword),
			builder.WithPredicates(inDexNS, changesOf(passwordFingerprint)),
		).
		Watches(
			controller.Object(controller.UserIdentityGVK),
			handler.EnqueueRequestsFromMapFunc(mapByName),
			builder.WithPredicates(inDexNS, unmarkedIdentity()),
		).
		WatchesRawSource(source.Channel(m.events, handler.EnqueueRequestsFromMapFunc(mapByName))).
		Complete(m)
}

// Reconcile copies the pair req.Name, the name of its OfflineSessions and UserIdentity.
func (m *Migrator) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	result, err := m.syncPair(ctx, req.Name)
	m.results.WithLabelValues(result).Inc()
	if err != nil {
		m.progress.add(req.Name)
		return reconcile.Result{}, err
	}
	m.progress.done(req.Name)
	return reconcile.Result{}, nil
}

func (m *Migrator) syncPair(ctx context.Context, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return resultFailed, err
	}
	now := m.now()

	result := resultUnchanged
	var skipReason string
	var seen pair
	var marked bool
	outcome, err := useridentity.Mutate(ctx, m.apiReader, m.client, name, func(current *useridentity.Identity) (*useridentity.Identity, error) {
		result, skipReason, marked = resultUnchanged, "", false
		p, reason, err := m.places.readPair(ctx, name)
		if err != nil {
			return nil, err
		}
		seen = p
		if reason != "" {
			result, skipReason = resultSkipped, reason
			return nil, nil
		}
		if p.UserID == "" {
			// Neither OfflineSessions nor Password: nothing to copy. A UserIdentity Dex created for
			// the pair is marked all the same.
			desired, err := markMigrated(current, now)
			marked = desired != nil
			return desired, err
		}
		desired, err := plan(current, p, now)
		if errors.Is(err, errPairMismatch) {
			result, skipReason = resultSkipped, skipPairMismatch
			return nil, nil
		}
		return desired, err
	})
	if err != nil {
		return resultFailed, fmt.Errorf("copy pair %s into useridentity: %w", name, err)
	}

	switch {
	case skipReason == skipLocalOrphan && seen.Session != nil:
		// Expected once for every user deleted before 1.78: deleting the User deleted only its Password.
		if err := m.deleteOrphanSession(ctx, name, seen.Session); err != nil {
			return resultFailed, err
		}
	case skipReason == skipLocalNoPassword:
		// The Password of the User comes next, and its event brings the pair back.
		m.log.V(1).Info("Not copying the pair into UserIdentity", "pair", name, "reason", skipReason)
	case skipReason != "":
		m.log.Info("Not copying the pair into UserIdentity", "pair", name, "reason", skipReason)
	case marked && outcome == useridentity.OutcomeUpdated:
		m.log.V(1).Info("Marked the UserIdentity as migrated, the old places hold nothing of the pair", "pair", name)
		result = resultMigrated
	case outcome == useridentity.OutcomeCreated || outcome == useridentity.OutcomeUpdated:
		m.log.Info("Copied the pair into UserIdentity",
			"pair", name,
			"connectorID", seen.ConnID,
			"userID", seen.UserID,
			"created", outcome == useridentity.OutcomeCreated,
		)
		result = resultMigrated
	}
	return result, nil
}

// markMigrated returns current with a record of what the controller copied, which is nothing, or
// nil when there is no object or it already has the annotation.
func markMigrated(current *useridentity.Identity, now time.Time) (*useridentity.Identity, error) {
	if current == nil || current.Migrated() {
		return nil, nil
	}
	if err := current.SetRecord(useridentity.Record{Time: useridentity.FormatTime(now)}); err != nil {
		return nil, err
	}
	return current, nil
}

func mapByName(_ context.Context, obj client.Object) []reconcile.Request {
	if obj.GetName() == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
}

func mapPassword(_ context.Context, obj client.Object) []reconcile.Request {
	names := controller.PasswordPair(obj)
	reqs := make([]reconcile.Request, 0, len(names))
	for _, name := range names {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	}
	return reqs
}

// changesOf passes the events that can change what the copy writes: objects created after the
// informers' initial list, and updates that change fingerprint. Deletions change nothing: the copy
// never removes data from a UserIdentity. Dex rewrites OfflineSessions on every token refresh;
// those updates are dropped here.
//
// The cache strips the TOTP secret, so setting a secret on an existing OfflineSessions is not
// seen by itself. Dex sets it at a login, which also replaces a refresh token, and the first
// pass after every controller start copies it regardless.
func changesOf(fingerprint func(*unstructured.Unstructured) any) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool { return !e.IsInInitialList },
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldObj, ok := e.ObjectOld.(*unstructured.Unstructured)
			if !ok {
				return true
			}
			newObj, ok := e.ObjectNew.(*unstructured.Unstructured)
			if !ok {
				return true
			}
			return !reflect.DeepEqual(fingerprint(oldObj), fingerprint(newObj))
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return true },
	}
}

// unmarkedIdentity passes the UserIdentity objects without the annotation that Dex creates or
// writes after the informers' initial list; the first pass takes those of the initial list. The
// controller's own writes carry the annotation and are dropped here.
func unmarkedIdentity() predicate.Funcs {
	unmarked := func(obj client.Object) bool {
		_, ok := obj.GetAnnotations()[useridentity.RecordAnnotation]
		return !ok
	}
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return !e.IsInInitialList && unmarked(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return unmarked(e.ObjectNew) },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return true },
	}
}

type sessionPrint struct {
	UID           string
	UserID        string
	ConnID        string
	Email         string
	TOTPConfirmed bool
	LockedUntil   string
	AdminLock     bool
	RefreshIDs    map[string]string
}

func sessionFingerprint(u *unstructured.Unstructured) any {
	p := sessionPrint{
		UID:       string(u.GetUID()),
		AdminLock: hasAdminLock(u.GetAnnotations()),
	}
	p.UserID, _, _ = unstructured.NestedString(u.Object, "userID")
	p.ConnID, _, _ = unstructured.NestedString(u.Object, "connID")
	p.Email, _, _ = unstructured.NestedString(u.Object, "email")
	p.TOTPConfirmed, _, _ = unstructured.NestedBool(u.Object, "totpConfirmed")
	p.LockedUntil, _, _ = unstructured.NestedString(u.Object, "lockedUntil")
	if refresh, ok := u.Object["refresh"].(map[string]any); ok {
		p.RefreshIDs = make(map[string]string, len(refresh))
		for client, entry := range refresh {
			if m, ok := entry.(map[string]any); ok {
				id, _ := m["ID"].(string)
				p.RefreshIDs[client] = id
			}
		}
	}
	return p
}

type passwordPrint struct {
	UID         string
	UserID      string
	Username    string
	Email       string
	Groups      []any
	LockedUntil string
	AdminLock   bool
}

func passwordFingerprint(u *unstructured.Unstructured) any {
	p := passwordPrint{
		UID:       string(u.GetUID()),
		AdminLock: hasAdminLock(u.GetAnnotations()),
	}
	p.UserID, _, _ = unstructured.NestedString(u.Object, "userID")
	p.Username, _, _ = unstructured.NestedString(u.Object, "username")
	p.Email, _, _ = unstructured.NestedString(u.Object, "email")
	p.Groups, _ = u.Object["groups"].([]any)
	p.LockedUntil, _, _ = unstructured.NestedString(u.Object, "lockedUntil")
	return p
}
