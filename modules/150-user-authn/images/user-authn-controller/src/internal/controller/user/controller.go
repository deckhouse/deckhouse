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

package user

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

var _ reconcile.Reconciler = (*Reconciler)(nil)

// maxConcurrentReconciles bounds in-flight User→Password syncs. Each worker
// is a cache Get plus an optional API Get; 8 covers a Console burst of User
// creates. Provider probes run on a different controller and do not share
// this pool.
const maxConcurrentReconciles = 8

// Reconciler syncs deckhouse.io User objects into Dex Password objects and User.status.
type Reconciler struct {
	client    client.Client
	apiReader client.Reader
	log       logr.Logger
	now       func() time.Time
}

// New constructs a Reconciler. now defaults to time.Now when nil.
// apiReader should bypass the informer cache so Password hash fields are visible
// (the UserAccount informer Transform strips them); it defaults to c when nil.
func New(c client.Client, apiReader client.Reader, log logr.Logger, now func() time.Time) *Reconciler {
	if now == nil {
		now = time.Now
	}
	if apiReader == nil {
		apiReader = c
	}
	return &Reconciler{client: c, apiReader: apiReader, log: log, now: now}
}

// Register wires the User/Password reconciler onto the manager.
func Register(mgr manager.Manager) error {
	r := New(mgr.GetClient(), mgr.GetAPIReader(), mgr.GetLogger().WithName("user"), nil)
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup user controller: %w", err)
	}
	return nil
}

// SetupWithManager registers watches for User, Group, and Password.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	inDexNS := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == naming.DexNamespace
	})

	return ctrl.NewControllerManagedBy(mgr).
		Named("user").
		WithOptions(ctrlcontroller.Options{MaxConcurrentReconciles: maxConcurrentReconciles}).
		For(controller.Object(controller.UserGVK)).
		Watches(controller.Object(controller.GroupGVK), handler.EnqueueRequestsFromMapFunc(r.mapGroup)).
		Watches(controller.Object(controller.PasswordGVK), handler.EnqueueRequestsFromMapFunc(r.mapPassword), builder.WithPredicates(inDexNS)).
		Complete(r)
}

// Reconcile creates, updates, or deletes the Dex Password for req's User and syncs User.status.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	now := r.now()

	nsExists, err := r.namespaceExists(ctx)
	if err != nil {
		return reconcile.Result{}, err
	}

	user, userFound, err := r.getUser(ctx, req.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	var lockRequeue time.Duration
	if userFound {
		lockRequeue, err = r.reconcileUser(ctx, user, nsExists, now)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	if nsExists {
		if err := r.cleanupPasswords(ctx, req.Name, userFound, user); err != nil {
			return reconcile.Result{}, err
		}
	}

	if userFound && !nsExists {
		return reconcile.Result{RequeueAfter: namespaceRequeueAfter}, nil
	}
	return reconcile.Result{RequeueAfter: lockRequeue}, nil
}

func (r *Reconciler) reconcileUser(ctx context.Context, user userView, nsExists bool, now time.Time) (time.Duration, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	groups, err := r.groupsForUserName(ctx, user.Name)
	if err != nil {
		return 0, err
	}

	expireAt, err := expireAtForUser(user, now)
	if err != nil {
		r.log.Error(err, "ignoring invalid user TTL", "user", user.Name, "ttl", user.TTL)
		expireAt = user.ExpireAt
	}

	var live passwordView
	if nsExists {
		email := strings.ToLower(user.Email)
		live, err = r.reconcilePassword(ctx, user, email, groups, now)
		if err != nil {
			return 0, err
		}
	}

	lock := lockFromPassword(live, now)
	if err := r.patchUserStatus(ctx, user, userStatusPatch{
		ExpireAt: expireAt,
		Groups:   groups,
		Lock:     lock,
	}); err != nil {
		return 0, err
	}
	return controller.RequeueAfterTime(live.LockedUntil, now), nil
}

func (r *Reconciler) namespaceExists(ctx context.Context) (bool, error) {
	ns := controller.Object(controller.NamespaceGVK)
	err := r.client.Get(ctx, types.NamespacedName{Name: naming.DexNamespace}, ns)
	if err == nil {
		return true, nil
	}
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("get namespace %s: %w", naming.DexNamespace, err)
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

func (r *Reconciler) groupsForUserName(ctx context.Context, userName string) ([]string, error) {
	groups, err := r.listGroupViews(ctx)
	if err != nil {
		return nil, err
	}
	return groupsForUser(groups, userName), nil
}

func (r *Reconciler) listGroupViews(ctx context.Context) ([]groupView, error) {
	list := controller.List(controller.GroupGVK)
	if err := r.client.List(ctx, list); err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	groups := make([]groupView, 0, len(list.Items))
	for i := range list.Items {
		g, err := decodeGroup(&list.Items[i])
		if err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func (r *Reconciler) mapPassword(_ context.Context, obj client.Object) []reconcile.Request {
	u, ok := controller.AsUnstructured(obj)
	if !ok {
		return nil
	}
	pw, err := decodePassword(u)
	if err != nil || pw.Username == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: pw.Username}}}
}

func (r *Reconciler) mapGroup(ctx context.Context, obj client.Object) []reconcile.Request {
	if err := ctx.Err(); err != nil {
		return nil
	}
	u, ok := controller.AsUnstructured(obj)
	if !ok {
		return nil
	}
	group, err := decodeGroup(u)
	if err != nil {
		r.log.Error(err, "map group: decode")
		return nil
	}
	groups, err := r.listGroupViews(ctx)
	if err != nil {
		r.log.Error(err, "map group: list groups")
		return nil
	}
	replaced := false
	for i, g := range groups {
		if g.SpecName == group.SpecName {
			groups[i] = group
			replaced = true
			break
		}
	}
	if !replaced {
		groups = append(groups, group)
	}
	names := usersInGroupSubtree(groups, group.SpecName)
	reqs := make([]reconcile.Request, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
	}
	return reqs
}
