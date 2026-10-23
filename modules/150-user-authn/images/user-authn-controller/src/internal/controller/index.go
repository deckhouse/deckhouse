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

// PasswordPairField indexes Password objects by the name of the (local, userID) pair they belong
// to: naming.OfflineTokenName(userID, "local"), which is also the name of the pair's
// OfflineSessions and UserIdentity. A Password object is named after the email, so without the
// index finding the Password of a pair is a scan of every Password.
const PasswordPairField = "dex.password.pair"

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

// SetupIndexes registers the cache indexes the controllers query. It must run before the
// manager starts.
func SetupIndexes(ctx context.Context, indexer client.FieldIndexer) error {
	if err := indexer.IndexField(ctx, Object(PasswordGVK), PasswordPairField, PasswordPair); err != nil {
		return fmt.Errorf("index %s: %w", PasswordPairField, err)
	}
	return nil
}
