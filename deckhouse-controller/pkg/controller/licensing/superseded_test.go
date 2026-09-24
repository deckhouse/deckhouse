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
	"strings"
	"testing"
	"time"

	equality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

const (
	newPackageID = "5f0a7c31-9b42-4e8d-a016-3c7e2d5f9014"
	newRecordID  = "1a7f93b2-0c58-4d61-8e30-9f4a6b2c8d17"
)

// reissue builds the pair of keys a reissue produces: the old one, and a new one
// whose record supersedes it.
func reissue(t *testing.T, start string, supersedes []string) (*v1alpha1.ClusterLicense, *v1alpha1.ClusterLicense, ed25519.PublicKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate vendor key: %v", err)
	}

	old := platformRecord(testRecordID, "2026-01-01T00:00:00Z", "2026-07-01T00:00:00Z", fullLimits(10, 100, 0))
	fresh := platformRecord(newRecordID, start, "2027-01-01T00:00:00Z", fullLimits(12, 200, 0))
	fresh["supersedes"] = supersedes

	return &v1alpha1.ClusterLicense{
			ObjectMeta: metav1.ObjectMeta{Name: "key-old"},
			Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: signPackage(t, priv, testPackageID, old)},
		}, &v1alpha1.ClusterLicense{
			ObjectMeta: metav1.ObjectMeta{Name: "key-new"},
			Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: signPackage(t, priv, newPackageID, fresh)},
		}, pub
}

func exists(t *testing.T, env *testEnv, name string) bool {
	t.Helper()

	var license v1alpha1.ClusterLicense
	err := env.cl.Get(context.Background(), types.NamespacedName{Name: name}, &license)
	if err == nil {
		return true
	}
	if !apierrors.IsNotFound(err) {
		t.Fatalf("get cluster license %s: %v", name, err)
	}
	return false
}

// R1, R2, R10: a reissue extinguishes the old key, the controller deletes it and
// says so, and the cluster data file then names only the new key.
func TestSupersededKeyIsDeleted(t *testing.T) {
	old, fresh, vendorKey := reissue(t, "2026-04-01T00:00:00Z", []string{testRecordID})
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, old, fresh, &worker, discoverySecret())
	if _, err := env.r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if exists(t, env, "key-old") {
		t.Fatal("the superseded key was not deleted")
	}
	if !exists(t, env, "key-new") {
		t.Fatal("the key in force was deleted")
	}

	event := nextEvent(t, env)
	if !strings.Contains(event, eventKeySuperseded) ||
		!strings.Contains(event, testPackageID) || !strings.Contains(event, newPackageID) {
		t.Fatalf("event = %q, want a %s naming both keys", event, eventKeySuperseded)
	}

	claims := requestClaims(t, publishedRequest(t, env))
	keys := claims["active_keys"].([]any)
	if len(keys) != 1 || keys[0] != newPackageID {
		t.Fatalf("active_keys = %v, want only the new key", keys)
	}
	records := claims["records"].([]any)
	if len(records) != 1 || records[0] != newRecordID {
		t.Fatalf("records = %v, want only the record of the new key", records)
	}

	status := effectiveStatusOf(t, env)
	if status.Key == nil || status.Key.Jti != newPackageID {
		t.Fatalf("key = %+v, want the new one", status.Key)
	}
}

// R4: a reissue that has not started yet extinguishes nothing, so both keys stay.
func TestNotYetValidSuccessorDeletesNothing(t *testing.T) {
	old, fresh, vendorKey := reissue(t, "2026-09-01T00:00:00Z", []string{testRecordID})
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, old, fresh, &worker, discoverySecret())
	if _, err := env.r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if !exists(t, env, "key-old") || !exists(t, env, "key-new") {
		t.Fatal("a key was deleted although the successor is not in force yet")
	}
}

// R5: a successor nobody can verify extinguishes nothing.
func TestRejectedSuccessorDeletesNothing(t *testing.T) {
	old, fresh, vendorKey := reissue(t, "2026-04-01T00:00:00Z", []string{testRecordID})
	fresh.Spec.LicenseKey = corrupt(fresh.Spec.LicenseKey)
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, old, fresh, &worker, discoverySecret())
	if _, err := env.r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if !exists(t, env, "key-old") {
		t.Fatal("the old key was deleted although the successor does not verify")
	}
	if !exists(t, env, "key-new") {
		t.Fatal("a rejected key was deleted; the customer has to be able to read the reason")
	}

	var rejected v1alpha1.ClusterLicense
	if err := env.cl.Get(context.Background(), types.NamespacedName{Name: "key-new"}, &rejected); err != nil {
		t.Fatalf("get rejected key: %v", err)
	}
	if rejected.Status.Accepted || rejected.Status.Message == "" {
		t.Fatalf("rejected key status = %+v", rejected.Status)
	}
	if !effectiveStatusHasCondition(t, env, conditionKeyRejected) {
		t.Fatalf("condition %s is not True with a rejected key", conditionKeyRejected)
	}
}

