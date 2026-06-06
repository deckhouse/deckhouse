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

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"

	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newTestValidator(t *testing.T, objs ...client.Object) *ClusterObjectGrantValidator {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return NewClusterResourceGrantValidator(logr.Discard(), cl, jsonpath.NewWithCache())
}

func projectNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "proj", Labels: map[string]string{"environment": "production"}},
	}
}

func storageClassPolicy() *v1alpha1.ClusterObjectGrantPolicy {
	return &v1alpha1.ClusterObjectGrantPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "storageclasses"},
		Spec: v1alpha1.ClusterObjectGrantPolicySpec{
			GrantedResource: v1alpha1.GrantedResource{
				TypeMeta: metav1.TypeMeta{APIVersion: "storage.k8s.io/v1", Kind: "StorageClass"},
			},
			UsageReferences: []v1alpha1.GrantedResourceUsageReference{
				{APIVersion: "v1", Resource: "configmaps", FieldPath: "$.data.scName"},
			},
		},
	}
}

func grantFor(selectorEnv string, allowed []string) *v1alpha1.ClusterObjectGrant {
	return &v1alpha1.ClusterObjectGrant{
		ObjectMeta: metav1.ObjectMeta{Name: "grant"},
		Spec: v1alpha1.ClusterObjectGrantSpec{
			ProjectSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"environment": selectorEnv}},
			Policies: []v1alpha1.ApplicablePolicy{
				{Name: "storageclasses", Allowed: allowed},
			},
		},
	}
}

func runReview(t *testing.T, v *ClusterObjectGrantValidator, review admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	t.Helper()
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/is-granted", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	v.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", rec.Code, rec.Body.String())
	}
	out := &admissionv1.AdmissionReview{}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out.Response == nil {
		t.Fatal("nil admission response")
	}
	return out.Response
}

func configMapRaw(t *testing.T, scName string) []byte {
	t.Helper()
	raw, err := json.Marshal(map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   map[string]any{"name": "cm", "namespace": "proj"},
		"data":       map[string]any{"scName": scName},
	})
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func review(op admissionv1.Operation, newRaw, oldRaw []byte) admissionv1.AdmissionReview {
	return admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:       "1",
			Namespace: "proj",
			Operation: op,
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			Object:    runtime.RawExtension{Raw: newRaw},
			OldObject: runtime.RawExtension{Raw: oldRaw},
		},
	}
}

func TestValidator_AllowsWhitelistedValue(t *testing.T) {
	v := newTestValidator(t, projectNamespace(), storageClassPolicy(), grantFor("production", []string{"fast-ssd"}))
	resp := runReview(t, v, review(admissionv1.Create, configMapRaw(t, "fast-ssd"), nil))
	if !resp.Allowed {
		t.Fatalf("expected allow, got deny: %v", resp.Result)
	}
}

func TestValidator_DeniesForbiddenValue(t *testing.T) {
	v := newTestValidator(t, projectNamespace(), storageClassPolicy(), grantFor("production", []string{"fast-ssd"}))
	resp := runReview(t, v, review(admissionv1.Create, configMapRaw(t, "forbidden"), nil))
	if resp.Allowed {
		t.Fatal("expected deny, got allow")
	}
}

func TestValidator_AllowsWhenNoGrantMatches(t *testing.T) {
	// Grant selects a different environment, so no grant applies → allow.
	v := newTestValidator(t, projectNamespace(), storageClassPolicy(), grantFor("staging", []string{"fast-ssd"}))
	resp := runReview(t, v, review(admissionv1.Create, configMapRaw(t, "anything"), nil))
	if !resp.Allowed {
		t.Fatalf("expected allow, got deny: %v", resp.Result)
	}
}

func TestValidator_UpdateGrandfathersUnchangedValue(t *testing.T) {
	v := newTestValidator(t, projectNamespace(), storageClassPolicy(), grantFor("production", []string{"fast-ssd"}))
	// The object already had a now-forbidden value; an update that keeps it must pass.
	old := configMapRaw(t, "legacy")
	resp := runReview(t, v, review(admissionv1.Update, configMapRaw(t, "legacy"), old))
	if !resp.Allowed {
		t.Fatalf("expected allow (grandfathered), got deny: %v", resp.Result)
	}
}

func TestValidator_UpdateDeniesNewlyForbiddenValue(t *testing.T) {
	v := newTestValidator(t, projectNamespace(), storageClassPolicy(), grantFor("production", []string{"fast-ssd"}))
	old := configMapRaw(t, "fast-ssd")
	resp := runReview(t, v, review(admissionv1.Update, configMapRaw(t, "forbidden"), old))
	if resp.Allowed {
		t.Fatal("expected deny for newly introduced forbidden value")
	}
}

func TestValidator_SystemNamespaceBypassed(t *testing.T) {
	v := newTestValidator(t, projectNamespace(), storageClassPolicy(), grantFor("production", []string{"fast-ssd"}))
	r := review(admissionv1.Create, configMapRaw(t, "forbidden"), nil)
	r.Request.Namespace = "kube-system"
	resp := runReview(t, v, r)
	if !resp.Allowed {
		t.Fatal("expected allow for system namespace")
	}
}
