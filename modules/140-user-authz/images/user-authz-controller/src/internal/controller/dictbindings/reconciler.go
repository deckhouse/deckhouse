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

// Package dictbindings grants the d8:use:dict ClusterRole to every subject that holds a use role.
//
// Subjects are collected from RoleBindings of the experimental role model (roleRef d8:use:role:*,
// not created by Deckhouse) and from the RoleBindings the module itself creates for the current
// model's namespaced rules (roleRef user-authz:user|privileged-user|editor|admin). Each distinct
// subject gets one ClusterRoleBinding d8:dict:*; bindings whose subject no longer holds any use
// role, duplicates, and bindings that lost their roleRef or subject are removed. The whole set is
// recomputed on every change, so the reconciler is keyed by a single constant request; the
// contributing RoleBindings are read through a cache index, so a reconcile copies only them and
// not every RoleBinding of the cluster.
package dictbindings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// RequestName is the constant key every event is mapped to.
	RequestName = "dict-bindings"

	// SourceIndexField indexes the RoleBindings whose subjects must hold d8:use:dict.
	SourceIndexField = "user-authz.deckhouse.io/dict-source"
	sourceIndexValue = "true"
	// OwnedIndexField indexes the dict ClusterRoleBindings this reconciler owns: a plain label
	// selector on the cache would scan every ClusterRoleBinding of the cluster on every reconcile.
	OwnedIndexField = "user-authz.deckhouse.io/dict-binding"
	ownedIndexValue = "true"

	DictRoleName      = "d8:use:dict"
	NamePrefix        = "d8:dict:"
	SubjectAnnotation = "rbac.deckhouse.io/subject"

	labelHeritage  = "heritage"
	labelAutomated = "rbac.deckhouse.io/automated"
	labelDict      = "rbac.deckhouse.io/dict"

	useRolePrefix = "d8:use:role:"
)

// DictLabels mark the ClusterRoleBindings this reconciler owns.
var DictLabels = map[string]string{
	labelHeritage:  "deckhouse",
	labelAutomated: "true",
	labelDict:      "true",
}

// reservedRoleRefs are the current-model use roles bound by the module's own RoleBindings.
var reservedRoleRefs = []string{
	"user-authz:user",
	"user-authz:privileged-user",
	"user-authz:editor",
	"user-authz:admin",
}

// Reconciler keeps one d8:dict:* ClusterRoleBinding per subject holding a use role.
type Reconciler struct {
	client client.Client
	log    logr.Logger
}

// New constructs a Reconciler.
func New(c client.Client, log logr.Logger) *Reconciler {
	return &Reconciler{client: c, log: log}
}

// SourceIndexValue is the indexer of SourceIndexField.
func SourceIndexValue(obj client.Object) []string {
	rb, ok := obj.(*rbacv1.RoleBinding)
	if !ok || !contributesSubjects(rb) {
		return nil
	}
	return []string{sourceIndexValue}
}

// OwnedIndexValue is the indexer of OwnedIndexField.
func OwnedIndexValue(obj client.Object) []string {
	if _, ok := obj.(*rbacv1.ClusterRoleBinding); !ok || !hasLabels(obj.GetLabels(), DictLabels) {
		return nil
	}
	return []string{ownedIndexValue}
}

// Register indexes the contributing RoleBindings and the dict ClusterRoleBindings and wires the
// reconciler onto the manager.
func Register(ctx context.Context, mgr manager.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(ctx, &rbacv1.RoleBinding{}, SourceIndexField, SourceIndexValue); err != nil {
		return fmt.Errorf("index rolebindings by dict source: %w", err)
	}
	if err := mgr.GetFieldIndexer().IndexField(ctx, &rbacv1.ClusterRoleBinding{}, OwnedIndexField, OwnedIndexValue); err != nil {
		return fmt.Errorf("index dict clusterrolebindings: %w", err)
	}
	r := New(mgr.GetClient(), mgr.GetLogger().WithName("dict-bindings"))
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup dict-bindings controller: %w", err)
	}
	return nil
}

