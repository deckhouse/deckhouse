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

// Package resolve turns the cluster-scoped grant model (GrantableClusterResourceDefinition + ClusterResourceGrantPolicy)
// into per-project decisions: which grants apply to a namespace, and the resolved allow/deny/excluded/
// reserved name sets and default for a registration. Selector-based rules are expanded against the live granted
// objects so the webhook and reconciler share identical semantics.
package resolve

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/engine"
	"controller/internal/jsonpath"
	"controller/internal/naming"
)

// ProjectName returns the project a namespace belongs to: the projects.deckhouse.io/project label
// value, or "" for a namespace no project owns. The label is the only mark: the controller stamps it
// on every namespace it creates or adopts, and an empty value names no project, so a namespace is not
// counted towards a project merely for sharing its name.
func ProjectName(ns *corev1.Namespace) string {
	return ns.Labels[naming.ProjectLabel]
}

// ApplicableGrants returns every ClusterResourceGrantPolicy that applies to the namespace of the
// given name (see GrantsForNamespace). A namespace that does not exist has no grants.
func ApplicableGrants(ctx context.Context, cl client.Reader, namespace string) ([]*grantsv1alpha2.ClusterResourceGrantPolicy, error) {
	ns := &corev1.Namespace{}
	if err := cl.Get(ctx, client.ObjectKey{Name: namespace}, ns); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get namespace %s: %w", namespace, err)
	}
	return GrantsForNamespace(ctx, cl, ns)
}

// GrantsForNamespace returns every ClusterResourceGrantPolicy that applies to the namespace (see
// PolicyMatches): its projectSelector is matched against the labels of the Project that owns the
// namespace, its namespaceSelector against the labels of the namespace itself.
//
// This is the one place the rule lives: /is-granted, /defaults, the catalog reconciler and the
// violation recount all come through here, so they cannot disagree about which policies apply.
func GrantsForNamespace(ctx context.Context, cl client.Reader, ns *corev1.Namespace) ([]*grantsv1alpha2.ClusterResourceGrantPolicy, error) {
	var projectLabels map[string]string
	if name := ProjectName(ns); name != "" {
		project := &v1alpha3.Project{}
		switch err := cl.Get(ctx, client.ObjectKey{Name: name}, project); {
		case err == nil:
			projectLabels = project.Labels
		case !k8serrors.IsNotFound(err):
			return nil, fmt.Errorf("get project %s: %w", name, err)
		}
	}
	// A namespace no Project owns yet (adoption pending) has no project labels: only a policy whose
	// projectSelector accepts an empty label set, or that sets none, can apply to it.

	grantList := &grantsv1alpha2.ClusterResourceGrantPolicyList{}
	if err := cl.List(ctx, grantList); err != nil {
		return nil, fmt.Errorf("list ClusterResourceGrantPolicies: %w", err)
	}
	out := make([]*grantsv1alpha2.ClusterResourceGrantPolicy, 0, len(grantList.Items))
	for i := range grantList.Items {
		if PolicyMatches(&grantList.Items[i], projectLabels, ns.Labels) {
			out = append(out, &grantList.Items[i])
		}
	}
	return out, nil
}

// PolicyMatches reports whether the policy applies to a namespace with the given namespace labels,
// owned by a project with the given project labels. Every selector the policy sets must match: the
// projectSelector against the project labels, the namespaceSelector against the namespace labels. A
// policy that sets neither is a library policy and matches nothing on its own; an explicit empty
// selector matches everything. A selector that does not parse matches nothing (the policy reconciler
// reports it in the SelectorsValid condition).
func PolicyMatches(policy *grantsv1alpha2.ClusterResourceGrantPolicy, projectLabels, namespaceLabels map[string]string) bool {
	if policy.Spec.IsLibrary() {
		return false
	}
	return selectorMatches(policy.Spec.ProjectSelector, projectLabels) &&
		selectorMatches(policy.Spec.NamespaceSelector, namespaceLabels)
}

// selectorMatches reports whether an optional selector matches the labels; an unset selector does not
// restrict anything.
func selectorMatches(selector *metav1.LabelSelector, set map[string]string) bool {
	if selector == nil {
		return true
	}
	sel, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return false
	}
	return sel.Matches(labels.Set(set))
}

