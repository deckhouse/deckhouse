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

package useroperation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// maxConcurrentReconciles is 4: UserOperations are rare and list Dex objects.
const maxConcurrentReconciles = 4

// Reconciler executes UserOperation objects against Dex Password / session CRs.
type Reconciler struct {
	client client.Client
	log    logr.Logger
	now    func() time.Time
}

// New constructs a Reconciler. now defaults to time.Now when nil.
func New(c client.Client, log logr.Logger, now func() time.Time) *Reconciler {
	if now == nil {
		now = time.Now
	}
	return &Reconciler{client: c, log: log, now: now}
}

// Register wires the UserOperation reconciler onto the manager.
func Register(mgr manager.Manager) error {
	r := New(mgr.GetClient(), mgr.GetLogger().WithName("useroperation"), nil)
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup useroperation controller: %w", err)
	}
	return nil
}

// SetupWithManager registers watches for UserOperation and Dex objects that
// must live in the informer cache for lock / session invalidation.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	inDexNS := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == naming.DexNamespace
	})
	warmCache := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return nil
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("useroperation").
		WithOptions(ctrlcontroller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(controller.Object(controller.UserOperationGVK)).
		Watches(controller.Object(controller.PasswordGVK), warmCache, builder.WithPredicates(inDexNS)).
		Watches(controller.Object(controller.OfflineSessionsGVK), warmCache, builder.WithPredicates(inDexNS)).
		Watches(controller.Object(controller.RefreshTokenGVK), warmCache, builder.WithPredicates(inDexNS)).
		Complete(r)
}

// Reconcile executes a pending UserOperation or deletes one past retention.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	obj := controller.Object(controller.UserOperationGVK)
	err := r.client.Get(ctx, req.NamespacedName, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("get useroperation %s: %w", req.Name, err)
	}

	now := r.now()
	op := decodeOperation(obj)

	if op.Status.Phase != "" {
		return r.reconcileTerminal(ctx, obj, op, now)
	}

	r.log.Info("Executing UserOperation", operationLogKV(op)...)
	execErr := r.execute(ctx, op, now)
	if execErr != nil {
		var perm failedError
		if !errors.As(execErr, &perm) {
			return reconcile.Result{}, execErr
		}
		r.log.Error(execErr, "Failed to execute UserOperation", operationLogKV(op)...)
		return r.complete(ctx, obj, op, phaseFailed, perm.Error(), now)
	}

	r.log.Info("UserOperation succeeded", operationLogKV(op)...)
	return r.complete(ctx, obj, op, phaseSucceeded, "", now)
}

func (r *Reconciler) reconcileTerminal(ctx context.Context, obj *unstructured.Unstructured, op operation, now time.Time) (reconcile.Result, error) {
	if err := r.wipeResetPassword(ctx, obj, op); err != nil {
		return reconcile.Result{}, err
	}

	age := now.Sub(op.CreationTimestamp.Time)
	if age >= retentionPeriod {
		r.log.Info("Deleting old UserOperation", operationLogKV(op)...)
		if err := r.client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("delete useroperation %s: %w", obj.GetName(), err)
		}
		return reconcile.Result{}, nil
	}

	remaining := retentionPeriod - age
	return reconcile.Result{RequeueAfter: remaining}, nil
}

func (r *Reconciler) complete(ctx context.Context, obj *unstructured.Unstructured, op operation, phase statusPhase, message string, now time.Time) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	status := map[string]any{
		"phase":       string(phase),
		"completedAt": now.UTC().Format(time.RFC3339),
		"message":     message,
	}
	if err := unstructured.SetNestedMap(obj.Object, status, "status"); err != nil {
		return reconcile.Result{}, fmt.Errorf("set useroperation %s status: %w", obj.GetName(), err)
	}
	if err := r.client.Status().Update(ctx, obj); err != nil {
		return reconcile.Result{}, fmt.Errorf("update useroperation %s status: %w", obj.GetName(), err)
	}

	op.Status.Phase = phase
	if err := r.wipeResetPassword(ctx, obj, op); err != nil {
		return reconcile.Result{}, err
	}

	remaining := retentionPeriod - now.Sub(op.CreationTimestamp.Time)
	if remaining < 0 {
		remaining = 0
	}
	return reconcile.Result{RequeueAfter: remaining}, nil
}

func (r *Reconciler) wipeResetPassword(ctx context.Context, obj *unstructured.Unstructured, op operation) error {
	specType, found, err := unstructured.NestedString(obj.Object, "spec", "type")
	if err != nil || !found {
		return nil
	}
	if specType != string(typeResetPassword) && op.Spec.Type != typeResetPassword {
		return nil
	}
	_, found, err = unstructured.NestedMap(obj.Object, "spec", "resetPassword")
	if err != nil || !found {
		return nil
	}

	raw, err := json.Marshal(map[string]any{
		"spec": map[string]any{"resetPassword": nil},
	})
	if err != nil {
		return fmt.Errorf("marshal wipe resetPassword: %w", err)
	}
	if err := r.client.Patch(ctx, obj, client.RawPatch(types.MergePatchType, raw)); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("wipe resetPassword on %s: %w", obj.GetName(), err)
	}
	return nil
}
