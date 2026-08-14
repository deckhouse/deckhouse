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

package userexpire

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
)

type groupMember struct {
	Kind string
	Name string
}

// Reconciler deletes Users whose status.expireAt is in the past and strips
// them from Group spec.members.
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

// Register wires the user expiry reconciler onto the manager.
func Register(mgr manager.Manager) error {
	r := New(mgr.GetClient(), mgr.GetLogger().WithName("userexpire"), nil)
	if err := r.SetupWithManager(mgr); err != nil {
		return fmt.Errorf("setup userexpire controller: %w", err)
	}
	return nil
}

// SetupWithManager watches Users and keeps Groups in the informer cache.
func (r *Reconciler) SetupWithManager(mgr manager.Manager) error {
	warmCache := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return nil
	})
	return ctrl.NewControllerManagedBy(mgr).
		Named("userexpire").
		For(controller.Object(controller.UserGVK)).
		Watches(controller.Object(controller.GroupGVK), warmCache).
		Complete(r)
}

// Reconcile deletes the User when status.expireAt is before now and drops
// Kind=User members with that name from Groups. Future expiry is RequeueAfter.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	if err := ctx.Err(); err != nil {
		return reconcile.Result{}, err
	}

	user := controller.Object(controller.UserGVK)
	err := r.client.Get(ctx, req.NamespacedName, user)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("get user %s: %w", req.Name, err)
	}

	expireAt, ok, err := expireAtFromUser(user)
	if err != nil {
		r.log.Error(err, "cannot parse user status.expireAt", "user", user.GetName())
		return reconcile.Result{}, nil
	}
	if !ok {
		return reconcile.Result{}, nil
	}

	now := r.now()
	if !expireAt.Before(now) {
		return reconcile.Result{RequeueAfter: expireAt.Sub(now)}, nil
	}

	if err := r.stripUserFromGroups(ctx, user.GetName()); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.client.Delete(ctx, user); err != nil && !apierrors.IsNotFound(err) {
		return reconcile.Result{}, fmt.Errorf("delete user %s: %w", user.GetName(), err)
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) stripUserFromGroups(ctx context.Context, userName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	list := controller.List(controller.GroupGVK)
	if err := r.client.List(ctx, list); err != nil {
		return fmt.Errorf("list groups: %w", err)
	}

	expired := map[string]struct{}{userName: {}}
	for i := range list.Items {
		group := &list.Items[i]
		members, err := groupMembers(group)
		if err != nil {
			return fmt.Errorf("group %s members: %w", group.GetName(), err)
		}
		kept, removed := removeUsersFromGroupMembers(members, expired)
		if len(removed) == 0 {
			continue
		}

		r.log.Info("Removing expired users from group members", "group", group.GetName(), "users", removed)
		if err := r.patchGroupMembers(ctx, group, kept); err != nil {
			return err
		}
	}
	return nil
}

func (r *Reconciler) patchGroupMembers(ctx context.Context, group *unstructured.Unstructured, members []groupMember) error {
	items := make([]any, 0, len(members))
	for _, m := range members {
		items = append(items, map[string]any{"kind": m.Kind, "name": m.Name})
	}
	raw, err := json.Marshal(map[string]any{
		"spec": map[string]any{"members": items},
	})
	if err != nil {
		return fmt.Errorf("marshal group %s members patch: %w", group.GetName(), err)
	}
	if err := r.client.Patch(ctx, group, client.RawPatch(types.MergePatchType, raw)); err != nil {
		return fmt.Errorf("patch group %s members: %w", group.GetName(), err)
	}
	return nil
}

func expireAtFromUser(obj *unstructured.Unstructured) (time.Time, bool, error) {
	raw, found, err := unstructured.NestedString(obj.Object, "status", "expireAt")
	if err != nil {
		return time.Time{}, false, fmt.Errorf("read expireAt: %w", err)
	}
	if !found || raw == "" {
		return time.Time{}, false, nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("cannot convert expireAt to time: %w", err)
	}
	return parsed, true, nil
}

func groupMembers(obj *unstructured.Unstructured) ([]groupMember, error) {
	raw, found, err := unstructured.NestedSlice(obj.Object, "spec", "members")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	out := make([]groupMember, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := m["kind"].(string)
		name, _ := m["name"].(string)
		out = append(out, groupMember{Kind: kind, Name: name})
	}
	return out, nil
}

func removeUsersFromGroupMembers(members []groupMember, expiredUsers map[string]struct{}) ([]groupMember, []string) {
	newMembers := make([]groupMember, 0, len(members))
	removedUsers := make([]string, 0, len(members))
	for _, member := range members {
		if member.Kind == "User" {
			if _, shouldRemove := expiredUsers[member.Name]; shouldRemove {
				removedUsers = append(removedUsers, member.Name)
				continue
			}
		}
		newMembers = append(newMembers, member)
	}
	return newMembers, removedUsers
}
