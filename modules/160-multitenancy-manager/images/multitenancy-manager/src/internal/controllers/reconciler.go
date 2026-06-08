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

// Package controllers reconciles the per-project catalog (AvailableResource) and object-quota usage
// (GrantQuota) from the cluster-scoped grant model. It is keyed by namespace: each project namespace
// gets its AvailableResource catalog, and the project's GrantQuota pool/rendered usage is recomputed.
package controllers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/api/v1alpha1"
	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/namespaces"
	"controller/internal/naming"
	"controller/internal/quota"
	"controller/internal/resolve"
)

// ResyncInterval is the period at which a project namespace is re-reconciled so that quota usage
// (which is recomputed from live usage objects, not watched) does not drift unbounded.
const ResyncInterval = 2 * time.Minute

// ProjectReconciler materializes AvailableResource and GrantQuota for project namespaces.
type ProjectReconciler struct {
	client.Client
	Mapper  meta.RESTMapper
	Factory jsonpath.Factory
}

// Reconcile reconciles a single (project) namespace.
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx).WithValues("namespace", req.Name)

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
	// when a registration's defaultAvailability is All. Clean up any catalog that lingers there
	// (e.g. left over from a namespace that lost its project membership).
	if _, isProjectNS := ns.Labels[naming.ProjectLabel]; !isProjectNS {
		return ctrl.Result{}, r.cleanupCatalog(ctx, ns.Name)
	}

	project := resolve.ProjectName(ns)

	if err := r.reconcileCatalog(ctx, ns, project); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile catalog: %w", err)
	}
	if err := r.reconcileQuota(ctx, project, log); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconcile quota: %w", err)
	}

	return ctrl.Result{RequeueAfter: ResyncInterval}, nil
}

// cleanupCatalog removes every AvailableResource from a namespace that is not (or no longer) a
// project namespace, so the catalog never leaks into the default/system/virtual-project namespaces.
func (r *ProjectReconciler) cleanupCatalog(ctx context.Context, ns string) error {
	list := &v1alpha1.AvailableResourceList{}
	if err := r.List(ctx, list, client.InNamespace(ns)); err != nil {
		return fmt.Errorf("list AvailableResource in %s: %w", ns, err)
	}
	for i := range list.Items {
		if err := r.Delete(ctx, &list.Items[i]); err != nil && !k8serrors.IsNotFound(err) {
			return fmt.Errorf("delete stale AvailableResource %s/%s: %w", ns, list.Items[i].Name, err)
		}
	}
	return nil
}

// reconcileCatalog upserts an AvailableResource per registration for the namespace, deleting catalogs
// that became empty.
func (r *ProjectReconciler) reconcileCatalog(ctx context.Context, ns *corev1.Namespace, project string) error {
	grants, err := resolve.GrantsForLabels(ctx, r.Client, ns.Labels)
	if err != nil {
		return err
	}
	regList := &v1alpha1.ClusterGrantableResourceList{}
	if err := r.List(ctx, regList); err != nil {
		return err
	}
	for i := range regList.Items {
		reg := &regList.Items[i]
		entries := resolve.EntriesFor(grants, reg.Name)
		resolved, err := resolve.Resolve(ctx, r.Client, reg, entries)
		if err != nil {
			return err
		}
		available := resolved.Available()
		if len(available) == 0 {
			// Nothing available here: ensure no stale catalog object lingers.
			_ = r.Delete(ctx, &v1alpha1.AvailableResource{ObjectMeta: metav1.ObjectMeta{Name: reg.Name, Namespace: ns.Name}})
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
	ar := &v1alpha1.AvailableResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}
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
		return fmt.Errorf("upsert AvailableResource %s/%s: %w", ns, name, err)
	}
	ar.Status.GrantedResourceKind = kind
	ar.Status.Available = available
	ar.Status.Default = def
	names := make([]string, 0, len(available))
	for i := range available {
		names = append(names, available[i].Name)
	}
	ar.Status.AvailableSummary = strings.Join(names, ", ")
	if err := r.Status().Update(ctx, ar); err != nil {
		return fmt.Errorf("update AvailableResource status %s/%s: %w", ns, name, err)
	}
	return nil
}

