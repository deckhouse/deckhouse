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

package revoke

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// PairDeleted counts what DeletePair deleted.
type PairDeleted struct {
	RefreshTokens int
	AuthSessions  int
	// OfflineSessions is whether the OfflineSessions object of the pair was there.
	OfflineSessions bool
}

// DeletePair deletes what Dex keeps of a pair, except its UserIdentity: the refresh tokens, the browser sessions and
// the OfflineSessions. It writes no revocation and raises no generation: it is for a pair whose connector is gone and
// whose tokens run out by themselves (the cleanup of the UserIdentity of a deleted DexProvider), not for ending what a
// live pair holds (LogoutPair). Every step can be repeated.
//
// The name of a pair is a hash of the user and the connector without a separator, so two pairs can share it: every
// object is checked for the user and the connector of this pair before it goes, as signInOf and offlineRefs do, and
// the OfflineSessions goes with the UID and the resourceVersion of the object checked.
func (r *Revoker) DeletePair(ctx context.Context, userID, connectorID string) (PairDeleted, error) {
	p := Pair{UserID: userID, ConnectorID: connectorID}
	name := p.name()
	var out PairDeleted

	tokens, err := r.pairObjects(ctx, controller.RefreshTokenGVK, controller.RefreshTokenPairField, name)
	if err != nil {
		return out, err
	}
	for i := range tokens {
		owner, _, _ := unstructured.NestedString(tokens[i].Object, "claims", "userID")
		conn, _, _ := unstructured.NestedString(tokens[i].Object, "connectorID")
		if owner != userID || conn != connectorID {
			continue
		}
		deleted, err := r.deleteDexObject(ctx, controller.RefreshTokenGVK, tokens[i].GetName())
		if err != nil {
			return out, err
		}
		if deleted {
			out.RefreshTokens++
		}
	}

	sessions, err := r.pairObjects(ctx, controller.AuthSessionGVK, controller.AuthSessionPairField, name)
	if err != nil {
		return out, err
	}
	for i := range sessions {
		owner, _, _ := unstructured.NestedString(sessions[i].Object, "userID")
		conn, _, _ := unstructured.NestedString(sessions[i].Object, "connectorID")
		if owner != userID || conn != connectorID {
			continue
		}
		deleted, err := r.deleteDexObject(ctx, controller.AuthSessionGVK, sessions[i].GetName())
		if err != nil {
			return out, err
		}
		if deleted {
			out.AuthSessions++
		}
	}

	out.OfflineSessions, err = r.deleteOfflineSessionsOf(ctx, p)
	return out, err
}

// pairObjects lists the objects of the kind that the cache index puts under the name of a pair.
func (r *Revoker) pairObjects(ctx context.Context, gvk schema.GroupVersionKind, field, pairName string) ([]unstructured.Unstructured, error) {
	list := controller.List(gvk)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace), client.MatchingFields{field: pairName}); err != nil {
		return nil, fmt.Errorf("list %s of pair %s: %w", gvk.Kind, pairName, err)
	}
	return list.Items, nil
}

// deleteOfflineSessionsOf deletes the OfflineSessions of the pair, read fresh, when it belongs to the pair: one of
// another pair with the same name stays. It reports whether it deleted one.
func (r *Revoker) deleteOfflineSessionsOf(ctx context.Context, p Pair) (bool, error) {
	obj := controller.Object(controller.OfflineSessionsGVK)
	err := r.apiReader.Get(ctx, types.NamespacedName{Namespace: naming.DexNamespace, Name: p.name()}, obj)
	switch {
	case apierrors.IsNotFound(err):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("get offlinesessions %s: %w", p.name(), err)
	}
	owner, _, _ := unstructured.NestedString(obj.Object, "userID")
	conn, _, _ := unstructured.NestedString(obj.Object, "connID")
	if owner != p.UserID || conn != p.ConnectorID {
		return false, nil
	}
	uid, resourceVersion := obj.GetUID(), obj.GetResourceVersion()
	err = r.client.Delete(ctx, obj, client.Preconditions{UID: &uid, ResourceVersion: &resourceVersion})
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	case apierrors.IsConflict(err):
		// Written in between by a sign-in. The caller must leave the UserIdentity for the next pass:
		// deleting it now would orphan the OfflineSessions, and identitymigration would recreate the identity from it.
		return false, fmt.Errorf("delete offlinesessions %s: %w", p.name(), err)
	default:
		return false, fmt.Errorf("delete offlinesessions %s: %w", p.name(), err)
	}
}
