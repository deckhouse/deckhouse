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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

// lbRegistration is a value-backed registration for loadBalancerClass with the given default availability.
func lbRegistration(defAvail v1alpha1.AvailabilityDefault) *v1alpha1.ClusterGrantableResource {
	return &v1alpha1.ClusterGrantableResource{
		ObjectMeta: metav1.ObjectMeta{Name: "loadbalancerclasses"},
		Spec: v1alpha1.ClusterGrantableResourceSpec{
			DefaultAvailability: defAvail,
			UsageReferences: []v1alpha1.UsageReference{{
				Rule:      v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"services"}},
				FieldPath: "$.spec.loadBalancerClass",
				Match:     &v1alpha1.MatchPredicate{FieldPath: "$.spec.type", Equals: "LoadBalancer"},
				Countable: true,
			}},
		},
	}
}

func lbGrant() *v1alpha1.ClusterObjectGrant {
	return &v1alpha1.ClusterObjectGrant{
		ObjectMeta: metav1.ObjectMeta{Name: "g"},
		Spec: v1alpha1.ClusterObjectGrantSpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources: []v1alpha1.GrantResource{{
				ResourceRef: "loadbalancerclasses",
				Allowed:     []string{"external", "internal"},
				Default:     "internal",
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

func TestIsGranted_ValueBacked_AllowDeny(t *testing.T) {
	cl := newClient(t, projectNS("proj", map[string]string{"env": "prod"}), lbRegistration(v1alpha1.AvailabilityNone), lbGrant())
	v := NewIsGrantedValidator(logr.Discard(), cl, jsonpath.NewWithCache())

	resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("external", "LoadBalancer"), nil))
	if !resp.Allowed {
		t.Fatalf("external should be allowed: %v", resp.Result)
	}
	resp = serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("forbidden", "LoadBalancer"), nil))
	if resp.Allowed {
		t.Fatal("forbidden class must be denied under None default")
	}
	// match guard false (ClusterIP) → not governed → allow.
	resp = serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("forbidden", "ClusterIP"), nil))
	if !resp.Allowed {
		t.Fatal("non-LoadBalancer service must be allowed (guard false)")
	}
}

func TestIsGranted_DefaultAll_AllowsUngranted(t *testing.T) {
	cl := newClient(t, projectNS("proj", map[string]string{"env": "prod"}), lbRegistration(v1alpha1.AvailabilityAll), lbGrant())
	v := NewIsGrantedValidator(logr.Discard(), cl, jsonpath.NewWithCache())
	resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("anything", "LoadBalancer"), nil))
	if !resp.Allowed {
		t.Fatal("All default must allow ungranted value")
	}
}

func TestIsGranted_NoGrant_NoneStillDenies(t *testing.T) {
	// No grant matches, but registration None must still deny.
	cl := newClient(t, projectNS("proj", nil), lbRegistration(v1alpha1.AvailabilityNone), lbGrant())
	v := NewIsGrantedValidator(logr.Discard(), cl, jsonpath.NewWithCache())
	resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "s", lbService("external", "LoadBalancer"), nil))
	if resp.Allowed {
		t.Fatal("None default must deny even with no matching grant")
	}
}

func TestIsGranted_UpdateGrandfathers(t *testing.T) {
	cl := newClient(t, projectNS("proj", map[string]string{"env": "prod"}), lbRegistration(v1alpha1.AvailabilityNone), lbGrant())
	v := NewIsGrantedValidator(logr.Discard(), cl, jsonpath.NewWithCache())
	old := lbService("legacy", "LoadBalancer")
	resp := serve(t, v, "/is-granted", review(admissionv1.Update, svcGVR, svcGVK, "proj", "s", lbService("legacy", "LoadBalancer"), old))
	if !resp.Allowed {
		t.Fatal("unchanged legacy value must be grandfathered on update")
	}
}

func TestIsGranted_SystemNamespaceBypass(t *testing.T) {
	cl := newClient(t, lbRegistration(v1alpha1.AvailabilityNone), lbGrant())
	v := NewIsGrantedValidator(logr.Discard(), cl, jsonpath.NewWithCache())
	resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "kube-system", "s", lbService("forbidden", "LoadBalancer"), nil))
	if !resp.Allowed {
		t.Fatal("system namespace must bypass")
	}
}

