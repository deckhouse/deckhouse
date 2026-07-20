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

// Package controllers reconciles the per-project catalog (AvailableClusterResource) from the
// cluster-scoped grant model, and maintains the gcrd↔reference binding status. It is keyed by
// namespace: each project namespace gets its AvailableClusterResource catalog.
package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/api/v1alpha1"
	"controller/internal/namespaces"
	"controller/internal/naming"
	"controller/internal/resolve"
)

// ResyncInterval is the period at which a project namespace is re-reconciled so the catalog
// (recomputed from live granted objects, not all watched) does not drift unbounded.
const ResyncInterval = 2 * time.Minute

// ProjectReconciler materializes AvailableClusterResource catalogs for project namespaces.
type ProjectReconciler struct {
	client.Client
	Mapper meta.RESTMapper
}

// Reconcile reconciles a single (project) namespace.
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if namespaces.IsSystem(req.Name) {
		return ctrl.Result{}, nil
	}
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get namespace: %w", err)
	}
	// Only project namespaces (carrying the project label) get a catalog. Any other namespace —
	// the default namespace, system namespaces, namespaces of "virtual" projects — must not, even
	// when a registration's defaultAvailability is All. Clean up any catalog that lingers there.
	if _, isProjectNS := ns.Labels[naming.ProjectLabel]; !isProjectNS {
		return ctrl.Result{}, r.cleanupCatalog(ctx, ns.Name)
	}

	project := resolve.ProjectName(ns)
	if err := r.reconcileCatalog(ctx, ns, project); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile catalog: %w", err)
	}
	return ctrl.Result{RequeueAfter: ResyncInterval}, nil
}

// cleanupCatalog removes every AvailableClusterResource from a namespace that is not (or no longer) a
// project namespace, so the catalog never leaks into the default/system/virtual-project namespaces.
func (r *ProjectReconciler) cleanupCatalog(ctx context.Context, ns string) error {
	list := &v1alpha1.AvailableClusterResourceList{}
	if err := r.List(ctx, list, client.InNamespace(ns)); err != nil {
		return fmt.Errorf("list AvailableClusterResource in %s: %w", ns, err)
	}
	for i := range list.Items {
		if err := r.Delete(ctx, &list.Items[i]); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete stale AvailableClusterResource %s/%s: %w", ns, list.Items[i].Name, err)
		}
	}
	return nil
}

// reconcileCatalog upserts an AvailableClusterResource per registration for the namespace, deleting
// catalogs that became empty.
func (r *ProjectReconciler) reconcileCatalog(ctx context.Context, ns *corev1.Namespace, project string) error {
	grants, err := resolve.GrantsForLabels(ctx, r.Client, ns.Labels)
	if err != nil {
		return err
	}
	regList := &v1alpha1.GrantableClusterResourceDefinitionList{}
	if err := r.List(ctx, regList); err != nil {
		return err
	}
	for i := range regList.Items {
		reg := &regList.Items[i]
		entries := resolve.EntriesFor(grants, reg.Name)
		resolved, err := resolve.Resolve(ctx, r.Client, r.Mapper, reg, entries)
		if err != nil {
			return err
		}
		available := resolved.Available()
		if len(available) == 0 {
			// Nothing available here: ensure no stale catalog object lingers.
			_ = r.Delete(ctx, &v1alpha1.AvailableClusterResource{ObjectMeta: metav1.ObjectMeta{Name: reg.Name, Namespace: ns.Name}})
			continue
		}
		kind := ""
		if reg.Spec.GrantedResource != nil {
			kind = reg.Spec.GrantedResource.Kind
		}
		if err := r.upsertAvailable(ctx, ns.Name, project, reg.Name, kind, available, resolved.Default()); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProjectReconciler) upsertAvailable(ctx context.Context, ns, project, name, kind string, available []v1alpha1.AvailableObject, def string) error {
	ar := &v1alpha1.AvailableClusterResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, ar, func() error {
		if ar.Labels == nil {
			ar.Labels = map[string]string{}
		}
		for k, v := range naming.ManagedLabels(project) {
			ar.Labels[k] = v
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("upsert AvailableClusterResource %s/%s: %w", ns, name, err)
	}
	ar.Status.GrantedResourceKind = kind
	ar.Status.Available = available
	ar.Status.Default = def
	ar.Status.AvailableCount = len(available)
	if err := r.Status().Update(ctx, ar); err != nil {
		return fmt.Errorf("update AvailableClusterResource status %s/%s: %w", ns, name, err)
	}
	return nil
}

// SetupWithManager wires the reconciler and its watches.
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
	enqueueProjectNamespaces := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, _ client.Object) []reconcile.Request {
			nsList := &corev1.NamespaceList{}
			if err := r.List(ctx, nsList); err != nil {
				return nil
			}
			reqs := make([]reconcile.Request, 0, len(nsList.Items))
			for i := range nsList.Items {
				if namespaces.IsSystem(nsList.Items[i].Name) {
					continue
				}
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: nsList.Items[i].Name}})
			}
			return reqs
		},
	)

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(&v1alpha1.ClusterResourceGrantPolicy{}, enqueueProjectNamespaces).
		Watches(&v1alpha1.GrantableClusterResourceDefinition{}, enqueueProjectNamespaces).
		Watches(&v1alpha1.GrantableClusterResourceReference{}, enqueueProjectNamespaces).
		Named("project-grants").
		Complete(r)
}
