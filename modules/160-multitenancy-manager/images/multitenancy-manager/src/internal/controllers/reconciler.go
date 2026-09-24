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
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/api/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/namespaces"
	"controller/internal/naming"
	"controller/internal/resolve"
)

// ResyncInterval is the period at which a project namespace is re-reconciled so the catalog
// (recomputed from live granted objects, not all watched) does not drift unbounded.
const ResyncInterval = 2 * time.Minute

// MapperReset drops the mappings of the grants REST mapper every Interval. A mapper never forgets a
// kind it has mapped, so without it a CRD re-created with another scope would keep its old scope
// until the pod restarts. It runs on every replica, since the webhooks serve on all of them.
type MapperReset struct {
	Mapper   meta.ResettableRESTMapper
	Interval time.Duration
}

// Start resets the mapper every Interval until ctx is done.
func (m *MapperReset) Start(ctx context.Context) error {
	ticker := time.NewTicker(m.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			m.Mapper.Reset()
		}
	}
}

// NeedLeaderElection is false: the webhooks of a replica that is not the leader use the mapper too.
func (m *MapperReset) NeedLeaderElection() bool { return false }

// ProjectReconciler materializes AvailableClusterResource catalogs for project namespaces and
// recounts the grant-violation metric of each namespace it reconciles.
type ProjectReconciler struct {
	client.Client
	Mapper meta.RESTMapper
	// Usage reads the objects a GrantableClusterResourceReference governs. It is the uncached API
	// reader in the controller; when nil (tests of the catalog alone) violations are not scanned.
	Usage client.Reader
	// Factory compiles the field paths of the references and the catalogFields paths of the
	// definitions; shared with the webhooks. When nil, the catalog carries no fields.
	Factory jsonpath.Factory
	// Recorder receives the CatalogFieldsSkipped warnings on the definition. When nil, the skips are
	// only logged.
	Recorder record.EventRecorder

	// loggedSkips holds, per namespace and definition, the fingerprint of the skip set last logged, so
	// the log is written when the set changes rather than on every pass of every project. Lazily
	// initialized.
	skipsMu     sync.Mutex
	loggedSkips map[string]map[string]uint64
}

// ReasonCatalogFieldsSkipped is the reason of the Warning event on a GrantableClusterResourceDefinition
// whose catalog fields were left out of a project's catalog.
const ReasonCatalogFieldsSkipped = "CatalogFieldsSkipped"

// maxSkippedSample bounds the "<object>/<field>" names an event and the Info log line list; the V(1)
// log line carries all of them.
const maxSkippedSample = 5