func TestIsGranted_QuotaDeny(t *testing.T) {
	pool := &v1alpha1.GrantQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "objects", Namespace: "proj"},
		Spec: v1alpha1.GrantQuotaSpec{Objects: map[string]map[string]map[string]resource.Quantity{
			"loadbalancerclasses": {"external": {"services": resource.MustParse("1")}},
		}},
	}
	lbClass := "external"
	existing := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "existing", Namespace: "proj"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer, LoadBalancerClass: &lbClass},
	}
	cl := newClient(t, projectNS("proj", map[string]string{"env": "prod"}), lbRegistration(v1alpha1.AvailabilityAll), lbGrant(), pool, existing)
	v := NewIsGrantedValidator(logr.Discard(), cl, jsonpath.NewWithCache())
	// One external LB already exists; limit is 1; adding a second must be denied.
	resp := serve(t, v, "/is-granted", review(admissionv1.Create, svcGVR, svcGVK, "proj", "new", lbService("external", "LoadBalancer"), nil))
	if resp.Allowed {
		t.Fatal("second external LB must be denied by quota")
	}
}

func TestDefaults_InjectsDefault(t *testing.T) {
	cl := newClient(t, projectNS("proj", map[string]string{"env": "prod"}), lbRegistration(v1alpha1.AvailabilityAll), lbGrant())
	m := NewDefaultsMutator(logr.Discard(), cl, jsonpath.NewWithCache())
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
	cl := newClient(t, projectNS("proj", map[string]string{"env": "prod"}), lbRegistration(v1alpha1.AvailabilityAll), lbGrant())
	m := NewDefaultsMutator(logr.Discard(), cl, jsonpath.NewWithCache())
	svc := raw(t, map[string]any{"apiVersion": "v1", "kind": "Service", "metadata": map[string]any{"name": "s"}, "spec": map[string]any{"type": "LoadBalancer"}})
	resp := serve(t, m, "/defaults", review(admissionv1.Update, svcGVR, svcGVK, "proj", "s", svc, svc))
	if len(resp.Patch) != 0 {
		t.Fatal("must not default on update")
	}
}

func TestDefaults_CoercesUnavailableValue(t *testing.T) {
	// Grant allows {external, internal} with default internal; the cluster default behaviour is
	// simulated by an explicit out-of-list value that must be coerced to the project default.
	cl := newClient(t, projectNS("proj", map[string]string{"env": "prod"}), lbRegistration(v1alpha1.AvailabilityNone), lbGrant())
	m := NewDefaultsMutator(logr.Discard(), cl, jsonpath.NewWithCache())

	// Not-available value -> coerced to the project default.
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

func TestDefaults_UnavailableDefaultNotCoerced(t *testing.T) {
	// storageclasses with defaultFrom annotation; the cluster default (replicated) is NOT allowed in
	// this project (allowed: local only). The annotation-derived default must be dropped, so a PVC
	// without a storageClassName gets no patch (and is then denied by /is-granted).
	reg := &v1alpha1.ClusterGrantableResource{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.ClusterGrantableResourceSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityNone,
			DefaultFrom:         &v1alpha1.DefaultFrom{AnnotationKey: "storageclass.kubernetes.io/is-default-class"},
			UsageReferences: []v1alpha1.UsageReference{{
				Rule:      v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
				FieldPath: "$.spec.storageClassName",
			}},
		},
	}
	grant := &v1alpha1.ClusterObjectGrant{
		ObjectMeta: metav1.ObjectMeta{Name: "g"},
		Spec: v1alpha1.ClusterObjectGrantSpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources:       []v1alpha1.GrantResource{{ResourceRef: "storageclasses", Allowed: []string{"local"}}},
		},
	}
	repl := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "replicated", Annotations: map[string]string{"storageclass.kubernetes.io/is-default-class": "true"}}, Provisioner: "x"}
	local := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "local"}, Provisioner: "x"}
	cl := newClient(t, projectNS("proj", map[string]string{"env": "prod"}), reg, grant, repl, local)
	m := NewDefaultsMutator(logr.Discard(), cl, jsonpath.NewWithCache())

	pvc := raw(t, map[string]any{"apiVersion": "v1", "kind": "PersistentVolumeClaim", "metadata": map[string]any{"name": "p", "namespace": "proj"}, "spec": map[string]any{}})
	if resp := serve(t, m, "/defaults", review(admissionv1.Create, pvcGVR, pvcGVK, "proj", "p", pvc, nil)); len(resp.Patch) != 0 {
		t.Fatalf("must not coerce to an unavailable default, got patch %s", resp.Patch)
	}
}

