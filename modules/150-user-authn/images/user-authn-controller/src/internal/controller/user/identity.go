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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/controller/revoke"
	"user-authn-controller/internal/naming"
	"user-authn-controller/internal/revocation"
	"user-authn-controller/internal/useridentity"
)

// cleanupDexRecord ends what Dex issued to the local user name once the User is gone, and removes
// what Dex keeps about it: the UserIdentity of the pair (second factors, lock, consents) and its
// OfflineSessions, where the older Dex keeps the TOTP secret. A User created again with the same
// name then starts without them, at the generation the revocations recorded (seedGeneration).
//
// The revocation raises the generation and writes a Revocation of the pair before the UserIdentity
// goes: the subscribers refuse the tokens of the deleted user, and Dex has nothing left to refresh.
// It also deletes the codes and auth requests the pair has in Dex: without the UserIdentity Dex
// reads generation 0, and a sign-in stamped 0 before the deletion would match it.
// It runs only while the pair still has a UserIdentity, a refresh token, or a browser session, as
// the API server sees them: the reconcile runs again on every change of them, and a revocation of a
// pair that has nothing would create its UserIdentity again.
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

	hasRecord, err := r.pairHasRecord(ctx, pair)
	if err != nil {
		return err
	}
	if hasRecord {
		p := revoke.Pair{UserID: name, ConnectorID: naming.LocalConnectorID, Claims: useridentity.Claims{UserID: name, Username: name}}
		if _, err := r.revoker.LogoutPair(ctx, p, revocation.ReasonUserDeleted, ""); err != nil {
			return err
		}
	}

	if err := r.deleteLocalSession(ctx, name, pair); err != nil {
		return err
	}
	if !hasRecord {
		return nil
	}
	if err := useridentity.Delete(ctx, r.client, pair); err != nil {
		return err
	}
	r.log.Info("deleted the Dex record of a deleted user", "user", name, "useridentity", pair)
	return nil
}

// pairHasRecord tells whether the pair has a UserIdentity, a refresh token, or a browser session
// left. The cache finds the candidates, the API server confirms them: the cache still holds what
// the last reconcile deleted, and acting on it would start the revocation over.
func (r *Reconciler) pairHasRecord(ctx context.Context, pair string) (bool, error) {
	found, err := r.existsInAPI(ctx, controller.UserIdentityGVK, pair)
	if err != nil || found {
		return found, err
	}
	for _, candidate := range []struct {
		gvk   schema.GroupVersionKind
		field string
	}{
		{gvk: controller.RefreshTokenGVK, field: controller.RefreshTokenPairField},
		{gvk: controller.AuthSessionGVK, field: controller.AuthSessionPairField},
	} {
		names, err := r.cachedNamesOfPair(ctx, candidate.gvk, candidate.field, pair)
		if err != nil {
			return false, err
		}
		for _, name := range names {
			found, err := r.existsInAPI(ctx, candidate.gvk, name)
			if err != nil || found {
				return found, err
			}
		}
	}
	return false, nil
}

// cachedNamesOfPair lists the names of the objects of gvk the cache index field has for pair.
func (r *Reconciler) cachedNamesOfPair(ctx context.Context, gvk schema.GroupVersionKind, field, pair string) ([]string, error) {
	list := controller.List(gvk)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingFields{field: pair}); err != nil {
		return nil, fmt.Errorf("list %s of pair %s: %w", gvk.Kind, pair, err)
	}
	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		names = append(names, list.Items[i].GetName())
	}
	return names, nil
}

// existsInAPI reads the metadata of an object in the Dex namespace through the API reader: no
// secret of it comes into the controller.
func (r *Reconciler) existsInAPI(ctx context.Context, gvk schema.GroupVersionKind, name string) (bool, error) {
	obj := &metav1.PartialObjectMetadata{}
	obj.SetGroupVersionKind(gvk)
	err := r.apiReader.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj)
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get %s %s: %w", gvk.Kind, name, err)
	}
	return true, nil
}