// Reconcile reconciles a single (project) namespace.
func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if namespaces.IsSystem(req.Name) {
		return ctrl.Result{}, nil
	}
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			clearViolations(req.Name)
			r.forgetSkips(req.Name, nil)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get namespace: %w", err)
	}
	// A namespace on its way out takes its catalog with it; writing into it only produces
	// "unable to create new content in namespace ... because it is being terminated" and a retry.
	if ns.DeletionTimestamp != nil {
		clearViolations(ns.Name)
		r.forgetSkips(ns.Name, nil)
		return ctrl.Result{}, nil
	}
	// Only project namespaces (carrying the project label) get a catalog. Any other namespace —
	// the default namespace, system namespaces, namespaces of "virtual" projects — must not, even
	// when a registration's defaultAvailability is All. Clean up any catalog that lingers there.
	if _, isProjectNS := ns.Labels[naming.ProjectLabel]; !isProjectNS {
		clearViolations(ns.Name)
		r.forgetSkips(ns.Name, nil)
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
// Unlike the orphan sweep of a project namespace, this one is not restricted to the module's own
// objects: no catalog of any origin belongs in such a namespace.
func (r *ProjectReconciler) cleanupCatalog(ctx context.Context, ns string) error {
	list := &v1alpha1.AvailableClusterResourceList{}
	if err := r.List(ctx, list, client.InNamespace(ns)); err != nil {
		return fmt.Errorf("list AvailableClusterResource in %s: %w", ns, err)
	}
	for i := range list.Items {
		if err := r.deleteCatalog(ctx, &list.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

// deleteOrphanCatalogs removes the catalogs of registrations that no longer exist. Without it a
// deleted GrantableClusterResourceDefinition leaves its AvailableClusterResource behind in every
// project namespace, showing the tenant a stale list and a stale default forever; the protective
// admission policy keeps the object read-only, so nobody can remove it by hand either.
//
// Only the module's own objects are listed: the module label stamped in upsertAvailable (see
// naming.ManagedLabels) is what separates a catalog this controller produced from anything else of
// the same name. The heritage label is deliberately not part of the selector -- its value changed
// between releases and an object of the older generation is relabelled only while its registration
// lives, which is precisely the object that has to be swept here.
func (r *ProjectReconciler) deleteOrphanCatalogs(ctx context.Context, ns string, registered map[string]struct{}) error {
	list := &v1alpha1.AvailableClusterResourceList{}
	if err := r.List(ctx, list, client.InNamespace(ns), client.MatchingLabels{naming.ModuleLabel: naming.ModuleValue}); err != nil {
		return fmt.Errorf("list AvailableClusterResource in %s: %w", ns, err)
	}
	for i := range list.Items {
		if _, ok := registered[list.Items[i].Name]; ok {
			continue
		}
		if err := r.deleteCatalog(ctx, &list.Items[i]); err != nil {
			return err
		}
	}
	return nil
}

// deleteCatalog deletes one catalog object; an object that is already gone (it can disappear between
// the List and the Delete) counts as deleted.
func (r *ProjectReconciler) deleteCatalog(ctx context.Context, ar *v1alpha1.AvailableClusterResource) error {
	if err := r.Delete(ctx, ar); err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("delete stale AvailableClusterResource %s/%s: %w", ar.Namespace, ar.Name, err)
	}
	return nil
}

// reconcileCatalog upserts an AvailableClusterResource per registration for the namespace and removes
// the ones whose registration is gone or being deleted.
//
// A failure of one registration (a failed list or write) does not stop the others: errors are
// collected and returned together at the end, after the orphan sweep has run. Otherwise a single
// broken registration would, in every project namespace and on every pass, skip the sweep and
// whatever registrations the cache happened to list after it.
//
// A registration that cannot be resolved for a configuration reason (resolve.IsConfigurationError: a
// namespaced grantedResource, or a kind the apiserver does not serve) is logged and not returned.
// Retrying cannot fix it, and a returned error would put every project namespace into the
// controller's back-off instead of the ResyncInterval requeue, slowing down the catalogs of all the
// other registrations there for as long as the broken one exists.
func (r *ProjectReconciler) reconcileCatalog(ctx context.Context, ns *corev1.Namespace, project string) error {
	grants, err := resolve.GrantsForNamespace(ctx, r.Client, ns)
	if err != nil {
		return err
	}
	regList := &v1alpha1.GrantableClusterResourceDefinitionList{}
	if err := r.List(ctx, regList); err != nil {
		return err
	}
	var errs []error
	if r.Usage != nil && r.Factory != nil {
		// The violation metric is secondary to the catalog: a scan failure (a failed resolve or list
		// of the governed objects) keeps the previous metric values but must not hold the catalog
		// back. A registration with a configuration error is skipped by the scan itself.
		violations, err := scanViolations(ctx, r.Client, r.Usage, r.Mapper, r.Factory, ns.Name, grants, regList.Items)
		if err != nil {
			errs = append(errs, fmt.Errorf("scan grant violations: %w", err))
		} else {
			publishViolations(ns.Name, violations)
		}
	}
	// The set of registrations whose catalog stays is fixed before the first upsert, from the list
	// alone: a registration whose resolve or upsert fails below still exists, and its catalog (last
	// good state) must not be swept as an orphan. The one exception is a namespaced grantedResource,
	// dropped from the set in the loop below. That is what makes the sweep safe to run
	// unconditionally. A registration with a deletionTimestamp (held by a finalizer) is left out of
	// both the set and the upserts: its catalog goes when the deletion is requested, not when the
	// object finally disappears.
	registered := make(map[string]struct{}, len(regList.Items))
	for i := range regList.Items {
		if regList.Items[i].DeletionTimestamp == nil {
			registered[regList.Items[i].Name] = struct{}{}
		}
	}
	for i := range regList.Items {
		reg := &regList.Items[i]
		if reg.DeletionTimestamp != nil {
			continue
		}
		err := r.reconcileRegistration(ctx, ns.Name, project, grants, reg)
		if err == nil {
			continue
		}
		// A namespaced grantedResource is refused for good, unlike a kind that is not served yet:
		// its catalog is swept, since keeping the last state would keep showing the names the
		// refusal exists to hide.
		if errors.Is(err, resolve.ErrNamespacedGrantedResource) {
			delete(registered, reg.Name)
		}
		if resolve.IsConfigurationError(err) {
			ctrl.LoggerFrom(ctx).V(1).Info("skipping registration: definition cannot be resolved",
				"namespace", ns.Name, "definition", reg.Name, "reason", err.Error())
			continue
		}
		errs = append(errs, fmt.Errorf("registration %s: %w", reg.Name, err))
	}
	r.forgetSkips(ns.Name, registered)
	if err := r.deleteOrphanCatalogs(ctx, ns.Name, registered); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// reconcileRegistration resolves one registration for the namespace and upserts its catalog.
func (r *ProjectReconciler) reconcileRegistration(ctx context.Context, ns, project string, grants []*v1alpha1.ClusterResourceGrantPolicy, reg *v1alpha1.GrantableClusterResourceDefinition) error {
	resolved, err := resolve.Resolve(ctx, r.Client, r.Mapper, reg, resolve.EntriesFor(grants, reg.Name))
	if err != nil {
		return err
	}
	available, skips := resolved.AvailableWithFields(r.Factory)
	r.reportSkips(ctx, ns, reg, skips)
	if available == nil {
		// An empty catalog is kept as an object with an empty list: a reader (the Console
		// among them) can then tell "nothing is available here" from "not reconciled yet",
		// which a missing object could not say.
		available = []v1alpha1.AvailableObject{}
	}
	kind := ""
	if reg.Spec.GrantedResource != nil {
		kind = reg.Spec.GrantedResource.Kind
	}
	return r.upsertAvailable(ctx, ns, project, reg.Name, kind, available, resolved.Default())
}

// reportSkips makes the fields left out of the namespace's catalog visible to the owner of the
// definition: a Warning event on the definition, next to its CatalogFieldsValid condition, and a log
// line.
//
// The event is emitted on every pass that skips, not only on a change: an event expires after an hour
// and must not vanish while the skip lasts. The recorder's spam filter bounds the events per
// definition (a burst, then one per five minutes), whatever the number of projects, so each message
// names its namespace and stands on its own.
//
// The log has no such filter, so it is written only when the skip set of the catalog changes (the
// oversized "<object>/<field>" names, and whether the catalog is over the budget; not its size, which
// moves with any value): an Info line with the event's text, and a V(1) line with the full list.
// Keyed per catalog rather than per definition, since the set differs between projects. A pass
// without skips forgets the catalog, and so does a pass after its definition is deleted, so a skip
// that comes back is logged again.
func (r *ProjectReconciler) reportSkips(ctx context.Context, ns string, reg *v1alpha1.GrantableClusterResourceDefinition, skips engine.CatalogSkips) {
	if skips.Empty() {
		r.skipsMu.Lock()
		delete(r.loggedSkips[ns], reg.Name)
		r.skipsMu.Unlock()
		return
	}
	var parts []string
	if n := len(skips.Oversized); n > 0 {
		parts = append(parts, fmt.Sprintf("values longer than %d bytes are left out, %d in total: %s",
			engine.MaxCatalogFieldValueBytes, n, sampleOf(skips.Oversized)))
	}
	if skips.OverBudgetBytes > 0 {
		parts = append(parts, fmt.Sprintf("all fields are left out: the catalog with them takes at least %d bytes, over the budget of %d bytes per catalog",
			skips.OverBudgetBytes, engine.MaxCatalogFieldsBytes))
	}
	msg := fmt.Sprintf("Catalog fields of AvailableClusterResource %s/%s: %s.", ns, reg.Name, strings.Join(parts, "; "))
	if r.Recorder != nil {
		r.Recorder.Event(reg, corev1.EventTypeWarning, ReasonCatalogFieldsSkipped, msg)
	}

	fingerprint := skipsFingerprint(string(reg.UID), skips)
	r.skipsMu.Lock()
	logged, ok := r.loggedSkips[ns][reg.Name]
	changed := !ok || logged != fingerprint
	if changed {
		if r.loggedSkips == nil {
			r.loggedSkips = map[string]map[string]uint64{}
		}
		if r.loggedSkips[ns] == nil {
			r.loggedSkips[ns] = map[string]uint64{}
		}
		r.loggedSkips[ns][reg.Name] = fingerprint
	}
	r.skipsMu.Unlock()
	if !changed {
		return
	}
	log := ctrllog.FromContext(ctx).WithValues("namespace", ns, "definition", reg.Name)
	log.Info("catalog fields skipped", "message", msg)
	log.V(1).Info("catalog fields skipped, full list", "oversized", skips.Oversized, "overBudgetBytes", skips.OverBudgetBytes)
}

// sampleOf lists the first maxSkippedSample names and says how many more there are.
func sampleOf(names []string) string {
	sample := names[:min(len(names), maxSkippedSample)]
	out := strings.Join(sample, ", ")
	if n := len(names) - len(sample); n > 0 {
		out += fmt.Sprintf(" and %d more", n)
	}
	return out
}

// skipsFingerprint hashes what the log tracks of a skip set: the oversized names, in catalog order,
// and whether the catalog is over the budget. The UID of the definition goes in too, so a definition
// deleted and re-created before a pass of the namespace noticed the gap is logged again all the same.
func skipsFingerprint(uid string, skips engine.CatalogSkips) uint64 {
	h := fnv.New64a()
	h.Write([]byte(uid))
	h.Write([]byte{0})
	for _, name := range skips.Oversized {
		h.Write([]byte(name))
		h.Write([]byte{0})
	}
	if skips.OverBudgetBytes > 0 {
		h.Write([]byte{1})
	}
	return h.Sum64()
}

// forgetSkips drops the logged skip sets of the namespace's catalogs whose definition is not in keep:
// all of them (keep nil) for a namespace that no longer gets a catalog, those of a deleted definition
// otherwise, so a definition re-created with the same skips is logged again.
func (r *ProjectReconciler) forgetSkips(ns string, keep map[string]struct{}) {
	r.skipsMu.Lock()
	defer r.skipsMu.Unlock()
	for name := range r.loggedSkips[ns] {
		if _, ok := keep[name]; !ok {
			delete(r.loggedSkips[ns], name)
		}
	}
	if len(r.loggedSkips[ns]) == 0 {
		delete(r.loggedSkips, ns)
	}
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
	// Reassigning the whole Available slice (not mutating it) means a value snapshot is enough to
	// detect a real change and skip the status write on the frequent no-op resync/grant-change passes.
	// The comparison is deep, so a changed catalog field value (AvailableObject.Fields) counts too.
	before := ar.Status
	ar.Status.GrantedResourceKind = kind
	ar.Status.Available = available
	ar.Status.Default = def
	ar.Status.AvailableCount = len(available)
	if reflect.DeepEqual(before, ar.Status) {
		return nil
	}
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
		// The policies are matched against the union of Project and namespace labels, so a label
		// change on the Project has to re-evaluate its namespaces; nothing else about a Project
		// matters here, hence the label predicate.
		Watches(&v1alpha3.Project{}, handler.EnqueueRequestsFromMapFunc(r.namespacesOfProject),
			builder.WithPredicates(predicate.LabelChangedPredicate{})).
		Named("project-grants").
		Complete(r)
}

// namespacesOfProject maps a Project to the reconcile requests of its namespaces: every namespace
// labelled with the project, plus the one named after it (the main namespace carries the label as
// well, but a project whose namespace is still being created does not have it yet).
func (r *ProjectReconciler) namespacesOfProject(ctx context.Context, obj client.Object) []reconcile.Request {
	nsList := &corev1.NamespaceList{}
	if err := r.List(ctx, nsList, client.MatchingLabels{naming.ProjectLabel: obj.GetName()}); err != nil {
		return nil
	}
	reqs := make([]reconcile.Request, 0, len(nsList.Items)+1)
	seen := map[string]struct{}{}
	for i := range nsList.Items {
		name := nsList.Items[i].Name
		if namespaces.IsSystem(name) {
			continue
		}
		seen[name] = struct{}{}
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	}
	if _, ok := seen[obj.GetName()]; !ok && !namespaces.IsSystem(obj.GetName()) {
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.GetName()}})
	}
	return reqs
}
