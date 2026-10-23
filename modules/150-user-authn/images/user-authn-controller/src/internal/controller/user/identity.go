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
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/useridentity"
)

// cleanupDexRecord removes what Dex keeps about the local user name once the User is gone: the
// UserIdentity of the pair (second factors, lock, consents) and its OfflineSessions, where the
// older Dex keeps the TOTP secret. A User created again with the same name then starts without
// them. Refresh tokens stop working without the Password and are left as they are.
//
// A Password that the module does not manage can hold the same pair; while one exists, its
// user is not deleted and keeps the record.
func (r *Reconciler) cleanupDexRecord(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	pair := useridentity.Name(name, naming.LocalConnectorID)

	hasPassword, err := r.pairHasPassword(ctx, pair)
	if err != nil || hasPassword {
		return err
	}

	if err := r.deleteLocalSession(ctx, name, pair); err != nil {
		return err
	}

	exists, err := r.dexObjectExists(ctx, controller.UserIdentityGVK, pair)
	if err != nil || !exists {
		return err
	}
	if err := useridentity.Delete(ctx, r.client, pair); err != nil {
		return err
	}
	r.log.Info("deleted the Dex record of a deleted user", "user", name, "useridentity", pair)
	return nil
}

// pairHasPassword tells whether a Password of the pair is left. The cache can still hold the
// Password that cleanupPasswords has just deleted; the record would then be kept, and a User created
// again right after would inherit it. So every Password the cache index finds is read again through
// the API reader, and only one the API server still has for the pair counts.
func (r *Reconciler) pairHasPassword(ctx context.Context, pair string) (bool, error) {
	cached := controller.List(controller.PasswordGVK)
	if err := r.client.List(ctx, cached, client.InNamespace(naming.DexNamespace), client.MatchingFields{controller.PasswordPairField: pair}); err != nil {
		return false, fmt.Errorf("list passwords of pair %s: %w", pair, err)
	}
	for i := range cached.Items {
		obj := controller.Object(controller.PasswordGVK)
		key := client.ObjectKeyFromObject(&cached.Items[i])
		err := r.apiReader.Get(ctx, key, obj)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("get password %s: %w", key.Name, err)
		}
		if slices.Contains(controller.PasswordPair(obj), pair) {
			return true, nil
		}
	}
	return false, nil
}

// deleteLocalSession deletes the OfflineSessions of the pair (name, local) only.
func (r *Reconciler) deleteLocalSession(ctx context.Context, name, pair string) error {
	obj := controller.Object(controller.OfflineSessionsGVK)
	err := r.client.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: pair}, obj)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("get offlinesessions %s: %w", pair, err)
	}
	userID, _, _ := unstructured.NestedString(obj.Object, "userID")
	connID, _, _ := unstructured.NestedString(obj.Object, "connID")
	if userID != name || connID != naming.LocalConnectorID {
		return nil
	}
	if err := r.client.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete offlinesessions %s: %w", pair, err)
	}
	r.log.Info("deleted the OfflineSessions of a deleted user", "user", name, "offlinesessions", pair)
	return nil
}

func (r *Reconciler) dexObjectExists(ctx context.Context, gvk schema.GroupVersionKind, name string) (bool, error) {
	obj := controller.Object(gvk)
	err := r.client.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get %s %s: %w", gvk.Kind, name, err)
	}
	return true, nil
}

// mapUserIdentity reconciles the user of a local UserIdentity that has no User: one written
// after the User was deleted, by a copy that read the Password before it went, is removed again.
func (r *Reconciler) mapUserIdentity(ctx context.Context, obj client.Object) []reconcile.Request {
	u, ok := controller.AsUnstructured(obj)
	if !ok {
		return nil
	}
	id := useridentity.FromUnstructured(u)
	if id.ConnectorID() != naming.LocalConnectorID || id.UserID() == "" {
		return nil
	}
	if _, found, err := r.getUser(ctx, id.UserID()); err != nil || found {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: id.UserID()}}}
}