// R6: the last key that expired, even past its grace, is the only record that
// the licence existed and when it ran out, so it is not deleted: the cluster has
// to keep reading Violation/Expired, not Unregistered.
func TestExpiredKeyWithoutSuccessorSurvives(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate vendor key: %v", err)
	}
	expired := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "key-old"},
		Spec: v1alpha1.ClusterLicenseSpec{LicenseKey: signPackage(t, priv, testPackageID,
			platformRecord(testRecordID, "2026-01-01T00:00:00Z", "2026-02-01T00:00:00Z", fullLimits(10, 100, 0)))},
	}
	worker := node("worker", "4", true)

	env := newTestEnv(t, pub, expired, &worker, discoverySecret())
	if _, err := env.r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if !exists(t, env, "key-old") {
		t.Fatal("an expired key without a successor was deleted")
	}
	status := effectiveStatusOf(t, env)
	if status.Compliance.State != v1alpha1.LicenseComplianceViolation || status.Licensed {
		t.Fatalf("compliance = %+v, licensed = %v", status.Compliance, status.Licensed)
	}
	// The expired key still tells the customer when the licence ran out.
	if status.Key == nil || status.Key.ValidUntil == nil {
		t.Fatalf("key = %+v, want the expired one with its date", status.Key)
	}
}

// R11: reinstalling a superseded key by hand gets it accepted, superseded and
// deleted again, without the policy moving.
func TestReinstalledSupersededKeyIsDeletedAgain(t *testing.T) {
	old, fresh, vendorKey := reissue(t, "2026-04-01T00:00:00Z", []string{testRecordID})
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, fresh, &worker, discoverySecret())
	ctx := context.Background()
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	before := effectiveStatusOf(t, env)

	old.ResourceVersion = ""
	if err := env.cl.Create(ctx, old); err != nil {
		t.Fatalf("reinstall the old key: %v", err)
	}
	env.at(testNow.Add(time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if exists(t, env, "key-old") {
		t.Fatal("the reinstalled key was not deleted again")
	}
	after := effectiveStatusOf(t, env)
	if before.RegistrationRequest != after.RegistrationRequest {
		t.Fatal("reinstalling a superseded key changed the cluster data file")
	}
}

// expiredPair builds a key that ran out on 2026-03-01, past the default grace at
// testNow, next to an unrelated key in force: the license server adds a key on
// every reissue instead of superseding the old one.
func expiredPair(t *testing.T, oldExpire string) (*v1alpha1.ClusterLicense, *v1alpha1.ClusterLicense, ed25519.PublicKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate vendor key: %v", err)
	}
	old := platformRecord(testRecordID, "2026-01-01T00:00:00Z", oldExpire, fullLimits(10, 100, 0))
	fresh := platformRecord(newRecordID, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", fullLimits(12, 200, 0))

	return &v1alpha1.ClusterLicense{
			ObjectMeta: metav1.ObjectMeta{Name: "key-old"},
			Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: signPackage(t, priv, testPackageID, old)},
		}, &v1alpha1.ClusterLicense{
			ObjectMeta: metav1.ObjectMeta{Name: "key-new"},
			Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: signPackage(t, priv, newPackageID, fresh)},
		}, pub
}

// A key whose every record ran out past its grace is deleted, the controller
// says so, and the cluster data file names only the key that is left.
func TestExpiredKeyIsDeleted(t *testing.T) {
	old, fresh, vendorKey := expiredPair(t, "2026-03-01T00:00:00Z")
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, old, fresh, &worker, discoverySecret())
	if _, err := env.r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if exists(t, env, "key-old") {
		t.Fatal("the expired key was not deleted")
	}
	if !exists(t, env, "key-new") {
		t.Fatal("the key in force was deleted")
	}

	event := nextEvent(t, env)
	if !strings.Contains(event, eventKeyExpired) ||
		!strings.Contains(event, testPackageID) || !strings.Contains(event, "2026-03-01T00:00:00Z") {
		t.Fatalf("event = %q, want a %s naming the key and its expiry", event, eventKeyExpired)
	}

	claims := requestClaims(t, publishedRequest(t, env))
	keys := claims["active_keys"].([]any)
	if len(keys) != 1 || keys[0] != newPackageID {
		t.Fatalf("active_keys = %v, want only the new key", keys)
	}
	records := claims["records"].([]any)
	if len(records) != 1 || records[0] != newRecordID {
		t.Fatalf("records = %v, want only the record of the new key", records)
	}
}

// A key in its grace period still counts, so it is not deleted.
func TestExpiredKeyInGraceIsKept(t *testing.T) {
	old, fresh, vendorKey := expiredPair(t, "2026-04-25T00:00:00Z")
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, old, fresh, &worker, discoverySecret())
	if _, err := env.r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if !exists(t, env, "key-old") || !exists(t, env, "key-new") {
		t.Fatal("a key in its grace period was deleted")
	}
}

