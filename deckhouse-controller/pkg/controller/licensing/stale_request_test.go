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

package licensing

import (
	"context"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// Installing or removing a key must reissue the registration request at once:
// the license server reads the installed set from it, and a customer topping
// up right after installing a key must not be shown an hour old request.
func TestRequestFollowsInstalledSet(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, &worker, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	unregistered := requestClaims(t, publishedRequest(t, env))
	if got := unregistered["records"].([]any); len(got) != 0 {
		t.Fatalf("records before any key = %v, want none", got)
	}
	if _, hasJWK := headerOf(t, publishedRequest(t, env))["jwk"]; !hasJWK {
		t.Fatal("request before any key must carry jwk")
	}

	// A key is installed ten minutes later: no sample is due, the request must
	// still be reissued with the record id and a kid header.
	if err := env.cl.Create(ctx, license); err != nil {
		t.Fatalf("create license: %v", err)
	}
	env.at(testNow.Add(10 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	registered := requestClaims(t, publishedRequest(t, env))
	if registered["seq"].(float64) != unregistered["seq"].(float64)+1 {
		t.Fatalf("seq = %v, want %v + 1", registered["seq"], unregistered["seq"])
	}
	if got := registered["records"].([]any); len(got) != 1 {
		t.Fatalf("records after install = %v, want one id", got)
	}
	if _, hasKID := headerOf(t, publishedRequest(t, env))["kid"]; !hasKID {
		t.Fatal("request after install must carry kid")
	}

	// Nothing changed: the same request stays put.
	env.at(testNow.Add(20 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("third reconcile: %v", err)
	}
	if again := requestClaims(t, publishedRequest(t, env)); again["jti"] != registered["jti"] {
		t.Fatal("request was reissued although the installed set did not change")
	}

	// The key is removed: back to an unregistered request.
	if err := env.cl.Delete(ctx, license); err != nil {
		t.Fatalf("delete license: %v", err)
	}
	env.at(testNow.Add(30 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("fourth reconcile: %v", err)
	}
	if got := requestClaims(t, publishedRequest(t, env))["records"].([]any); len(got) != 0 {
		t.Fatalf("records after removal = %v, want none", got)
	}
	if _, hasJWK := headerOf(t, publishedRequest(t, env))["jwk"]; !hasJWK {
		t.Fatal("request after removal must carry jwk again")
	}
}

func headerOf(t *testing.T, token string) map[string]any {
	t.Helper()
	tok, err := licensing.Parse(token)
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}
	return tok.Header
}