func TestIsGranted_ObjectBackedSelector(t *testing.T) {
	reg := &v1alpha1.ClusterGrantableResource{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.ClusterGrantableResourceSpec{
			GrantedResource:     &v1alpha1.GrantedResource{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
			DefaultAvailability: v1alpha1.AvailabilityNone,
			UsageReferences: []v1alpha1.UsageReference{{
				Rule:      v1alpha1.UsageRule{APIGroups: []string{""}, APIVersions: []string{"v1"}, Resources: []string{"persistentvolumeclaims"}},
				FieldPath: "$.spec.storageClassName",
			}},
		},
	}
	grant := &v1alpha1.ClusterObjectGrant{
		ObjectMeta: metav1.ObjectMeta{Name: "g"},
		Spec: v1alpha1.ClusterObjectGrantSpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			Resources: []v1alpha1.GrantResource{{
				ResourceRef:     "storageclasses",
				AllowedSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"shared": "true"}},
			}},
		},
	}
	shared := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "ssd", Labels: map[string]string{"shared": "true"}}, Provisioner: "x"}
	priv := &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: "secret"}, Provisioner: "x"}
	cl := newClient(t, projectNS("proj", map[string]string{"env": "prod"}), reg, grant, shared, priv)
	v := NewIsGrantedValidator(logr.Discard(), cl, jsonpath.NewWithCache())

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

func TestProtect(t *testing.T) {
	p := NewProtectValidator(logr.Discard(), "system:serviceaccount:d8-multitenancy-manager:coc")
	arGVK := metav1.GroupVersionKind{Group: "multitenancy.deckhouse.io", Version: "v1alpha1", Kind: "AvailableResource"}
	gqGVK := metav1.GroupVersionKind{Group: "multitenancy.deckhouse.io", Version: "v1alpha1", Kind: "GrantQuota"}

	mk := func(gvk metav1.GroupVersionKind, sub, user string) admissionv1.AdmissionReview {
		r := review(admissionv1.Update, metav1.GroupVersionResource{}, gvk, "proj", "objects", []byte("{}"), nil)
		r.Request.SubResource = sub
		r.Request.UserInfo.Username = user
		return r
	}
	// AvailableResource: user denied, controller allowed.
	if resp := serve(t, p, "/protect", mk(arGVK, "", "alice")); resp.Allowed {
		t.Fatal("user must not write AvailableResource")
	}
	if resp := serve(t, p, "/protect", mk(arGVK, "", "system:serviceaccount:d8-multitenancy-manager:coc")); !resp.Allowed {
		t.Fatal("controller must write AvailableResource")
	}
	// System controllers (kube-system namespace-controller / GC, masters) bypass
	// protection so namespace teardown and garbage collection can delete the
	// controller-owned catalog without deadlocking.
	sysReview := mk(arGVK, "", "system:serviceaccount:kube-system:namespace-controller")
	sysReview.Request.Operation = admissionv1.Delete
	sysReview.Request.UserInfo.Groups = []string{"system:serviceaccounts:kube-system"}
	if resp := serve(t, p, "/protect", sysReview); !resp.Allowed {
		t.Fatal("system controller must bypass AvailableResource protection")
	}
	// GrantQuota status: user denied; spec: user allowed (RBAC governs).
	if resp := serve(t, p, "/protect", mk(gqGVK, "status", "alice")); resp.Allowed {
		t.Fatal("user must not write GrantQuota status")
	}
	if resp := serve(t, p, "/protect", mk(gqGVK, "", "alice")); !resp.Allowed {
		t.Fatal("GrantQuota spec write is governed by RBAC, webhook allows")
	}
}