// EntriesFor collects the grant resource entries referencing the given registration name across all
// the supplied grants.
func EntriesFor(grants []*grantsv1alpha2.ClusterResourceGrantPolicy, resourceName string) []v1alpha1.GrantResource {
	out := make([]v1alpha1.GrantResource, 0, len(grants))
	for _, g := range grants {
		for i := range g.Spec.Resources {
			if g.Spec.Resources[i].ResourceName == resourceName {
				out = append(out, g.Spec.Resources[i])
			}
		}
	}
	return out
}

// MatchedReference pairs a GrantableClusterResourceReference whose rule matched the request with the
// GrantableClusterResourceDefinition it points at.
type MatchedReference struct {
	Reference  *v1alpha1.GrantableClusterResourceReference
	Definition *v1alpha1.GrantableClusterResourceDefinition
}

// ReferencesForRequest returns the validation paths governing a usage object of the given
// (group, version, resource): every GrantableClusterResourceReference whose rule matches and whose
// target definition exists and is Managed. Unbound references (dangling grantableClusterResourceName)
// and references to External definitions are skipped — they enforce nothing here.
func ReferencesForRequest(ctx context.Context, cl client.Reader, group, version, resource string) ([]MatchedReference, error) {
	refList := &v1alpha1.GrantableClusterResourceReferenceList{}
	if err := cl.List(ctx, refList); err != nil {
		return nil, fmt.Errorf("list GrantableClusterResourceReferences: %w", err)
	}
	defList := &v1alpha1.GrantableClusterResourceDefinitionList{}
	if err := cl.List(ctx, defList); err != nil {
		return nil, fmt.Errorf("list GrantableClusterResourceDefinitions: %w", err)
	}
	defByName := make(map[string]*v1alpha1.GrantableClusterResourceDefinition, len(defList.Items))
	for i := range defList.Items {
		defByName[defList.Items[i].Name] = &defList.Items[i]
	}
	out := make([]MatchedReference, 0, len(refList.Items))
	for i := range refList.Items {
		ref := &refList.Items[i]
		if !engine.RuleMatches(ref.Spec.Rule, group, version, resource) {
			continue
		}
		def := defByName[ref.Spec.GrantableClusterResourceName]
		if def == nil || def.Spec.Enforcement == v1alpha1.EnforcementExternal {
			continue
		}
		out = append(out, MatchedReference{Reference: ref, Definition: def})
	}
	return out, nil
}

// Resolved holds the resolved availability for one registration in one project.
type Resolved struct {
	Reg      *v1alpha1.GrantableClusterResourceDefinition
	allowed  map[string]struct{}
	denied   map[string]struct{}
	excluded map[string]struct{}
	// reserved are the names the registration keeps closed unless a grant allows them explicitly.
	reserved  map[string]struct{}
	liveNames []string
	// liveObjects are the listed granted objects by name, kept for the catalog field projection.
	liveObjects map[string]map[string]any
	// complete is set when liveObjects holds every granted object (Resolve), not a few read by name
	// (ResolveNames); only then may the catalog field memo forget the objects missing from it.
	complete bool
	// missing are the names ResolveNames found no object for.
	missing map[string]struct{}
	anyAll  bool
	anyNone bool
	def     string

	// availableSet/availableObjs memoize the available catalog, which is recomputed several times
	// per webhook/reconcile request otherwise. They are filled lazily and never change after Resolve
	// returns (the underlying allow/deny/exclude sets are immutable then).
	availableSet  map[string]struct{}
	availableObjs []v1alpha1.AvailableObject
}

// Decide reports whether the project may use the granted object of the given name, applying the
// precedence excluded → denied → allowed → reserved → grant availabilityDefault → registration
// defaultAvailability. A reserved name is opened only by an explicit allow (a literal allowed name or
// an allowedSelector match): neither availabilityDefault All of a grant nor the registration baseline
// reaches it.
func (r *Resolved) Decide(name string) bool {
	if _, ok := r.excluded[name]; ok {
		return false
	}
	if _, ok := r.denied[name]; ok {
		return false
	}
	if _, ok := r.allowed[name]; ok {
		return true
	}
	if _, ok := r.reserved[name]; ok {
		return false
	}
	if r.anyAll {
		return true
	}
	if r.anyNone {
		return false
	}
	return r.Reg.Spec.DefaultAvailability != v1alpha1.AvailabilityNone
}

