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

// Package managebindings projects the ClusterRoleBindings of the system and subsystem roles of the
// granular role model (d8:system:*, d8:subsystem:<subsystem>:*) into namespaced RoleBindings.
//
// A system or subsystem role carries the label rbac.deckhouse.io/use-role naming the namespace role
// its subjects get in the namespaces of the modules it manages. Those namespaces are found through
// the role's aggregation rule: every ClusterRole of Deckhouse matched by the selectors contributes the
// namespace in its rbac.deckhouse.io/namespace label (the system capabilities of the modules carry it;
// a ClusterRole a user creates adds no namespace, see isDeckhouseRole), and
// roles that aggregate further are followed to any depth (subsystem roles aggregating capabilities,
// system roles aggregating subsystems, superadmin roles aggregating managers). For each such
// ClusterRoleBinding and each namespace a RoleBinding d8:namespace:<use-role>:binding:<binding> is
// kept; automated RoleBindings that are no longer expected are removed. The whole set is recomputed
// on every change (single request key); the ClusterRoleBindings are read through a cache index by
// roleRef, one lookup per role, so that a reconcile does not copy every ClusterRoleBinding of the
// cluster. A manage role is any ClusterRole labelled rbac.deckhouse.io/scope=system or =subsystem,
// whatever its name and kind.
//
// The manage roles of the previous release keep the projection that release gave them, for the one
// release their names live on as compatibility aliases: a ClusterRole of Deckhouse named
// d8:manage:* (heritage: deckhouse) labelled rbac.deckhouse.io/kind=manage with a use role, which is
// how that release labelled d8:manage:<subsystem|all>:<level>, gets RoleBindings
// d8:use:<use-role>:binding:<binding> to d8:use:role:<use-role>, exactly the objects the previous
// release made, so they are kept as they are, and a ClusterRoleBinding keeps those it still has while
// it binds the role, wherever they are, unless they are older than it. The namespaces of a
// compatibility alias (rbac.deckhouse.io/deprecated=true) come from the selectors in its
// use-binding-selectors annotation, which name the modules of the old subsystem and the capabilities
// of the previous scheme, rather than from its aggregation rule, because the alias of a merged or
// renamed subsystem aggregates modules the old role did not cover. Any other role of the previous
// scheme, such as the role of a module that still ships that scheme, follows its aggregation rule as
// before. These roles and the capabilities of the previous scheme are held by a cache of their own
// (LegacyRoleSelector), and the roles of the current scheme are resolved without them, as before.
// The projection of a role of the previous scheme follows the ClusterRoles of Deckhouse only, so a
// ClusterRole a user labels the old way adds no namespace to it.
package managebindings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"user-authz-controller/internal/metrics"
)

const (
	// RequestName is the constant key every event is mapped to.
	RequestName = "manage-bindings"

	// RoleRefIndexField indexes the ClusterRoleBindings by the ClusterRole they reference.
	RoleRefIndexField = "user-authz.deckhouse.io/role-ref"
	// AutomatedIndexField indexes the use RoleBindings this reconciler owns: a plain label selector on
	// the cache would scan every RoleBinding of the cluster on every reconcile.
	AutomatedIndexField = "user-authz.deckhouse.io/automated-use-binding"
	automatedIndexValue = "true"

	LabelScope      = "rbac.deckhouse.io/scope"
	LabelUseRole    = "rbac.deckhouse.io/use-role"
	LabelNamespace  = "rbac.deckhouse.io/namespace"
	LabelKind       = "rbac.deckhouse.io/kind"
	labelHeritage   = "heritage"
	labelAutomated  = "rbac.deckhouse.io/automated"
	labelDict       = "rbac.deckhouse.io/dict"
	labelDeprecated = "rbac.deckhouse.io/deprecated"

	ScopeSystem    = "system"
	ScopeSubsystem = "subsystem"

	// KindManage is the kind of the manage roles and capabilities of the previous release.
	KindManage = "manage"

	// AnnotationUseBindingSelectors holds, on a manage role of the previous release, the label
	// selectors (a JSON list of metav1.LabelSelector) whose ClusterRoles give the namespaces of its
	// RoleBindings.
	AnnotationUseBindingSelectors = "user-authz.deckhouse.io/use-binding-selectors"

	// viewerLevel is the namespace level a role not built into the platform projects at most: the
	// namespaces of the modules hold the ServiceAccounts of the platform, and the use-role label of a
	// custom role is set by its author.
	viewerLevel = "viewer"
	// ReasonProjectionCapped is the reason of the event on a role whose projection was limited.
	ReasonProjectionCapped = "ProjectionCapped"

	useRolePrefix         = "d8:namespace:"
	legacyBindingPrefix   = "d8:use:"
	legacyUseRolePrefix   = "d8:use:role:"
	relatedWithAnnotation = "rbac.deckhouse.io/related-with"
	// legacyManagePrefix is the name prefix of the manage roles of the previous release and of their
	// compatibility aliases. The rbacv2-cluster-roles webhook lets nobody but Deckhouse write a
	// ClusterRole under it, on every build.
	legacyManagePrefix = "d8:manage:"
	// deckhousePrefix is the name prefix of the ClusterRoles of Deckhouse.
	deckhousePrefix = "d8:"
)

