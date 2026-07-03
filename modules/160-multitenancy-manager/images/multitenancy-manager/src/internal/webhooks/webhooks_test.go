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

package webhooks

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
)

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		corev1.AddToScheme, storagev1.AddToScheme, v1alpha1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func testMapper() meta.RESTMapper {
	m := meta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "", Version: "v1"},
		{Group: "storage.k8s.io", Version: "v1"},
	})
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}, meta.RESTScopeNamespace)
	m.Add(schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}, meta.RESTScopeRoot)
	return m
}

func projectNS(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func serve(t *testing.T, h http.Handler, path string, review admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	t.Helper()
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	out := &admissionv1.AdmissionReview{}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Response == nil {
		t.Fatal("nil response")
	}
	return out.Response
}

func raw(t *testing.T, obj map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func review(op admissionv1.Operation, gvr metav1.GroupVersionResource, gvk metav1.GroupVersionKind, ns, name string, newRaw, oldRaw []byte) admissionv1.AdmissionReview {
	return admissionv1.AdmissionReview{Request: &admissionv1.AdmissionRequest{
		UID:       "1",
		Namespace: ns,
		Name:      name,
		Operation: op,
		Resource:  gvr,
		Kind:      gvk,
		Object:    runtime.RawExtension{Raw: newRaw},
		OldObject: runtime.RawExtension{Raw: oldRaw},
	}}
}

var (
	svcGVR = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "services"}
	svcGVK = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}
	pvcGVR = metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"}
	pvcGVK = metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "PersistentVolumeClaim"}
)

// lbDef is a value-backed definition for loadBalancerClass with the given baseline availability.
func lbDef(defAvail v1alpha1.AvailabilityDefault) *v1alpha1.GrantableClusterResourceDefinition {
	return &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "loadbalancerclasses"},
		Spec:       v1alpha1.GrantableClusterResourceDefinitionSpec{DefaultAvailability: defAvail},
	}
}

// lbRef is the Service.spec.loadBalancerClass validation path with the given defaulting mode.
func lbRef(defaulting v1alpha1.DefaultingMode) *v1alpha1.GrantableClusterResourceReference {
	return &v1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: "loadbalancerclasses-service"},
		Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "loadbalancerclasses",
			Rule:                         v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}},
			FieldPaths: []v1alpha1.FieldPath{{
				Path:       "$.spec.loadBalancerClass",
				Match:      &v1alpha1.MatchPredicate{FieldPath: "$.spec.type", Equals: "LoadBalancer"},
				Defaulting: defaulting,
			}},
		},
	}
}

func lbGrant() *v1alpha1.ClusterResourceGrantPolicy {
	return &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "g"},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources: []v1alpha1.GrantResource{{
				ResourceName: "loadbalancerclasses",
				Allowed:      []string{"external", "internal"},
				Default:      "internal",
			}},
		},
	}
}

func lbService(class, typ string) []byte {
	b, _ := json.Marshal(map[string]any{
		"apiVersion": "v1", "kind": "Service",
		"metadata": map[string]any{"name": "s", "namespace": "proj"},
		"spec":     map[string]any{"type": typ, "loadBalancerClass": class},
	})
	return b
}

func isGranted(t *testing.T, objs ...client.Object) *IsGrantedValidator {
	return NewIsGrantedValidator(logr.Discard(), newClient(t, objs...), testMapper(), jsonpath.NewWithCache())
}

func TestIsGranted_ValueBacked_AllowDeny(t *testing.T) {
	v := isGranted(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityNone), lbRef(v1alpha1.DefaultingNone), lbGrant())

	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("external", "LoadBalancer"), nil)); !resp.Allowed {
		t.Fatalf("external should be allowed: %v", resp.Result)
	}
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("forbidden", "LoadBalancer"), nil)); resp.Allowed {
		t.Fatal("forbidden class must be denied under None default")
	}
	// match guard false (ClusterIP) → not governed → allow.
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("forbidden", "ClusterIP"), nil)); !resp.Allowed {
		t.Fatal("non-LoadBalancer service must be allowed (guard false)")
	}
}

func TestIsGranted_DefaultAll_AllowsUngranted(t *testing.T) {
	v := isGranted(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityAll), lbRef(v1alpha1.DefaultingNone))
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("anything", "LoadBalancer"), nil)); !resp.Allowed {
		t.Fatal("All default must allow ungranted value")
	}
}