// Reapplying an expired key by hand gets it deleted again, without the cluster
// data file moving.
func TestReinstalledExpiredKeyIsDeletedAgain(t *testing.T) {
	old, fresh, vendorKey := expiredPair(t, "2026-03-01T00:00:00Z")
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, fresh, &worker, discoverySecret())
	ctx := context.Background()
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	before := effectiveStatusOf(t, env)

	old.ResourceVersion = ""
	if err := env.cl.Create(ctx, old); err != nil {
		t.Fatalf("reinstall the expired key: %v", err)
	}
	env.at(testNow.Add(time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if exists(t, env, "key-old") {
		t.Fatal("the reinstalled expired key was not deleted again")
	}
	after := effectiveStatusOf(t, env)
	if before.RegistrationRequest != after.RegistrationRequest {
		t.Fatal("reinstalling an expired key changed the cluster data file")
	}
}

// A reissue under the new issuing rule: the new key carries the record of the
// old one verbatim plus a record starting later. The old key sorts first, so
// only coverage keeps its copy from being the accepted one. It is deleted with a
// KeyCovered event, the cluster data file names only the new key, and the policy
// is the one the old key alone granted.
func TestCoveredKeyIsDeleted(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate vendor key: %v", err)
	}
	carried := platformRecord(testRecordID, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", fullLimits(10, 100, 0))
	later := platformRecord(newRecordID, "2027-01-01T00:00:00Z", "2028-01-01T00:00:00Z", fullLimits(12, 200, 0))
	old := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "key-1"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: signPackage(t, priv, testPackageID, carried)},
	}
	fresh := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "key-2"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: signPackage(t, priv, newPackageID, carried, later)},
	}
	worker := node("worker", "4", true)
	ctx := context.Background()

	env := newTestEnv(t, pub, old, &worker, discoverySecret())
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	alone := effectiveStatusOf(t, env)

	if err := env.cl.Create(ctx, fresh); err != nil {
		t.Fatalf("install the new key: %v", err)
	}
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	if exists(t, env, "key-1") {
		t.Fatal("the covered key was not deleted")
	}
	if !exists(t, env, "key-2") {
		t.Fatal("the covering key was deleted")
	}

	event := nextEvent(t, env)
	if !strings.Contains(event, eventKeyCovered) || !strings.Contains(event, "key-2") ||
		!strings.Contains(event, testPackageID) || !strings.Contains(event, newPackageID) {
		t.Fatalf("event = %q, want a %s naming both keys", event, eventKeyCovered)
	}

	var covering v1alpha1.ClusterLicense
	if err := env.cl.Get(ctx, types.NamespacedName{Name: "key-2"}, &covering); err != nil {
		t.Fatalf("get covering key: %v", err)
	}
	if len(covering.Status.Records) != 2 || covering.Status.Records[0].ID != testRecordID || !covering.Status.Records[0].Accepted {
		t.Fatalf("covering key records = %+v, want the carried record accepted", covering.Status.Records)
	}

	claims := requestClaims(t, publishedRequest(t, env))
	keys := claims["active_keys"].([]any)
	if len(keys) != 1 || keys[0] != newPackageID {
		t.Fatalf("active_keys = %v, want only the covering key", keys)
	}

	both := effectiveStatusOf(t, env)
	if !equality.Semantic.DeepEqual(alone.Limits, both.Limits) || !equality.Semantic.DeepEqual(alone.Compliance, both.Compliance) {
		t.Fatalf("policy changed: %+v/%+v -> %+v/%+v", alone.Limits, alone.Compliance, both.Limits, both.Compliance)
	}
	if both.Key == nil || both.Key.Jti != newPackageID {
		t.Fatalf("key = %+v, want the covering one", both.Key)
	}

	// With the covered key gone the next pass changes nothing, so the deletion
	// did not flap the cluster data file.
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("third reconcile: %v", err)
	}
	after := effectiveStatusOf(t, env)
	if both.RegistrationRequest != after.RegistrationRequest ||
		!equality.Semantic.DeepEqual(both.Limits, after.Limits) || !equality.Semantic.DeepEqual(both.Key, after.Key) {
		t.Fatal("deleting the covered key changed the effective licence")
	}
}

// corrupt flips one byte of the payload so the signature no longer verifies.
func corrupt(token string) string {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 || parts[1] == "" {
		return token
	}
	body := []byte(parts[1])
	if body[0] == 'e' {
		body[0] = 'f'
	} else {
		body[0] = 'e'
	}
	return parts[0] + "." + string(body) + "." + parts[2]
}

func nextEvent(t *testing.T, env *testEnv) string {
	t.Helper()

	select {
	case e := <-env.events.Events:
		return e
	default:
		t.Fatal("no event was recorded")
		return ""
	}
}

func effectiveStatusHasCondition(t *testing.T, env *testEnv, conditionType string) bool {
	t.Helper()

	for _, c := range effectiveStatusOf(t, env).Conditions {
		if c.Type == conditionType {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}
