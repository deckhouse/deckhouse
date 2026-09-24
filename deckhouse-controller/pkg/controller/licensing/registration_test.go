// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package licensing

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// requestClaims decodes the payload of a published registration request.
func requestClaims(t *testing.T, token string) map[string]any {
	t.Helper()

	parsed, err := licensing.Parse(token)
	if err != nil {
		t.Fatalf("parse registration request: %v", err)
	}
	claims := map[string]any{}
	if err := json.Unmarshal(parsed.Payload, &claims); err != nil {
		t.Fatalf("decode registration payload: %v", err)
	}
	return claims
}

func publishedRequest(t *testing.T, env *testEnv) string {
	t.Helper()
	return effectiveStatusOf(t, env).RegistrationRequest
}

func headerOf(t *testing.T, token string) map[string]any {
	t.Helper()
	tok, err := licensing.Parse(token)
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}
	return tok.Header
}

func storedSeq(t *testing.T, env *testEnv) uint64 {
	t.Helper()

	var cm corev1.ConfigMap
	if err := env.cl.Get(context.Background(), env.r.seqKey(), &cm); err != nil {
		t.Fatalf("get registration counter: %v", err)
	}
	seq, err := strconv.ParseUint(cm.Data[seqField], 10, 64)
	if err != nil {
		t.Fatalf("parse stored seq %q: %v", cm.Data[seqField], err)
	}
	return seq
}

func licenseObject(t *testing.T) (*v1alpha1.ClusterLicense, ed25519.PublicKey) {
	t.Helper()

	token, vendorKey := issueTestPackage(t, fullLimits(10, 100, 0))
	return &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: token},
	}, vendorKey
}