// Default returns the effective per-project default name (may be empty).
func (r *Resolved) Default() string { return r.def }

// availableNames returns the set of names available to this project: the live objects that Decide()
// allows, plus any explicitly allowed name (e.g. value-backed values) that is neither excluded nor
// denied; an explicit allow opens a reserved name, so reserved is not checked here. It is memoized and independent
// of the chosen default, so it can be reused for membership tests during Resolve.
func (r *Resolved) availableNames() map[string]struct{} {
	if r.availableSet != nil {
		return r.availableSet
	}
	names := map[string]struct{}{}
	for _, n := range r.liveNames {
		if r.Decide(n) {
			names[n] = struct{}{}
		}
	}
	for n := range r.allowed {
		if _, excluded := r.excluded[n]; excluded {
			continue
		}
		if _, denied := r.denied[n]; denied {
			continue
		}
		names[n] = struct{}{}
	}
	r.availableSet = names
	return names
}

// Available returns the catalog of available names for this project, sorted, with the default flagged.
// For object-backed resources it is the live objects that Decide() allows, plus any explicitly allowed
// name; for value-backed resources it is the allowed names. The result is memoized.
func (r *Resolved) Available() []v1alpha1.AvailableObject {
	if r.availableObjs != nil {
		return r.availableObjs
	}
	names := r.availableNames()
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	slices.Sort(sorted)
	out := make([]v1alpha1.AvailableObject, 0, len(sorted))
	for _, n := range sorted {
		out = append(out, v1alpha1.AvailableObject{Name: n, Default: n == r.def})
	}
	r.availableObjs = out
	return out
}

// AvailableWithFields is Available with the definition's catalogFields projected from the live
// objects into each entry. It returns a fresh slice, so the memoized Available is never mutated. A
// name with no live object (a value-backed resource, an allowed name that does not exist) and a nil
// factory get no fields.
//
// The second result says what was left out. When the catalog with its fields exceeds
// engine.MaxCatalogFieldsBytes (see engine.CatalogSize), no entry carries any: all or nothing, so the
// result depends only on the catalog and not on the order the objects are visited in.
func (r *Resolved) AvailableWithFields(factory jsonpath.Factory) ([]v1alpha1.AvailableObject, engine.CatalogSkips) {
	return r.AvailableWithFieldsMemo(factory, nil)
}

// AvailableWithFieldsMemo is AvailableWithFields that takes the projection of an object from memo
// when the object has not changed since it was projected, so a pass over every project namespace
// projects each object once rather than once per namespace. A nil memo projects every time.
func (r *Resolved) AvailableWithFieldsMemo(factory jsonpath.Factory, memo *FieldsMemo) ([]v1alpha1.AvailableObject, engine.CatalogSkips) {
	var skips engine.CatalogSkips
	available := r.Available()
	if factory == nil || len(r.Reg.Spec.CatalogFields) == 0 || len(r.liveObjects) == 0 {
		return available, skips
	}
	project := func(name string, obj map[string]any) (map[string]apiextensionsv1.JSON, []string) {
		return engine.ProjectCatalogFields(factory, r.Reg.Spec.CatalogFields, obj)
	}
	if memo != nil {
		project = memo.begin(r, factory)
		defer memo.end()
	}
	out := slices.Clone(available)
	for i := range out {
		obj, ok := r.liveObjects[out[i].Name]
		if !ok {
			continue
		}
		fields, oversized := project(out[i].Name, obj)
		out[i].Fields = fields
		for _, name := range oversized {
			skips.Oversized = append(skips.Oversized, out[i].Name+"/"+name)
		}
	}
	if size, fits := engine.CatalogSize(out); !fits {
		skips.OverBudgetBytes = size
		for i := range out {
			out[i].Fields = nil
		}
	}
	return out, skips
}

// FieldsMemo keeps the catalog field projection of every granted object per definition, keyed by
// the resourceVersion of the object and the generation of the definition, so an object is projected
// again only when it or the catalogFields change. Every catalog gets its own copy of the field map.
// Safe for concurrent use.
type FieldsMemo struct {
	mu   sync.Mutex
	defs map[types.UID]*definitionFields
}

type definitionFields struct {
	generation int64
	objects    map[string]projectedFields
}