// ManageRoleSelector selects the manage ClusterRoles: the roles and capabilities of the system and
// subsystem scopes. The capabilities carry no use-role and project nothing themselves, but they are
// the leaves of the aggregation graph that carry the namespaces, so they have to be in the cache.
var ManageRoleSelector = labels.NewSelector().Add(mustRequirement(LabelScope, selection.In, ScopeSystem, ScopeSubsystem))

// LegacyRoleSelector selects the ClusterRoles of the previous scheme: the compatibility aliases of its
// manage roles and the capabilities of the modules that still ship it, which carry no scope.
var LegacyRoleSelector = labels.NewSelector().Add(mustRequirement(LabelKind, selection.Equals, KindManage))

func mustRequirement(key string, op selection.Operator, values ...string) labels.Requirement {
	requirement, err := labels.NewRequirement(key, op, values)
	if err != nil {
		panic(err)
	}
	return *requirement
}

// isManageRole reports whether a ClusterRole with the given labels belongs to the system or
// subsystem scope.
func isManageRole(roleLabels map[string]string) bool {
	scope := roleLabels[LabelScope]
	return scope == ScopeSystem || scope == ScopeSubsystem
}

// isLegacyManageRole reports whether role is a manage role of the previous scheme that Deckhouse
// renders, which keeps the projection of that scheme: named d8:manage:*, labelled heritage:
// deckhouse and the manage kind, with a use role. Both the name and heritage: deckhouse are out of
// reach of users (the rbacv2-cluster-roles and system-roles webhooks), so a role a user labels the
// old way is not projected, unlike in the hook of the previous release. A role with the scope of the
// current scheme is projected the current way.
func isLegacyManageRole(role *rbacv1.ClusterRole) bool {
	_, hasUseRole := role.Labels[LabelUseRole]
	isOldManageKind := role.Labels[LabelKind] == KindManage && !isManageRole(role.Labels)
	isOfDeckhouse := strings.HasPrefix(role.Name, legacyManagePrefix) && role.Labels[labelHeritage] == "deckhouse"
	return isOldManageKind && isOfDeckhouse && hasUseRole
}

// isLegacyAlias reports whether role is the compatibility alias of a manage role of the previous
// release, whose namespaces come from its use-binding-selectors annotation.
func isLegacyAlias(role *rbacv1.ClusterRole) bool {
	return isLegacyManageRole(role) && role.Labels[labelDeprecated] == "true"
}

// AutomatedLabels mark the use RoleBindings this reconciler owns.
var AutomatedLabels = map[string]string{labelHeritage: "deckhouse", labelAutomated: "true"}

// Reconciler projects manage ClusterRoleBindings into use RoleBindings.
type Reconciler struct {
	client client.Client
	// legacy reads the ClusterRoles of the previous scheme (LegacyRoleSelector), which the cache
	// behind client does not hold.
	legacy   client.Reader
	recorder events.EventRecorder
	metrics  *metrics.Collector
	log      logr.Logger
}

// New constructs a Reconciler. legacy reads the ClusterRoles of the previous scheme; recorder gets
// the events on the roles whose projection is limited.
func New(
	c client.Client,
	legacy client.Reader,
	recorder events.EventRecorder,
	m *metrics.Collector,
	log logr.Logger,
) *Reconciler {
	return &Reconciler{client: c, legacy: legacy, recorder: recorder, metrics: m, log: log}
}

// RoleRefIndexValue is the indexer of RoleRefIndexField: the name of the referenced ClusterRole.
func RoleRefIndexValue(obj client.Object) []string {
	crb, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok || crb.RoleRef.Kind != "ClusterRole" {
		return nil
	}
	return []string{crb.RoleRef.Name}
}

// IsManageBinding reports whether obj is a ClusterRoleBinding to a system or subsystem role, looked
// up through reader (the manager's cache, which holds the ClusterRoles of those scopes only, so a
// miss means "not a manage role").
func IsManageBinding(reader client.Reader, obj client.Object) bool {
	return bindsRoleWith(reader, obj, func(role *rbacv1.ClusterRole) bool { return isManageRole(role.Labels) })
}