// seedGeneration prepares the pair of a User whose Password is about to be created. A User with
// the same name may have existed before, and its pair was revoked when it was deleted:
//
//   - the refresh tokens and browser sessions of the pair left from it are deleted;
//   - the UserIdentity gets the highest generation the stored pair revocations name, so that the
//     tokens of the old user stay revoked in Dex (another generation) and the subscribers do not
//     refuse the new user (the same one).
//
// The revocations are listed through the API reader: the cache may not have the one the deletion
// has just written. An error stops the creation of the Password, so nobody signs in at a
// generation the subscribers refuse.
func (r *Reconciler) seedGeneration(ctx context.Context, user userView, email string, groups []string, now time.Time) error {
	pair := useridentity.Name(user.Name, naming.LocalConnectorID)

	generation, err := r.revokedGeneration(ctx, pair)
	if err != nil {
		return err
	}
	for _, leftover := range []struct {
		gvk   schema.GroupVersionKind
		field string
	}{
		{gvk: controller.RefreshTokenGVK, field: controller.RefreshTokenPairField},
		{gvk: controller.AuthSessionGVK, field: controller.AuthSessionPairField},
	} {
		names, err := r.cachedNamesOfPair(ctx, leftover.gvk, leftover.field, pair)
		if err != nil {
			return err
		}
		for _, name := range names {
			obj := controller.Object(leftover.gvk)
			obj.SetNamespace(naming.DexNamespace)
			obj.SetName(name)
			if err := r.client.Delete(ctx, obj); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("delete %s %s left by a deleted user: %w", leftover.gvk.Kind, name, err)
			}
		}
	}
	if generation == 0 {
		return nil
	}

	_, err = useridentity.Mutate(ctx, r.apiReader, r.client, pair, func(current *useridentity.Identity) (*useridentity.Identity, error) {
		if current == nil {
			id := useridentity.New(user.Name, naming.LocalConnectorID)
			id.SetClaims(useridentity.Claims{
				UserID:        user.Name,
				Username:      user.Name,
				Email:         email,
				EmailVerified: true,
				Groups:        slices.Clone(groups),
			})
			if err := id.SetCreatedAt(now); err != nil {
				return nil, err
			}
			id.SetGeneration(generation)
			// A new user has nothing in the old places to copy.
			rec := useridentity.Record{Time: useridentity.FormatTime(now)}
			rec.AddSource(useridentity.SourceUser)
			if err := id.SetRecord(rec); err != nil {
				return nil, err
			}
			return id, nil
		}
		if current.UserID() != user.Name || current.ConnectorID() != naming.LocalConnectorID {
			return nil, fmt.Errorf("useridentity %s belongs to another pair", pair)
		}
		old, err := current.Generation()
		if err != nil {
			return nil, err
		}
		if old >= generation {
			return nil, nil
		}
		current.SetGeneration(generation)
		return current, nil
	})
	if err != nil {
		return fmt.Errorf("seed the generation of useridentity %s: %w", pair, err)
	}
	r.log.Info("seeded the generation of a user created again", "user", user.Name, "useridentity", pair, "generation", generation)
	return nil
}

// revokedGeneration is the highest generation the stored pair revocations of pair name.
func (r *Reconciler) revokedGeneration(ctx context.Context, pair string) (uint64, error) {
	list := controller.List(controller.RevocationGVK)
	if err := r.apiReader.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingLabels{
		revocation.PairLabel: pair,
		revocation.TypeLabel: revocation.TypePair,
	}); err != nil {
		return 0, fmt.Errorf("list revocations of pair %s: %w", pair, err)
	}
	var generation uint64
	for i := range list.Items {
		rev, err := revocation.FromObject(&list.Items[i])
		if err != nil {
			return 0, err
		}
		generation = max(generation, rev.Generation)
	}
	return generation, nil
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

// localIdentity reads the UserIdentity of the pair (user, local) from the cache, or nil when Dex
// keeps none. The user ID of a local user is the one in its Password.
func (r *Reconciler) localIdentity(ctx context.Context, pw passwordView, userName string) (*useridentity.Identity, error) {
	userID := pw.UserID
	if userID == "" {
		userID = userName
	}
	name := useridentity.Name(userID, naming.LocalConnectorID)
	obj := controller.Object(controller.UserIdentityGVK)
	err := r.client.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: name}, obj)
	if apierrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get useridentity %s: %w", name, err)
	}
	ident := useridentity.FromUnstructured(obj)
	if ident.UserID() != userID || ident.ConnectorID() != naming.LocalConnectorID {
		return nil, nil
	}
	return ident, nil
}

// mapUserIdentity reconciles the user of a local UserIdentity: its lock goes into User.status, and
// one written after the User was deleted, by a copy that read the Password before it went, is
// removed again.
func (r *Reconciler) mapUserIdentity(_ context.Context, obj client.Object) []reconcile.Request {
	u, ok := controller.AsUnstructured(obj)
	if !ok {
		return nil
	}
	id := useridentity.FromUnstructured(u)
	if id.ConnectorID() != naming.LocalConnectorID || id.UserID() == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: id.UserID()}}}
}
