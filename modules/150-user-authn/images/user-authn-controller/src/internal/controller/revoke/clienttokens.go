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
	"cmp"
	"context"
	"fmt"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// DeleteClientTokensOutside deletes the refresh tokens of the client clientID that were issued through a connector
// outside allowed, and the references to them in the OfflineSessions of the pairs, and returns how many tokens it
// deleted. An empty allowed list allows every connector and deletes nothing.
//
// Dex refuses such a token while the client does not allow its connector. Without the deletion the token would work
// again when the connector is allowed back, though its user may have lost the right to sign in in the meantime.
func (r *Revoker) DeleteClientTokensOutside(ctx context.Context, clientID string, allowed []string) (int, error) {
	if len(allowed) == 0 {
		return 0, nil
	}
	return r.deleteTokens(ctx, func(token tokenOwner) bool {
		return token.clientID == clientID && !slices.Contains(allowed, token.connectorID)
	}, "clientID", clientID)
}

// tokenOwner is whose refresh token it is: the client it was issued to, the connector it was issued through and the
// user.
type tokenOwner struct {
	clientID    string
	connectorID string
	userID      string
}

// deleteTokens deletes the refresh tokens that match, and the references to them in the OfflineSessions of their
// pairs. It touches neither the generations of the pairs nor the browser sessions. Every step can be repeated: a
// token or a reference that is gone is not an error. logFields describe what matched, for the log.
func (r *Revoker) deleteTokens(ctx context.Context, match func(tokenOwner) bool, logFields ...any) (int, error) {
	list := controller.List(controller.RefreshTokenGVK)
	if err := r.client.List(ctx, list, client.InNamespace(naming.DexNamespace)); err != nil {
		return 0, fmt.Errorf("list refresh tokens: %w", err)
	}

	// The names of the tokens to delete, by pair.
	type pairKey struct{ userID, connectorID string }
	byPair := map[pairKey][]string{}
	for i := range list.Items {
		item := &list.Items[i]
		owner := tokenOwner{}
		owner.clientID, _, _ = unstructured.NestedString(item.Object, "clientID")
		owner.connectorID, _, _ = unstructured.NestedString(item.Object, "connectorID")
		owner.userID, _, _ = unstructured.NestedString(item.Object, "claims", "userID")
		if !match(owner) {
			continue
		}
		key := pairKey{userID: owner.userID, connectorID: owner.connectorID}
		byPair[key] = append(byPair[key], item.GetName())
	}

	keys := make([]pairKey, 0, len(byPair))
	for key := range byPair {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b pairKey) int {
		if a.connectorID != b.connectorID {
			return cmp.Compare(a.connectorID, b.connectorID)
		}
		return cmp.Compare(a.userID, b.userID)
	})

	deleted := 0
	for _, key := range keys {
		ids := byPair[key]
		slices.Sort(ids)
		for _, id := range ids {
			ok, err := r.deleteDexObject(ctx, controller.RefreshTokenGVK, id)
			if err != nil {
				return deleted, err
			}
			if ok {
				deleted++
			}
		}
		if key.userID == "" {
			continue
		}
		pair := Pair{UserID: key.userID, ConnectorID: key.connectorID}
		refs, err := r.offlineRefs(ctx, pair)
		if err != nil {
			return deleted, err
		}
		for _, id := range ids {
			rf, ok := refs[id]
			if !ok {
				continue
			}
			if err := r.removeRef(ctx, pair.name(), rf); err != nil {
				return deleted, err
			}
		}
	}

	if deleted > 0 {
		r.log.Info("Deleted refresh tokens", append(logFields, "refreshTokensDeleted", deleted, "pairs", len(keys))...)
	}
	return deleted, nil
}