// IsLegacyManageBinding reports whether obj is a ClusterRoleBinding to a manage role of the previous
// scheme that Deckhouse renders, looked up through reader (the cache of the ClusterRoles of that
// scheme).
func IsLegacyManageBinding(reader client.Reader, obj client.Object) bool {
	return bindsRoleWith(reader, obj, isLegacyManageRole)
}

// bindsRoleWith reports whether obj is a ClusterRoleBinding to a ClusterRole that reader holds and
// that passes match.
func bindsRoleWith(reader client.Reader, obj client.Object, match func(*rbacv1.ClusterRole) bool) bool {
	crb, ok := obj.(*rbacv1.ClusterRoleBinding)
	if !ok || crb.RoleRef.Kind != "ClusterRole" {
		return false
	}
	role := &rbacv1.ClusterRole{}
	if err := reader.Get(context.Background(), client.ObjectKey{Name: crb.RoleRef.Name}, role); err != nil {
		return false
	}
	return match(role)
}

// AutomatedIndexValue is the indexer of AutomatedIndexField.
func AutomatedIndexValue(obj client.Object) []string {
	if _, ok := obj.(*rbacv1.RoleBinding); !ok || !isAutomatedUseBinding(obj.GetLabels()) {
		return nil
	}
	return []string{automatedIndexValue}
}

// Register indexes the ClusterRoleBindings by roleRef and the automated use RoleBindings, and wires
// the reconciler onto the manager.
func Register(ctx context.Context, mgr manager.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &rbacv1.ClusterRoleBinding{}, RoleRefIndexField, RoleRefIndexValue); err != nil {
		return fmt.Errorf("index clusterrolebindings by roleRef: %w", err)
	}
	if err := mgr.GetFieldIndexer().IndexField(ctx, &rbacv1.RoleBinding{}, AutomatedIndexField, AutomatedIndexValue); err != nil {
		return fmt.Errorf("index automated rolebindings: %w", err)
	}
	legacy, err := newLegacyRoleCache(mgr)
	if err != nil {
		return fmt.Errorf("create the cache of the clusterroles of the previous scheme: %w", err)
	}
	if err := mgr.Add(legacy); err != nil {
		return fmt.Errorf("add the cache of the clusterroles of the previous scheme: %w", err)
	}
	recorder := mgr.GetEventRecorder("user-authz-controller")
	r := New(mgr.GetClient(), legacy, recorder, metrics.Default, mgr.GetLogger().WithName("manage-bindings"))
	if err := r.SetupWithManager(mgr, legacy); err != nil {
		return fmt.Errorf("setup manage-bindings controller: %w", err)
	}
	return nil
}

// legacyRoleCache holds the ClusterRoles of the previous scheme. The cache of the manager holds the
// ClusterRoles of the system and subsystem scopes only, and one label selector cannot ask for either
// of two labels, so these get a cache of their own. GetCache makes the manager start and sync it with
// its own caches, before the controllers.
type legacyRoleCache struct {
	cache.Cache
}

// GetCache returns the cache, which marks legacyRoleCache as a cache for the manager.
func (c legacyRoleCache) GetCache() cache.Cache {
	return c.Cache
}

func newLegacyRoleCache(mgr manager.Manager) (legacyRoleCache, error) {
	c, err := cache.New(mgr.GetConfig(), cache.Options{
		HTTPClient:           mgr.GetHTTPClient(),
		Scheme:               mgr.GetScheme(),
		Mapper:               mgr.GetRESTMapper(),
		DefaultLabelSelector: LegacyRoleSelector,
		DefaultTransform:     cache.TransformStripManagedFields(),
	})
	if err != nil {
		return legacyRoleCache{}, err
	}
	return legacyRoleCache{Cache: c}, nil
}

// SetupWithManager watches manage ClusterRoleBindings, manage ClusterRoles, the ClusterRoles of the
// previous scheme in legacy and the automated use RoleBindings; every event maps to the single
// request.
func (r *Reconciler) SetupWithManager(mgr manager.Manager, legacy cache.Cache) error {
	single := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: RequestName}}}
	})

	manageBinding := eitherSide(func(obj client.Object) bool {
		return IsManageBinding(r.client, obj) || IsLegacyManageBinding(r.legacy, obj)
	})

	manageRole := eitherSide(func(obj client.Object) bool {
		return isManageRole(obj.GetLabels())
	})

	automatedUseBinding := eitherSide(func(obj client.Object) bool {
		return isAutomatedUseBinding(obj.GetLabels())
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("manage-bindings").
		Watches(&rbacv1.ClusterRoleBinding{}, single, builder.WithPredicates(manageBinding)).
		Watches(&rbacv1.ClusterRole{}, single, builder.WithPredicates(manageRole)).
		WatchesRawSource(source.Kind[client.Object](legacy, &rbacv1.ClusterRole{}, single)).
		Watches(&rbacv1.RoleBinding{}, single, builder.WithPredicates(automatedUseBinding)).
		Complete(r)
}

// eitherSide builds a predicate from match that, on updates, passes when either object matches.
func eitherSide(match func(client.Object) bool) predicate.Funcs {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return match(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return match(e.Object) },
		GenericFunc: func(e event.GenericEvent) bool { return match(e.Object) },
		UpdateFunc:  func(e event.UpdateEvent) bool { return match(e.ObjectOld) || match(e.ObjectNew) },
	}
}