// Installing or removing a key must rebuild the cluster data file at once: the
// license server reads the installed set out of it, and a customer reissuing
// right after installing a key must not be handed a stale file.
func TestRequestFollowsInstalledSet(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, &worker, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	// D1: no key at all.
	unregistered := requestClaims(t, publishedRequest(t, env))
	if got := unregistered["records"].([]any); len(got) != 0 {
		t.Fatalf("records before any key = %v, want none", got)
	}
	if got := unregistered["active_keys"].([]any); len(got) != 0 {
		t.Fatalf("active_keys before any key = %v, want none", got)
	}
	if _, hasJWK := headerOf(t, publishedRequest(t, env))["jwk"]; !hasJWK {
		t.Fatal("request before any key must carry jwk")
	}

	// D2: a key is installed, so the file is rebuilt with the record id, the key
	// jti and a kid header.
	if err := env.cl.Create(ctx, license); err != nil {
		t.Fatalf("create license: %v", err)
	}
	env.at(testNow.Add(10 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	registered := requestClaims(t, publishedRequest(t, env))
	// D4: consecutive requests differ in jti and grow in seq.
	if registered["seq"].(float64) != unregistered["seq"].(float64)+1 {
		t.Fatalf("seq = %v, want %v + 1", registered["seq"], unregistered["seq"])
	}
	if registered["jti"] == unregistered["jti"] {
		t.Fatalf("jti = %v twice, want a fresh one per request", registered["jti"])
	}
	if got := registered["records"].([]any); len(got) != 1 {
		t.Fatalf("records after install = %v, want one id", got)
	}
	if got := registered["active_keys"].([]any); len(got) != 1 || got[0] != testPackageID {
		t.Fatalf("active_keys after install = %v, want [%s]", got, testPackageID)
	}
	if _, hasKID := headerOf(t, publishedRequest(t, env))["kid"]; !hasKID {
		t.Fatal("request after install must carry kid")
	}

	// Nothing changed: the same file stays put, and so does the counter.
	env.at(testNow.Add(20 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("third reconcile: %v", err)
	}
	if again := requestClaims(t, publishedRequest(t, env)); again["jti"] != registered["jti"] {
		t.Fatal("request was rebuilt although nothing changed")
	}
	if got := storedSeq(t, env); float64(got) != registered["seq"].(float64) {
		t.Fatalf("stored seq = %d, want %v", got, registered["seq"])
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

// The consumption is part of the cluster data file, so a node joining rebuilds
// it: a file the license server would price against last month's cluster is
// worse than no file.
func TestRequestFollowsConsumption(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, license, &worker, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	before := requestClaims(t, publishedRequest(t, env))
	if got := before["metrics"].(map[string]any)[licensing.MetricVCPU]; got != float64(4) {
		t.Fatalf("vCPU = %v, want 4", got)
	}

	second := node("worker-2", "8", true)
	if err := env.cl.Create(ctx, &second); err != nil {
		t.Fatalf("create node: %v", err)
	}
	env.at(testNow.Add(time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	after := requestClaims(t, publishedRequest(t, env))
	metrics := after["metrics"].(map[string]any)
	if metrics[licensing.MetricVCPU] != float64(12) || metrics[licensing.MetricServers] != float64(2) {
		t.Fatalf("metrics = %v, want 2 servers and 12 vCPU", metrics)
	}
	if after["seq"].(float64) != before["seq"].(float64)+1 {
		t.Fatalf("seq = %v, want %v + 1", after["seq"], before["seq"])
	}
}

// A request that has to be rebuilt because the status was wiped is still a new
// request, so it carries the next seq and that seq is persisted.
func TestLostRequestIsReissuedWithANewSeq(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, license, &worker, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	first := requestClaims(t, publishedRequest(t, env))

	// Somebody wiped the status; nothing else changed.
	var effective v1alpha1.EffectiveLicense
	if err := env.cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &effective); err != nil {
		t.Fatalf("get effective license: %v", err)
	}
	effective.Status.RegistrationRequest = ""
	if err := env.cl.Status().Update(ctx, &effective); err != nil {
		t.Fatalf("wipe registration request: %v", err)
	}

	env.at(testNow.Add(10 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	second := requestClaims(t, publishedRequest(t, env))
	if second["seq"].(float64) != first["seq"].(float64)+1 {
		t.Fatalf("seq = %v, want %v + 1", second["seq"], first["seq"])
	}
	if got := storedSeq(t, env); float64(got) != second["seq"].(float64) {
		t.Fatalf("stored seq = %d, want %v", got, second["seq"])
	}
}

// The counter outlives the status: it is the one piece of state a license server
// cannot forgive going backwards.
func TestSeqSurvivesAWipedStatus(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)

	seed := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: seqConfigMapName, Namespace: app.NamespaceDeckhouse},
		Data:       map[string]string{seqField: "41"},
	}

	env := newTestEnv(t, vendorKey, license, &worker, discoverySecret(), seed)
	if _, err := env.r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if got := requestClaims(t, publishedRequest(t, env))["seq"].(float64); got != 42 {
		t.Fatalf("seq = %v, want 42", got)
	}
}

// N12: an unreadable pod list aborts the reconcile, leaves the status alone and
// does not move the counter.
func TestReconcileFailsWhenPodsCannotBeListed(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)
	master := node("master", "8", true, taint(taintControlPlane))

	env := newTestEnv(t, vendorKey, license, &worker, &master, discoverySecret())
	env.pods.err = errors.New("apiserver is unhappy")

	ctx := context.Background()
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err == nil {
		t.Fatal("reconcile succeeded although the pods of a reserved node could not be listed")
	}

	var cm corev1.ConfigMap
	if err := env.cl.Get(ctx, env.r.seqKey(), &cm); err == nil {
		t.Fatalf("a counter was written from a failed observation: %q", cm.Data[seqField])
	}
	var effective v1alpha1.EffectiveLicense
	if err := env.cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &effective); err == nil {
		t.Fatalf("a status was published from a failed observation: %+v", effective.Status)
	}
}

// A cluster key of the wrong size is a broken identity, not an invitation to
// mint a new one: minting would invalidate every key already issued.
func TestMalformedClusterKeyIsNotReplaced(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: keySecretName, Namespace: app.NamespaceDeckhouse},
		Data:       map[string][]byte{keySecretField: []byte("too short")},
	}

	env := newTestEnv(t, vendorKey, license, &worker, discoverySecret(), secret)
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err == nil {
		t.Fatal("reconcile succeeded on a malformed cluster key")
	}

	var stored corev1.Secret
	key := types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: keySecretName}
	if err := env.cl.Get(ctx, key, &stored); err != nil {
		t.Fatalf("get cluster key secret: %v", err)
	}
	if string(stored.Data[keySecretField]) != "too short" {
		t.Fatalf("seed = %q, want the malformed one left untouched", stored.Data[keySecretField])
	}
	if stored.ResourceVersion != secret.ResourceVersion {
		t.Fatalf("secret was rewritten: %s -> %s", secret.ResourceVersion, stored.ResourceVersion)
	}
}

// requeueAfter picks the soonest moment the policy can change on its own.
// TestRequestRefreshesBeforeTheAntiReplayWindow covers D10/D11: a cluster whose
// fleet and keys never change still republishes the file before the license
// server starts rejecting it.
func TestRequestRefreshesBeforeTheAntiReplayWindow(t *testing.T) {
	for _, tc := range []struct {
		name    string
		elapsed time.Duration
		rebuilt bool
	}{
		{name: "23h keeps the published request", elapsed: 23 * time.Hour},
		{name: "25h rebuilds it", elapsed: 25 * time.Hour, rebuilt: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			license, vendorKey := licenseObject(t)
			worker := node("worker", "4", true)

			env := newTestEnv(t, vendorKey, license, &worker, discoverySecret())
			ctx := context.Background()

			if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
				t.Fatalf("first reconcile: %v", err)
			}
			before := requestClaims(t, publishedRequest(t, env))

			env.at(testNow.Add(tc.elapsed))
			if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
				t.Fatalf("second reconcile: %v", err)
			}
			after := requestClaims(t, publishedRequest(t, env))

			if !tc.rebuilt {
				if after["jti"] != before["jti"] || after["iat"] != before["iat"] || after["seq"] != before["seq"] {
					t.Fatalf("request rebuilt after %s on identical inputs: %v -> %v", tc.elapsed, before, after)
				}
				return
			}
			if after["jti"] == before["jti"] {
				t.Fatalf("jti = %v twice, want a fresh one per request", after["jti"])
			}
			if after["iat"] == before["iat"] {
				t.Fatalf("iat = %v twice, want the time of the rebuild", after["iat"])
			}
			if after["seq"].(float64) != before["seq"].(float64)+1 {
				t.Fatalf("seq = %v, want %v + 1", after["seq"], before["seq"])
			}
			if got := storedSeq(t, env); float64(got) != after["seq"].(float64) {
				t.Fatalf("stored seq = %d, want %v", got, after["seq"])
			}
		})
	}
}