type projectedFields struct {
	resourceVersion string
	fields          map[string]apiextensionsv1.JSON
	oversized       []string
}

// begin locks the memo for one catalog and returns the projection of an object through it. On a
// Resolved that holds every granted object, the objects that are gone are forgotten first.
func (m *FieldsMemo) begin(r *Resolved, factory jsonpath.Factory) func(string, map[string]any) (map[string]apiextensionsv1.JSON, []string) {
	m.mu.Lock()
	if m.defs == nil {
		m.defs = map[types.UID]*definitionFields{}
	}
	d := m.defs[r.Reg.UID]
	if d == nil || d.generation != r.Reg.Generation {
		d = &definitionFields{generation: r.Reg.Generation, objects: map[string]projectedFields{}}
		m.defs[r.Reg.UID] = d
	}
	if r.complete {
		for name := range d.objects {
			if _, ok := r.liveObjects[name]; !ok {
				delete(d.objects, name)
			}
		}
	}
	return func(name string, obj map[string]any) (map[string]apiextensionsv1.JSON, []string) {
		rv, _, _ := unstructured.NestedString(obj, "metadata", "resourceVersion")
		p, ok := d.objects[name]
		if !ok || rv == "" || p.resourceVersion != rv {
			fields, oversized := engine.ProjectCatalogFields(factory, r.Reg.Spec.CatalogFields, obj)
			p = projectedFields{resourceVersion: rv, fields: fields, oversized: oversized}
			d.objects[name] = p
		}
		// The catalog gets a map of its own: a status update decodes the response of the API server
		// into the catalog, and with it into this map. The values are not decoded into in place.
		return maps.Clone(p.fields), p.oversized
	}
}

func (m *FieldsMemo) end() { m.mu.Unlock() }

