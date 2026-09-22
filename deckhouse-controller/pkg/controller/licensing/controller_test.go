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

// fullLimits is what the license server issues: all three metrics, the ones the
// customer did not buy as an explicit zero.
func fullLimits(servers, vcpu, cores int) map[string]any {
	return map[string]any{
		licensing.MetricServers: servers,
		licensing.MetricVCPU:    vcpu,
		licensing.MetricCores:   cores,
	}
}

// issueTestPackage signs a one-record Platform package with a throwaway vendor
// key, so that the test never depends on the keys shipped with the build.
func issueTestPackage(t *testing.T, limits map[string]any) (string, ed25519.PublicKey) {
	t.Helper()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate vendor key: %v", err)
	}
	return signPackage(t, priv, testPackageID, platformRecord(testRecordID, "2026-01-01T00:00:00Z", "2027-01-01T00:00:00Z", limits)), pub
}

// platformRecord builds one Platform record of the test cluster.
func platformRecord(id, start, expire string, limits map[string]any) map[string]any {
	return map[string]any{
		"type":       licensing.TypePlatform,
		"id":         id,
		"start_at":   start,
		"expire_at":  expire,
		"cluster_id": testClusterID,
		"origin":     "self-service",
		"platform":   map[string]any{"dkp": map[string]any{"edition": "EE", "resource_limits": limits}},
	}
}

func signPackage(t *testing.T, priv ed25519.PrivateKey, jti string, records ...map[string]any) string {
	t.Helper()

	licenses := make([]any, 0, len(records))
	for _, r := range records {
		licenses = append(licenses, r)
	}
	token, err := licensing.Sign(map[string]any{"typ": licensing.TypLicense}, map[string]any{
		"ver":           licensing.SchemaVersion,
		"iss":           licensing.Issuer,
		"sub":           testClusterID,
		"jti":           jti,
		"iat":           "2026-01-01T00:00:00Z",
		"customer_name": "Acme",
		"licenses":      licenses,
	}, priv)
	if err != nil {
		t.Fatalf("sign package: %v", err)
	}
	return token
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
	token, vendorKey := issueTestPackage(t, fullLimits(1, 8, 0))

	license := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: token},
	}
	big := node("worker-big", "16", true)
	small := node("worker-small", "8", true)
	master := node("master", "8", true, taint(taintControlPlane))

	r, cl := newTestReconciler(t, vendorKey, license, &big, &small, &master, discoverySecret())

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

	if !status.Licensed || status.Compliance.State != v1alpha1.LicenseComplianceValid {
		t.Fatalf("licensed/state = %v/%q", status.Licensed, status.Compliance.State)
	}
	if got := status.Limits.Values[licensing.MetricServers]; got == nil || *got != 1 {
		t.Fatalf("servers limit = %v, want 1", derefOrNil(got))
	}

	// The tainted master is free, so the consumption is the two workers.
	want := v1alpha1.LicenseConsumption{Servers: 2, VCPU: 24, Cores: 12, FreeNodes: 1}
	if status.Consumption != want {
		t.Fatalf("consumption = %+v, want %+v", status.Consumption, want)
	}

	// One server licence for the larger worker, eight vCPU of pool for the other.
	if status.Allocation.Servers.Used != 1 || status.Allocation.Pool.Nodes != 1 ||
		status.Allocation.Unlicensed.Nodes != 0 {
		t.Fatalf("allocation = %+v", status.Allocation)
	}
	if status.OverLimitSince != nil {
		t.Fatalf("overLimitSince = %v, want null while every node is covered", status.OverLimitSince)
	}
	if !meta.IsStatusConditionTrue(status.Conditions, conditionLimitsSatisfied) {
		t.Fatalf("condition %s is not True: %+v", conditionLimitsSatisfied, status.Conditions)
	}

	assertNodes(t, status.Nodes, map[string]v1alpha1.LicenseNodeBilling{
		"worker-big":   v1alpha1.LicenseNodeServer,
		"worker-small": v1alpha1.LicenseNodePool,
		"master":       v1alpha1.LicenseNodeFree,
	})
	if status.Key == nil || status.Key.Jti != testPackageID || status.Key.CustomerName != "Acme" {
		t.Fatalf("key = %+v", status.Key)
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
		ClusterID  string           `json:"cluster_id"`
		Records    []string         `json:"records"`
		ActiveKeys []string         `json:"active_keys"`
		Seq        uint64           `json:"seq"`
		Metrics    map[string]int64 `json:"metrics"`
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
	if len(payload.ActiveKeys) != 1 || payload.ActiveKeys[0] != testPackageID {
		t.Fatalf("active_keys = %v, want [%s]", payload.ActiveKeys, testPackageID)
	}
	// D9: the payload carries numbers, never node names.
	if got := string(request.Payload); containsAny(got, "worker-big", "worker-small", "master") {
		t.Fatalf("the registration payload names nodes: %s", got)
	}
	if payload.Metrics[licensing.MetricVCPU] != 24 || payload.Metrics[licensing.MetricCores] != 12 {
		t.Fatalf("metrics = %v", payload.Metrics)
	}

	var stored v1alpha1.ClusterLicense
	if err := cl.Get(ctx, types.NamespacedName{Name: "primary"}, &stored); err != nil {
		t.Fatalf("get cluster license: %v", err)
	}
	if !stored.Status.Accepted || stored.Status.Superseded {
		t.Fatalf("cluster license status = %+v", stored.Status)
	}
	if len(stored.Status.Records) != 1 || !stored.Status.Records[0].Accepted {
		t.Fatalf("records = %+v, want one accepted record", stored.Status.Records)
	}
	if stored.Status.PackageJti != testPackageID {
		t.Fatalf("packageJti = %q, want %q", stored.Status.PackageJti, testPackageID)
	}

	assertMatchesCRD(t, cl, &effective, v1alpha1.EffectiveLicenseKind)
	assertMatchesCRD(t, cl, &stored, v1alpha1.ClusterLicenseKind)
}

