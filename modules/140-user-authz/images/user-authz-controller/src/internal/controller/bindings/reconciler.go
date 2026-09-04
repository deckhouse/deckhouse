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

// Package bindings reconciles the (Cluster)RoleBindings of ClusterAuthorizationRules and
// AuthorizationRules.
//
// For every rule the controller computes the desired bindings (package desired) and makes the
// cluster match: missing bindings are created, differing ones are updated in place, and bindings
// of the rule that are no longer desired are deleted. Only objects carrying the module labels
// (heritage=deckhouse, module=user-authz) and named user-authz:<rule>:* are considered ours; that
// covers the bindings the Helm chart used to render, so they are adopted in place, and the
// per-custom-role bindings of older versions, which are removed once the aggregated binding is
// applied (creation always happens before deletion within one reconcile).
package bindings

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "user-authz-controller/api/v1"
	"user-authz-controller/api/v1alpha1"
	"user-authz-controller/internal/desired"
)

const (
	// ConditionReady is the single condition the controller maintains on a rule.
	ConditionReady = "Ready"

	ReasonBindingsApplied = "BindingsApplied"
	ReasonInvalidSpec     = "InvalidSpec"
	ReasonApplyError      = "ApplyError"

	// DefaultMaxConcurrentReconciles bounds parallel reconciles. A reconcile is a couple of indexed
	// cache reads and at most a handful of writes, so the workers are mostly waiting on the API
	// server; eight of them keep the initial adoption of tens of thousands of bindings within the
	// client rate limit while a burst of rule changes is still absorbed quickly.
	DefaultMaxConcurrentReconciles = 8

	// RuleIndexField is the cache index of (Cluster)RoleBindings by the rule they belong to.
	RuleIndexField = "user-authz.deckhouse.io/rule"

	// cacheLagRequeue is how soon a reconcile is retried when the rule exists on the API server
	// but not in the informer cache yet.
	cacheLagRequeue = time.Second

	// maxStatusMessage bounds the Ready condition message: a joined error of many bindings could
	// otherwise grow to the status size limit.
	maxStatusMessage = 2048
)

// helmAnnotations are the only annotations the controller removes from an adopted binding: the
// Helm ownership markers and the keep policy the migration hook stamped. Everything else on the
// object (annotations of admission controllers, of users) is preserved, so an outside writer does
// not make the controller fight over the object.
var helmAnnotations = []string{
	"meta.helm.sh/release-name",
	"meta.helm.sh/release-namespace",
	"helm.sh/resource-policy",
}

// Options tunes a Reconciler.
type Options struct {
	MaxConcurrentReconciles int
}

// Reconciler reconciles bindings of one rule kind: cluster-scoped (ClusterAuthorizationRule →
// ClusterRoleBinding) or namespaced (AuthorizationRule → RoleBinding).
type Reconciler struct {
	client client.Client
	// reader bypasses the cache; used to confirm a rule is really gone before its bindings are
	// deleted and to adopt an object the cache did not know about yet.
	reader     client.Reader
	recorder   events.EventRecorder
	log        logr.Logger
	namespaced bool
}

// NewCluster constructs the ClusterAuthorizationRule reconciler.
func NewCluster(c client.Client, reader client.Reader, recorder events.EventRecorder, log logr.Logger) *Reconciler {
	return &Reconciler{client: c, reader: reader, recorder: recorder, log: log, namespaced: false}
}

// NewNamespaced constructs the AuthorizationRule reconciler.
func NewNamespaced(c client.Client, reader client.Reader, recorder events.EventRecorder, log logr.Logger) *Reconciler {
	return &Reconciler{client: c, reader: reader, recorder: recorder, log: log, namespaced: true}
}

// IsModuleBinding reports whether obj carries the module labels and a rule binding name.
func IsModuleBinding(obj client.Object) bool {
	labels := obj.GetLabels()
	if labels[desired.LabelHeritage] != desired.HeritageValue || labels[desired.LabelModule] != desired.ModuleName {
		return false
	}
	_, ok := desired.RuleNameOf(obj.GetName())
	return ok
}

// RuleIndexValue is the indexer of RuleIndexField: the rule name of a module binding, nothing for
// any other object.
func RuleIndexValue(obj client.Object) []string {
	if !IsModuleBinding(obj) {
		return nil
	}
	name, _ := desired.RuleNameOf(obj.GetName())
	return []string{name}
}

