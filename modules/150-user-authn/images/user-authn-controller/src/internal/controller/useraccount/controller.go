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

package useraccount

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/api/v1alpha1"
	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// maxConcurrentReconciles bounds in-flight UserAccount projections. Each
// worker is a named Get (Password / User) plus a status write; 8 matches the
// User controller so a create burst is not serialized here.
const maxConcurrentReconciles = 8

// Reconciler projects Dex Password and OfflineSessions into UserAccount objects.
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

// Register wires the UserAccount reconciler onto the manager.
func Register(mgr manager.Manager) error {
	r := New(mgr.GetClient(), mgr.GetLogger().WithName("useraccount"), nil)
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup useraccount controller: %w", err)
	}
	return nil
}

// SetupWithManager registers watches for UserAccount and its Dex/User sources.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	inDexNS := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == naming.DexNamespace
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("useraccount").
		WithOptions(ctrlcontroller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(&v1alpha1.UserAccount{}).
		Watches(controller.Object(controller.PasswordGVK), handler.EnqueueRequestsFromMapFunc(r.mapPassword), builder.WithPredicates(inDexNS)).
		Watches(controller.Object(controller.OfflineSessionsGVK), handler.EnqueueRequestsFromMapFunc(r.mapOfflineSessions), builder.WithPredicates(inDexNS)).
		Watches(controller.Object(controller.DexProviderGVK), handler.EnqueueRequestsFromMapFunc(r.mapDexProvider)).
		Watches(controller.Object(controller.UserGVK), handler.EnqueueRequestsFromMapFunc(r.mapUser)).
		Complete(r)
}

// Reconcile creates, updates, or deletes the UserAccount named in req.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	desired, err := r.desiredForName(ctx, req.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	existing := &v1alpha1.UserAccount{}
	err = r.client.Get(ctx, req.NamespacedName, existing)
	if err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("get useraccount %s: %w", req.Name, err)
	}
	exists := err == nil

	switch {
	case desired != nil && !exists:
		if err := r.create(ctx, desired); err != nil {
			return reconcile.Result{}, err
		}
	case desired != nil && exists:
		if err := r.updateIfChanged(ctx, existing, desired); err != nil {
			return reconcile.Result{}, err
		}
	case desired == nil && exists:
		return reconcile.Result{}, r.delete(ctx, existing)
	default:
		return reconcile.Result{}, nil
	}
	return reconcile.Result{RequeueAfter: requeueAfterLockedUntil(desired.Status.LockedUntil, r.now())}, nil
}

func requeueAfterLockedUntil(until *metav1.Time, now time.Time) time.Duration {
	if until == nil {
		return 0
	}
	t := until.Time
	return controller.RequeueAfterTime(&t, now)
}

func (r *Reconciler) create(ctx context.Context, desired *desiredAccount) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	ua := &v1alpha1.UserAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:   desired.Name,
			Labels: maps.Clone(desired.Labels),
		},
	}
	if desired.Owner != nil {
		owner := *desired.Owner
		ua.OwnerReferences = []metav1.OwnerReference{owner}
	}
	if err := r.client.Create(ctx, ua); err != nil {
		return fmt.Errorf("create useraccount %s: %w", desired.Name, err)
	}

	ua.Status = *desired.Status.DeepCopy()
	if err := r.client.Status().Update(ctx, ua); err != nil {
		return fmt.Errorf("update useraccount %s status: %w", desired.Name, err)
	}
	r.log.Info("created useraccount", "name", desired.Name)
	return nil
}

func (r *Reconciler) updateIfChanged(ctx context.Context, existing *v1alpha1.UserAccount, desired *desiredAccount) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	metaChanged := !ourLabelsEqual(existing.Labels, desired.Labels) || !ownerEqual(existing.OwnerReferences, desired.Owner)
	statusChanged := !statusEqual(existing.Status, desired.Status)
	if !metaChanged && !statusChanged {
		return nil
	}

	if metaChanged {
		existing.Labels = applyLabels(existing.Labels, desired.Labels)
		if desired.Owner != nil {
			owner := *desired.Owner
			existing.OwnerReferences = []metav1.OwnerReference{owner}
		} else {
			existing.OwnerReferences = nil
		}
		if err := r.client.Update(ctx, existing); err != nil {
			return fmt.Errorf("update useraccount %s: %w", existing.Name, err)
		}
	}

	if statusChanged {
		existing.Status = *desired.Status.DeepCopy()
		if err := r.client.Status().Update(ctx, existing); err != nil {
			return fmt.Errorf("update useraccount %s status: %w", existing.Name, err)
		}
	}
	return nil
}