// Reconcile recomputes the expected use RoleBindings from the manage ClusterRoleBindings and makes
// the cluster match: missing ones are created, differing ones updated, unexpected automated ones
// deleted. Creation and update come before deletion, and nothing is deleted while an expected
// binding could not be written.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	roles := &rbacv1.ClusterRoleList{}
	if err := r.client.List(ctx, roles, client.MatchingLabelsSelector{Selector: ManageRoleSelector}); err != nil {
		return reconcile.Result{}, fmt.Errorf("list manage clusterroles: %w", err)
	}
	legacyRoles := &rbacv1.ClusterRoleList{}
	if err := r.legacy.List(ctx, legacyRoles, client.MatchingLabelsSelector{Selector: LegacyRoleSelector}); err != nil {
		return reconcile.Result{}, fmt.Errorf("list clusterroles of the previous scheme: %w", err)
	}
	automated := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, automated, client.MatchingFields{AutomatedIndexField: automatedIndexValue}); err != nil {
		return reconcile.Result{}, fmt.Errorf("list automated rolebindings: %w", err)
	}
	// The roles of the current scheme are resolved as before, without the ClusterRoles of the
	// previous scheme; the roles of the previous scheme reach the capabilities of both.
	current := projection{resolver: NewResolver(roles.Items), build: UseBinding}
	legacy := projection{
		resolver: NewResolver(ofDeckhouse(roles.Items, legacyRoles.Items)),
		build:    LegacyUseBinding,
		kept:     legacyProjections(automated.Items),
	}

	expected := make(map[client.ObjectKey]*rbacv1.RoleBinding)
	capped := []string{}
	for i := range roles.Items {
		if err := r.project(ctx, &roles.Items[i], current, expected, &capped); err != nil {
			return reconcile.Result{}, err
		}
	}
	for i := range legacyRoles.Items {
		if !isLegacyManageRole(&legacyRoles.Items[i]) {
			continue
		}
		if err := r.project(ctx, &legacyRoles.Items[i], legacy, expected, &capped); err != nil {
			return reconcile.Result{}, err
		}
	}
	r.metrics.SetProjections(projectionsByLevel(expected), capped)

	var (
		errs  []error
		drift metrics.Drift
	)
	for _, key := range slices.SortedFunc(maps.Keys(expected), compareKeys) {
		if err := r.ensure(ctx, key, expected[key], &drift); err != nil {
			errs = append(errs, err)
		}
	}
	observe := func() {
		// drift is what is still wrong after the writes: projections that could not be written and
		// stale bindings that could not be removed.
		r.metrics.Observe(metrics.KindManage, RequestName, metrics.Observation{Desired: len(expected), Actual: len(expected) - drift.Missing - drift.Changed, Drift: drift})
	}
	if len(errs) != 0 {
		observe()
		return reconcile.Result{}, errors.Join(errs...)
	}

	for i := range automated.Items {
		rb := &automated.Items[i]
		if !isAutomatedUseBinding(rb.Labels) {
			continue
		}
		if _, ok := expected[client.ObjectKeyFromObject(rb)]; ok {
			continue
		}
		err := r.client.Delete(ctx, rb)
		if apierrors.IsNotFound(err) {
			err = nil
		}
		r.metrics.RecordApply(metrics.KindManage, metrics.OpDelete, err)
		if err != nil {
			drift.Extra++
			errs = append(errs, fmt.Errorf("delete use binding %s/%s: %w", rb.Namespace, rb.Name, err))
			continue
		}
		r.log.Info("use binding removed", "namespace", rb.Namespace, "name", rb.Name)
	}

	observe()
	return reconcile.Result{}, errors.Join(errs...)
}

