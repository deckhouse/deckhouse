/*
Copyright 2025 Flant JSC

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

package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	"github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

// --- helpers ---

func providerSecret(nodeGroupNames ...string) *corev1.Secret {
	ngs := make([]map[string]string, len(nodeGroupNames))
	for i, n := range nodeGroupNames {
		ngs[i] = map[string]string{"name": n}
	}
	configYAML := "nodeGroups:\n"
	for _, n := range nodeGroupNames {
		configYAML += "  - name: " + n + "\n"
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-provider-cluster-configuration", Namespace: "kube-system"},
		Data:       map[string][]byte{"cloud-provider-cluster-configuration.yaml": []byte(configYAML)},
	}
}

func makeV1NodeGroupJSON(name string, nodeType v1.NodeType) []byte {
	ng := v1.NodeGroup{
		TypeMeta:   metav1.TypeMeta{APIVersion: "deckhouse.io/v1", Kind: "NodeGroup"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: nodeType},
	}
	raw, _ := json.Marshal(ng)
	return raw
}

func makeV1Alpha2NodeGroupJSON(name string, nodeType v1alpha2.NodeType) []byte {
	ng := v1alpha2.NodeGroup{
		TypeMeta:   metav1.TypeMeta{APIVersion: "deckhouse.io/v1alpha2", Kind: "NodeGroup"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1alpha2.NodeGroupSpec{NodeType: nodeType},
	}
	raw, _ := json.Marshal(ng)
	return raw
}

func buildConversionReview(uid types.UID, desiredVersion string, objects ...[]byte) *apix.ConversionReview {
	rawObjects := make([]runtime.RawExtension, len(objects))
	for i, o := range objects {
		rawObjects[i] = runtime.RawExtension{Raw: o}
	}
	return &apix.ConversionReview{
		TypeMeta: metav1.TypeMeta{Kind: "ConversionReview", APIVersion: "apiextensions.k8s.io/v1"},
		Request: &apix.ConversionRequest{
			UID:               uid,
			DesiredAPIVersion: desiredVersion,
			Objects:           rawObjects,
		},
	}
}

func doConversionRequest(t *testing.T, handler http.Handler, review *apix.ConversionReview) *apix.ConversionReview {
	t.Helper()
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	result := &apix.ConversionReview{}
	if err := json.Unmarshal(respBody, result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	return result
}

func extractV1NodeGroup(t *testing.T, raw []byte) *v1.NodeGroup {
	t.Helper()
	ng := &v1.NodeGroup{}
	if err := json.Unmarshal(raw, ng); err != nil {
		t.Fatalf("failed to unmarshal v1 NodeGroup: %v", err)
	}
	return ng
}

func extractV1Alpha2NodeGroup(t *testing.T, raw []byte) *v1alpha2.NodeGroup {
	t.Helper()
	ng := &v1alpha2.NodeGroup{}
	if err := json.Unmarshal(raw, ng); err != nil {
		t.Fatalf("failed to unmarshal v1alpha2 NodeGroup: %v", err)
	}
	return ng
}

// --- isCloudPermanent ---

func TestIsCloudPermanent_Master(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	cfg := &ProviderClusterConfiguration{}
	if !h.isCloudPermanent("master", cfg) {
		t.Fatal("master should always be CloudPermanent")
	}
}

func TestIsCloudPermanent_InProviderConfig(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "frontend"}, {Name: "backend"}},
	}
	if !h.isCloudPermanent("frontend", cfg) {
		t.Fatal("frontend is in provider config, should be CloudPermanent")
	}
}

func TestIsCloudPermanent_NotInProviderConfig(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "frontend"}},
	}
	if h.isCloudPermanent("worker", cfg) {
		t.Fatal("worker is not in provider config, should not be CloudPermanent")
	}
}

// --- convertObject: same version ---

func TestConvertObject_SameVersion(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1NodeGroupJSON("test", v1.NodeTypeStatic)

	result, err := h.convertObject(raw, "deckhouse.io/v1", &ProviderClusterConfiguration{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(raw, result) {
		t.Fatal("same version should return object as-is")
	}
}

// --- convertObject: v1alpha2 Cloud → v1 CloudEphemeral ---

func TestConvertObject_V1Alpha2CloudToV1(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("ephemeral", v1alpha2.NodeTypeCloud)

	result, err := h.convertObject(raw, "deckhouse.io/v1", &ProviderClusterConfiguration{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		t.Fatalf("expected CloudEphemeral, got %s", ng.Spec.NodeType)
	}
	if ng.APIVersion != "deckhouse.io/v1" {
		t.Fatalf("expected apiVersion deckhouse.io/v1, got %s", ng.APIVersion)
	}
}

// --- convertObject: v1alpha2 Static → v1 Static ---

func TestConvertObject_V1Alpha2StaticToV1(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("worker", v1alpha2.NodeTypeStatic)

	result, err := h.convertObject(raw, "deckhouse.io/v1", &ProviderClusterConfiguration{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeStatic {
		t.Fatalf("expected Static, got %s", ng.Spec.NodeType)
	}
}

// --- convertObject: v1alpha2 Hybrid → v1 CloudPermanent (master) ---

func TestConvertObject_V1Alpha2HybridMasterToV1(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("master", v1alpha2.NodeTypeHybrid)

	result, err := h.convertObject(raw, "deckhouse.io/v1", &ProviderClusterConfiguration{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("expected CloudPermanent for master Hybrid, got %s", ng.Spec.NodeType)
	}
}

// --- convertObject: v1alpha2 Hybrid → v1 CloudPermanent (in provider config) ---

func TestConvertObject_V1Alpha2HybridInProviderToV1(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("frontend", v1alpha2.NodeTypeHybrid)
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "frontend"}},
	}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("expected CloudPermanent for frontend (in provider config), got %s", ng.Spec.NodeType)
	}
}

// --- convertObject: v1alpha2 Hybrid → v1 CloudStatic (not in provider config) ---

func TestConvertObject_V1Alpha2HybridNotInProviderToV1(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("worker", v1alpha2.NodeTypeHybrid)
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "frontend"}},
	}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeCloudStatic {
		t.Fatalf("expected CloudStatic for worker (not in provider config), got %s", ng.Spec.NodeType)
	}
}

// --- convertObject: v1 → v1alpha2 reverse mappings ---

func TestConvertObject_V1CloudEphemeralToV1Alpha2(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1NodeGroupJSON("ephemeral", v1.NodeTypeCloudEphemeral)

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", &ProviderClusterConfiguration{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha2NodeGroup(t, result)
	if ng.Spec.NodeType != v1alpha2.NodeTypeCloud {
		t.Fatalf("expected Cloud, got %s", ng.Spec.NodeType)
	}
}

func TestConvertObject_V1CloudPermanentToV1Alpha2(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1NodeGroupJSON("perm", v1.NodeTypeCloudPermanent)

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", &ProviderClusterConfiguration{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha2NodeGroup(t, result)
	if ng.Spec.NodeType != v1alpha2.NodeTypeHybrid {
		t.Fatalf("expected Hybrid, got %s", ng.Spec.NodeType)
	}
}

func TestConvertObject_V1CloudStaticToV1Alpha2(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1NodeGroupJSON("cs", v1.NodeTypeCloudStatic)

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", &ProviderClusterConfiguration{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha2NodeGroup(t, result)
	if ng.Spec.NodeType != v1alpha2.NodeTypeHybrid {
		t.Fatalf("expected Hybrid, got %s", ng.Spec.NodeType)
	}
}

func TestConvertObject_V1StaticToV1Alpha2(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1NodeGroupJSON("worker", v1.NodeTypeStatic)

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", &ProviderClusterConfiguration{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha2NodeGroup(t, result)
	if ng.Spec.NodeType != v1alpha2.NodeTypeStatic {
		t.Fatalf("expected Static, got %s", ng.Spec.NodeType)
	}
}

// --- convertObject: unsupported version ---

func TestConvertObject_UnsupportedSourceVersion(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := []byte(`{"apiVersion":"deckhouse.io/v999","kind":"NodeGroup","metadata":{"name":"x"},"spec":{"nodeType":"Static"}}`)

	_, err := h.convertObject(raw, "deckhouse.io/v1", &ProviderClusterConfiguration{})
	if err == nil {
		t.Fatal("expected error for unsupported source version")
	}
}

func TestConvertObject_UnsupportedDesiredVersion(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	raw := makeV1NodeGroupJSON("test", v1.NodeTypeStatic)

	_, err := h.convertObject(raw, "deckhouse.io/v999", &ProviderClusterConfiguration{})
	if err == nil {
		t.Fatal("expected error for unsupported desired version")
	}
}

// --- handleConversion: multiple objects ---

func TestHandleConversion_MultipleObjects(t *testing.T) {
	h := &NodeGroupConversionHandler{}
	cfg := &ProviderClusterConfiguration{}

	req := &apix.ConversionRequest{
		UID:               "test-uid",
		DesiredAPIVersion: "deckhouse.io/v1",
		Objects: []runtime.RawExtension{
			{Raw: makeV1Alpha2NodeGroupJSON("ng1", v1alpha2.NodeTypeCloud)},
			{Raw: makeV1Alpha2NodeGroupJSON("ng2", v1alpha2.NodeTypeStatic)},
		},
	}

	resp := h.handleConversion(req, cfg)
	if resp.Result.Status != "Success" {
		t.Fatalf("expected Success, got %s: %s", resp.Result.Status, resp.Result.Message)
	}
	if len(resp.ConvertedObjects) != 2 {
		t.Fatalf("expected 2 converted objects, got %d", len(resp.ConvertedObjects))
	}
	if string(resp.UID) != "test-uid" {
		t.Fatalf("expected UID test-uid, got %s", resp.UID)
	}

	ng1 := extractV1NodeGroup(t, resp.ConvertedObjects[0].Raw)
	if ng1.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		t.Fatalf("ng1: expected CloudEphemeral, got %s", ng1.Spec.NodeType)
	}
	ng2 := extractV1NodeGroup(t, resp.ConvertedObjects[1].Raw)
	if ng2.Spec.NodeType != v1.NodeTypeStatic {
		t.Fatalf("ng2: expected Static, got %s", ng2.Spec.NodeType)
	}
}

// --- ServeHTTP integration ---

func TestServeHTTP_FullRoundTrip(t *testing.T) {
	s := newScheme()
	sec := providerSecret("frontend")
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sec).Build()

	handler := &NodeGroupConversionHandler{Client: c, Scheme: s}
	review := buildConversionReview(
		"uid-123",
		"deckhouse.io/v1",
		makeV1Alpha2NodeGroupJSON("frontend", v1alpha2.NodeTypeHybrid),
		makeV1Alpha2NodeGroupJSON("worker", v1alpha2.NodeTypeHybrid),
	)

	result := doConversionRequest(t, handler, review)

	if result.Response == nil {
		t.Fatal("response is nil")
	}
	if result.Response.Result.Status != "Success" {
		t.Fatalf("expected Success, got %s: %s", result.Response.Result.Status, result.Response.Result.Message)
	}
	if len(result.Response.ConvertedObjects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(result.Response.ConvertedObjects))
	}

	ng1 := extractV1NodeGroup(t, result.Response.ConvertedObjects[0].Raw)
	if ng1.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("frontend: expected CloudPermanent (in provider config), got %s", ng1.Spec.NodeType)
	}

	ng2 := extractV1NodeGroup(t, result.Response.ConvertedObjects[1].Raw)
	if ng2.Spec.NodeType != v1.NodeTypeCloudStatic {
		t.Fatalf("worker: expected CloudStatic (not in provider config), got %s", ng2.Spec.NodeType)
	}
}

func TestServeHTTP_NilRequest(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	handler := &NodeGroupConversionHandler{Client: c, Scheme: s}

	review := &apix.ConversionReview{
		TypeMeta: metav1.TypeMeta{Kind: "ConversionReview", APIVersion: "apiextensions.k8s.io/v1"},
		Request:  nil,
	}

	body, _ := json.Marshal(review)
	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	result := &apix.ConversionReview{}
	_ = json.Unmarshal(respBody, result)

	if result.Response == nil || result.Response.Result.Status != "Failure" {
		t.Fatal("expected Failure response for nil request")
	}
}

func TestServeHTTP_InvalidBody(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	handler := &NodeGroupConversionHandler{Client: c, Scheme: s}

	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	resp := rec.Result()
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	result := &apix.ConversionReview{}
	_ = json.Unmarshal(respBody, result)

	if result.Response == nil || result.Response.Result.Status != "Failure" {
		t.Fatal("expected Failure response for invalid body")
	}
}

func TestServeHTTP_NoProviderSecret(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build() // no secret

	handler := &NodeGroupConversionHandler{Client: c, Scheme: s}
	review := buildConversionReview(
		"uid-456",
		"deckhouse.io/v1",
		makeV1Alpha2NodeGroupJSON("master", v1alpha2.NodeTypeHybrid),
	)

	result := doConversionRequest(t, handler, review)

	if result.Response.Result.Status != "Success" {
		t.Fatalf("expected Success even without provider secret, got: %s", result.Response.Result.Message)
	}

	ng := extractV1NodeGroup(t, result.Response.ConvertedObjects[0].Raw)
	// master is always CloudPermanent regardless of provider config
	if ng.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("master: expected CloudPermanent, got %s", ng.Spec.NodeType)
	}
}

// --- loadProviderConfig ---

func TestLoadProviderConfig_Success(t *testing.T) {
	s := newScheme()
	sec := providerSecret("ng1", "ng2")
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sec).Build()
	h := &NodeGroupConversionHandler{Client: c, Scheme: s}

	cfg := h.loadProviderConfig(context.Background())
	if len(cfg.NodeGroups) != 2 {
		t.Fatalf("expected 2 node groups, got %d", len(cfg.NodeGroups))
	}
	if cfg.NodeGroups[0].Name != "ng1" || cfg.NodeGroups[1].Name != "ng2" {
		t.Fatalf("unexpected node group names: %v", cfg.NodeGroups)
	}
}

func TestLoadProviderConfig_SecretMissing(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	h := &NodeGroupConversionHandler{Client: c, Scheme: s}

	cfg := h.loadProviderConfig(context.Background())
	if len(cfg.NodeGroups) != 0 {
		t.Fatalf("expected empty config when secret missing, got %d groups", len(cfg.NodeGroups))
	}
}

func TestLoadProviderConfig_MissingKey(t *testing.T) {
	s := newScheme()
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-provider-cluster-configuration", Namespace: "kube-system"},
		Data:       map[string][]byte{"wrong-key.yaml": []byte("nodeGroups:\n  - name: x\n")},
	}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sec).Build()
	h := &NodeGroupConversionHandler{Client: c, Scheme: s}

	cfg := h.loadProviderConfig(context.Background())
	if len(cfg.NodeGroups) != 0 {
		t.Fatalf("expected empty config when key missing, got %d groups", len(cfg.NodeGroups))
	}
}