func (r *Reconciler) delete(ctx context.Context, existing *v1alpha1.UserAccount) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.client.Delete(ctx, existing); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete useraccount %s: %w", existing.Name, err)
	}
	r.log.Info("deleted useraccount", "name", existing.Name)
	return nil
}

func (r *Reconciler) desiredForName(ctx context.Context, name string) (*desiredAccount, error) {
	now := r.now()
	desired, err := r.desiredLocal(ctx, name, now)
	if err != nil {
		return nil, err
	}
	if desired != nil {
		return desired, nil
	}
	return r.desiredExternal(ctx, name, now)
}

func (r *Reconciler) desiredLocal(ctx context.Context, name string, now time.Time) (*desiredAccount, error) {
	pw, ok, err := r.passwordForLocalAccount(ctx, name)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	user, err := r.userForPassword(ctx, pw)
	if err != nil {
		return nil, err
	}
	desired := projectLocal(pw, user, now)
	return &desired, nil
}

func (r *Reconciler) desiredExternal(ctx context.Context, name string, now time.Time) (*desiredAccount, error) {
	sess, ok, err := r.getSession(ctx, sessionNameFromAccount(name))
	if err != nil {
		return nil, err
	}
	if !ok || !isExternalCandidate(sess) || naming.ExternalName(sess.ConnID, sess.UserID) != name {
		return nil, nil
	}
	provider, ok, err := r.getProvider(ctx, sess.ConnID)
	if err != nil {
		return nil, err
	}
	if !ok || !isLockableProvider(provider.Type) {
		return nil, nil
	}
	desired := projectExternal(sess, provider.Type, now)
	return &desired, nil
}

func (r *Reconciler) passwordForLocalAccount(ctx context.Context, name string) (passwordView, bool, error) {
	if pw, ok, err := r.getPassword(ctx, passwordNameFromLocalAccount(name)); err != nil {
		return passwordView{}, false, err
	} else if ok && isProjectablePassword(pw) && naming.LocalName(localNameInput(pw.Email, pw.Username)) == name {
		return pw, true, nil
	}

	// External account names are <connID>-<hash>. A full Password list here
	// is O(N) JSON per reconcile and is only useful for local leftovers.
	if !couldBeLocalAccountName(name) {
		return passwordView{}, false, nil
	}

	passwords, err := r.listPasswords(ctx)
	if err != nil {
		return passwordView{}, false, err
	}
	for _, pw := range passwords {
		if !isProjectablePassword(pw) {
			continue
		}
		if naming.LocalName(localNameInput(pw.Email, pw.Username)) != name {
			continue
		}
		return pw, true, nil
	}
	return passwordView{}, false, nil
}

func passwordNameFromLocalAccount(accountName string) string {
	prefix := naming.LocalConnectorID + "-"
	if strings.HasPrefix(accountName, prefix) {
		return strings.TrimPrefix(accountName, prefix)
	}
	return accountName
}

func couldBeLocalAccountName(name string) bool {
	prefix := naming.LocalConnectorID + "-"
	if strings.HasPrefix(name, prefix) {
		return true
	}
	// Truncated LocalName is the FNV hash alone (no dash).
	return !strings.Contains(name, "-")
}

// sessionNameFromAccount is Dex OfflineTokenName: the hash suffix of
// ExternalName, or the whole name when the prefix was truncated.
func sessionNameFromAccount(accountName string) string {
	if i := strings.LastIndex(accountName, "-"); i >= 0 && i+1 < len(accountName) {
		return accountName[i+1:]
	}
	return accountName
}