func TestIsGranted_AllowListImpliesRestrict(t *testing.T) {
	v := isGranted(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityAll), lbRef(v1alpha1.DefaultingNone), lbGrant())
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("internal", "LoadBalancer"), nil)); !resp.Allowed {
		t.Fatal("allowed value must pass")
	}
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("anything", "LoadBalancer"), nil)); resp.Allowed {
		t.Fatal("value outside the allow-list must be denied (allow-list implies None baseline)")
	}
}

func TestIsGranted_NoGrant_NoneStillDenies(t *testing.T) {
	v := isGranted(t, projectNS("proj", nil), lbDef(v1alpha1.AvailabilityNone), lbRef(v1alpha1.DefaultingNone), lbGrant())
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("external", "LoadBalancer"), nil)); resp.Allowed {
		t.Fatal("None default must deny even with no matching grant")
	}
}

func TestIsGranted_UpdateGrandfathers(t *testing.T) {
	v := isGranted(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityNone), lbRef(v1alpha1.DefaultingNone), lbGrant())
	old := lbService("legacy", "LoadBalancer")
	if resp := serve(t, v, "/is-granted", review(admissionv1.Update, svcGVR, svcGVK, "proj", "s", lbService("legacy", "LoadBalancer"), old)); !resp.Allowed {
		t.Fatal("unchanged legacy value must be grandfathered on update")
	}
}

func TestIsGranted_SystemNamespaceBypass(t *testing.T) {
	v := isGranted(t, lbDef(v1alpha1.AvailabilityNone), lbRef(v1alpha1.DefaultingNone), lbGrant())
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "kube-system", "s", lbService("forbidden", "LoadBalancer"), nil)); !resp.Allowed {
		t.Fatal("system namespace must bypass")
	}
}

func TestIsGranted_SystemRequestBypass(t *testing.T) {
	// The deckhouse-controller SA (group system:serviceaccounts:d8-system) applies every module's Helm
	// release server-side; a denial here fails the install and addon-operator retries it forever,
	// deadlocking the module's queue. Such a request must ALWAYS pass, even for a value a user would be
	// denied (forbidden class under a None default).
	v := isGranted(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityNone), lbRef(v1alpha1.DefaultingNone), lbGrant())
	r := review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("forbidden", "LoadBalancer"), nil)
	r.Request.UserInfo.Groups = []string{"system:serviceaccounts:d8-system"}
	if resp := serve(t, v, "/is-granted", r); !resp.Allowed {
		t.Fatal("a system (d8-system) request must bypass the grant allow-list — never lock a module's Helm release")
	}
	// A plain user with the same forbidden value is still denied (fast, terminal — no retry, no lock).
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("forbidden", "LoadBalancer"), nil)); resp.Allowed {
		t.Fatal("a normal user must still be denied an ungranted value")
	}
	// system:masters is NOT an automated writer: a human cluster-admin stays subject to the guardrail.
	rm := review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("forbidden", "LoadBalancer"), nil)
	rm.Request.UserInfo.Groups = []string{"system:masters"}
	if resp := serve(t, v, "/is-granted", rm); resp.Allowed {
		t.Fatal("system:masters must remain subject to the grant guardrail (not an automated system writer)")
	}
}

func TestIsGranted_ManagedByNamespaceBypass(t *testing.T) {
	// An auto-wrapped (managed-by-namespace) project is a plain orphan namespace wrapped only for
	// accounting; it must behave like an ordinary namespace and NOT enforce the grant allow-list
	// (allowNamespacesWithoutProjects, card-16) — even an ungranted value from a normal user passes.
	ns := projectNS("proj", map[string]string{"env": "prod", "multitenancy.deckhouse.io/project-managed-by-namespace": "true"})
	v := isGranted(t, ns, lbDef(v1alpha1.AvailabilityNone), lbRef(v1alpha1.DefaultingNone), lbGrant())
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("forbidden", "LoadBalancer"), nil)); !resp.Allowed {
		t.Fatal("a managed-by-namespace (auto-wrapped) project must bypass the grant allow-list")
	}
}