// Register indexes the bindings by rule and wires both reconcilers onto the manager.
func Register(ctx context.Context, mgr manager.Manager, opts Options) error {
	if opts.MaxConcurrentReconciles <= 0 {
		opts.MaxConcurrentReconciles = DefaultMaxConcurrentReconciles
	}

	for _, obj := range []client.Object{&rbacv1.ClusterRoleBinding{}, &rbacv1.RoleBinding{}} {
		if err := mgr.GetFieldIndexer().IndexField(ctx, obj, RuleIndexField, RuleIndexValue); err != nil {
			return fmt.Errorf("index %T by rule: %w", obj, err)
		}
	}

	recorder := mgr.GetEventRecorder("user-authz-controller")

	cluster := NewCluster(mgr.GetClient(), mgr.GetAPIReader(), recorder, mgr.GetLogger().WithName("clusterauthorizationrule"))
	if err := cluster.SetupWithManager(mgr, opts); err != nil {
		return fmt.Errorf("setup clusterauthorizationrule controller: %w", err)
	}

	namespaced := NewNamespaced(mgr.GetClient(), mgr.GetAPIReader(), recorder, mgr.GetLogger().WithName("authorizationrule"))
	if err := namespaced.SetupWithManager(mgr, opts); err != nil {
		return fmt.Errorf("setup authorizationrule controller: %w", err)
	}

	return nil
}

// SetupWithManager registers the watches: the rule kind itself (spec changes only: the status
// writes of the controller do not bump the generation and must not requeue) and the bindings
// carrying the module labels, mapped back to their rule so that drift and leftovers are repaired.
func (r *Reconciler) SetupWithManager(mgr manager.Manager, opts Options) error {
	// An update is relevant when either side is ours: a binding that just lost or gained the
	// module labels has to be reconciled too.
	ours := predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return IsModuleBinding(e.Object) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return IsModuleBinding(e.Object) },
		GenericFunc: func(e event.GenericEvent) bool { return IsModuleBinding(e.Object) },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return IsModuleBinding(e.ObjectOld) || IsModuleBinding(e.ObjectNew)
		},
	}

	mapBinding := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []reconcile.Request {
		name, ok := desired.RuleNameOf(obj.GetName())
		if !ok {
			return nil
		}
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: name, Namespace: obj.GetNamespace()}}}
	})

	specChanged := builder.WithPredicates(predicate.GenerationChangedPredicate{})
	controllerOptions := ctrlcontroller.Options{MaxConcurrentReconciles: opts.MaxConcurrentReconciles}

	if r.namespaced {
		return ctrl.NewControllerManagedBy(mgr).
			Named("authorizationrule-bindings").
			WithOptions(controllerOptions).
			For(&v1alpha1.AuthorizationRule{}, specChanged).
			Watches(&rbacv1.RoleBinding{}, mapBinding, builder.WithPredicates(ours)).
			Complete(r)
	}

	return ctrl.NewControllerManagedBy(mgr).
		Named("clusterauthorizationrule-bindings").
		WithOptions(controllerOptions).
		For(&v1.ClusterAuthorizationRule{}, specChanged).
		Watches(&rbacv1.ClusterRoleBinding{}, mapBinding, builder.WithPredicates(ours)).
		Complete(r)
}