func (r *Reconciler) getSession(ctx context.Context, name string) (sessionView, bool, error) {
	obj := controller.Object(controller.OfflineSessionsGVK)
	err := r.client.Get(ctx, types.NamespacedName{Name: name, Namespace: naming.DexNamespace}, obj)
	if apierrors.IsNotFound(err) {
		return sessionView{}, false, nil
	}
	if err != nil {
		return sessionView{}, false, fmt.Errorf("get offlinesessions %s: %w", name, err)
	}
	sess, err := decodeSession(obj)
	if err != nil {
		return sessionView{}, false, err
	}
	return sess, true, nil
}

func (r *Reconciler) getProvider(ctx context.Context, name string) (providerView, bool, error) {
	if name == "" {
		return providerView{}, false, nil
	}
	obj := controller.Object(controller.DexProviderGVK)
	err := r.client.Get(ctx, types.NamespacedName{Name: name}, obj)
	if apierrors.IsNotFound(err) {
		return providerView{}, false, nil
	}
	if err != nil {
		return providerView{}, false, fmt.Errorf("get dexprovider %s: %w", name, err)
	}
	p, err := decodeProvider(obj)
	if err != nil {
		return providerView{}, false, err
	}
	return p, true, nil
}

func (r *Reconciler) getPassword(ctx context.Context, name string) (passwordView, bool, error) {
	obj := controller.Object(controller.PasswordGVK)
	err := r.client.Get(ctx, types.NamespacedName{Name: name, Namespace: naming.DexNamespace}, obj)
	if apierrors.IsNotFound(err) {
		return passwordView{}, false, nil
	}
	if err != nil {
		return passwordView{}, false, fmt.Errorf("get password %s: %w", name, err)
	}
	pw, err := decodePassword(obj)
	if err != nil {
		return passwordView{}, false, err
	}
	return pw, true, nil
}

func (r *Reconciler) getUser(ctx context.Context, name string) (userView, bool, error) {
	obj := controller.Object(controller.UserGVK)
	err := r.client.Get(ctx, types.NamespacedName{Name: name}, obj)
	if apierrors.IsNotFound(err) {
		return userView{}, false, nil
	}
	if err != nil {
		return userView{}, false, fmt.Errorf("get user %s: %w", name, err)
	}
	u, err := decodeUser(obj)
	if err != nil {
		return userView{}, false, err
	}
	return u, true, nil
}

func (r *Reconciler) userForPassword(ctx context.Context, pw passwordView) (*userView, error) {
	if pw.Username != "" {
		u, ok, err := r.getUser(ctx, pw.Username)
		if err != nil {
			return nil, err
		}
		if ok && (pw.Email == "" || strings.EqualFold(u.Email, pw.Email)) {
			return &u, nil
		}
	}
	if pw.Email == "" {
		return nil, nil
	}
	users, err := r.listUsers(ctx)
	if err != nil {
		return nil, err
	}
	return matchUserByEmail(users, pw.Email), nil
}

func (r *Reconciler) listPasswords(ctx context.Context) ([]passwordView, error) {
	list := controller.List(controller.PasswordGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return nil, fmt.Errorf("list passwords: %w", err)
	}
	out := make([]passwordView, 0, len(list.Items))
	for i := range list.Items {
		pw, err := decodePassword(&list.Items[i])
		if err != nil {
			return nil, err
		}
		out = append(out, pw)
	}
	return out, nil
}

func (r *Reconciler) listSessions(ctx context.Context) ([]sessionView, error) {
	list := controller.List(controller.OfflineSessionsGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return nil, fmt.Errorf("list offlinesessions: %w", err)
	}
	out := make([]sessionView, 0, len(list.Items))
	for i := range list.Items {
		sess, err := decodeSession(&list.Items[i])
		if err != nil {
			return nil, err
		}
		out = append(out, sess)
	}
	return out, nil
}

func (r *Reconciler) listUsers(ctx context.Context) ([]userView, error) {
	list := controller.List(controller.UserGVK)
	if err := r.client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	out := make([]userView, 0, len(list.Items))
	for i := range list.Items {
		u, err := decodeUser(&list.Items[i])
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, nil
}

func (r *Reconciler) mapPassword(_ context.Context, obj client.Object) []reconcile.Request {
	u, ok := controller.AsUnstructured(obj)
	if !ok {
		return nil
	}
	pw, err := decodePassword(u)
	if err != nil || !isProjectablePassword(pw) {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: naming.LocalName(localNameInput(pw.Email, pw.Username))}}}
}