func TestIsGranted_UnboundReferenceIgnored(t *testing.T) {
	// A reference whose definition does not exist enforces nothing.
	v := isGranted(t, projectNS("proj", map[string]string{"env": "prod"}), lbRef(v1alpha1.DefaultingNone), lbGrant())
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("forbidden", "LoadBalancer"), nil)); !resp.Allowed {
		t.Fatal("unbound reference must not enforce anything")
	}
}

func TestIsGranted_ObjectBackedSelector(t *testing.T) {
	def := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityNone,
		},
	}
	ref := &v1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses-pvc"},
		Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "storageclasses",
			Rule:                         v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
			FieldPaths:                   []v1alpha1.FieldPath{{Path: "$.spec.storageClassName"}},
		},
	}
	grant := &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "g"},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources: []v1alpha1.GrantResource{{
				ResourceName:    "storageclasses",
				AllowedSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"shared": "true"}},
			}},
		},
	}
	shared := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "ssd", Labels: map[string]string{"shared": "true"}}, Provisioner: "x"}
	priv := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "secret"}, Provisioner: "x"}
	v := isGranted(t, projectNS("proj", map[string]string{"env": "prod"}), def, ref, grant, shared, priv)

	pvc := func(sc string) []byte {
		return raw(t, map[string]any{"apiVersion": "v1", "kind": "PersistentVolumeClaim", "metadata": map[string]any{"name": "p", "namespace": "proj"}, "spec": map[string]any{"storageClassName": sc}})
	}
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, pvcGVR, pvcGVK, "proj", "p", pvc("ssd"), nil)); !resp.Allowed {
		t.Fatalf("ssd (shared) must be allowed: %v", resp.Result)
	}
	if resp := serve(t, v, "/is-granted", review(admissionv1.Create, pvcGVR, pvcGVK, "proj", "p", pvc("secret"), nil)); resp.Allowed {
		t.Fatal("secret (not shared) must be denied under None")
	}
}

func defaults(t *testing.T, objs ...client.Object) *DefaultsMutator {
	return NewDefaultsMutator(logr.Discard(), newClient(t, objs...), testMapper(), jsonpath.NewWithCache())
}

func TestDefaults_FillEmpty(t *testing.T) {
	m := defaults(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityAll), lbRef(v1alpha1.DefaultingFillEmpty), lbGrant())
	svc := raw(t, map[string]any{
		"apiVersion": "v1", "kind": "Service",
		"metadata": map[string]any{"name": "s", "namespace": "proj"},
		"spec":     map[string]any{"type": "LoadBalancer"},
	})
	resp := serve(t, m, "/defaults", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", svc, nil))
	if !resp.Allowed || len(resp.Patch) == 0 {
		t.Fatalf("expected a default patch, got allowed=%v patch=%s", resp.Allowed, resp.Patch)
	}
	var patches []map[string]any
	if err := json.Unmarshal(resp.Patch, &patches); err != nil {
		t.Fatal(err)
	}
	if len(patches) != 1 || patches[0]["path"] != "/spec/loadBalancerClass" || patches[0]["value"] != "internal" {
		t.Fatalf("unexpected patch: %v", patches)
	}
}

func TestDefaults_NoOverrideOnUpdate(t *testing.T) {
	m := defaults(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityAll), lbRef(v1alpha1.DefaultingFillEmpty), lbGrant())
	svc := raw(t, map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "s"}, "spec": map[string]any{"type": "LoadBalancer"}})
	if resp := serve(t, m, "/defaults", review(admissionv1.Update, svcGVR, svcGVK, "proj", "s", svc, svc)); len(resp.Patch) != 0 {
		t.Fatal("must not default on update")
	}
}

func TestDefaults_Coerce(t *testing.T) {
	// Coerce rewrites an explicit out-of-list value to the project default.
	m := defaults(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityNone), lbRef(v1alpha1.DefaultingCoerce), lbGrant())
	bad := raw(t, map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "s", "namespace": "proj"},
		"spec": map[string]any{"type": "LoadBalancer", "loadBalancerClass": "forbidden"}})
	resp := serve(t, m, "/defaults", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", bad, nil))
	var patches []map[string]any
	if err := json.Unmarshal(resp.Patch, &patches); err != nil {
		t.Fatal(err)
	}
	if len(patches) != 1 || patches[0]["path"] != "/spec/loadBalancerClass" || patches[0]["value"] != "internal" {
		t.Fatalf("expected coercion to internal, got: %v", patches)
	}
	// An already-available value is left untouched.
	good := raw(t, map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "s", "namespace": "proj"},
		"spec": map[string]any{"type": "LoadBalancer", "loadBalancerClass": "external"}})
	if resp := serve(t, m, "/defaults", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", good, nil)); len(resp.Patch) != 0 {
		t.Fatalf("available value must not be coerced, got patch %s", resp.Patch)
	}
}

