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
// role are removed. The whole set is recomputed on every change, so the reconciler is keyed by a
// single constant request.
package dictbindings

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// RequestName is the constant key every event is mapped to.
	RequestName = "dict-bindings"

	DictRoleName   = "d8:use:dict"
	NamePrefix     = "d8:dict:"
	SubjectAnnotat = "rbac.deckhouse.io/subject"

	labelHeritage  = "heritage"
	labelAutomated = "rbac.deckhouse.io/automated"
	labelDict      = "rbac.deckhouse.io/dict"

	useRolePrefix = "d8:use:role:"

	// subjectKeyMaxLen bounds the subject key, as the hook this reconciler replaces did.
	subjectKeyMaxLen = 55
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

// Register wires the reconciler onto the manager.
func Register(mgr manager.Manager) error {
	r := New(mgr.GetClient(), mgr.GetLogger().WithName("dict-bindings"))
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup dict-bindings controller: %w", err)
	}
	return nil
}

// SetupWithManager watches the RoleBindings that contribute subjects and the dict
// ClusterRoleBindings themselves; every event maps to the single request.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	single := handler.EnqueueRequestsFromMapFunc(func(_ context.Context, _ client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: RequestName}}}
	})

	contributes := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		rb, ok := obj.(*rbacv1.RoleBinding)
		return ok && contributesSubjects(rb)
	})

	isDict := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return hasLabels(obj.GetLabels(), DictLabels)
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("dict-bindings").
		Watches(&rbacv1.RoleBinding{}, single, builder.WithPredicates(contributes)).
		Watches(&rbacv1.ClusterRoleBinding{}, single, builder.WithPredicates(isDict)).
		Complete(r)
}

// Reconcile recomputes the set of subjects and makes the dict ClusterRoleBindings match it.
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	roleBindings := &rbacv1.RoleBindingList{}
	if err := r.client.List(ctx, roleBindings); err != nil {
		return reconcile.Result{}, fmt.Errorf("list rolebindings: %w", err)
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
	if err := r.client.List(ctx, existing, client.MatchingLabels(DictLabels)); err != nil {
		return reconcile.Result{}, fmt.Errorf("list dict clusterrolebindings: %w", err)
	}

	for i := range existing.Items {
		crb := &existing.Items[i]
		if len(crb.Subjects) == 0 {
			continue
		}

		key := SubjectKey(crb.Subjects[0])
		if _, wanted := subjects[key]; !wanted {
			if err := r.client.Delete(ctx, crb); err != nil && !apierrors.IsNotFound(err) {
				return reconcile.Result{}, fmt.Errorf("delete dict binding %s: %w", crb.Name, err)
			}
			r.log.Info("dict binding removed", "name", crb.Name, "subject", key)
			continue
		}
		// Already granted; a subject may be bound by more than one object (legacy random names),
		// deleting only the first is enough to keep one per subject over time.
		delete(subjects, key)
	}

	for key, subject := range subjects {
		crb := Binding(key, subject)
		if err := r.client.Create(ctx, crb); err != nil {
			if apierrors.IsAlreadyExists(err) {
				continue
			}
			return reconcile.Result{}, fmt.Errorf("create dict binding for %s: %w", key, err)
		}
		r.log.Info("dict binding created", "name", crb.Name, "subject", key)
	}

	return reconcile.Result{}, nil
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

// SubjectKey is the identity of a subject as the hook this reconciler replaces computed it:
// kind[:namespace]:name, ServiceAccount shortened to sa, lower-cased and cut at 55 characters.
// Existing dict bindings are matched by this key of their first subject, so it must not change.
func SubjectKey(subject rbacv1.Subject) string {
	kind := subject.Kind
	if kind == rbacv1.ServiceAccountKind {
		kind = "sa"
	}

	key := kind + ":" + subject.Name
	if subject.Namespace != "" {
		key = kind + ":" + subject.Namespace + ":" + subject.Name
	}
	if len(key) > subjectKeyMaxLen {
		key = key[:subjectKeyMaxLen]
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
			Labels:      copyLabels(DictLabels),
			Annotations: map[string]string{SubjectAnnotat: key},
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

func copyLabels(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