func (r *Reconciler) mapOfflineSessions(_ context.Context, obj client.Object) []reconcile.Request {
	u, ok := controller.AsUnstructured(obj)
	if !ok {
		return nil
	}
	sess, err := decodeSession(u)
	if err != nil || sess.ConnID == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: naming.ExternalName(sess.ConnID, sess.UserID)}}}
}

func (r *Reconciler) mapUser(_ context.Context, obj client.Object) []reconcile.Request {
	u, ok := controller.AsUnstructured(obj)
	if !ok {
		return nil
	}
	user, err := decodeUser(u)
	if err != nil || user.Email == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: naming.LocalName(user.Email)}}}
}

func (r *Reconciler) mapDexProvider(ctx context.Context, obj client.Object) []reconcile.Request {
	if err := ctx.Err(); err != nil {
		return nil
	}
	connID := obj.GetName()
	if connID == "" {
		return nil
	}

	reqs := make([]reconcile.Request, 0)
	uaList := &v1alpha1.UserAccountList{}
	if err := r.client.List(ctx, uaList, client.MatchingLabels{v1alpha1.LabelConnectorID: connID}); err == nil {
		for i := range uaList.Items {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: uaList.Items[i].Name}})
		}
	}

	sessions, err := r.listSessions(ctx)
	if err != nil {
		return uniqueRequests(reqs)
	}
	for _, sess := range sessions {
		if sess.ConnID != connID {
			continue
		}
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: naming.ExternalName(sess.ConnID, sess.UserID)}})
	}
	return uniqueRequests(reqs)
}

func uniqueRequests(reqs []reconcile.Request) []reconcile.Request {
	if len(reqs) < 2 {
		return reqs
	}
	seen := make(map[string]struct{}, len(reqs))
	out := make([]reconcile.Request, 0, len(reqs))
	for _, req := range reqs {
		if _, ok := seen[req.Name]; ok {
			continue
		}
		seen[req.Name] = struct{}{}
		out = append(out, req)
	}
	return out
}

func applyLabels(existing, desired map[string]string) map[string]string {
	out := maps.Clone(existing)
	if out == nil {
		out = make(map[string]string, len(desired))
	}
	for k, v := range desired {
		out[k] = v
	}
	return out
}

func ourLabelsEqual(existing, desired map[string]string) bool {
	for _, key := range []string{v1alpha1.LabelKind, v1alpha1.LabelConnectorID, v1alpha1.LabelLocked} {
		if existing[key] != desired[key] {
			return false
		}
	}
	return true
}

func ownerEqual(existing []metav1.OwnerReference, desired *metav1.OwnerReference) bool {
	if desired == nil {
		return len(existing) == 0
	}
	if len(existing) != 1 {
		return false
	}
	got := existing[0]
	return got.APIVersion == desired.APIVersion &&
		got.Kind == desired.Kind &&
		got.Name == desired.Name &&
		got.UID == desired.UID &&
		boolPtrVal(got.Controller) == boolPtrVal(desired.Controller) &&
		boolPtrVal(got.BlockOwnerDeletion) == boolPtrVal(desired.BlockOwnerDeletion)
}

func boolPtrVal(v *bool) bool {
	return v != nil && *v
}

func statusEqual(a, b v1alpha1.UserAccountStatus) bool {
	return a.Email == b.Email &&
		a.Username == b.Username &&
		a.UserID == b.UserID &&
		a.Kind == b.Kind &&
		a.ConnectorID == b.ConnectorID &&
		a.ProviderType == b.ProviderType &&
		a.IncorrectLoginAttempts == b.IncorrectLoginAttempts &&
		a.Locked == b.Locked &&
		a.LockedByAdministrator == b.LockedByAdministrator &&
		a.UserRef == b.UserRef &&
		metaTimeEqual(a.LockedUntil, b.LockedUntil) &&
		metaTimeEqual(a.ExpireAt, b.ExpireAt) &&
		slices.Equal(a.Groups, b.Groups)
}

func metaTimeEqual(a, b *metav1.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.UTC().Equal(b.UTC())
}