// ofDeckhouse returns the ClusterRoles of the lists that Deckhouse renders, named d8:* and labelled
// heritage: deckhouse, the graph the projection of a manage role of the previous scheme follows.
// That scheme guarded none of its labels, and a module that still ships it, with a subsystem of its
// own such as virtualization, aggregates a lineage the rbacv2-cluster-roles webhook does not guard
// either. A ClusterRole a user creates on that lineage, or a custom role the previous release
// documented, must not add the namespace in its label to the RoleBindings of every binding to an old
// manage role. Nobody but the deckhouse ServiceAccount can give a ClusterRole heritage: deckhouse
// (the system-roles webhook), but an UPDATE of a ClusterRole that already has it passes that
// webhook, and some of them, the user-authz:* roles, have no d8: name. The rbacv2-cluster-roles
// webhook lets nobody but Deckhouse write a ClusterRole under d8: on any build, apart from the
// d8:custom: and d8:commander: ones, which their writers cannot label heritage: deckhouse. Every
// ClusterRole of the previous scheme that Deckhouse and its modules ship is named d8:*.
func ofDeckhouse(lists ...[]rbacv1.ClusterRole) []rbacv1.ClusterRole {
	var out []rbacv1.ClusterRole
	for _, list := range lists {
		for i := range list {
			if isDeckhouseRole(&list[i]) {
				out = append(out, list[i])
			}
		}
	}
	return out
}

// projection is how a scheme projects the ClusterRoleBindings to its manage roles.
type projection struct {
	// resolver finds the use role and the namespaces of a manage role.
	resolver *Resolver
	// build makes the RoleBinding of a ClusterRoleBinding in a namespace, given the use role.
	build func(*rbacv1.ClusterRoleBinding, string, string) *rbacv1.RoleBinding
	// kept holds the existing RoleBindings a ClusterRoleBinding keeps wherever they are, by the name
	// of the ClusterRoleBinding; nil for a scheme that keeps none.
	kept map[string][]*rbacv1.RoleBinding
}

// project adds to expected the RoleBindings that p makes for every ClusterRoleBinding to role: one
// per namespace the resolver finds for it, and one in the namespace of every RoleBinding p keeps for
// the binding that is not older than the binding and that build made under the use role of the role
// or under a higher one (keptUseRole).
func (r *Reconciler) project(
	ctx context.Context,
	role *rbacv1.ClusterRole,
	p projection,
	expected map[client.ObjectKey]*rbacv1.RoleBinding,
	capped *[]string,
) error {
	useRole, namespaces := p.resolver.UseRoleAndNamespaces(role.Name)
	if useRole == "" || (len(namespaces) == 0 && len(p.kept) == 0) {
		return nil
	}
	// The label of a custom role is set by its author, who must not be able to hand the
	// ServiceAccounts of the modules to the subjects of the role through it. The rbacv2-cluster-roles
	// webhook refuses such a label; a role that carries one from before is projected as viewer and
	// says so on the role, once it is bound.
	labelled := useRole
	isCapped := !isDeckhouseRole(role) && useRole != viewerLevel
	if isCapped {
		useRole = viewerLevel
	}

	bindings := &rbacv1.ClusterRoleBindingList{}
	if err := r.client.List(ctx, bindings, client.MatchingFields{RoleRefIndexField: role.Name}); err != nil {
		return fmt.Errorf("list clusterrolebindings of %s: %w", role.Name, err)
	}
	if isCapped && len(bindings.Items) != 0 {
		r.recorder.Eventf(
			role,
			nil,
			corev1.EventTypeWarning,
			ReasonProjectionCapped,
			"Reconcile",
			"the role is not built into the platform, so its subjects get %q in the namespaces of its modules; the label %s: %s is ignored",
			viewerLevel,
			LabelUseRole,
			labelled,
		)
		*capped = append(*capped, role.Name)
	}
	for i := range bindings.Items {
		binding := &bindings.Items[i]
		for _, namespace := range namespaces {
			rb := p.build(binding, useRole, namespace)
			expected[client.ObjectKeyFromObject(rb)] = rb
		}
		for _, existing := range p.kept[binding.Name] {
			// The reconciler and the hook of the previous release create a RoleBinding only once its
			// ClusterRoleBinding exists, so an older one was made for a predecessor under the same name.
			if existing.CreationTimestamp.Before(&binding.CreationTimestamp) {
				continue
			}
			if keptUseRole(p, binding, existing, useRole) {
				rb := p.build(binding, useRole, existing.Namespace)
				expected[client.ObjectKeyFromObject(rb)] = rb
			}
		}
	}
	return nil
}

// legacyUseRoleRank orders the use roles of the previous scheme, d8:use:role:<use-role>.
var legacyUseRoleRank = map[string]int{"viewer": 0, "user": 1, "manager": 2, "admin": 3}