// A recompute of an unchanged cluster must not rewrite anything.
func TestReconcileIsIdempotent(t *testing.T) {
	token, vendorKey := issueTestPackage(t, fullLimits(10, 100, 0))

	license := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: token},
	}
	worker := node("worker", "4", true)

	env := newTestEnv(t, vendorKey, license, &worker, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	var first v1alpha1.EffectiveLicense
	if err := env.cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &first); err != nil {
		t.Fatalf("get effective license: %v", err)
	}

	env.at(testNow.Add(10 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	var second v1alpha1.EffectiveLicense
	if err := env.cl.Get(ctx, types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &second); err != nil {
		t.Fatalf("get effective license: %v", err)
	}

	if first.ResourceVersion != second.ResourceVersion {
		t.Fatalf("effective license was rewritten: %s -> %s", first.ResourceVersion, second.ResourceVersion)
	}
	if !equality.Semantic.DeepEqual(first.Status, second.Status) {
		t.Fatalf("status changed ten minutes later:\n%+v\n%+v", first.Status, second.Status)
	}
}

// S7 to S9 through the controller: an allocation of equally sized nodes stays
// put, because the previous one is read back off the published status.
func TestAllocationIsStableAcrossReconciles(t *testing.T) {
	// One server licence and no pool: exactly one of the two equal nodes wins.
	token, vendorKey := issueTestPackage(t, fullLimits(1, 0, 0))
	license := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: token},
	}
	a := node("a", "16", true)
	b := node("b", "16", true)

	env := newTestEnv(t, vendorKey, license, &a, &b, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	// S6: with no history the lower name wins.
	if got := serverNodes(t, env); len(got) != 1 || got[0] != "a" {
		t.Fatalf("servers = %v, want [a]", got)
	}

	// S8: a larger node takes the licence over.
	c := node("c", "32", true)
	if err := env.cl.Create(ctx, &c); err != nil {
		t.Fatalf("create node c: %v", err)
	}
	env.at(testNow.Add(time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if got := serverNodes(t, env); len(got) != 1 || got[0] != "c" {
		t.Fatalf("servers = %v, want [c]", got)
	}

	// S9: the server node disappears and the licence falls back to the survivors.
	if err := env.cl.Delete(ctx, &c); err != nil {
		t.Fatalf("delete node c: %v", err)
	}
	env.at(testNow.Add(2 * time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("third reconcile: %v", err)
	}
	if got := serverNodes(t, env); len(got) != 1 || got[0] != "a" {
		t.Fatalf("servers = %v, want the previous holder [a]", got)
	}
}

// A7: unlicensed nodes warn and stamp the moment they appeared.
func TestUnlicensedNodesWarnAndStampTheStatus(t *testing.T) {
	token, vendorKey := issueTestPackage(t, fullLimits(1, 0, 0))
	license := &v1alpha1.ClusterLicense{
		ObjectMeta: metav1.ObjectMeta{Name: "primary"},
		Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: token},
	}
	big := node("big", "32", true)
	small := node("small", "8", true)

	env := newTestEnv(t, vendorKey, license, &big, &small, discoverySecret())
	ctx := context.Background()

	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	status := effectiveStatusOf(t, env)

	if status.Compliance.State != v1alpha1.LicenseComplianceWarning ||
		status.Compliance.Reason != licensing.ReasonUnlicensedNodes {
		t.Fatalf("compliance = %+v", status.Compliance)
	}
	if !status.Licensed {
		t.Fatal("a warning must still count as licensed")
	}
	if status.OverLimitSince == nil || !status.OverLimitSince.Time.Equal(testNow) {
		t.Fatalf("overLimitSince = %v, want %s", status.OverLimitSince, testNow)
	}
	if status.Allocation.Unlicensed.Nodes != 1 || status.Allocation.Unlicensed.VCPU != 8 {
		t.Fatalf("unlicensed = %+v", status.Allocation.Unlicensed)
	}
	if meta.IsStatusConditionTrue(status.Conditions, conditionLimitsSatisfied) {
		t.Fatalf("condition %s is True with an unlicensed node", conditionLimitsSatisfied)
	}

	// A9: seven days on, the same overuse is a violation, and the stamp does not
	// move in between.
	env.at(testNow.Add(7 * 24 * time.Hour))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("reconcile a week later: %v", err)
	}
	later := effectiveStatusOf(t, env)
	if later.Compliance.State != v1alpha1.LicenseComplianceViolation || later.Licensed {
		t.Fatalf("compliance = %+v, licensed = %v", later.Compliance, later.Licensed)
	}
	if !later.OverLimitSince.Time.Equal(testNow) {
		t.Fatalf("overLimitSince moved to %v", later.OverLimitSince)
	}

	// A10: the node goes away and the violation ends on the first recompute.
	if err := env.cl.Delete(ctx, &small); err != nil {
		t.Fatalf("delete node: %v", err)
	}
	env.at(testNow.Add(7*24*time.Hour + time.Minute))
	if _, err := env.r.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("reconcile after the node left: %v", err)
	}
	recovered := effectiveStatusOf(t, env)
	if recovered.Compliance.State != v1alpha1.LicenseComplianceValid || recovered.OverLimitSince != nil {
		t.Fatalf("compliance = %+v, overLimitSince = %v", recovered.Compliance, recovered.OverLimitSince)
	}
}

