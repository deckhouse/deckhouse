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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	equality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/app"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/licensing"
	"github.com/deckhouse/deckhouse/testing/controller/testclient"
)

const (
	testClusterID = "7f3c1a94-2b6e-4d51-9c08-a5e7d2f81b30"
	testRecordID  = "c0d100e0-4ed0-4da4-9742-a1b2c3d4e5f6"
	testPackageID = "9c4e12ab-7f30-4d88-b512-6a0e3d7c9f21"
)

var testNow = time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

// issueTestPackage signs a one-record Platform package with a throwaway vendor
// key, so that the test never depends on the keys shipped with the build.
func issueTestPackage(t *testing.T, limits map[string]any) (string, ed25519.PublicKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate vendor key: %v", err)
	}

	token, err := licensing.Sign(map[string]any{"typ": licensing.TypLicense}, map[string]any{
		"ver":           licensing.SchemaVersion,
		"iss":           licensing.Issuer,
		"sub":           testClusterID,
		"jti":           testPackageID,
		"iat":           "2026-01-01T00:00:00Z",
		"customer_name": "Acme",
		"licenses": []any{map[string]any{
			"type":       licensing.TypePlatform,
			"id":         testRecordID,
			"start_at":   "2026-01-01T00:00:00Z",
			"expire_at":  "2027-01-01T00:00:00Z",
			"cluster_id": testClusterID,
			"origin":     "purchase",
			"platform":   map[string]any{"dkp": map[string]any{"edition": "EE", "resource_limits": limits}},
		}},
	}, priv)
	if err != nil {
		t.Fatalf("sign package: %v", err)
	}
	return token, pub
}

func newTestReconciler(t *testing.T, vendorKey ed25519.PublicKey, objects ...client.Object) (*reconciler, *testclient.Client) {
	t.Helper()

	env := newTestEnv(t, vendorKey, objects...)
	return env.r, env.cl
}

func discoverySecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: app.SecretDiscovery, Namespace: app.NamespaceDeckhouse},
		Data:       map[string][]byte{"clusterUUID": []byte(testClusterID)},
	}
}