// keptUseRole reports whether a RoleBinding p keeps for binding goes on under useRole in its
// namespace. It does when it is the RoleBinding build gives the binding there under useRole, or under
// a higher use role of the previous scheme: an alias may lower the projection of the previous release,
// as a manager alias projects viewer where that release projected user, and the RoleBinding is then
// replaced by the lower one rather than dropped, so the subjects keep read access there. A projection
// is never raised, so a kept namespace never gets more than the previous release gave it.
func keptUseRole(p projection, binding *rbacv1.ClusterRoleBinding, existing *rbacv1.RoleBinding, useRole string) bool {
	madeUnder, ok := strings.CutPrefix(existing.RoleRef.Name, legacyUseRolePrefix)
	if !ok {
		return false
	}
	made := p.build(binding, madeUnder, existing.Namespace)
	if existing.Name != made.Name || !reflect.DeepEqual(existing.RoleRef, made.RoleRef) {
		return false
	}
	if madeUnder == useRole {
		return true
	}
	from, isKnownFrom := legacyUseRoleRank[madeUnder]
	to, isKnownTo := legacyUseRoleRank[useRole]
	return isKnownFrom && isKnownTo && to < from
}

// isDeckhouseRole reports whether role is a ClusterRole Deckhouse renders: named d8:* and labelled
// heritage: deckhouse. The heritage label alone is not enough, see ofDeckhouse: an UPDATE of a
// ClusterRole that already has it passes the system-roles webhook, and the user-authz:* roles have it
// without a d8: name, while the rbacv2-cluster-roles webhook lets nobody but Deckhouse write a
// ClusterRole under d8: on any build, apart from d8:custom: and d8:commander:, which their writers
// cannot label heritage: deckhouse.
func isDeckhouseRole(role *rbacv1.ClusterRole) bool {
	return strings.HasPrefix(role.Name, deckhousePrefix) && role.Labels[labelHeritage] == "deckhouse"
}

// projectionsByLevel counts the expected RoleBindings by the namespace level of their role.
func projectionsByLevel(expected map[client.ObjectKey]*rbacv1.RoleBinding) map[string]int {
	byLevel := make(map[string]int)
	for _, rb := range expected {
		level := strings.TrimPrefix(strings.TrimPrefix(rb.RoleRef.Name, useRolePrefix), legacyUseRolePrefix)
		byLevel[level]++
	}
	return byLevel
}

// legacyProjections indexes the automated RoleBindings named the way the previous release named
// its projections, d8:use:<use-role>:binding:<binding>, by the ClusterRoleBinding their related-with
// annotation names. A ClusterRoleBinding that still binds a manage role of the previous scheme keeps
// these RoleBindings even where the selectors of the role no longer reach, as when a module released
// on its own moves to the labels of the current scheme at the upgrade, so its subjects lose no rights
// the previous release gave them. They go with the ClusterRoleBinding, or once it binds no such role.
// project keeps only a RoleBinding with the roleRef it would give it or with a higher one, which it
// replaces by the lower one, so the reconciler never raises one, and only one that is not older than
// the ClusterRoleBinding, so
// a ClusterRoleBinding created again under the same name does not take over the RoleBindings of its
// predecessor.
func legacyProjections(items []rbacv1.RoleBinding) map[string][]*rbacv1.RoleBinding {
	out := make(map[string][]*rbacv1.RoleBinding)
	for i := range items {
		rb := &items[i]
		binding, ok := rb.Annotations[relatedWithAnnotation]
		if !ok || !isAutomatedUseBinding(rb.Labels) || !strings.HasPrefix(rb.Name, legacyBindingPrefix) {
			continue
		}
		out[binding] = append(out[binding], rb)
	}
	return out
}

func compareKeys(a, b client.ObjectKey) int {
	if c := strings.Compare(a.Namespace, b.Namespace); c != 0 {
		return c
	}
	return strings.Compare(a.Name, b.Name)
}