// reconcileQuota recomputes the pool GrantQuota status (project totals) and renders read-only
// per-namespace GrantQuota copies for the workload namespaces.
func (r *ProjectReconciler) reconcileQuota(ctx context.Context, project string, log logr.Logger) error {
	pool := &v1alpha1.GrantQuota{}
	if err := r.Get(ctx, types.NamespacedName{Namespace: project, Name: naming.GrantQuotaName}, pool); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil // no object quota for this project
		}
		return err
	}

	projectNS, err := resolve.ProjectNamespaces(ctx, r.Client, project)
	if err != nil {
		return err
	}
	regList := &v1alpha1.ClusterGrantableResourceList{}
	if err := r.List(ctx, regList); err != nil {
		return err
	}

	var poolStatus []v1alpha1.GrantQuotaMeasureStatus
	perNS := map[string][]v1alpha1.GrantQuotaMeasureStatus{}

	for i := range regList.Items {
		reg := &regList.Items[i]
		if len(engine.Measures(reg)) == 0 {
			continue
		}
		specForReg := pool.Spec.Objects[reg.Name]

		nsUsage := map[string]quota.Usage{}
		projUsage := quota.Usage{}
		for _, n := range projectNS {
			u, err := r.computeUsage(ctx, reg, n)
			if err != nil {
				return err
			}
			nsUsage[n] = u
			projUsage.Add(u)
		}

		keys := measureKeys(specForReg, projUsage)
		for _, k := range keys {
			limit, hasLimit := engine.LimitFor(specForReg, k.name, k.measure)
			used := projUsage.Get(k.name, k.measure)
			ms := v1alpha1.GrantQuotaMeasureStatus{Resource: reg.Name, Name: k.name, Measure: k.measure, Used: used}
			if hasLimit {
				l := limit
				ms.Limit = &l
				if !engine.IsUnlimited(l) && used.Cmp(l) > 0 {
					log.Info("object quota exceeded", "project", project, "resource", reg.Name, "name", k.name, "measure", k.measure)
				}
			}
			poolStatus = append(poolStatus, ms)

			for _, n := range projectNS {
				if n == project {
					continue // the pool object itself carries the project totals
				}
				nu := nsUsage[n].Get(k.name, k.measure)
				pu := projUsage.Get(k.name, k.measure)
				entry := v1alpha1.GrantQuotaMeasureStatus{Resource: reg.Name, Name: k.name, Measure: k.measure, Used: nu, ProjectUsed: &pu}
				if hasLimit {
					l := limit
					entry.Limit = &l
					pl := limit
					entry.ProjectLimit = &pl
				}
				perNS[n] = append(perNS[n], entry)
			}
		}
	}

	pool.Status.Objects = poolStatus
	if err := r.Status().Update(ctx, pool); err != nil {
		return fmt.Errorf("update pool status: %w", err)
	}

	for _, n := range projectNS {
		if n == project {
			continue
		}
		if err := r.upsertRenderedQuota(ctx, n, project, perNS[n]); err != nil {
			return err
		}
	}
	return nil
}

func (r *ProjectReconciler) upsertRenderedQuota(ctx context.Context, ns, project string, status []v1alpha1.GrantQuotaMeasureStatus) error {
	gq := &v1alpha1.GrantQuota{ObjectMeta: metav1.ObjectMeta{Name: naming.GrantQuotaName, Namespace: ns}}
	_, err := ctrl.CreateOrUpdate(ctx, r.Client, gq, func() error {
		if gq.Labels == nil {
			gq.Labels = map[string]string{}
		}
		for k, v := range naming.ManagedLabels(project) {
			gq.Labels[k] = v
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("upsert rendered GrantQuota %s: %w", ns, err)
	}
	gq.Status.Objects = status
	if err := r.Status().Update(ctx, gq); err != nil {
		return fmt.Errorf("update rendered GrantQuota status %s: %w", ns, err)
	}
	return nil
}

type measureKey struct{ name, measure string }

func measureKeys(spec map[string]map[string]resource.Quantity, usage quota.Usage) []measureKey {
	seen := map[measureKey]bool{}
	var out []measureKey
	add := func(k measureKey) {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for name, ms := range spec {
		for measure := range ms {
			add(measureKey{name, measure})
		}
	}
	for name, ms := range usage {
		for measure := range ms {
			add(measureKey{name, measure})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].name != out[j].name {
			return out[i].name < out[j].name
		}
		return out[i].measure < out[j].measure
	})
	return out
}

// computeUsage recounts object-quota usage for one registration in one namespace, listing each
// distinct target resource via the REST mapper.
func (r *ProjectReconciler) computeUsage(ctx context.Context, reg *v1alpha1.ClusterGrantableResource, ns string) (quota.Usage, error) {
	total := quota.Usage{}
	for _, target := range r.usageTargets(reg) {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Group: target.gvk.Group, Version: target.gvk.Version, Kind: target.gvk.Kind + "List"})
		if err := r.List(ctx, list, client.InNamespace(ns)); err != nil {
			// A resource type that is not installed is not an error; just skip it.
			continue
		}
		for i := range list.Items {
			contribs, err := engine.Contributions(r.Factory, reg, list.Items[i].Object, target.gvk.Group, target.gvk.Version, target.plural)
			if err != nil {
				return nil, err
			}
			total.Add(quota.ContributionUsage(contribs))
		}
	}
	return total, nil
}

type usageTarget struct {
	gvk    schema.GroupVersionKind
	plural string
}

// usageTargets resolves the distinct (GVK, plural) listing targets for a registration's usage
// references, using the REST mapper to map plural resources to kinds. Wildcard groups are skipped.
func (r *ProjectReconciler) usageTargets(reg *v1alpha1.ClusterGrantableResource) []usageTarget {
	seen := map[schema.GroupVersionKind]bool{}
	var out []usageTarget
	for i := range reg.Spec.UsageReferences {
		rule := reg.Spec.UsageReferences[i].Rule
		for _, g := range rule.APIGroups {
			if g == "*" {
				continue
			}
			for _, res := range rule.Resources {
				versions := rule.APIVersions
				if len(versions) == 0 {
					versions = []string{"*"}
				}
				for _, v := range versions {
					ver := v
					if ver == "*" {
						ver = ""
					}
					gvk, err := r.Mapper.KindFor(schema.GroupVersionResource{Group: g, Version: ver, Resource: res})
					if err != nil {
						continue
					}
					if seen[gvk] {
						continue
					}
					seen[gvk] = true
					out = append(out, usageTarget{gvk: gvk, plural: res})
				}
			}
		}
	}
	return out
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
		Watches(&v1alpha1.ClusterObjectGrant{}, enqueueProjectNamespaces).
		Watches(&v1alpha1.ClusterGrantableResource{}, enqueueProjectNamespaces).
		Watches(&v1alpha1.GrantQuota{}, enqueueProjectNamespaces).
		Named("project-grants").
		Complete(r)
}
