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
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
)

// signOneRecord issues a single record Workload package with the given vendor
// key, so that two keys of the same test share an issuer.
func signOneRecord(t *testing.T, priv ed25519.PrivateKey, jti, recordID, expireAt string) string {
	t.Helper()

	token, err := licensing.Sign(map[string]any{"typ": licensing.TypLicense}, map[string]any{
		"ver":           licensing.SchemaVersion,
		"iss":           licensing.Issuer,
		"sub":           testClusterID,
		"jti":           jti,
		"iat":           "2026-01-01T00:00:00Z",
		"customer_name": "Acme",
		"licenses": []any{map[string]any{
			"type":       licensing.TypeWorkload,
			"id":         recordID,
			"start_at":   "2026-01-01T00:00:00Z",
			"expire_at":  expireAt,
			"cluster_id": testClusterID,
			"origin":     "purchase",
			"workload": map[string]any{
				"dkp": map[string]any{"edition": "EE", "resource_limits": map[string]any{"vCPU": 50}},
			},
		}},
	}, priv)
	if err != nil {
		t.Fatalf("sign package: %v", err)
	}
	return token
}

// The key whose only record has expired can be deleted; the key still in force
// cannot. The customer reads the verdict off the object, not off a CLI.
func TestReconcileMarksRetirableKeys(t *testing.T) {
	const (
		expiredPackageID = "2b7f93b2-0c58-4d61-8e30-9f4a6b2c8d17"
		expiredRecordID  = "1a7f93b2-0c58-4d61-8e30-9f4a6b2c8d17"
	)

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate vendor key: %v", err)
	}

	live := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "live"},
		Spec: v1alpha1.ClusterLicenseSpec{
			LicenseKey: signOneRecord(t, priv, testPackageID, testRecordID, "2027-01-01T00:00:00Z"),
		},
	}
	// testNow is 2026-05-01, so this one is two months past its expiry.
	spent := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "spent"},
		Spec: v1alpha1.ClusterLicenseSpec{
			LicenseKey: signOneRecord(t, priv, expiredPackageID, expiredRecordID, "2026-03-01T00:00:00Z"),
		},
	}
	worker := node("worker", "4", true)

	r, cl := newTestReconciler(t, pub, live, spent, &worker, discoverySecret())

	ctx := context.Background()
	if _, err := r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	cases := []struct {
		name       string
		retirable  bool
		wantStatus metav1.ConditionStatus
		wantReason string
	}{
		{"spent", true, metav1.ConditionTrue, "Retirable"},
		{"live", false, metav1.ConditionFalse, "InUse"},
	}
	for _, tc := range cases {
		var key v1alpha1.ClusterLicense
		if err := cl.Get(ctx, types.NamespacedName{Name: tc.name}, &key); err != nil {
			t.Fatalf("get cluster license %s: %v", tc.name, err)
		}
		if key.Status.Retirable != tc.retirable {
			t.Fatalf("%s: retirable = %v, want %v (records: %+v)", tc.name, key.Status.Retirable, tc.retirable, key.Status.Records)
		}

		cond := meta.FindStatusCondition(key.Status.Conditions, "Retirable")
		if cond == nil {
			t.Fatalf("%s: no Retirable condition in %+v", tc.name, key.Status.Conditions)
		}
		if cond.Status != tc.wantStatus || cond.Reason != tc.wantReason {
			t.Fatalf("%s: condition = %s/%s, want %s/%s", tc.name, cond.Status, cond.Reason, tc.wantStatus, tc.wantReason)
		}
		if cond.Message == "" {
			t.Fatalf("%s: condition carries no message", tc.name)
		}
	}

	var effective v1alpha1.EffectiveLicense
	if err := cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &effective); err != nil {
		t.Fatalf("get effective license: %v", err)
	}
	if got := effective.Status.Counts.Retirable; got != 1 {
		t.Fatalf("counts.retirable = %d, want 1 (counts: %+v)", got, effective.Status.Counts)
	}
}