// ensure makes the RoleBinding at key equal to want in everything the reconciler owns: its labels
// and annotation, roleRef and subjects. Foreign labels and annotations are preserved. roleRef is
// immutable, so a binding with a different one is recreated. drift counts what could not be fixed.
func (r *Reconciler) ensure(ctx context.Context, key client.ObjectKey, want *rbacv1.RoleBinding, drift *metrics.Drift) error {
	current := &rbacv1.RoleBinding{}
	err := r.client.Get(ctx, key, current)
	if apierrors.IsNotFound(err) {
		if err := r.create(ctx, key, want); err != nil {
			drift.Missing++
			return err
		}
		return nil
	}
	if err != nil {
		drift.Changed++
		return fmt.Errorf("get use binding %s: %w", key, err)
	}

	if !reflect.DeepEqual(current.RoleRef, want.RoleRef) {
		err := r.client.Delete(ctx, current)
		if apierrors.IsNotFound(err) {
			err = nil
		}
		r.metrics.RecordApply(metrics.KindManage, metrics.OpDelete, err)
		if err != nil {
			drift.Changed++
			return fmt.Errorf("delete use binding %s with a stale roleRef: %w", key, err)
		}
		r.log.Info("use binding recreated: roleRef changed", "namespace", key.Namespace, "name", key.Name, "from", current.RoleRef.Name, "to", want.RoleRef.Name)
		if err := r.create(ctx, key, want); err != nil {
			drift.Changed++
			return err
		}
		return nil
	}

	if hasEntries(current.Labels, want.Labels) && hasEntries(current.Annotations, want.Annotations) &&
		reflect.DeepEqual(current.Subjects, want.Subjects) {
		return nil
	}

	updated := current.DeepCopy()
	updated.Labels = merged(current.Labels, want.Labels)
	updated.Annotations = merged(current.Annotations, want.Annotations)
	updated.Subjects = want.Subjects
	err = r.client.Update(ctx, updated)
	r.metrics.RecordApply(metrics.KindManage, metrics.OpUpdate, err)
	if err != nil {
		drift.Changed++
		return fmt.Errorf("update use binding %s: %w", key, err)
	}
	r.log.Info("use binding updated", "namespace", key.Namespace, "name", key.Name)

	return nil
}

func (r *Reconciler) create(ctx context.Context, key client.ObjectKey, want *rbacv1.RoleBinding) error {
	err := r.client.Create(ctx, want)
	if apierrors.IsAlreadyExists(err) {
		err = nil
	}
	r.metrics.RecordApply(metrics.KindManage, metrics.OpCreate, err)
	if err != nil {
		return fmt.Errorf("create use binding %s: %w", key, err)
	}
	r.log.Info("use binding created", "namespace", key.Namespace, "name", key.Name)
	return nil
}

// hasEntries reports whether have contains every entry of want.
func hasEntries(have, want map[string]string) bool {
	for k, v := range want {
		if got, ok := have[k]; !ok || got != v {
			return false
		}
	}
	return true
}

// merged returns have with every entry of want set.
func merged(have, want map[string]string) map[string]string {
	out := maps.Clone(have)
	if out == nil {
		out = make(map[string]string, len(want))
	}
	maps.Copy(out, want)
	return out
}

// Resolver answers, for a system or subsystem role, which namespace role it grants and in which
// namespaces, following the aggregation rules to any depth. Results are memoised per role.
type Resolver struct {
	roles      map[string]*rbacv1.ClusterRole
	selectors  map[string][]labels.Selector
	namespaces map[string][]string
}

// NewResolver indexes the manage ClusterRoles.
func NewResolver(roles []rbacv1.ClusterRole) *Resolver {
	r := &Resolver{
		roles:      make(map[string]*rbacv1.ClusterRole, len(roles)),
		selectors:  make(map[string][]labels.Selector, len(roles)),
		namespaces: make(map[string][]string),
	}
	for i := range roles {
		role := &roles[i]
		r.roles[role.Name] = role
		if selectors := projectionSelectors(role); len(selectors) != 0 {
			r.selectors[role.Name] = selectors
		}
	}
	return r
}

// projectionSelectors returns the selectors whose ClusterRoles give the namespaces of role: those of
// its use-binding-selectors annotation for a compatibility alias of a manage role of the previous
// release that has one, the aggregation rule otherwise. Aggregation selectors that cannot be parsed
// are ignored, as the API server ignores them. An annotation that cannot be read gives no selectors,
// so a binding to the alias gets no new RoleBinding, rather than RoleBindings in the namespaces its
// aggregation rule reaches and the previous release did not, and keeps those it has.
func projectionSelectors(role *rbacv1.ClusterRole) []labels.Selector {
	if raw, ok := role.Annotations[AnnotationUseBindingSelectors]; ok && isLegacyAlias(role) {
		return annotationSelectors(raw)
	}
	if role.AggregationRule == nil {
		return nil
	}
	selectors := make([]labels.Selector, 0, len(role.AggregationRule.ClusterRoleSelectors))
	for i := range role.AggregationRule.ClusterRoleSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&role.AggregationRule.ClusterRoleSelectors[i])
		if err != nil {
			continue
		}
		selectors = append(selectors, selector)
	}
	return selectors
}

