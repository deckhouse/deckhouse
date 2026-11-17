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

package identitymigration

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"user-authn-controller/internal/controller"
	"user-authn-controller/internal/naming"
)

// The migration reads the refresh tokens of a pair through the cached client, and the cache keeps a
// RefreshToken reduced: what it reads from the reduced token must be what it reads from the whole
// one.
func TestDecodeTokenReadsTheCachedToken(t *testing.T) {
	t.Parallel()

	full := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion":  "dex.coreos.com/v1",
		"kind":        "RefreshToken",
		"metadata":    map[string]any{"name": "rt-1", "namespace": naming.DexNamespace},
		"clientID":    "console",
		"connectorID": "ldap",
		"token":       "secret-token",
		"scopes":      []any{"openid", "groups"},
		"claims": map[string]any{
			"userID":            "u-1",
			"username":          "jane",
			"preferredUsername": "jane.doe",
			"email":             "jane@example.com",
			"emailVerified":     true,
			"groups":            []any{"admins", "dev"},
		},
		"CreatedAt": "2026-11-01T10:00:00Z",
		"LastUsed":  "2026-11-16T10:00:00Z",
	}}
	want, err := decodeToken(full)
	if err != nil {
		t.Fatalf("decode the whole token: %v", err)
	}

	reduced, err := controller.ReduceRefreshToken(full)
	if err != nil {
		t.Fatalf("ReduceRefreshToken: %v", err)
	}
	cached, ok := reduced.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("ReduceRefreshToken returned %T", reduced)
	}
	got, err := decodeToken(cached)
	if err != nil {
		t.Fatalf("decode the cached token: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("cached token decodes to %+v, want %+v", got, want)
	}
	if got.Claims.Email != "jane@example.com" || len(got.Claims.Groups) != 2 || got.LastUsed.IsZero() {
		t.Errorf("decoded token lost what the migration copies: %+v", got)
	}
}
