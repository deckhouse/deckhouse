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

package k8s

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func passwordObject(email, username string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "dex.coreos.com/v1",
		"kind":       "Password",
		"metadata":   map[string]any{"name": "nrxwgylmfuydaocamrsxqllfgjss45dfon2mx4u44scceizf", "namespace": "d8-user-authn"},
		"email":      email,
		"hash":       "secret",
		"userID":     "dex-e2e-local-008",
		"username":   username,
	}}
}

// client-go may run the transform of an informer again on an object it already trimmed. The second
// run must keep the name of the UserAccount: computed from the username, it names an account that does
// not exist, and the devices of the user read as none.
func TestTrimPasswordIsIdempotent(t *testing.T) {
	t.Parallel()

	once, err := trimPassword(passwordObject("local-008@dex-e2e.test", "dex-e2e-local-008"))
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	twice, err := trimPassword(once)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	want := LocalAccountName("local-008@dex-e2e.test", "dex-e2e-local-008")
	for name, obj := range map[string]any{"first run": once, "second run": twice} {
		if got := passwordField(obj, accountNameField); got != want {
			t.Errorf("%s: account name %q, want %q", name, got, want)
		}
		if got := passwordField(obj, "email"); got != "" {
			t.Errorf("%s: the email stays in the cache: %q", name, got)
		}
		if got := passwordField(obj, "hash"); got != "" {
			t.Errorf("%s: the hash stays in the cache", name)
		}
	}
}

// A Password without an email is named after its username, the first run and every later one alike.
func TestTrimPasswordWithoutEmail(t *testing.T) {
	t.Parallel()

	once, err := trimPassword(passwordObject("", "jdoe"))
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	twice, err := trimPassword(once)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	want := LocalAccountName("", "jdoe")
	if got := passwordField(twice, accountNameField); got != want {
		t.Errorf("account name %q, want %q", got, want)
	}
}