// Forget drops the projections of the definitions that are not in keep (by UID).
func (m *FieldsMemo) Forget(keep map[types.UID]struct{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for uid := range m.defs {
		if _, ok := keep[uid]; !ok {
			delete(m.defs, uid)
		}
	}
}

// Resolve builds the resolved availability for a registration given the applicable grant entries. For
// object-backed resources it lists the live granted objects to expand allowed/denied/excluded/reserved
// selectors.
//
// cl serves the granted objects; the controller passes a cache-backed reader (see grantcache), so the
// list is read with UnsafeDisableDeepCopy: the objects are only read, never changed or kept beyond
// the Resolved.
func Resolve(
	ctx context.Context,
	cl client.Reader,
	mapper meta.RESTMapper,
	reg *v1alpha1.GrantableClusterResourceDefinition,
	entries []v1alpha1.GrantResource,
) (*Resolved, error) {
	r := newResolved(reg, entries)

	// Object-backed: list live objects and expand selectors.
	if !reg.IsValueBacked() {
		gvk, err := grantedGVK(mapper, reg)
		if err != nil {
			return nil, err
		}
		m, err := newLiveMatcher(reg, entries)
		if err != nil {
			return nil, err
		}
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
		if err := cl.List(ctx, list, client.UnsafeDisableDeepCopy); err != nil {
			if k8serrors.IsNotFound(err) {
				// A list has no name to be absent, so a 404 means the kind is no longer served (its
				// CRD was deleted) while the mapper still maps it. Drop the stale mapping, so the next
				// resolve sees the kind as unknown, or with its new scope if it was re-created.
				if rm, ok := mapper.(meta.ResettableRESTMapper); ok {
					rm.Reset()
				}
				return nil, fmt.Errorf("list granted resource %s: %w: %w", reg.Spec.GrantedResource.Kind, ErrGrantedResourceNotServed, err)
			}
			return nil, fmt.Errorf("list granted resource %s: %w", reg.Spec.GrantedResource.Kind, err)
		}
		r.liveNames = slices.Grow(r.liveNames, len(list.Items))
		r.liveObjects = make(map[string]map[string]any, len(list.Items))
		r.complete = true
		for i := range list.Items {
			r.addLive(m, &list.Items[i])
		}
		slices.Sort(r.liveNames)
	}

	if err := r.settleDefault(ctx, cl, mapper, nil); err != nil {
		return nil, err
	}
	return r, nil
}

// ResolveNames is Resolve narrowed to the given names: instead of listing every granted object it
// reads only the objects of these names, one Get each, so its cost does not grow with the number of
// granted objects. Decide answers for each of the names exactly as on the Resolve of the same
// registration and entries, and so does membership in Available; the effective Default is the same
// too, its object being read alongside. Available lists only these names (plus explicitly allowed
// ones), and a catalog field projection makes no sense on the result, so it is for the webhooks and
// checks that decide on a known set of names.
//
// A name without an object is decided by the literal lists (allowed, denied, excluded and reserved
// names) and the baselines alone, as in Resolve, where selectors only ever see listed objects: matching
// them against an empty label set would let a NotIn or DoesNotExist selector decide about an object
// that does not exist.
//
// A kind the API server does not serve any more (its CRD was deleted while the mapper still maps it)
// is reported with ErrGrantedResourceNotServed and the mapper is reset, as Resolve does on its list.
func ResolveNames(
	ctx context.Context,
	cl client.Reader,
	mapper meta.RESTMapper,
	reg *v1alpha1.GrantableClusterResourceDefinition,
	entries []v1alpha1.GrantResource,
	names []string,
) (*Resolved, error) {
	return resolveNames(ctx, cl, mapper, reg, entries, names, true)
}

// DecideNames is ResolveNames for a caller that only decides on the names: it leaves the default
// unresolved (Default returns ""), which spares the list of the whole kind that defaultFrom takes.
// Decide and IsAvailable answer for the names exactly as on ResolveNames.
func DecideNames(
	ctx context.Context,
	cl client.Reader,
	mapper meta.RESTMapper,
	reg *v1alpha1.GrantableClusterResourceDefinition,
	entries []v1alpha1.GrantResource,
	names []string,
) (*Resolved, error) {
	return resolveNames(ctx, cl, mapper, reg, entries, names, false)
}

func resolveNames(
	ctx context.Context,
	cl client.Reader,
	mapper meta.RESTMapper,
	reg *v1alpha1.GrantableClusterResourceDefinition,
	entries []v1alpha1.GrantResource,
	names []string,
	withDefault bool,
) (*Resolved, error) {
	r := newResolved(reg, entries)
	if !withDefault {
		r.def = ""
	}
	if reg.IsValueBacked() {
		if withDefault {
			if err := r.settleDefault(ctx, cl, mapper, nil); err != nil {
				return nil, err
			}
		}
		return r, nil
	}
	gvk, err := grantedGVK(mapper, reg)
	if err != nil {
		return nil, err
	}
	m, err := newLiveMatcher(reg, entries)
	if err != nil {
		return nil, err
	}
	r.liveObjects = map[string]map[string]any{}
	fetch := func(name string) error {
		if name == "" {
			return nil
		}
		if _, done := r.liveObjects[name]; done {
			return nil
		}
		if _, missing := r.missing[name]; missing {
			return nil
		}
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		if err := cl.Get(ctx, client.ObjectKey{Name: name}, obj); err != nil {
			if k8serrors.IsNotFound(err) && !objectNotFound(err, name) {
				// The 404 names no object: the kind itself is not served. Drop the stale mapping, as
				// Resolve does on a 404 of its list.
				if rm, ok := mapper.(meta.ResettableRESTMapper); ok {
					rm.Reset()
				}
				return fmt.Errorf("get granted resource %s %q: %w: %w", reg.Spec.GrantedResource.Kind, name, ErrGrantedResourceNotServed, err)
			}
			if k8serrors.IsNotFound(err) {
				if r.missing == nil {
					r.missing = map[string]struct{}{}
				}
				r.missing[name] = struct{}{}
				return nil
			}
			return fmt.Errorf("get granted resource %s %q: %w", reg.Spec.GrantedResource.Kind, name, err)
		}
		r.addLive(m, obj)
		return nil
	}
	for _, name := range names {
		if err := fetch(name); err != nil {
			return nil, err
		}
	}
	if withDefault {
		if err := r.settleDefault(ctx, cl, mapper, fetch); err != nil {
			return nil, err
		}
	}
	slices.Sort(r.liveNames)
	return r, nil
}

// objectNotFound reports whether a 404 is about the object of the given name, rather than about the
// resource. A missing object is a Status from the resource handler naming the object. A resource that
// is not served (its CRD deleted) is answered by the fallback handler of the API server with a plain
// "404 page not found", which client-go turns into an error marked as an unexpected server response;
// it fills in the requested name all the same, so the name alone does not tell the two apart.
func objectNotFound(err error, name string) bool {
	var status k8serrors.APIStatus
	if !errors.As(err, &status) {
		return false
	}
	details := status.Status().Details
	if details == nil || details.Name != name {
		return false
	}
	for _, cause := range details.Causes {
		if cause.Type == metav1.CauseTypeUnexpectedServerResponse {
			return false
		}
	}
	return true
}

// newResolved collects what the entries and the registration say by name: the literal allowed and
// denied names, the first explicit default, the baselines and the excluded and reserved names.
func newResolved(reg *v1alpha1.GrantableClusterResourceDefinition, entries []v1alpha1.GrantResource) *Resolved {
	r := &Resolved{
		Reg:      reg,
		allowed:  map[string]struct{}{},
		denied:   map[string]struct{}{},
		excluded: map[string]struct{}{},
		reserved: map[string]struct{}{},
	}

	// Literal names and defaults from grant entries.
	for i := range entries {
		e := &entries[i]
		for _, n := range e.Allowed {
			r.allowed[n] = struct{}{}
		}
		for _, n := range e.Denied {
			r.denied[n] = struct{}{}
		}
		if e.Default != "" && r.def == "" {
			r.def = e.Default
		}
		switch e.AvailabilityDefault {
		case v1alpha1.AvailabilityAll:
			r.anyAll = true
		case v1alpha1.AvailabilityNone:
			r.anyNone = true
		default:
			// No explicit baseline: an allow-list (names or selector) means "restrict to it", so the
			// baseline for everything else is None. This lets a grant omit availabilityDefault in the
			// common allow-list case; set it explicitly only to flip the baseline without a list
			// (All to open a project fully, None to lock it down). Reserved names do not change this: an
			// allow-list of reserved names alone narrows the project to them, like any other allow-list.
			if len(e.Allowed) > 0 || e.AllowedSelector != nil {
				r.anyNone = true
			}
		}
	}
	// Registration excluded literal names (union across all excluded filters).
	for i := range reg.Spec.Excluded {
		for _, n := range reg.Spec.Excluded[i].Names {
			r.excluded[n] = struct{}{}
		}
	}
	// Registration reserved literal names (union across all reserved filters).
	for i := range reg.Spec.Reserved {
		for _, n := range reg.Spec.Reserved[i].Names {
			r.reserved[n] = struct{}{}
		}
	}
	return r
}

// liveMatcher holds the selectors a live granted object is matched against, compiled once per
// resolve instead of per object, so the match is O(objects × entries) comparisons rather than
// recompilations.
type liveMatcher struct {
	excluded []labels.Selector
	reserved []labels.Selector
	allowed  []labels.Selector
	denied   []labels.Selector
}

func newLiveMatcher(reg *v1alpha1.GrantableClusterResourceDefinition, entries []v1alpha1.GrantResource) (*liveMatcher, error) {
	m := &liveMatcher{
		excluded: make([]labels.Selector, 0, len(reg.Spec.Excluded)),
		reserved: make([]labels.Selector, 0, len(reg.Spec.Reserved)),
		allowed:  make([]labels.Selector, len(entries)),
		denied:   make([]labels.Selector, len(entries)),
	}
	for i := range reg.Spec.Excluded {
		sel, err := filterSelector("excluded", &reg.Spec.Excluded[i])
		if err != nil {
			return nil, err
		}
		if sel != nil {
			m.excluded = append(m.excluded, sel)
		}
	}
	for i := range reg.Spec.Reserved {
		sel, err := filterSelector("reserved", &reg.Spec.Reserved[i])
		if err != nil {
			return nil, err
		}
		if sel != nil {
			m.reserved = append(m.reserved, sel)
		}
	}
	for j := range entries {
		m.allowed[j] = compiledSelector(entries[j].AllowedSelector)
		m.denied[j] = compiledSelector(entries[j].DeniedSelector)
	}
	return m, nil
}

// addLive records a live granted object and expands the selectors against its labels.
func (r *Resolved) addLive(m *liveMatcher, obj *unstructured.Unstructured) {
	name := obj.GetName()
	objLabels := labels.Set(obj.GetLabels())
	r.liveNames = append(r.liveNames, name)
	r.liveObjects[name] = obj.Object
	for _, sel := range m.excluded {
		if sel.Matches(objLabels) {
			r.excluded[name] = struct{}{}
			break
		}
	}
	for _, sel := range m.reserved {
		if sel.Matches(objLabels) {
			r.reserved[name] = struct{}{}
			break
		}
	}
	for j := range m.allowed {
		if m.denied[j] != nil && m.denied[j].Matches(objLabels) {
			r.denied[name] = struct{}{}
		}
		if m.allowed[j] != nil && m.allowed[j].Matches(objLabels) {
			r.allowed[name] = struct{}{}
		}
	}
}

// settleDefault finishes the effective default: it falls back to the registration's defaultFrom
// annotation, and is dropped when the project cannot use it. fetch, when set, reads the object of the
// default first (ResolveNames), so its availability is decided on the object as in Resolve.
func (r *Resolved) settleDefault(ctx context.Context, cl client.Reader, mapper meta.RESTMapper, fetch func(string) error) error {
	// Effective default falls back to the registration's defaultFrom annotation.
	if r.def == "" && !r.Reg.IsValueBacked() && r.Reg.Spec.DefaultFrom != nil && r.Reg.Spec.DefaultFrom.AnnotationKey != "" {
		def, err := defaultFromAnnotation(ctx, cl, mapper, r.Reg)
		if err == nil {
			r.def = def
		}
	}
	if r.def != "" && fetch != nil {
		if err := fetch(r.def); err != nil {
			return err
		}
	}

	// A default the project cannot use is not a usable default. This happens when the cluster-default
	// object (e.g. the default StorageClass via defaultFrom) — or even an explicit grant default — is
	// not in the project's available set. Drop it so Default() never returns an unavailable name and
	// the /defaults webhook never coerces to something /is-granted will reject.
	if r.def != "" && !r.isAvailable(r.def) {
		r.def = ""
	}
	return nil
}

// IsAvailable reports whether a name is in the project's available set, the set Available lists.
// Unlike Decide, a name without a live object is available only when explicitly allowed.
func (r *Resolved) IsAvailable(name string) bool { return r.isAvailable(name) }

// isAvailable reports whether a name is in the project's available set.
func (r *Resolved) isAvailable(name string) bool {
	_, ok := r.availableNames()[name]
	return ok
}

// filterSelector compiles the label part of a registration filter; field names the filter list
// (excluded or reserved) in the error.
func filterSelector(field string, f *v1alpha1.ResourceFilter) (labels.Selector, error) {
	if f == nil || (len(f.MatchLabels) == 0 && len(f.MatchExpressions) == 0) {
		return nil, nil
	}
	sel, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: f.MatchLabels, MatchExpressions: f.MatchExpressions})
	if err != nil {
		return nil, fmt.Errorf("invalid %s selector: %w", field, err)
	}
	return sel, nil
}