// Reconcile brings the bindings of the rule named in req to the desired state.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	rule, obj, found, err := r.getRule(ctx, r.client, req.NamespacedName)
	if err != nil {
		return reconcile.Result{}, err
	}

	if !found {
		// The cache may lag behind a rule that was just created while a binding event arrived
		// first; deleting bindings on a stale miss would strip a live rule of its access. Confirm
		// against the API server before cleaning up.
		if _, _, live, err := r.getRule(ctx, r.reader, req.NamespacedName); err != nil {
			return reconcile.Result{}, err
		} else if live {
			r.log.V(1).Info("rule not in cache yet, retrying", "rule", req.NamespacedName)
			return reconcile.Result{RequeueAfter: cacheLagRequeue}, nil
		}

		existing, err := r.existingBindings(ctx, req.Name, req.Namespace)
		if err != nil {
			return reconcile.Result{}, err
		}
		// Whatever is left with the rule's prefix is a leftover: ownerReferences take care of the
		// controller-created bindings, this covers the ones inherited from the chart.
		return reconcile.Result{}, r.deleteAll(ctx, existing)
	}

	existing, err := r.existingBindings(ctx, req.Name, req.Namespace)
	if err != nil {
		return reconcile.Result{}, err
	}

	wanted, err := desired.Bindings(rule)
	if errors.Is(err, desired.ErrInvalidSpec) {
		r.recorder.Eventf(obj, nil, corev1.EventTypeWarning, ReasonInvalidSpec, "Reconcile", "%s", err.Error())
		return reconcile.Result{}, r.setStatus(ctx, obj, rule, metav1.ConditionFalse, ReasonInvalidSpec, err.Error(), int32(len(existing)))
	}
	if err != nil {
		return reconcile.Result{}, err
	}

	if err := r.apply(ctx, rule, wanted, existing); err != nil {
		r.recorder.Eventf(obj, nil, corev1.EventTypeWarning, ReasonApplyError, "Reconcile", "%s", err.Error())
		if statusErr := r.setStatus(ctx, obj, rule, metav1.ConditionFalse, ReasonApplyError, err.Error(), int32(len(wanted))); statusErr != nil {
			r.log.Error(statusErr, "set status after apply error", "rule", req.NamespacedName)
		}
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.setStatus(ctx, obj, rule, metav1.ConditionTrue, ReasonBindingsApplied, fmt.Sprintf("%d bindings applied", len(wanted)), int32(len(wanted)))
}

// getRule loads the rule of the reconciler's kind through the given reader. found is false when
// it does not exist.
func (r *Reconciler) getRule(ctx context.Context, reader client.Reader, key client.ObjectKey) (desired.Rule, client.Object, bool, error) {
	if r.namespaced {
		ar := &v1alpha1.AuthorizationRule{}
		if err := reader.Get(ctx, key, ar); err != nil {
			if apierrors.IsNotFound(err) {
				return desired.Rule{}, nil, false, nil
			}
			return desired.Rule{}, nil, false, fmt.Errorf("get authorizationrule %s: %w", key, err)
		}
		return desired.FromAuthorizationRule(ar), ar, true, nil
	}

	car := &v1.ClusterAuthorizationRule{}
	if err := reader.Get(ctx, key, car); err != nil {
		if apierrors.IsNotFound(err) {
			return desired.Rule{}, nil, false, nil
		}
		return desired.Rule{}, nil, false, fmt.Errorf("get clusterauthorizationrule %s: %w", key, err)
	}
	return desired.FromClusterAuthorizationRule(car), car, true, nil
}

// existingBindings lists the module bindings of the rule in its scope through the rule index.
func (r *Reconciler) existingBindings(ctx context.Context, ruleName, namespace string) ([]client.Object, error) {
	byRule := client.MatchingFields{RuleIndexField: ruleName}

	if r.namespaced {
		list := &rbacv1.RoleBindingList{}
		if err := r.client.List(ctx, list, client.InNamespace(namespace), byRule); err != nil {
			return nil, fmt.Errorf("list rolebindings of %s in %s: %w", ruleName, namespace, err)
		}
		out := make([]client.Object, 0, len(list.Items))
		for i := range list.Items {
			out = append(out, &list.Items[i])
		}
		return out, nil
	}

	list := &rbacv1.ClusterRoleBindingList{}
	if err := r.client.List(ctx, list, byRule); err != nil {
		return nil, fmt.Errorf("list clusterrolebindings of %s: %w", ruleName, err)
	}
	out := make([]client.Object, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, &list.Items[i])
	}
	return out, nil
}

