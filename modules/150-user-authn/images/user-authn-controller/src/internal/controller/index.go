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

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"user-authn-controller/internal/naming"
)

const (
	// PasswordPairField indexes Password objects by the name of the (local, userID) pair they belong
	// to: naming.OfflineTokenName(userID, "local"), which is also the name of the pair's
	// OfflineSessions and UserIdentity. A Password object is named after the email, so without the
	// index finding the Password of a pair is a scan of every Password.
	PasswordPairField = "dex.password.pair"
	// RefreshTokenPairField indexes RefreshToken objects by the name of the pair they were issued
	// to, naming.OfflineTokenName(claims.userID, connectorID). Ending the sessions of a pair finds its
	// grants by it.
	RefreshTokenPairField = "dex.refreshtoken.pair"
	// AuthSessionPairField indexes AuthSession objects (Dex browser sessions) by the name of their pair.
	AuthSessionPairField = "dex.authsession.pair"
	// RevocationClientField indexes Revocation objects by every client in their clients field: the
	// revocation feed sends a client only the revocations that name it.
	RevocationClientField = "dex.revocation.client"
	// OAuth2ClientIDField indexes OAuth2Client objects by their client id, which the revocation feed
	// authenticates a DexAuthenticator with. The object name is a hash of the id.
	OAuth2ClientIDField = "dex.oauth2client.id"
)

// Index is a cache index the controllers query.
type Index struct {
	Object  client.Object
	Field   string
	Extract client.IndexerFunc
}

// Indexes returns every cache index of the controller. Tests register the same list on their fake
// clients.
func Indexes() []Index {
	return []Index{
		{Object: Object(PasswordGVK), Field: PasswordPairField, Extract: PasswordPair},
		{Object: Object(RefreshTokenGVK), Field: RefreshTokenPairField, Extract: RefreshTokenPair},
		{Object: Object(AuthSessionGVK), Field: AuthSessionPairField, Extract: AuthSessionPair},
		{Object: Object(RevocationGVK), Field: RevocationClientField, Extract: RevocationClients},
		{Object: Object(OAuth2ClientGVK), Field: OAuth2ClientIDField, Extract: OAuth2ClientID},
	}
}

// PasswordPair returns the pair name of a Password object, or nothing when it has no userID.
func PasswordPair(obj client.Object) []string {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	userID, _, err := unstructured.NestedString(u.Object, "userID")
	if err != nil || userID == "" {
		return nil
	}
	return []string{naming.OfflineTokenName(userID, naming.LocalConnectorID)}
}

// RefreshTokenPair returns the pair name of a RefreshToken object, or nothing when it lacks the
// user ID or the connector.
func RefreshTokenPair(obj client.Object) []string {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	userID, _, err := unstructured.NestedString(u.Object, "claims", "userID")
	if err != nil {
		return nil
	}
	connectorID, _, err := unstructured.NestedString(u.Object, "connectorID")
	if err != nil || userID == "" || connectorID == "" {
		return nil
	}
	return []string{naming.OfflineTokenName(userID, connectorID)}
}

// AuthSessionPair returns the pair name of an AuthSession object, or nothing when it lacks the
// user ID or the connector.
func AuthSessionPair(obj client.Object) []string {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	userID, _, err := unstructured.NestedString(u.Object, "userID")
	if err != nil {
		return nil
	}
	connectorID, _, err := unstructured.NestedString(u.Object, "connectorID")
	if err != nil || userID == "" || connectorID == "" {
		return nil
	}
	return []string{naming.OfflineTokenName(userID, connectorID)}
}

// RevocationClients returns the clients a Revocation object names.
func RevocationClients(obj client.Object) []string {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	clients, _, err := unstructured.NestedStringSlice(u.Object, "clients")
	if err != nil {
		return nil
	}
	return clients
}

// OAuth2ClientID returns the client id of an OAuth2Client object.
func OAuth2ClientID(obj client.Object) []string {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	id, _, err := unstructured.NestedString(u.Object, "id")
	if err != nil || id == "" {
		return nil
	}
	return []string{id}
}

// SetupIndexes registers the cache indexes the controllers query. It must run before the
// manager starts.
func SetupIndexes(ctx context.Context, indexer client.FieldIndexer) error {
	for _, index := range Indexes() {
		if err := indexer.IndexField(ctx, index.Object, index.Field, index.Extract); err != nil {
			return fmt.Errorf("index %s: %w", index.Field, err)
		}
	}
	return nil
}