func TestReconcilePublishesPolicy(t *testing.T) {
	token, vendorKey := issueTestPackage(t, map[string]any{"vCPU": 50, "nodes": 10})

	license := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: token, Alias: "Acme"},
	}
	worker := node("worker", "4", true)
	master := node("master", "8", true, taint("node-role.kubernetes.io/control-plane"))

	r, cl := newTestReconciler(t, vendorKey, license, &worker, &master, discoverySecret())

	ctx := context.Background()
	res, err := r.Reconcile(ctx, ctrl.Request{})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter <= 0 {
		t.Fatalf("requeueAfter = %s, want a positive delay", res.RequeueAfter)
	}

	// The cluster identity is created on first use and never leaves the secret.
	var keySecret corev1.Secret
	keyName := types.NamespacedName{Namespace: app.NamespaceDeckhouse, Name: keySecretName}
	if err := cl.Get(ctx, keyName, &keySecret); err != nil {
		t.Fatalf("get cluster key secret: %v", err)
	}
	if len(keySecret.Data[keySecretField]) != ed25519.SeedSize {
		t.Fatalf("seed is %d bytes, want %d", len(keySecret.Data[keySecretField]), ed25519.SeedSize)
	}

	var effective v1alpha1.EffectiveLicense
	if err := cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &effective); err != nil {
		t.Fatalf("get effective license: %v", err)
	}
	status := effective.Status

	if got := status.Effective.Limits["vCPU"]; got == nil || *got != 50 {
		t.Fatalf("effective vCPU limit = %v, want 50", derefOrNil(got))
	}
	if got := status.Effective.Limits["nodes"]; got == nil || *got != 10 {
		t.Fatalf("effective nodes limit = %v, want 10", derefOrNil(got))
	}
	// Only the untainted worker is counted, so four cores on one node.
	if got := status.Metrics["vCPU"].Instant; got != 4 {
		t.Fatalf("vCPU instant = %v, want 4", got)
	}
	if got := status.Metrics["nodes"].Instant; got != 1 {
		t.Fatalf("nodes instant = %v, want 1", got)
	}
	if status.Compliance.State != v1alpha1.LicenseComplianceValid {
		t.Fatalf("compliance state = %q, want %q", status.Compliance.State, v1alpha1.LicenseComplianceValid)
	}
	if !status.WithinLimits {
		t.Fatal("withinLimits = false, want true")
	}
	if status.Compliance.Since == nil {
		t.Fatal("compliance.since is not set")
	}
	if !meta.IsStatusConditionTrue(status.Conditions, conditionRegistered) {
		t.Fatalf("condition %s is not True: %+v", conditionRegistered, status.Conditions)
	}
	if !meta.IsStatusConditionTrue(status.Conditions, conditionLimitsSatisfied) {
		t.Fatalf("condition %s is not True: %+v", conditionLimitsSatisfied, status.Conditions)
	}

	// The request references the key by thumbprint: a record is accepted, so the
	// license server already knows the identity.
	request, err := licensing.Parse(status.RegistrationRequest)
	if err != nil {
		t.Fatalf("parse registration request: %v", err)
	}
	if _, ok := request.Header["kid"]; !ok {
		t.Fatalf("registration request header = %v, want a kid", request.Header)
	}
	if _, ok := request.Header["jwk"]; ok {
		t.Fatalf("registration request carries a jwk although a record is accepted")
	}

	var payload struct {
		ClusterID string   `json:"cluster_id"`
		Records   []string `json:"records"`
		Seq       uint64   `json:"seq"`
		Metrics   map[string]struct {
			Instant float64 `json:"instant"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(request.Payload, &payload); err != nil {
		t.Fatalf("decode registration payload: %v", err)
	}
	if payload.ClusterID != testClusterID {
		t.Fatalf("cluster_id = %q, want %q", payload.ClusterID, testClusterID)
	}
	if len(payload.Records) != 1 || payload.Records[0] != testRecordID {
		t.Fatalf("records = %v, want [%s]", payload.Records, testRecordID)
	}
	if payload.Metrics["vCPU"].Instant != 4 {
		t.Fatalf("payload vCPU instant = %v, want 4", payload.Metrics["vCPU"].Instant)
	}

	var stored v1alpha1.ClusterLicense
	if err := cl.Get(ctx, types.NamespacedName{Name: "primary"}, &stored); err != nil {
		t.Fatalf("get cluster license: %v", err)
	}
	if !stored.Status.Accepted {
		t.Fatalf("cluster license is not accepted: %q", stored.Status.Message)
	}
	if len(stored.Status.Records) != 1 || !stored.Status.Records[0].Accepted {
		t.Fatalf("records = %+v, want one accepted record", stored.Status.Records)
	}
	if stored.Status.PackageJti != testPackageID {
		t.Fatalf("packageJti = %q, want %q", stored.Status.PackageJti, testPackageID)
	}

	assertMatchesCRD(t, cl, &effective, v1alpha1.EffectiveLicenseKind)
	assertMatchesCRD(t, cl, &stored, v1alpha1.ClusterLicenseKind)

	// The journal holds exactly one observation and the counter it was signed with.
	var journalCM corev1.ConfigMap
	if err := cl.Get(ctx, r.journalKey(), &journalCM); err != nil {
		t.Fatalf("get journal: %v", err)
	}
	stored2 := new(licensing.Journal)
	if err := json.Unmarshal([]byte(journalCM.Data[journalField]), stored2); err != nil {
		t.Fatalf("decode journal: %v", err)
	}
	if len(stored2.Samples) != 1 || stored2.Seq != payload.Seq {
		t.Fatalf("journal = %+v, want one sample and seq %d", stored2, payload.Seq)
	}
}

// A second reconcile in the same minute must not take a second sample and must
// not rewrite a status that did not change.
func TestReconcileIsIdempotent(t *testing.T) {
	token, vendorKey := issueTestPackage(t, map[string]any{"vCPU": 50})

	license := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: token},
	}
	worker := node("worker", "4", true)

	r, cl := newTestReconciler(t, vendorKey, license, &worker, discoverySecret())

	ctx := context.Background()
	if _, err := r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}

	var first v1alpha1.EffectiveLicense
	if err := cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &first); err != nil {
		t.Fatalf("get effective license: %v", err)
	}

	if _, err := r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	var second v1alpha1.EffectiveLicense
	if err := cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &second); err != nil {
		t.Fatalf("get effective license: %v", err)
	}

	if first.ResourceVersion != second.ResourceVersion {
		t.Fatalf("effective license was rewritten: %s -> %s", first.ResourceVersion, second.ResourceVersion)
	}

	var journalCM corev1.ConfigMap
	if err := cl.Get(ctx, r.journalKey(), &journalCM); err != nil {
		t.Fatalf("get journal: %v", err)
	}
	stored := new(licensing.Journal)
	if err := json.Unmarshal([]byte(journalCM.Data[journalField]), stored); err != nil {
		t.Fatalf("decode journal: %v", err)
	}
	if len(stored.Samples) != 1 {
		t.Fatalf("journal holds %d samples, want 1", len(stored.Samples))
	}

	// With a second observation in the window the consumption views become a
	// least squares fit, which drifts in the last bits of the mantissa as the
	// clock moves. Ten minutes later, on unchanged consumption, nothing may be
	// rewritten.
	stored.Samples = append([]licensing.Sample{{
		At:     testNow.Add(-30 * time.Minute),
		Values: stored.Samples[0].Values,
	}}, stored.Samples...)
	raw, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("marshal journal: %v", err)
	}
	journalCM.Data[journalField] = string(raw)
	if err := cl.Update(ctx, &journalCM); err != nil {
		t.Fatalf("seed a second sample: %v", err)
	}

	r.now = func() time.Time { return testNow.Add(10 * time.Minute) }
	if _, err := r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("third reconcile: %v", err)
	}

	var third v1alpha1.EffectiveLicense
	if err := cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &third); err != nil {
		t.Fatalf("get effective license: %v", err)
	}
	if !equality.Semantic.DeepEqual(second.Status.Metrics, third.Status.Metrics) {
		t.Fatalf("consumption moved on an unchanged window:\n%+v\n%+v", second.Status.Metrics, third.Status.Metrics)
	}
	// licensing.timeline anchors the current segment at "now", so that one field
	// moves on every reconcile whatever the consumption did. Everything else has
	// to stand still.
	settledStatus := *third.Status.DeepCopy()
	if len(settledStatus.Timeline) > 0 && len(second.Status.Timeline) > 0 {
		settledStatus.Timeline[0].From = second.Status.Timeline[0].From
	}
	if !equality.Semantic.DeepEqual(second.Status, settledStatus) {
		t.Fatalf("effective license changed ten minutes later:\n%+v\n%+v", second.Status, settledStatus)
	}

	var settled corev1.ConfigMap
	if err := cl.Get(ctx, r.journalKey(), &settled); err != nil {
		t.Fatalf("get journal: %v", err)
	}
	if settled.ResourceVersion != journalCM.ResourceVersion {
		t.Fatalf("journal was rewritten without a sample being due: %s -> %s", journalCM.ResourceVersion, settled.ResourceVersion)
	}
}

// Without a key at all the cluster is Unregistered, and the request it publishes
// declares its identity inline so that the customer can obtain a first key.
func TestReconcileWithoutKeys(t *testing.T) {
	_, vendorKey := issueTestPackage(t, map[string]any{"vCPU": 50})
	worker := node("worker", "4", true)

	r, cl := newTestReconciler(t, vendorKey, &worker, discoverySecret())

	ctx := context.Background()
	if _, err := r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var effective v1alpha1.EffectiveLicense
	if err := cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &effective); err != nil {
		t.Fatalf("get effective license: %v", err)
	}
	if effective.Status.Compliance.Reason != licensing.ReasonUnregistered {
		t.Fatalf("compliance reason = %q, want %q", effective.Status.Compliance.Reason, licensing.ReasonUnregistered)
	}
	if meta.IsStatusConditionTrue(effective.Status.Conditions, conditionRegistered) {
		t.Fatalf("condition %s is True without any key", conditionRegistered)
	}

	request, err := licensing.Parse(effective.Status.RegistrationRequest)
	if err != nil {
		t.Fatalf("parse registration request: %v", err)
	}
	if _, ok := request.Header["jwk"]; !ok {
		t.Fatalf("registration request header = %v, want an inline jwk", request.Header)
	}
}

// assertMatchesCRD runs the published object through the schema validator built
// from the shipped CRD: the status is an API surface, and a field the schema
// rejects would be rejected by a real API server too.
func assertMatchesCRD(t *testing.T, cl *testclient.Client, obj client.Object, kind string) {
	t.Helper()

	obj.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "deckhouse.io",
		Version: "v1alpha1",
		Kind:    kind,
	})

	result := cl.Validator().Validate(obj)
	if result == nil {
		t.Fatalf("no schema validator for %s", kind)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("%s does not satisfy its CRD schema: %v", kind, result.Errors)
	}
}

func derefOrNil(v *int64) any {
	if v == nil {
		return nil
	}
	return *v
}