// compiledSelector compiles a label selector once for reuse. It returns nil for a nil or invalid
// selector, which callers treat as "matches nothing" (mirroring the previous per-call matchSel).
func compiledSelector(ls *metav1.LabelSelector) labels.Selector {
	if ls == nil {
		return nil
	}
	sel, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return nil
	}
	return sel
}

// ErrNamespacedGrantedResource is wrapped by the resolve error of a definition whose grantedResource is
// a namespaced kind. Unlike an unknown kind, which may appear once its CRD is installed, this refusal is
// permanent, so the catalog reconciler removes the catalog of such a definition instead of keeping it.
var ErrNamespacedGrantedResource = errors.New("only cluster-scoped resources can be granted")

// ErrGrantedResourceNotServed is wrapped by the resolve error of a definition whose grantedResource the
// REST mapper still maps but the apiserver answers with 404 for: the kind was served and is no longer.
var ErrGrantedResourceNotServed = errors.New("granted resource is not served")

// IsConfigurationError reports whether a Resolve error comes from the definition itself rather than
// from the API: its grantedResource is namespaced (ErrNamespacedGrantedResource) or is a kind the
// apiserver does not serve (a REST mapper no-match, or ErrGrantedResourceNotServed for a kind removed
// after it was mapped). Retrying does not fix either, so the webhooks skip such a definition and the
// catalog reconciler logs it instead of failing the pass. This is the single definition of the rule,
// so the webhooks and the reconciler cannot drift apart.
func IsConfigurationError(err error) bool {
	return errors.Is(err, ErrNamespacedGrantedResource) || errors.Is(err, ErrGrantedResourceNotServed) || meta.IsNoMatchError(err)
}