func TestDefaults_SystemRequestBypass(t *testing.T) {
	// Same anti-deadlock guarantee as the validator: /defaults must never act on a system/module
	// writer's request (the deckhouse-controller applies every module's Helm release), so a slow or
	// failing handler can't block them. A system writer is passed through untouched (no patch); a
	// normal user with the same out-of-list value is coerced. Covers both the group- and the
	// username-based bypass (e.g. the multitenancy-manager controller SA).
	m := defaults(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityNone), lbRef(v1alpha1.DefaultingCoerce), lbGrant())
	bad := raw(t, map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "s", "namespace": "proj"},
		"spec": map[string]any{"type": "LoadBalancer", "loadBalancerClass": "forbidden"}})

	rSys := review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", bad, nil)
	rSys.Request.UserInfo.Groups = []string{"system:serviceaccounts:d8-system"}
	if resp := serve(t, m, "/defaults", rSys); len(resp.Patch) != 0 {
		t.Fatalf("a system (d8-system) writer must not be mutated by /defaults, got patch %s", resp.Patch)
	}

	rCtrl := review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", bad, nil)
	rCtrl.Request.UserInfo.Username = "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"
	if resp := serve(t, m, "/defaults", rCtrl); len(resp.Patch) != 0 {
		t.Fatalf("the multitenancy-manager controller SA must not be mutated by /defaults, got patch %s", resp.Patch)
	}

	if resp := serve(t, m, "/defaults", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", bad, nil)); len(resp.Patch) == 0 {
		t.Fatal("a normal user's out-of-list value must still be coerced")
	}
}

func TestDefaults_FillEmptyDoesNotCoerce(t *testing.T) {
	// FillEmpty fills an empty field but never rewrites an explicit out-of-list value.
	m := defaults(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityNone), lbRef(v1alpha1.DefaultingFillEmpty), lbGrant())
	bad := raw(t, map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "s", "namespace": "proj"},
		"spec": map[string]any{"type": "LoadBalancer", "loadBalancerClass": "forbidden"}})
	if resp := serve(t, m, "/defaults", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", bad, nil)); len(resp.Patch) != 0 {
		t.Fatalf("FillEmpty must not coerce an explicit value, got patch %s", resp.Patch)
	}
	empty := raw(t, map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "s", "namespace": "proj"},
		"spec": map[string]any{"type": "LoadBalancer"}})
	if resp := serve(t, m, "/defaults", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", empty, nil)); len(resp.Patch) == 0 {
		t.Fatal("empty field must still be defaulted")
	}
}

func TestDefaults_NoneNotDefaulted(t *testing.T) {
	// Defaulting None never fills in (feature-toggle annotations stay absent).
	m := defaults(t, projectNS("proj", map[string]string{"env": "prod"}), lbDef(v1alpha1.AvailabilityAll), lbRef(v1alpha1.DefaultingNone), lbGrant())
	svc := raw(t, map[string]any{
		"apiVersion": "v1", "kind": "Service",
		"metadata": map[string]any{"name": "s", "namespace": "proj"},
		"spec":     map[string]any{"type": "LoadBalancer"},
	})
	if resp := serve(t, m, "/defaults", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", svc, nil)); len(resp.Patch) != 0 {
		t.Fatalf("None defaulting must not patch, got patch %s", resp.Patch)
	}
}