// apply creates or updates every wanted binding, then deletes the existing ones that are not
// wanted. Every binding is attempted even if another one failed, and the errors are reported
// together. Creation first: an old binding is only removed once its replacement is in place, so
// nothing is deleted at all while a wanted binding could not be written.
//
// A binding the API server rejects as invalid (an immutable roleRef that changed, a name it does
// not accept) is a terminal error: retrying cannot fix it, the rule status reports it and the
// controller waits for the next change of the rule.
func (r *Reconciler) apply(ctx context.Context, rule desired.Rule, wanted []desired.Binding, existing []client.Object) error {
	owner := desired.OwnerReference(rule)

	byName := make(map[string]client.Object, len(existing))
	for _, obj := range existing {
		byName[obj.GetName()] = obj
	}

	var retryable, terminal []error
	collect := func(err error) {
		switch {
		case err == nil:
		case errors.Is(err, reconcile.TerminalError(nil)):
			terminal = append(terminal, err)
		default:
			retryable = append(retryable, err)
		}
	}

	keep := make(map[string]struct{}, len(wanted))
	for _, b := range wanted {
		keep[b.Name] = struct{}{}

		want := r.materialise(b, owner)
		if current, exists := byName[b.Name]; exists {
			collect(r.update(ctx, current, want))
		} else {
			collect(r.create(ctx, want))
		}
	}

	if len(retryable) == 0 && len(terminal) == 0 {
		for _, obj := range existing {
			if _, wantedStill := keep[obj.GetName()]; wantedStill {
				continue
			}
			collect(r.delete(ctx, obj, "binding removed"))
		}
	}

	// A terminal error only stops the retries when nothing retryable is left. Otherwise the
	// retryable ones drive the backoff and the terminal ones are reported as plain text: a
	// TerminalError anywhere in the chain would make the controller drop the request.
	if len(retryable) != 0 {
		for _, err := range terminal {
			retryable = append(retryable, errors.New(err.Error()))
		}
		return errors.Join(retryable...)
	}
	return errors.Join(terminal...)
}

func (r *Reconciler) materialise(b desired.Binding, owner metav1.OwnerReference) client.Object {
	if r.namespaced {
		return desired.RoleBinding(b, owner)
	}
	return desired.ClusterRoleBinding(b, owner)
}

// create writes a new binding. AlreadyExists means the cache lags behind an object that is really
// there: it is read from the API server and adopted like any other existing binding.
func (r *Reconciler) create(ctx context.Context, want client.Object) error {
	err := r.client.Create(ctx, want)
	if err == nil {
		r.log.V(1).Info("binding created", "name", want.GetName(), "namespace", want.GetNamespace())
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return classify(fmt.Errorf("create %s: %w", want.GetName(), err))
	}

	live, ok := want.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("create %s: unexpected object type %T", want.GetName(), want)
	}
	if err := r.reader.Get(ctx, client.ObjectKeyFromObject(want), live); err != nil {
		return fmt.Errorf("read existing %s after create conflict: %w", want.GetName(), err)
	}
	if !IsModuleBinding(live) {
		return reconcile.TerminalError(fmt.Errorf("%s exists but does not carry the module labels: refusing to take over a foreign object", want.GetName()))
	}
	return r.update(ctx, live, want)
}

// update rewrites current to want when anything the controller owns differs.
func (r *Reconciler) update(ctx context.Context, current, want client.Object) error {
	if !needsUpdate(current, want) {
		return nil
	}
	updated, err := merge(current, want)
	if err != nil {
		return err
	}
	if err := r.client.Update(ctx, updated); err != nil {
		return classify(fmt.Errorf("update %s: %w", want.GetName(), err))
	}
	r.log.V(1).Info("binding updated", "name", want.GetName(), "namespace", want.GetNamespace())
	return nil
}

// delete removes obj; the UID precondition makes sure a binding recreated under the same name in
// the meantime (by another worker, by a user) is not the one that goes away.
func (r *Reconciler) delete(ctx context.Context, obj client.Object, msg string) error {
	var opts []client.DeleteOption
	if uid := obj.GetUID(); uid != "" {
		opts = append(opts, client.Preconditions{UID: &uid})
	}
	if err := r.client.Delete(ctx, obj, opts...); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete %s: %w", obj.GetName(), err)
	}
	r.log.Info(msg, "name", obj.GetName(), "namespace", obj.GetNamespace())
	return nil
}

