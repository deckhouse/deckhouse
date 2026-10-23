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

package revocationfeed

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	toolscache "k8s.io/client-go/tools/cache"

	"user-authn-controller/internal/revocation"
)

// The vectors of appendix A of the MR3 contract, with the client secret s3cr3t. dex-authenticator
// checks the same ones.
func TestKeys_AppendixA(t *testing.T) {
	t.Parallel()

	salt := Salt([]byte("s3cr3t"))
	if got := hex.EncodeToString(salt); got != "81dac58c605535cd1c2c5bfd371e03b58a946289ab806a32d2ee3397ec693e62" {
		t.Errorf("salt = %s", got)
	}
	tests := []struct {
		kind     string
		value    string
		expected string
	}{
		{kind: keyPair, value: revocation.Subject("jane", "local"), expected: "BtOeixcwfXZ3z6kDWHNMTw"},
		{kind: keySession, value: "fehzwdh5qy4oyqcn7qhkcgzbm", expected: "0-SQllhFkuuG1ttZoMxAiA"},
		{kind: keyGrant, value: "3w4l2zgokgsohvjnqzjrv7lmz", expected: "jVFqq8vBRSjPWcgLzpRzaA"},
	}
	for _, tt := range tests {
		if got := Key(salt, tt.kind, tt.value); got != tt.expected {
			t.Errorf("Key(%s, %s) = %s, want %s", tt.kind, tt.value, got, tt.expected)
		}
	}
	sum := sha256.Sum256([]byte("s3cr3t"))
	if got := hex.EncodeToString(sum[:]); got != "4e738ca5563c06cfd0018299933d58db1dd8bf97f6973dc99bf6cdc64b5550bd" {
		t.Errorf("SHA-256 = %s", got)
	}
	if revocation.Subject("jane", "local") != "CgRqYW5lEgVsb2NhbA" {
		t.Error("the subject of the vector changed")
	}
}

// client-go may run the transform again on an object it has already transformed: the second run
// must keep the hash and the salt, or every client of the feed is refused.
func TestTransformClient_IsIdempotent(t *testing.T) {
	t.Parallel()

	once, err := TransformClient(rawClient("console-d8-console-dex-authenticator", "s3cr3t", true))
	if err != nil {
		t.Fatal(err)
	}
	twice, err := TransformClient(once)
	if err != nil {
		t.Fatal(err)
	}
	first, second := once.(*unstructured.Unstructured), twice.(*unstructured.Unstructured)
	for _, field := range []string{secretHashField, saltField, "id"} {
		if second.Object[field] == nil || second.Object[field] != first.Object[field] {
			t.Errorf("%s after the second transform = %v, want %v", field, second.Object[field], first.Object[field])
		}
	}
}

func TestTransformClient_KeepsNoSecret(t *testing.T) {
	t.Parallel()

	raw := rawClient("console-d8-console-dex-authenticator", "s3cr3t", true)
	raw.Object["redirectURIs"] = []any{"https://console.example.com/callback"}
	raw.SetAnnotations(map[string]string{"meta.helm.sh/release-name": "user-authn"})

	out, err := TransformClient(raw)
	if err != nil {
		t.Fatal(err)
	}
	reduced, ok := out.(*unstructured.Unstructured)
	if !ok {
		t.Fatalf("TransformClient = %T", out)
	}
	if strings.Contains(stringOf(reduced.Object), "s3cr3t") {
		t.Errorf("the cached object holds the secret: %v", reduced.Object)
	}
	if _, ok := raw.Object["secret"]; !ok {
		t.Error("the input object was changed")
	}
	if reduced.Object["id"] != "console-d8-console-dex-authenticator" || reduced.GetName() != raw.GetName() ||
		reduced.GetLabels()[DexAuthenticatorLabel] != "console" || reduced.GetResourceVersion() != "7" {
		t.Errorf("reduced = %v", reduced.Object)
	}
	if reduced.Object[secretHashField] != "4e738ca5563c06cfd0018299933d58db1dd8bf97f6973dc99bf6cdc64b5550bd" ||
		reduced.Object[saltField] != "81dac58c605535cd1c2c5bfd371e03b58a946289ab806a32d2ee3397ec693e62" {
		t.Errorf("derived fields = %v, %v", reduced.Object[secretHashField], reduced.Object[saltField])
	}
	for _, field := range []string{"redirectURIs", "secret"} {
		if _, ok := reduced.Object[field]; ok {
			t.Errorf("%s is kept", field)
		}
	}
	if len(reduced.GetAnnotations()) != 0 {
		t.Errorf("annotations = %v", reduced.GetAnnotations())
	}

	tombstone, err := TransformClient(toolscache.DeletedFinalStateUnknown{Key: "x", Obj: rawClient("a", "s3cr3t", true)})
	if err != nil {
		t.Fatal(err)
	}
	inner, ok := tombstone.(toolscache.DeletedFinalStateUnknown).Obj.(*unstructured.Unstructured)
	if !ok || strings.Contains(stringOf(inner.Object), "s3cr3t") {
		t.Errorf("tombstone = %v", tombstone)
	}
}

func TestClientCache_SelectsDexAuthenticators(t *testing.T) {
	t.Parallel()

	_, byObject, err := ClientCache()
	if err != nil {
		t.Fatal(err)
	}
	if !byObject.Label.Matches(labels.Set{DexAuthenticatorLabel: "console"}) || byObject.Label.Matches(labels.Set{"app": "dex"}) {
		t.Errorf("label selector = %v", byObject.Label)
	}
	if byObject.Transform == nil || len(byObject.Namespaces) != 1 {
		t.Errorf("cache config = %+v", byObject)
	}
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	reader := newTestReader(t,
		cachedClient("console", "s3cr3t", true),
		cachedClient("odd:id", "p@ss:wo%rd", true),
		cachedClient("kubernetes", "s3cr3t", false),
	)
	tests := []struct {
		name     string
		id       string
		secret   string
		noAuth   bool
		expected bool
	}{
		{name: "dex-authenticator client", id: "console", secret: "s3cr3t", expected: true},
		{name: "form-encoded credentials", id: url.QueryEscape("odd:id"), secret: url.QueryEscape("p@ss:wo%rd"), expected: true},
		{name: "wrong secret", id: "console", secret: "s3cr3t!"},
		{name: "unknown client", id: "grafana", secret: "s3cr3t"},
		{name: "client that is not a dex-authenticator", id: "kubernetes", secret: "s3cr3t"},
		{name: "no credentials", noAuth: true},
		{name: "broken encoding", id: "console", secret: "%zz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest("GET", Path, nil)
			if !tt.noAuth {
				r.SetBasicAuth(tt.id, tt.secret)
			}
			c, ok, err := authenticate(t.Context(), reader, r)
			if err != nil {
				t.Fatal(err)
			}
			if ok != tt.expected {
				t.Errorf("authenticated = %v, want %v", ok, tt.expected)
			}
			if ok && len(c.salt) != sha256.Size {
				t.Errorf("salt = %x", c.salt)
			}
		})
	}
}