// grantedGVK resolves the granted resource's group+kind to a served GVK via the REST mapper. It is the
// only way to the granted objects, so it also refuses a namespaced kind: listing one would read the
// objects of every namespace in the cluster (Secrets, with catalogFields their values too) into the
// catalog of every project, and objects of the same name in different namespaces would collide.
func grantedGVK(mapper meta.RESTMapper, reg *v1alpha1.GrantableClusterResourceDefinition) (schema.GroupVersionKind, error) {
	gr := reg.Spec.GrantedResource
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: gr.APIGroup, Kind: gr.Kind})
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("map grantedResource %s/%s: %w", gr.APIGroup, gr.Kind, err)
	}
	if mapping.Scope.Name() != meta.RESTScopeNameRoot {
		return schema.GroupVersionKind{}, fmt.Errorf("grantedResource %s/%s is namespaced: %w", gr.APIGroup, gr.Kind, ErrNamespacedGrantedResource)
	}
	return mapping.GroupVersionKind, nil
}

// GrantedResourceProblem reports a definition whose grantedResource is a namespaced kind, in the words
// of the GrantableClusterResourceDefinition webhook; empty for a cluster-scoped kind and for a
// value-backed definition. It asks grantedGVK, so it refuses exactly what Resolve refuses. err is set
// when the scope cannot be told: a meta.IsNoMatchError for a kind the mapper does not know (its CRD is
// not installed yet), anything else for a failed discovery.
func GrantedResourceProblem(mapper meta.RESTMapper, reg *v1alpha1.GrantableClusterResourceDefinition) (string, error) {
	if reg.IsValueBacked() {
		return "", nil
	}
	_, err := grantedGVK(mapper, reg)
	if errors.Is(err, ErrNamespacedGrantedResource) {
		gk := schema.GroupKind{Group: reg.Spec.GrantedResource.APIGroup, Kind: reg.Spec.GrantedResource.Kind}
		return fmt.Sprintf("'spec.grantedResource' %s is namespaced: %v", gk, ErrNamespacedGrantedResource), nil
	}
	return "", err
}