// SetupWithManager watches the RoleBindings that contribute subjects and the dict
// ClusterRoleBindings themselves; every event maps to the single request. Updates are relevant
// when either the old or the new object is of interest, so a binding that stops contributing
// (roleRef cannot change, but labels can) is still processed.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	single := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: RequestName}}}
	})

	contributes := eitherSide(func(obj client.Object) bool {
		rb, ok := obj.(*rbacv1.RoleBinding)
		return ok && contributesSubjects(rb)
	})

	isDict := eitherSide(func(obj client.Object) bool {
		return hasLabels(obj.GetLabels(), DictLabels)
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("dict-bindings").
		Watches(&rbacv1.RoleBinding{}, single, builder.WithPredicates(contributes)).
		Watches(&rbacv1.ClusterRoleBinding{}, single, builder.WithPredicates(isDict)).
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

// Reconcile recomputes the set of subjects and makes the dict ClusterRoleBindings match it. Every
// object is attempted even when another one failed; the errors are reported together.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	roleBindings := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, roleBindings, client.MatchingFields{SourceIndexField: sourceIndexValue}); err != nil {
		return reconcile.Result{}, fmt.Errorf("list contributing rolebindings: %w", err)
	}

	subjects := make(map[string]rbacv1.Subject)
	for i := range roleBindings.Items {
		rb := &roleBindings.Items[i]
		if !contributesSubjects(rb) {
			continue
		}
		for _, subject := range rb.Subjects {
			if subject.Kind == rbacv1.ServiceAccountKind && subject.Namespace == "" {
				subject.Namespace = rb.Namespace
			}
			subjects[SubjectKey(subject)] = subject
		}
	}

	existing := &rbacv1.ClusterRoleBindingList{}
	if err := r.client.List(ctx, existing, client.MatchingFields{OwnedIndexField: ownedIndexValue}); err != nil {
		return reconcile.Result{}, fmt.Errorf("list dict clusterrolebindings: %w", err)
	}

	var errs []error
	granted := make(map[string]struct{}, len(existing.Items))
	for i := range existing.Items {
		crb := &existing.Items[i]

		reason := ""
		switch {
		case len(crb.Subjects) == 0:
			reason = "no subject"
		case crb.RoleRef.Kind != "ClusterRole" || crb.RoleRef.Name != DictRoleName:
			// roleRef is immutable; a wrong one is replaced by deleting the object, the subject is
			// granted again below.
			reason = "wrong roleRef " + crb.RoleRef.Name
		}
		if reason == "" {
			if len(crb.Subjects) != 1 {
				reason = "more than one subject"
			} else {
				// The first binding of a subject is kept and the subject leaves the wanted set, so a
				// duplicate must be recognised through granted before the wanted set is consulted.
				key := SubjectKey(crb.Subjects[0])
				switch _, dup := granted[key]; {
				case dup:
					reason = "duplicate of another dict binding"
				default:
					if _, wanted := subjects[key]; !wanted {
						reason = "subject holds no use role"
					} else {
						granted[key] = struct{}{}
						delete(subjects, key)
						continue
					}
				}
			}
		}

		if err := r.client.Delete(ctx, crb); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("delete dict binding %s: %w", crb.Name, err))
			continue
		}
		r.log.Info("dict binding removed", "name", crb.Name, "reason", reason)
	}

	for _, key := range slices.Sorted(maps.Keys(subjects)) {
		crb := Binding(key, subjects[key])
		if err := r.client.Create(ctx, crb); err != nil {
			if apierrors.IsAlreadyExists(err) {
				continue
			}
			errs = append(errs, fmt.Errorf("create dict binding for %s: %w", key, err))
			continue
		}
		r.log.Info("dict binding created", "name", crb.Name, "subject", key)
	}

	return reconcile.Result{}, errors.Join(errs...)
}

// contributesSubjects reports whether the RoleBinding's subjects must hold d8:use:dict: either a
// user-created binding to an experimental use role, or a module-created binding of a namespaced
// rule of the current model.
func contributesSubjects(rb *rbacv1.RoleBinding) bool {
	if rb.RoleRef.Kind != "ClusterRole" {
		return false
	}

	deckhouse := rb.Labels[labelHeritage] == "deckhouse"

	if !deckhouse && strings.HasPrefix(rb.RoleRef.Name, useRolePrefix) {
		return true
	}

	return deckhouse && slices.Contains(reservedRoleRefs, rb.RoleRef.Name)
}

// SubjectKey is the identity of a subject: kind[:namespace]:name, ServiceAccount shortened to sa,
// lower-cased. Existing dict bindings are matched by the key of their first subject, so the
// key must stay a pure function of the subject. Unlike the hook this reconciler replaces, the key
// is not truncated: two subjects sharing a long prefix are two subjects. The lower-casing is kept
// from the hook (the bindings it created are matched by the same key); subjects that differ only in
// case share one grant, which is accepted, RBAC subject names being case-sensitive but such pairs
// not occurring in practice.
func SubjectKey(subject rbacv1.Subject) string {
	kind := subject.Kind
	if kind == rbacv1.ServiceAccountKind {
		kind = "sa"
	}

	key := kind + ":" + subject.Name
	if subject.Namespace != "" {
		key = kind + ":" + subject.Namespace + ":" + subject.Name
	}

	return strings.ToLower(key)
}

// Binding builds the ClusterRoleBinding for a subject. The name is derived from the subject key,
// so a subject is never granted twice by a race; bindings with legacy generated names are still
// recognised by their subject.
func Binding(key string, subject rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	sum := sha256.Sum256([]byte(key))

	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{APIVersion: rbacv1.SchemeGroupVersion.String(), Kind: "ClusterRoleBinding"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        NamePrefix + hex.EncodeToString(sum[:])[:16],
			Labels:      maps.Clone(DictLabels),
			Annotations: map[string]string{SubjectAnnotation: key},
		},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: DictRoleName},
		Subjects: []rbacv1.Subject{subject},
	}
}

func hasLabels(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}
