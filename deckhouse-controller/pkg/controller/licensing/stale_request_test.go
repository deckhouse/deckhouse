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
	"encoding/json"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"

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

// forgeJournal rewrites the stored journal so that its latest observation
// claims a consumption nobody ever measured.
func forgeJournal(t *testing.T, env *testEnv, values map[string]float64) {
	t.Helper()

	ctx := context.Background()
	var cm corev1.ConfigMap
	if err := env.cl.Get(ctx, env.r.journalKey(), &cm); err != nil {
		t.Fatalf("get journal: %v", err)
	}
	journal := new(licensing.Journal)
	if err := json.Unmarshal([]byte(cm.Data[journalField]), journal); err != nil {
		t.Fatalf("decode journal: %v", err)
	}
	if len(journal.Samples) == 0 {
		t.Fatal("journal holds no samples to forge")
	}
	journal.Samples[len(journal.Samples)-1].Values = values

	raw, err := json.Marshal(journal)
	if err != nil {
		t.Fatalf("encode journal: %v", err)
	}
	cm.Data[journalField] = string(raw)
	if err := env.cl.Update(ctx, &cm); err != nil {
		t.Fatalf("write forged journal: %v", err)
	}
}

func metricOf(t *testing.T, claims map[string]any, name, kind string) float64 {
	t.Helper()

	metrics, ok := claims["metrics"].(map[string]any)
	if !ok {
		t.Fatalf("request carries no metrics: %v", claims["metrics"])
	}
	value, ok := metrics[name].(map[string]any)
	if !ok {
		t.Fatalf("request carries no %q metric: %v", name, metrics)
	}
	number, ok := value[kind].(float64)
	if !ok {
		t.Fatalf("metric %q has no %q: %v", name, kind, value)
	}
	return number
}

// §7.2: the instant a request carries is always a live node count. A journal
// ConfigMap anyone in the cluster can write must not be a cheap way to
// understate consumption in a request issued between two sampling ticks.
func TestReissuedRequestCarriesALiveInstant(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, &worker, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	forgeJournal(t, env, map[string]float64{metricVCPU: 999, metricNodes: 99})

	// Ten minutes on no sample is due, but the installed set changed, so the
	// request has to be rebuilt.
	if err := env.cl.Create(ctx, license); err != nil {
		t.Fatalf("create license: %v", err)
	}
	env.at(testNow.Add(10 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	claims := requestClaims(t, publishedRequest(t, env))
	for name, want := range map[string]float64{metricVCPU: 4, metricNodes: 1} {
		if got := metricOf(t, claims, name, "instant"); got != want {
			t.Fatalf("instant %s = %v, want the live %v", name, got, want)
		}
	}
	// The window itself is untouched: only the instant is taken live.
	for name, want := range map[string]float64{metricVCPU: 999, metricNodes: 99} {
		if got := metricOf(t, claims, name, "avg_7d"); got != want {
			t.Fatalf("avg_7d %s = %v, want the journal %v", name, got, want)
		}
	}

	journal := storedJournal(t, env)
	if len(journal.Samples) != 1 {
		t.Fatalf("journal holds %d samples, want the live observation not appended", len(journal.Samples))
	}
	if got := journal.Samples[0].Values[metricNodes]; got != 99 {
		t.Fatalf("stored sample nodes = %v, want the forged 99 left alone", got)
	}
}

// The live observation of a reissue is held to the same rule as a sampling
// tick: if it cannot be taken, no request is issued and the counter stays put.
func TestReissueFailsWhenTheLiveObservationFails(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)
	master := node("master", "8", true, taint("node-role.kubernetes.io/control-plane"))

	env := newTestEnv(t, vendorKey, &worker, &master, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	first := publishedRequest(t, env)
	seq := storedJournal(t, env).Seq

	if err := env.cl.Create(ctx, license); err != nil {
		t.Fatalf("create license: %v", err)
	}
	env.pods.err = errors.New("apiserver is unhappy")
	env.at(testNow.Add(10 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err == nil {
		t.Fatal("reconcile succeeded although the live observation failed")
	}

	if got := publishedRequest(t, env); got != first {
		t.Fatal("a request was published from a failed live observation")
	}
	if got := storedJournal(t, env).Seq; got != seq {
		t.Fatalf("seq = %d, want it left at %d", got, seq)
	}
}