// defaultFromAnnotation finds the single granted object annotated as the cluster-wide default.
func defaultFromAnnotation(ctx context.Context, cl client.Reader, mapper meta.RESTMapper, reg *v1alpha1.GrantableClusterResourceDefinition) (string, error) {
	gvk, err := grantedGVK(mapper, reg)
	if err != nil {
		return "", err
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
	if err := cl.List(ctx, list, client.UnsafeDisableDeepCopy); err != nil {
		return "", err
	}
	// The annotation has to say "true": the Kubernetes convention marks a class that is NOT the
	// default with the same key set to "false" (storageclass.kubernetes.io/is-default-class), and the
	// presence of the key alone would have made that class the default.
	var found []string
	for i := range list.Items {
		if value, ok := list.Items[i].GetAnnotations()[reg.Spec.DefaultFrom.AnnotationKey]; ok && strings.EqualFold(value, "true") {
			found = append(found, list.Items[i].GetName())
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	return "", fmt.Errorf("no single default annotated object (found %d)", len(found))
}

// ProjectNamespaces returns the names of namespaces belonging to the project, including the control
// namespace (the project name). The project namespaces are those labelled projects.deckhouse.io/project.
func ProjectNamespaces(ctx context.Context, cl client.Reader, project string) ([]string, error) {
	nsList := &corev1.NamespaceList{}
	if err := cl.List(ctx, nsList, client.MatchingLabels{naming.ProjectLabel: project}); err != nil {
		return nil, fmt.Errorf("list project namespaces: %w", err)
	}
	out := make([]string, 0, len(nsList.Items)+1)
	for i := range nsList.Items {
		out = append(out, nsList.Items[i].Name)
	}
	if !slices.Contains(out, project) {
		// Control namespace is named after the project; include it even if it lacks the label.
		ns := &corev1.Namespace{}
		if err := cl.Get(ctx, client.ObjectKey{Name: project}, ns); err == nil {
			out = append(out, project)
		}
	}
	slices.Sort(out)
	return out, nil
}
