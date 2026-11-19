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
	"encoding/base64"
	"testing"
	"time"

	"user-authn-controller/internal/controller"
)

func TestReconcile_CreatePasswordFromMaskedCache(t *testing.T) {
	t.Parallel()

	const encodedHash = "JDJ5JDEwJGRNWGVGUVBkdUdYYVMyWDFPcGdZdk9HSy81LkdsNm5sdU9mUkhnNWlQdDhuSlh6SzhpeS5H"
	hash, err := base64.StdEncoding.DecodeString(encodedHash)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	r := newTestReconciler(t, now,
		namespaceObj(), userObj(adminUserName, adminEmail, controller.MaskedValue, ""))
	r.apiReader = newFakeClientBuilder(t, userObj(adminUserName, adminEmail, string(hash), "")).Build()

	if _, err := r.Reconcile(t.Context(), userReq(adminUserName)); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	pw := getPassword(t, r, adminEncodedPW)
	decoded, err := base64.StdEncoding.DecodeString(strField(pw, "hash"))
	if err != nil {
		t.Fatalf("decode Password hash: %v", err)
	}
	if string(decoded) != string(hash) {
		t.Errorf("Password hash = %q, want the unmasked API value", decoded)
	}
}