func TestRequeueAfter(t *testing.T) {
	th := licensing.DefaultThresholds()
	noRequest := time.Time{}
	expire := testNow.Add(30 * time.Minute)
	res := licensing.Result{Records: []licensing.RecordStatus{{
		Record: licensing.Record{StartAt: testNow.Add(-time.Hour), ExpireAt: &expire},
	}}}

	if got := requeueAfter(res, th, noRequest, testNow); got != 30*time.Minute {
		t.Fatalf("requeueAfter = %s, want the record expiry in 30m", got)
	}

	// Anything further out than the resync period is the resync period.
	far := testNow.Add(3 * time.Hour)
	if got := requeueAfter(licensing.Result{Records: []licensing.RecordStatus{{
		Record: licensing.Record{StartAt: testNow.Add(-time.Hour), ExpireAt: &far},
	}}}, th, noRequest, testNow); got != resyncPeriod {
		t.Fatalf("requeueAfter = %s, want the resync period", got)
	}

	since := testNow.Add(-7*24*time.Hour + 30*time.Minute)
	over := licensing.Result{OverLimitSince: &since}
	if got := requeueAfter(over, th, noRequest, testNow); got != 30*time.Minute {
		t.Fatalf("requeueAfter = %s, want the end of the over-limit window in 30m", got)
	}

	// Nothing ahead at all still resyncs, and a breakpoint on top of us does not
	// turn into a hot loop.
	if got := requeueAfter(licensing.Result{}, th, noRequest, testNow); got != resyncPeriod {
		t.Fatalf("requeueAfter = %s, want the resync period", got)
	}
	now := expire.Add(-time.Second)
	if got := requeueAfter(res, th, noRequest, now); got != time.Minute {
		t.Fatalf("requeueAfter = %s, want the one minute floor", got)
	}

	// A record that has already expired still wakes the controller at the end of
	// its grace: that is when Grace turns into Violation and the key may go.
	grace := 1
	expired := testNow.Add(-24*time.Hour + 30*time.Minute)
	inGrace := licensing.Result{Records: []licensing.RecordStatus{{
		Record: licensing.Record{StartAt: testNow.Add(-48 * time.Hour), ExpireAt: &expired, GraceDays: &grace},
	}}}
	if got := requeueAfter(inGrace, th, noRequest, testNow); got != 30*time.Minute {
		t.Fatalf("requeueAfter = %s, want the end of grace in 30m", got)
	}

	issued := testNow.Add(-requestMaxAge + 30*time.Minute)
	if got := requeueAfter(licensing.Result{}, th, issued, testNow); got != 30*time.Minute {
		t.Fatalf("requeueAfter = %s, want the request refresh in 30m", got)
	}
}

// A request signed by a key the cluster no longer holds is worthless to the
// license server: with a jwk header it would even pin the wrong identity. After
// the cluster key is regenerated the published request has to follow at once,
// not on the 24 hour refresh.
func TestRequestFollowsARegeneratedClusterKey(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, license, &worker, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	before := publishedRequest(t, env)

	var secret corev1.Secret
	key := types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: keySecretName}
	if err := env.cl.Get(ctx, key, &secret); err != nil {
		t.Fatalf("get cluster key secret: %v", err)
	}
	if err := env.cl.Delete(ctx, &secret); err != nil {
		t.Fatalf("delete cluster key secret: %v", err)
	}

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	after := publishedRequest(t, env)
	if after == before {
		t.Fatal("published request unchanged after the cluster key was regenerated")
	}

	var regenerated corev1.Secret
	if err := env.cl.Get(ctx, key, &regenerated); err != nil {
		t.Fatalf("get regenerated cluster key secret: %v", err)
	}
	pub := ed25519.NewKeyFromSeed(regenerated.Data[keySecretField]).Public().(ed25519.PublicKey)
	header := headerOf(t, after)
	if kid, ok := header["kid"]; ok {
		if kid != licensing.Thumbprint(pub) {
			t.Fatalf("request kid = %v, want the thumbprint of the regenerated key", kid)
		}
		return
	}
	jwk, _ := header["jwk"].(map[string]any)
	if got := jwk["x"]; got != base64.RawURLEncoding.EncodeToString(pub) {
		t.Fatalf("request jwk.x = %v, want the regenerated key", got)
	}
}
