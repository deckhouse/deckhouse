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

package providercheck

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
)

// maxConcurrentReconciles is the in-flight LDAP/HTTP probe cap. Each worker
// runs one DexProviderCheck; 16 covers a large provider set without an
// unbounded goroutine pool. User reconcile uses a different controller and
// is not blocked by these workers.
const maxConcurrentReconciles = 16

// Reconciler runs DexProvider connectivity checks.
type Reconciler struct {
	client client.Client
	log    logr.Logger
	http   HTTPFactory
	ldap   LDAPDialer
	now    func() time.Time
}

// New constructs a Reconciler. http/ldap/now default when nil.
func New(c client.Client, log logr.Logger, http HTTPFactory, ldap LDAPDialer, now func() time.Time) *Reconciler {
	if now == nil {
		now = time.Now
	}
	if http == nil {
		http = NewDefaultHTTP()
	}
	if ldap == nil {
		ldap = NewDefaultLDAP()
	}
	return &Reconciler{client: c, log: log, http: http, ldap: ldap, now: now}
}

// Register wires the DexProviderCheck reconciler onto the manager.
//
// Required RBAC:
//
//	dexproviderchecks: get, list, watch, create, update, patch, delete
//	dexproviderchecks/status: get, update, patch
//	dexproviders: get, list, watch
//	secrets: get (Kerberos keytab in d8-user-authn)
func Register(mgr manager.Manager) error {
	if err := AddToScheme(mgr.GetScheme()); err != nil {
		return fmt.Errorf("add dexprovidercheck types: %w", err)
	}
	r := New(mgr.GetClient(), mgr.GetLogger().WithName("dexprovidercheck"), nil, nil, nil)
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup dexprovidercheck controller: %w", err)
	}
	return nil
}

// SetupWithManager registers watches for DexProviderCheck and DexProvider.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("dexprovidercheck").
		WithOptions(ctrlcontroller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(&DexProviderCheck{}).
		Watches(controller.Object(controller.DexProviderGVK), handler.EnqueueRequestsFromMapFunc(mapDexProvider)).
		Complete(r)
}

func mapDexProvider(_ context.Context, obj client.Object) []reconcile.Request {
	if obj == nil || obj.GetName() == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
}

// Reconcile acknowledges a check as Pending, runs probes, and writes
// Succeeded/Failed. Fresh results are requeued after the 1h recheck interval.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	check := &DexProviderCheck{}
	err := r.client.Get(ctx, req.NamespacedName, check)
	if apierrors.IsNotFound(err) {
		return reconcile.Result{}, nil
	}
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("get dexprovidercheck %s: %w", req.Name, err)
	}

	provider, found, err := r.getProvider(ctx, check.Spec.ProviderName)
	if err != nil {
		return reconcile.Result{}, err
	}
	if !found || provider.Name == "" || check.Name != canonicalCheckName(provider.Name) {
		if err := r.deleteCheck(ctx, check); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	now := r.now()
	if checkCompleted(check) && checkUpToDate(check, provider.Generation, now) {
		return reconcile.Result{RequeueAfter: requeueRemaining(check.Status.CompletedAt.Time, now)}, nil
	}

	if err := r.markPending(ctx, check, provider.Generation); err != nil {
		return reconcile.Result{}, err
	}

	probeCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	status := r.execute(probeCtx, check, provider)

	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	check.Status = status
	if err := r.client.Status().Update(ctx, check); err != nil {
		return reconcile.Result{}, fmt.Errorf("update dexprovidercheck %s status: %w", check.Name, err)
	}
	return reconcile.Result{RequeueAfter: recheckInterval}, nil
}

func (r *Reconciler) getProvider(ctx context.Context, name string) (DexProviderForCheck, bool, error) {
	if name == "" {
		return DexProviderForCheck{}, false, nil
	}
	obj := controller.Object(controller.DexProviderGVK)
	err := r.client.Get(ctx, types.NamespacedName{Name: name}, obj)
	if apierrors.IsNotFound(err) {
		return DexProviderForCheck{}, false, nil
	}
	if err != nil {
		return DexProviderForCheck{}, false, fmt.Errorf("get dexprovider %s: %w", name, err)
	}
	provider, err := decodeProvider(obj)
	if err != nil {
		return DexProviderForCheck{}, false, err
	}
	return provider, true, nil
}

func (r *Reconciler) deleteCheck(ctx context.Context, check *DexProviderCheck) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.client.Delete(ctx, check); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete dexprovidercheck %s: %w", check.Name, err)
	}
	r.log.Info("deleted dexprovidercheck", "name", check.Name)
	return nil
}

func (r *Reconciler) markPending(ctx context.Context, check *DexProviderCheck, generation int64) error {
	if check.Status.Phase == DexProviderCheckPhasePending &&
		check.Status.ObservedDexProviderGeneration == generation &&
		len(check.Status.Checks) == 0 &&
		check.Status.CompletedAt == nil {
		return nil
	}

	check.Status.Phase = DexProviderCheckPhasePending
	check.Status.Message = pendingMessage
	check.Status.ObservedDexProviderGeneration = generation
	check.Status.Checks = nil
	check.Status.CompletedAt = nil
	if err := r.client.Status().Update(ctx, check); err != nil {
		return fmt.Errorf("update dexprovidercheck %s pending status: %w", check.Name, err)
	}
	return nil
}
