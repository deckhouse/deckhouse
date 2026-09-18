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
	"encoding/json"
	"errors"
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

	var effective v1alpha1.EffectiveLicense
	if err := env.cl.Get(context.Background(), types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &effective); err != nil {
		t.Fatalf("get effective license: %v", err)
	}
	return effective.Status.RegistrationRequest
}

func storedJournal(t *testing.T, env *testEnv) *licensing.Journal {
	t.Helper()

	var cm corev1.ConfigMap
	if err := env.cl.Get(context.Background(), env.r.journalKey(), &cm); err != nil {
		t.Fatalf("get journal: %v", err)
	}
	journal := new(licensing.Journal)
	if err := json.Unmarshal([]byte(cm.Data[journalField]), journal); err != nil {
		t.Fatalf("decode journal: %v", err)
	}
	return journal
}

func licenseObject(t *testing.T) (*v1alpha1.ClusterLicense, ed25519.PublicKey) {
	t.Helper()

	token, vendorKey := issueTestPackage(t, map[string]any{"vCPU": 50})
	return &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: token},
	}, vendorKey
}

// D4 at the controller level: every issued request is a new one, and the counter
// the license server reads only ever grows.
func TestConsecutiveSamplesAdvanceSeqAndJTI(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, license, &worker, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	first := requestClaims(t, publishedRequest(t, env))

	env.at(testNow.Add(time.Hour))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	second := requestClaims(t, publishedRequest(t, env))

	if first["jti"] == second["jti"] {
		t.Fatalf("jti = %v twice, want a fresh one per request", first["jti"])
	}
	if first["seq"].(float64) >= second["seq"].(float64) {
		t.Fatalf("seq = %v then %v, want it to grow", first["seq"], second["seq"])
	}
	if got := storedJournal(t, env).Seq; float64(got) != second["seq"].(float64) {
		t.Fatalf("persisted seq = %d, want the seq of the published request %v", got, second["seq"])
	}
}

// A request that has to be rebuilt outside a sampling tick is still a new
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

	// Ten minutes later no sample is due, but the request has to come back.
	env.at(testNow.Add(10 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	second := requestClaims(t, publishedRequest(t, env))
	if second["seq"].(float64) != first["seq"].(float64)+1 {
		t.Fatalf("seq = %v, want %v + 1", second["seq"], first["seq"])
	}

	journal := storedJournal(t, env)
	if float64(journal.Seq) != second["seq"].(float64) {
		t.Fatalf("persisted seq = %d, want %v", journal.Seq, second["seq"])
	}
	if len(journal.Samples) != 1 {
		t.Fatalf("journal holds %d samples, want the single one of the first tick", len(journal.Samples))
	}
}

// An unreadable pod list must leave the window alone: no sample, no counter
// move, and an error for controller-runtime to back off on.
func TestReconcileFailsWhenPodsCannotBeListed(t *testing.T) {
	license, vendorKey := licenseObject(t)
	worker := node("worker", "4", true)
	master := node("master", "8", true, taint("node-role.kubernetes.io/control-plane"))

	env := newTestEnv(t, vendorKey, license, &worker, &master, discoverySecret())
	env.pods.err = errors.New("apiserver is unhappy")

	ctx := context.Background()
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err == nil {
		t.Fatal("reconcile succeeded although the pods of a reserved node could not be listed")
	}

	var cm corev1.ConfigMap
	err := env.cl.Get(ctx, env.r.journalKey(), &cm)
	if err == nil {
		t.Fatalf("a journal was written from a failed observation: %q", cm.Data[journalField])
	}
}

// A journal nobody can decode restarts the window and keeps the broken bytes
// for whoever investigates. The counter survives whenever it can still be read:
// a seq that went backwards is the one part the license server cannot shrug off.
func TestCorruptJournalIsSalvagedAndParked(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantSeq uint64
	}{
		{
			// Valid JSON, wrong shape: seq is still there to be read.
			name:    "a journal with a broken samples field keeps its counter",
			payload: `{"seq":42,"samples":{"what":"is this"}}`,
			wantSeq: 43,
		},
		{
			// Nothing can be read out of this one, so the counter restarts and
			// the customer may have to register once more.
			name:    "an unparseable journal restarts the counter",
			payload: `{"seq": 42, "samples": [ this is not json`,
			wantSeq: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			license, vendorKey := licenseObject(t)
			worker := node("worker", "4", true)

			journalCM := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: journalConfigMapName, Namespace: app.NamespaceDeckhouse},
				Data:       map[string]string{journalField: tc.payload},
			}

			env := newTestEnv(t, vendorKey, license, &worker, discoverySecret(), journalCM)
			ctx := context.Background()

			if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			var cm corev1.ConfigMap
			if err := env.cl.Get(ctx, env.r.journalKey(), &cm); err != nil {
				t.Fatalf("get journal: %v", err)
			}
			if cm.Data[journalCorruptField] != tc.payload {
				t.Fatalf("journal.corrupt = %q, want the original payload", cm.Data[journalCorruptField])
			}

			journal := storedJournal(t, env)
			if journal.Seq != tc.wantSeq {
				t.Fatalf("seq = %d, want %d", journal.Seq, tc.wantSeq)
			}
			if len(journal.Samples) != 1 {
				t.Fatalf("journal holds %d samples, want the window restarted with one", len(journal.Samples))
			}
			if claims := requestClaims(t, publishedRequest(t, env)); claims["seq"].(float64) != float64(tc.wantSeq) {
				t.Fatalf("request seq = %v, want %d", claims["seq"], tc.wantSeq)
			}
		})
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