// A12: without a key at all the cluster is Unregistered, every node is
// unlicensed, and the request it publishes declares its identity inline so that
// the customer can obtain a first key.
func TestReconcileWithoutKeys(t *testing.T) {
	_, vendorKey := issueTestPackage(t, fullLimits(10, 100, 0))
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
	status := effective.Status
	if status.Licensed || status.Compliance.Reason != licensing.ReasonUnregistered {
		t.Fatalf("licensed = %v, reason = %q", status.Licensed, status.Compliance.Reason)
	}
	if status.Key != nil {
		t.Fatalf("key = %+v, want none", status.Key)
	}
	if status.Allocation.Unlicensed.Nodes != 1 {
		t.Fatalf("allocation = %+v, want the worker unlicensed", status.Allocation)
	}

	request, err := licensing.Parse(status.RegistrationRequest)
	if err != nil {
		t.Fatalf("parse registration request: %v", err)
	}
	if _, ok := request.Header["jwk"]; !ok {
		t.Fatalf("registration request header = %v, want an inline jwk", request.Header)
	}

	assertMatchesCRD(t, cl, &effective, v1alpha1.EffectiveLicenseKind)
}

// S14: three hundred nodes land in the status and the object still validates.
func TestReconcileWithThreeHundredNodes(t *testing.T) {
	token, vendorKey := issueTestPackage(t, fullLimits(12, 200, 0))
	objects := []client.Object{
		&v1alpha1.ClusterLicense{
			ObjectMeta: metav1.ObjectMeta{Name: "primary"},
			Spec:       v1alpha1.ClusterLicenseSpec{LicenseKey: token},
		},
		discoverySecret(),
	}
	for i := range 300 {
		n := node(nodeName(i), "8", true)
		objects = append(objects, &n)
	}

	env := newTestEnv(t, vendorKey, objects...)
	if _, err := env.r.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var effective v1alpha1.EffectiveLicense
	if err := env.cl.Get(context.Background(), types.NamespacedName{Name: v1alpha1.EffectiveLicenseName}, &effective); err != nil {
		t.Fatalf("get effective license: %v", err)
	}
	if len(effective.Status.Nodes) != 300 {
		t.Fatalf("status lists %d nodes, want 300", len(effective.Status.Nodes))
	}
	assertMatchesCRD(t, env.cl, &effective, v1alpha1.EffectiveLicenseKind)
}

// The node predicate is the debounce of specification 10.6: only a change the
// policy actually reads wakes the controller.
func TestNodePredicate(t *testing.T) {
	base := node("worker", "4", true)

	unchanged := base
	if (nodeChanged{}).Update(updateOf(&base, &unchanged)) {
		t.Fatal("an identical node woke the controller")
	}

	heartbeat := base
	heartbeat.Status.Conditions[0].LastHeartbeatTime = metav1.NewTime(testNow)
	if (nodeChanged{}).Update(updateOf(&base, &heartbeat)) {
		t.Fatal("a kubelet heartbeat woke the controller")
	}

	resized := node("worker", "8", true)
	if !(nodeChanged{}).Update(updateOf(&base, &resized)) {
		t.Fatal("a change of capacity did not wake the controller")
	}

	tainted := node("worker", "4", true, taint(taintControlPlane))
	if !(nodeChanged{}).Update(updateOf(&base, &tainted)) {
		t.Fatal("a change of taints did not wake the controller")
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