func (r *Reconciler) deleteAll(ctx context.Context, existing []client.Object) error {
	var errs []error
	for _, obj := range existing {
		if err := r.delete(ctx, obj, "binding of a deleted rule removed"); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// classify marks the write errors that no retry can fix as terminal.
func classify(err error) error {
	if apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) {
		return reconcile.TerminalError(err)
	}
	return err
}

// needsUpdate reports whether current differs from want in anything the controller owns: roleRef,
// subjects, labels, owner references, or leftover Helm annotations.
func needsUpdate(current, want client.Object) bool {
	if !reflect.DeepEqual(current.GetLabels(), want.GetLabels()) {
		return true
	}
	if hasHelmAnnotations(current) {
		return true
	}
	if !reflect.DeepEqual(current.GetOwnerReferences(), want.GetOwnerReferences()) {
		return true
	}

	switch c := current.(type) {
	case *rbacv1.ClusterRoleBinding:
		w, ok := want.(*rbacv1.ClusterRoleBinding)
		return !ok || !reflect.DeepEqual(c.RoleRef, w.RoleRef) || !reflect.DeepEqual(c.Subjects, w.Subjects)
	case *rbacv1.RoleBinding:
		w, ok := want.(*rbacv1.RoleBinding)
		return !ok || !reflect.DeepEqual(c.RoleRef, w.RoleRef) || !reflect.DeepEqual(c.Subjects, w.Subjects)
	default:
		return true
	}
}

func hasHelmAnnotations(obj client.Object) bool {
	annotations := obj.GetAnnotations()
	for _, key := range helmAnnotations {
		if _, present := annotations[key]; present {
			return true
		}
	}
	return false
}

// merge returns current with everything the controller owns replaced by want: labels, owner
// references, roleRef and subjects. Of the annotations only the Helm ones are removed.
//
// roleRef is immutable on bindings; a differing roleRef is surfaced as an Invalid error by the
// API server and reported in the rule status rather than worked around by recreation.
func merge(current, want client.Object) (client.Object, error) {
	updated, ok := current.DeepCopyObject().(client.Object)
	if !ok {
		return nil, fmt.Errorf("merge %s: unexpected object type %T", current.GetName(), current)
	}
	updated.SetLabels(want.GetLabels())
	updated.SetOwnerReferences(want.GetOwnerReferences())
	updated.SetAnnotations(withoutHelmAnnotations(current.GetAnnotations()))

	switch u := updated.(type) {
	case *rbacv1.ClusterRoleBinding:
		w, ok := want.(*rbacv1.ClusterRoleBinding)
		if !ok {
			return nil, fmt.Errorf("merge %s: want is %T, current is %T", current.GetName(), want, current)
		}
		u.RoleRef = w.RoleRef
		u.Subjects = w.Subjects
	case *rbacv1.RoleBinding:
		w, ok := want.(*rbacv1.RoleBinding)
		if !ok {
			return nil, fmt.Errorf("merge %s: want is %T, current is %T", current.GetName(), want, current)
		}
		u.RoleRef = w.RoleRef
		u.Subjects = w.Subjects
	default:
		return nil, fmt.Errorf("merge %s: unsupported object type %T", current.GetName(), current)
	}

	return updated, nil
}

func withoutHelmAnnotations(annotations map[string]string) map[string]string {
	if len(annotations) == 0 {
		return nil
	}
	out := maps.Clone(annotations)
	for _, key := range helmAnnotations {
		delete(out, key)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// setStatus writes the Ready condition, the binding count and the observed generation to the
// rule, skipping the write when nothing changed.
func (r *Reconciler) setStatus(ctx context.Context, obj client.Object, rule desired.Rule, status metav1.ConditionStatus, reason, message string, bindings int32) error {
	var current *v1.RuleStatus
	switch o := obj.(type) {
	case *v1.ClusterAuthorizationRule:
		current = &o.Status
	case *v1alpha1.AuthorizationRule:
		current = &o.Status
	default:
		return fmt.Errorf("unexpected rule type %T", obj)
	}

	before, ok := obj.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("unexpected rule type %T", obj)
	}

	changed := apimeta.SetStatusCondition(&current.Conditions, metav1.Condition{
		Type:               ConditionReady,
		Status:             status,
		Reason:             reason,
		Message:            truncate(message, maxStatusMessage),
		ObservedGeneration: rule.Generation,
	})
	if current.Bindings != bindings {
		current.Bindings = bindings
		changed = true
	}
	if current.ObservedGeneration != rule.Generation {
		current.ObservedGeneration = rule.Generation
		changed = true
	}
	if !changed {
		return nil
	}

	if err := r.client.Status().Patch(ctx, obj, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("patch status of %s/%s: %w", rule.Namespace, rule.Name, err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