// annotationSelectors reads the use-binding-selectors annotation, a JSON list of label selectors. It
// reads the whole annotation or nothing: a key a label selector does not have, an entry that is null
// or selects by nothing, an entry that does not parse and data after the list each give no selectors
// at all. A lenient reading would turn such an entry into a selector of every ClusterRole, or drop it
// and keep the rest, and one defective entry must not widen or change the projection.
func annotationSelectors(raw string) []labels.Selector {
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var entries []*metav1.LabelSelector
	if err := decoder.Decode(&entries); err != nil {
		return nil
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil
	}
	selectors := make([]labels.Selector, 0, len(entries))
	for _, entry := range entries {
		if entry == nil || (len(entry.MatchLabels) == 0 && len(entry.MatchExpressions) == 0) {
			return nil
		}
		selector, err := metav1.LabelSelectorAsSelector(entry)
		if err != nil {
			return nil
		}
		selectors = append(selectors, selector)
	}
	return selectors
}

// UseRoleAndNamespaces resolves the namespace role granted by the role roleName and the sorted
// namespaces it applies to. The empty use role means "not a role we project" (capabilities carry no
// use-role).
func (r *Resolver) UseRoleAndNamespaces(roleName string) (string, []string) {
	role, ok := r.roles[roleName]
	if !ok {
		return "", nil
	}
	useRole, ok := role.Labels[LabelUseRole]
	if !ok {
		return "", nil
	}

	if namespaces, done := r.namespaces[roleName]; done {
		return useRole, namespaces
	}

	set := make(map[string]struct{})
	r.collect(roleName, map[string]struct{}{roleName: {}}, set)
	namespaces := slices.Sorted(maps.Keys(set))
	r.namespaces[roleName] = namespaces

	return useRole, namespaces
}

// collect adds to set the namespaces of every role aggregated by roleName, directly or through
// other aggregating roles. visited guards against aggregation cycles.
func (r *Resolver) collect(roleName string, visited, set map[string]struct{}) {
	selectors := r.selectors[roleName]
	if len(selectors) == 0 {
		return
	}
	for name, candidate := range r.roles {
		if _, seen := visited[name]; seen {
			continue
		}
		if !matchesAny(selectors, candidate.Labels) {
			continue
		}
		visited[name] = struct{}{}
		// Only a ClusterRole of Deckhouse names a namespace: a capability a user creates with the label
		// would otherwise add any namespace, kube-system or that of another project, to every role
		// that aggregates it, a built-in superadmin role among them.
		if namespace, ok := candidate.Labels[LabelNamespace]; ok && isDeckhouseRole(candidate) {
			set[namespace] = struct{}{}
		}
		r.collect(name, visited, set)
	}
}

func matchesAny(selectors []labels.Selector, roleLabels map[string]string) bool {
	set := labels.Set(roleLabels)
	for _, selector := range selectors {
		if selector.Matches(set) {
			return true
		}
	}
	return false
}

// UseBinding builds the namespaced projection of a manage ClusterRoleBinding.
func UseBinding(manage *rbacv1.ClusterRoleBinding, useRole, namespace string) *rbacv1.RoleBinding {
	return useBinding(manage, useRolePrefix, useRolePrefix+useRole, useRole, namespace)
}

// LegacyUseBinding builds the namespaced projection of a ClusterRoleBinding to a manage role of the
// previous scheme as the previous release built it: d8:use:<use-role>:binding:<binding> to
// d8:use:role:<use-role>.
func LegacyUseBinding(manage *rbacv1.ClusterRoleBinding, useRole, namespace string) *rbacv1.RoleBinding {
	return useBinding(manage, legacyBindingPrefix, legacyUseRolePrefix+useRole, useRole, namespace)
}

func useBinding(manage *rbacv1.ClusterRoleBinding, namePrefix, roleName, useRole, namespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{APIVersion: rbacv1.SchemeGroupVersion.String(), Kind: "RoleBinding"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s%s:binding:%s", namePrefix, useRole, manage.Name),
			Namespace:   namespace,
			Labels:      maps.Clone(AutomatedLabels),
			Annotations: map[string]string{relatedWithAnnotation: manage.Name},
		},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: roleName},
		Subjects: slices.Clone(manage.Subjects),
	}
}

// isAutomatedUseBinding matches the RoleBindings this reconciler owns: automated Deckhouse bindings
// that are not dict bindings (those are ClusterRoleBindings anyway, the check is defence in depth).
// The automated label is used by nothing else in Deckhouse, so every such RoleBinding is a
// projection of this reconciler or of the hook it replaced.
func isAutomatedUseBinding(objLabels map[string]string) bool {
	return objLabels[labelHeritage] == "deckhouse" && objLabels[labelAutomated] == "true" && objLabels[labelDict] != "true"
}