func TestDefaults_UnavailableDefaultNotCoerced(t *testing.T) {
	// storageclasses with defaultFrom annotation; the cluster default (replicated) is NOT allowed in
	// this project (allowed: local only). The annotation-derived default must be dropped, so a PVC
	// without a storageClassName gets no patch (and is then denied by /is-granted).
	def := &v1alpha1.GrantableClusterResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.GrantableClusterResourceDefinitionSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIGroup: "storage.k8s.io", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityNone,
			DefaultFrom:         &v1alpha1.DefaultFrom{AnnotationKey: "storageclass.kubernetes.io/is-default-class"},
		},
	}
	ref := &v1alpha1.GrantableClusterResourceReference{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses-pvc"},
		Spec: v1alpha1.GrantableClusterResourceReferenceSpec{
			GrantableClusterResourceName: "storageclasses",
			Rule:                         v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
			FieldPaths:                   []v1alpha1.FieldPath{{Path: "$.spec.storageClassName", Defaulting: v1alpha1.DefaultingCoerce}},
		},
	}
	grant := &v1alpha1.ClusterResourceGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "g"},
		Spec: v1alpha1.ClusterResourceGrantPolicySpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources:       []v1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"local"}}},
		},
	}
	repl := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "replicated", Annotations: map[string]string{"storageclass.kubernetes.io/is-default-class": "true"}}, Provisioner: "x"}
	local := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "local"}, Provisioner: "x"}
	m := defaults(t, projectNS("proj", map[string]string{"env": "prod"}), def, ref, grant, repl, local)

	pvc := raw(t, map[string]any{"apiVersion": "v1", "kind": "PersistentVolumeClaim", "metadata": map[string]any{"name": "p", "namespace": "proj"}, "spec": map[string]any{}})
	if resp := serve(t, m, "/defaults", review(admissionv1.Create, pvcGVR, pvcGVK, "proj", "p", pvc, nil)); len(resp.Patch) != 0 {
		t.Fatalf("must not coerce to an unavailable default, got patch %s", resp.Patch)
	}
}

func TestDecodeReview_RejectsOversizeBody(t *testing.T) {
	p := NewProtectValidator(logr.Discard(), "system:serviceaccount:d8-multitenancy-manager:coc")
	// a body larger than the limit must be rejected before it is fully buffered into memory.
	huge := bytes.Repeat([]byte("a"), maxAdmissionRequestBytes+1024)
	body := []byte(`{"kind":"AdmissionReview","request":{"uid":"1","name":"`)
	body = append(body, huge...)
	body = append(body, []byte(`"}}`)...)

	req := httptest.NewRequest(http.MethodPost, "/protect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Fatalf("an oversize body must be rejected, got status %d", rec.Code)
	}
}

func TestDecodeReview_RejectsWrongKind(t *testing.T) {
	p := NewProtectValidator(logr.Discard(), "system:serviceaccount:d8-multitenancy-manager:coc")
	body := []byte(`{"kind":"NotAReview","request":{"uid":"1"}}`)

	req := httptest.NewRequest(http.MethodPost, "/protect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	p.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a non-AdmissionReview payload must be rejected with 400, got status %d", rec.Code)
	}
}

func TestProtect(t *testing.T) {
	p := NewProtectValidator(logr.Discard(), "system:serviceaccount:d8-multitenancy-manager:coc")
	arGVK := metav1.GroupVersionKind{Group: "multitenancy.deckhouse.io", Version: "v1alpha1", Kind: "AvailableClusterResource"}

	mk := func(gvk metav1.GroupVersionKind, sub, user string) admissionv1.AdmissionReview {
		r := review(admissionv1.Update, metav1.GroupVersionResource{}, gvk, "proj", "objects", []byte("{}"), nil)
		r.Request.SubResource = sub
		r.Request.UserInfo.Username = user
		return r
	}
	// AvailableClusterResource: user denied, controller allowed.
	if resp := serve(t, p, "/protect", mk(arGVK, "", "alice")); resp.Allowed {
		t.Fatal("user must not write AvailableClusterResource")
	}
	if resp := serve(t, p, "/protect", mk(arGVK, "", "system:serviceaccount:d8-multitenancy-manager:coc")); !resp.Allowed {
		t.Fatal("controller must write AvailableClusterResource")
	}
	// System controllers bypass protection so namespace teardown / GC can delete the catalog.
	sysReview := mk(arGVK, "", "system:serviceaccount:kube-system:namespace-controller")
	sysReview.Request.Operation = admissionv1.Delete
	sysReview.Request.UserInfo.Groups = []string{"system:serviceaccounts:kube-system"}
	if resp := serve(t, p, "/protect", sysReview); !resp.Allowed {
		t.Fatal("system controller must bypass AvailableClusterResource protection")
	}
}
