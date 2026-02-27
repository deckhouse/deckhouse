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
	"github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha1"
	"github.com/deckhouse/node-controller/api/deckhouse.io/v1alpha2"
)

func providerSecretWithNodeGroups(nodeGroupNames ...string) *corev1.Secret {
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

func makeV1Alpha1NodeGroupJSON(name string, nodeType string) []byte {
	// v1alpha1 uses string for NodeType, not a typed constant
	ng := map[string]interface{}{
		"apiVersion": "deckhouse.io/v1alpha1",
		"kind":       "NodeGroup",
		"metadata": map[string]interface{}{
			"name": name,
		},
		"spec": map[string]interface{}{
			"nodeType": nodeType,
		},
	}
	raw, _ := json.Marshal(ng)
	return raw
}

func makeV1Alpha1InstanceJSON(name string, phase v1alpha1.InstancePhase) []byte {
	instance := v1alpha1.Instance{
		TypeMeta: metav1.TypeMeta{APIVersion: "deckhouse.io/v1alpha1", Kind: "Instance"},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1alpha1.InstanceStatus{
			NodeRef: v1alpha1.NodeRef{Name: name},
			MachineRef: v1alpha1.MachineRef{
				APIVersion: "machine.sapcloud.io/v1alpha1",
				Kind:       "Machine",
				Name:       name,
				Namespace:  "d8-cloud-instance-manager",
			},
			ClassReference: v1alpha1.ClassReference{
				Kind: "DVPInstanceClass",
				Name: "worker",
			},
			CurrentStatus: v1alpha1.CurrentStatus{Phase: phase},
		},
	}
	raw, _ := json.Marshal(instance)
	return raw
}

func makeV1Alpha1InstanceJSONWithoutClassReference(name string, phase v1alpha1.InstancePhase) []byte {
	instance := v1alpha1.Instance{
		TypeMeta: metav1.TypeMeta{APIVersion: "deckhouse.io/v1alpha1", Kind: "Instance"},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1alpha1.InstanceStatus{
			NodeRef: v1alpha1.NodeRef{Name: name},
			MachineRef: v1alpha1.MachineRef{
				APIVersion: "machine.sapcloud.io/v1alpha1",
				Kind:       "Machine",
				Name:       name,
				Namespace:  "d8-cloud-instance-manager",
			},
			CurrentStatus: v1alpha1.CurrentStatus{Phase: phase},
		},
	}
	raw, _ := json.Marshal(instance)
	return raw
}

func makeV1Alpha1InstanceJSONWithoutMachineRef(name string, phase v1alpha1.InstancePhase) []byte {
	instance := v1alpha1.Instance{
		TypeMeta: metav1.TypeMeta{APIVersion: "deckhouse.io/v1alpha1", Kind: "Instance"},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Status: v1alpha1.InstanceStatus{
			NodeRef:        v1alpha1.NodeRef{Name: name},
			CurrentStatus:  v1alpha1.CurrentStatus{Phase: phase},
			ClassReference: v1alpha1.ClassReference{Kind: "DVPInstanceClass", Name: "worker"},
		},
	}
	raw, _ := json.Marshal(instance)
	return raw
}

func makeV1Alpha2InstanceJSON(name string, phase v1alpha2.InstancePhase) []byte {
	instance := v1alpha2.Instance{
		TypeMeta: metav1.TypeMeta{APIVersion: "deckhouse.io/v1alpha2", Kind: "Instance"},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1alpha2.InstanceSpec{
			NodeRef: v1alpha2.NodeRef{Name: name},
			MachineRef: &v1alpha2.MachineRef{
				APIVersion: "machine.sapcloud.io/v1alpha1",
				Kind:       "Machine",
				Name:       name,
				Namespace:  "d8-cloud-instance-manager",
			},
			ClassReference: &v1alpha2.ClassReference{
				Kind: "DVPInstanceClass",
				Name: "worker",
			},
		},
		Status: v1alpha2.InstanceStatus{
			Phase: phase,
		},
	}
	raw, _ := json.Marshal(instance)
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

func extractV1Alpha1NodeGroup(t *testing.T, raw []byte) *v1alpha1.NodeGroup {
	t.Helper()
	ng := &v1alpha1.NodeGroup{}
	if err := json.Unmarshal(raw, ng); err != nil {
		t.Fatalf("failed to unmarshal v1alpha1 NodeGroup: %v", err)
	}
	return ng
}

func extractV1Alpha2Instance(t *testing.T, raw []byte) *v1alpha2.Instance {
	t.Helper()
	instance := &v1alpha2.Instance{}
	if err := json.Unmarshal(raw, instance); err != nil {
		t.Fatalf("failed to unmarshal v1alpha2 Instance: %v", err)
	}
	return instance
}

func extractV1Alpha1Instance(t *testing.T, raw []byte) *v1alpha1.Instance {
	t.Helper()
	instance := &v1alpha1.Instance{}
	if err := json.Unmarshal(raw, instance); err != nil {
		t.Fatalf("failed to unmarshal v1alpha1 Instance: %v", err)
	}
	return instance
}

func TestIsCloudPermanent_Master(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{}
	if !h.isCloudPermanent("master", cfg) {
		t.Fatal("master should always be CloudPermanent")
	}
}

func TestIsCloudPermanent_InProviderConfig(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "front-nm"}, {Name: "backend"}},
	}
	if !h.isCloudPermanent("front-nm", cfg) {
		t.Fatal("front-nm is in provider config, should be CloudPermanent")
	}
}

func TestIsCloudPermanent_NotInProviderConfig(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "frontend"}},
	}
	if h.isCloudPermanent("another", cfg) {
		t.Fatal("'another' is not in provider config, should not be CloudPermanent")
	}
}

func TestAlpha2ToV1_CloudToCloudEphemeral(t *testing.T) {
	// Python: test_change_node_type_from_cloud_to_cloud_ephemeral
	h := &ConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("worker-static", v1alpha2.NodeTypeCloud)
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "front-nm"}},
	}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
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

func TestAlpha2ToV1_HybridMasterToCloudPermanent(t *testing.T) {
	// Python: test_change_node_type_from_hybrid_to_cloud_permanent_for_master_ng
	h := &ConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("master", v1alpha2.NodeTypeHybrid)
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("expected CloudPermanent for master Hybrid, got %s", ng.Spec.NodeType)
	}
}

func TestAlpha2ToV1_HybridInProviderConfigToCloudPermanent(t *testing.T) {
	// Python: test_change_node_type_from_hybrid_to_cloud_permanent_for_ng_in_provider_cluster_config
	h := &ConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("front-nm", v1alpha2.NodeTypeHybrid)
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "front-nm"}},
	}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("expected CloudPermanent for front-nm (in provider config), got %s", ng.Spec.NodeType)
	}
}

func TestAlpha2ToV1_HybridNotInProviderConfigToCloudStatic(t *testing.T) {
	// Python: test_change_node_type_from_hybrid_to_cloud_static_for_ng_not_in_provider_cluster_config
	h := &ConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("another", v1alpha2.NodeTypeHybrid)
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "front-nm"}},
	}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeCloudStatic {
		t.Fatalf("expected CloudStatic for 'another' (not in provider config), got %s", ng.Spec.NodeType)
	}
}

func TestAlpha2ToV1_StaticToStatic(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1Alpha2NodeGroupJSON("worker", v1alpha2.NodeTypeStatic)
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeStatic {
		t.Fatalf("expected Static, got %s", ng.Spec.NodeType)
	}
}

func TestV1ToAlpha2_CloudEphemeralToCloud(t *testing.T) {
	// Python: test_change_node_type_from_cloud_ephemeral_to_cloud
	h := &ConversionHandler{}
	raw := makeV1NodeGroupJSON("worker-static", v1.NodeTypeCloudEphemeral)
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha2NodeGroup(t, result)
	if ng.Spec.NodeType != v1alpha2.NodeTypeCloud {
		t.Fatalf("expected Cloud, got %s", ng.Spec.NodeType)
	}
	if ng.APIVersion != "deckhouse.io/v1alpha2" {
		t.Fatalf("expected apiVersion deckhouse.io/v1alpha2, got %s", ng.APIVersion)
	}
}

func TestV1ToAlpha2_CloudPermanentToHybrid(t *testing.T) {
	// Python: test_change_node_type_from_cloud_permanent_to_hybrid
	h := &ConversionHandler{}
	raw := makeV1NodeGroupJSON("master", v1.NodeTypeCloudPermanent)
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha2NodeGroup(t, result)
	if ng.Spec.NodeType != v1alpha2.NodeTypeHybrid {
		t.Fatalf("expected Hybrid, got %s", ng.Spec.NodeType)
	}
}

func TestV1ToAlpha2_CloudStaticToHybrid(t *testing.T) {
	// Python: test_change_node_type_from_cloud_static_to_hybrid
	h := &ConversionHandler{}
	raw := makeV1NodeGroupJSON("another", v1.NodeTypeCloudStatic)
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha2NodeGroup(t, result)
	if ng.Spec.NodeType != v1alpha2.NodeTypeHybrid {
		t.Fatalf("expected Hybrid, got %s", ng.Spec.NodeType)
	}
}

func TestV1ToAlpha2_StaticToStatic(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1NodeGroupJSON("worker", v1.NodeTypeStatic)
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha2NodeGroup(t, result)
	if ng.Spec.NodeType != v1alpha2.NodeTypeStatic {
		t.Fatalf("expected Static, got %s", ng.Spec.NodeType)
	}
}

func TestAlpha1ToV1_CloudToCloudEphemeral(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1Alpha1NodeGroupJSON("worker", "Cloud")
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		t.Fatalf("expected CloudEphemeral, got %s", ng.Spec.NodeType)
	}
}

func TestAlpha1ToV1_StaticToStatic(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1Alpha1NodeGroupJSON("worker", "Static")
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeStatic {
		t.Fatalf("expected Static, got %s", ng.Spec.NodeType)
	}
}

func TestAlpha1ToV1_HybridMasterToCloudPermanent(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1Alpha1NodeGroupJSON("master", "Hybrid")
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1NodeGroup(t, result)
	if ng.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("expected CloudPermanent, got %s", ng.Spec.NodeType)
	}
}

func TestV1ToAlpha1_CloudEphemeralToCloud(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1NodeGroupJSON("worker", v1.NodeTypeCloudEphemeral)
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha1NodeGroup(t, result)
	if ng.Spec.NodeType != "Cloud" {
		t.Fatalf("expected Cloud, got %s", ng.Spec.NodeType)
	}
	if ng.APIVersion != "deckhouse.io/v1alpha1" {
		t.Fatalf("expected apiVersion deckhouse.io/v1alpha1, got %s", ng.APIVersion)
	}
}

func TestV1ToAlpha1_CloudPermanentToHybrid(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1NodeGroupJSON("master", v1.NodeTypeCloudPermanent)
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha1NodeGroup(t, result)
	if ng.Spec.NodeType != "Hybrid" {
		t.Fatalf("expected Hybrid, got %s", ng.Spec.NodeType)
	}
}

func TestV1ToAlpha1_CloudStaticToHybrid(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1NodeGroupJSON("worker", v1.NodeTypeCloudStatic)
	cfg := &ProviderClusterConfiguration{}

	result, err := h.convertObject(raw, "deckhouse.io/v1alpha1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ng := extractV1Alpha1NodeGroup(t, result)
	if ng.Spec.NodeType != "Hybrid" {
		t.Fatalf("expected Hybrid, got %s", ng.Spec.NodeType)
	}
}

func TestConvertObject_SameVersion(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1NodeGroupJSON("test", v1.NodeTypeStatic)

	result, err := h.convertObject(raw, "deckhouse.io/v1", &ProviderClusterConfiguration{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(raw, result) {
		t.Fatal("same version should return object as-is")
	}
}

func TestConvertObject_UnsupportedSourceVersion(t *testing.T) {
	h := &ConversionHandler{}
	raw := []byte(`{"apiVersion":"deckhouse.io/v999","kind":"NodeGroup","metadata":{"name":"x"},"spec":{"nodeType":"Static"}}`)

	_, err := h.convertObject(raw, "deckhouse.io/v1", &ProviderClusterConfiguration{})
	if err == nil {
		t.Fatal("expected error for unsupported source version")
	}
}

func TestConvertObject_UnsupportedDesiredVersion(t *testing.T) {
	h := &ConversionHandler{}
	raw := makeV1NodeGroupJSON("test", v1.NodeTypeStatic)

	_, err := h.convertObject(raw, "deckhouse.io/v999", &ProviderClusterConfiguration{})
	if err == nil {
		t.Fatal("expected error for unsupported desired version")
	}
}

func TestHandleConversion_MultipleObjects_V1ToAlpha2(t *testing.T) {
	// Python: test_should_convert_from_v1_to_alpha2
	// Tests that multiple objects are converted correctly with nodeType mapping
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{}

	req := &apix.ConversionRequest{
		UID:               "test-uid",
		DesiredAPIVersion: "deckhouse.io/v1alpha2",
		Objects: []runtime.RawExtension{
			{Raw: makeV1NodeGroupJSON("master", v1.NodeTypeCloudPermanent)},
			{Raw: makeV1NodeGroupJSON("worker", v1.NodeTypeStatic)},
			{Raw: makeV1NodeGroupJSON("worker-small-a2", v1.NodeTypeCloudEphemeral)},
			{Raw: makeV1NodeGroupJSON("worker-static", v1.NodeTypeCloudStatic)},
		},
	}

	resp := h.handleConversion(req, cfg)
	if resp.Result.Status != "Success" {
		t.Fatalf("expected Success, got %s: %s", resp.Result.Status, resp.Result.Message)
	}
	if len(resp.ConvertedObjects) != 4 {
		t.Fatalf("expected 4 converted objects, got %d", len(resp.ConvertedObjects))
	}

	// master: CloudPermanent -> Hybrid
	ng0 := extractV1Alpha2NodeGroup(t, resp.ConvertedObjects[0].Raw)
	if ng0.Spec.NodeType != v1alpha2.NodeTypeHybrid {
		t.Fatalf("master: expected Hybrid, got %s", ng0.Spec.NodeType)
	}
	if ng0.Name != "master" {
		t.Fatalf("expected name 'master', got %s", ng0.Name)
	}

	// worker: Static -> Static
	ng1 := extractV1Alpha2NodeGroup(t, resp.ConvertedObjects[1].Raw)
	if ng1.Spec.NodeType != v1alpha2.NodeTypeStatic {
		t.Fatalf("worker: expected Static, got %s", ng1.Spec.NodeType)
	}

	// worker-small-a2: CloudEphemeral -> Cloud
	ng2 := extractV1Alpha2NodeGroup(t, resp.ConvertedObjects[2].Raw)
	if ng2.Spec.NodeType != v1alpha2.NodeTypeCloud {
		t.Fatalf("worker-small-a2: expected Cloud, got %s", ng2.Spec.NodeType)
	}

	// worker-static: CloudStatic -> Hybrid
	ng3 := extractV1Alpha2NodeGroup(t, resp.ConvertedObjects[3].Raw)
	if ng3.Spec.NodeType != v1alpha2.NodeTypeHybrid {
		t.Fatalf("worker-static: expected Hybrid, got %s", ng3.Spec.NodeType)
	}
}

func TestHandleConversion_MultipleObjects_Alpha2ToV1(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "frontend"}},
	}

	req := &apix.ConversionRequest{
		UID:               "test-uid",
		DesiredAPIVersion: "deckhouse.io/v1",
		Objects: []runtime.RawExtension{
			{Raw: makeV1Alpha2NodeGroupJSON("ng1", v1alpha2.NodeTypeCloud)},
			{Raw: makeV1Alpha2NodeGroupJSON("ng2", v1alpha2.NodeTypeStatic)},
			{Raw: makeV1Alpha2NodeGroupJSON("master", v1alpha2.NodeTypeHybrid)},
			{Raw: makeV1Alpha2NodeGroupJSON("frontend", v1alpha2.NodeTypeHybrid)},
			{Raw: makeV1Alpha2NodeGroupJSON("backend", v1alpha2.NodeTypeHybrid)},
		},
	}

	resp := h.handleConversion(req, cfg)
	if resp.Result.Status != "Success" {
		t.Fatalf("expected Success, got %s: %s", resp.Result.Status, resp.Result.Message)
	}
	if len(resp.ConvertedObjects) != 5 {
		t.Fatalf("expected 5 converted objects, got %d", len(resp.ConvertedObjects))
	}

	// ng1: Cloud -> CloudEphemeral
	ng1 := extractV1NodeGroup(t, resp.ConvertedObjects[0].Raw)
	if ng1.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		t.Fatalf("ng1: expected CloudEphemeral, got %s", ng1.Spec.NodeType)
	}

	// ng2: Static -> Static
	ng2 := extractV1NodeGroup(t, resp.ConvertedObjects[1].Raw)
	if ng2.Spec.NodeType != v1.NodeTypeStatic {
		t.Fatalf("ng2: expected Static, got %s", ng2.Spec.NodeType)
	}

	// master: Hybrid -> CloudPermanent (always for master)
	ng3 := extractV1NodeGroup(t, resp.ConvertedObjects[2].Raw)
	if ng3.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("master: expected CloudPermanent, got %s", ng3.Spec.NodeType)
	}

	// frontend: Hybrid -> CloudPermanent (in provider config)
	ng4 := extractV1NodeGroup(t, resp.ConvertedObjects[3].Raw)
	if ng4.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("frontend: expected CloudPermanent (in provider config), got %s", ng4.Spec.NodeType)
	}

	// backend: Hybrid -> CloudStatic (not in provider config)
	ng5 := extractV1NodeGroup(t, resp.ConvertedObjects[4].Raw)
	if ng5.Spec.NodeType != v1.NodeTypeCloudStatic {
		t.Fatalf("backend: expected CloudStatic (not in provider config), got %s", ng5.Spec.NodeType)
	}
}

func TestServeHTTP_FullRoundTrip(t *testing.T) {
	s := newScheme()
	sec := providerSecretWithNodeGroups("frontend")
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sec).Build()

	handler := &ConversionHandler{Client: c, Scheme: s}
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
	handler := &ConversionHandler{Client: c, Scheme: s}

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
	handler := &ConversionHandler{Client: c, Scheme: s}

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

func TestServeHTTP_NoProviderSecret_StaticCluster(t *testing.T) {
	// Simulates Static cluster where d8-provider-cluster-configuration doesn't exist
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build() // no secret

	handler := &ConversionHandler{Client: c, Scheme: s}
	review := buildConversionReview(
		"uid-456",
		"deckhouse.io/v1",
		makeV1Alpha2NodeGroupJSON("master", v1alpha2.NodeTypeHybrid),
		makeV1Alpha2NodeGroupJSON("worker", v1alpha2.NodeTypeHybrid),
	)

	result := doConversionRequest(t, handler, review)

	if result.Response.Result.Status != "Success" {
		t.Fatalf("expected Success even without provider secret, got: %s", result.Response.Result.Message)
	}

	// master is always CloudPermanent regardless of provider config
	ng1 := extractV1NodeGroup(t, result.Response.ConvertedObjects[0].Raw)
	if ng1.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("master: expected CloudPermanent, got %s", ng1.Spec.NodeType)
	}

	// worker without provider config -> CloudStatic
	ng2 := extractV1NodeGroup(t, result.Response.ConvertedObjects[1].Raw)
	if ng2.Spec.NodeType != v1.NodeTypeCloudStatic {
		t.Fatalf("worker: expected CloudStatic (no provider config), got %s", ng2.Spec.NodeType)
	}
}

func TestLoadProviderConfig_Success(t *testing.T) {
	s := newScheme()
	sec := providerSecretWithNodeGroups("ng1", "ng2")
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sec).Build()
	h := &ConversionHandler{Client: c, Scheme: s}

	cfg, err := h.loadProviderConfig(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.NodeGroups) != 2 {
		t.Fatalf("expected 2 node groups, got %d", len(cfg.NodeGroups))
	}
	if cfg.NodeGroups[0].Name != "ng1" || cfg.NodeGroups[1].Name != "ng2" {
		t.Fatalf("unexpected node group names: %v", cfg.NodeGroups)
	}
}

func TestLoadProviderConfig_SecretNotFound(t *testing.T) {
	// Secret not found should return empty config, not error (Static cluster case)
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	h := &ConversionHandler{Client: c, Scheme: s}

	cfg, err := h.loadProviderConfig(context.Background())
	if err != nil {
		t.Fatalf("expected no error for missing secret (Static cluster), got: %v", err)
	}
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
	h := &ConversionHandler{Client: c, Scheme: s}

	cfg, err := h.loadProviderConfig(context.Background())
	if err != nil {
		t.Fatalf("expected no error for missing key, got: %v", err)
	}
	if len(cfg.NodeGroups) != 0 {
		t.Fatalf("expected empty config when key missing, got %d groups", len(cfg.NodeGroups))
	}
}

func TestLoadProviderConfig_InvalidYAML(t *testing.T) {
	s := newScheme()
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-provider-cluster-configuration", Namespace: "kube-system"},
		Data:       map[string][]byte{"cloud-provider-cluster-configuration.yaml": []byte("not: valid: yaml: {{{{")},
	}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sec).Build()
	h := &ConversionHandler{Client: c, Scheme: s}

	_, err := h.loadProviderConfig(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestRoundTrip_V1Alpha2_V1_V1Alpha2(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{
		NodeGroups: []ProviderNodeGroup{{Name: "frontend"}},
	}

	// Start with v1alpha2 Hybrid
	original := makeV1Alpha2NodeGroupJSON("frontend", v1alpha2.NodeTypeHybrid)

	// Convert to v1
	v1Result, err := h.convertObject(original, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("v1alpha2 -> v1 failed: %v", err)
	}
	ngV1 := extractV1NodeGroup(t, v1Result)
	if ngV1.Spec.NodeType != v1.NodeTypeCloudPermanent {
		t.Fatalf("expected CloudPermanent in v1, got %s", ngV1.Spec.NodeType)
	}

	// Convert back to v1alpha2
	v1a2Result, err := h.convertObject(v1Result, "deckhouse.io/v1alpha2", cfg)
	if err != nil {
		t.Fatalf("v1 -> v1alpha2 failed: %v", err)
	}
	ngV1A2 := extractV1Alpha2NodeGroup(t, v1a2Result)
	if ngV1A2.Spec.NodeType != v1alpha2.NodeTypeHybrid {
		t.Fatalf("expected Hybrid after round-trip, got %s", ngV1A2.Spec.NodeType)
	}
	if ngV1A2.Name != "frontend" {
		t.Fatalf("expected name 'frontend' after round-trip, got %s", ngV1A2.Name)
	}
}

func TestRoundTrip_V1_V1Alpha2_V1(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{}

	// Start with v1 CloudEphemeral
	original := makeV1NodeGroupJSON("ephemeral", v1.NodeTypeCloudEphemeral)

	// Convert to v1alpha2
	v1a2Result, err := h.convertObject(original, "deckhouse.io/v1alpha2", cfg)
	if err != nil {
		t.Fatalf("v1 -> v1alpha2 failed: %v", err)
	}
	ngV1A2 := extractV1Alpha2NodeGroup(t, v1a2Result)
	if ngV1A2.Spec.NodeType != v1alpha2.NodeTypeCloud {
		t.Fatalf("expected Cloud in v1alpha2, got %s", ngV1A2.Spec.NodeType)
	}

	// Convert back to v1
	v1Result, err := h.convertObject(v1a2Result, "deckhouse.io/v1", cfg)
	if err != nil {
		t.Fatalf("v1alpha2 -> v1 failed: %v", err)
	}
	ngV1 := extractV1NodeGroup(t, v1Result)
	if ngV1.Spec.NodeType != v1.NodeTypeCloudEphemeral {
		t.Fatalf("expected CloudEphemeral after round-trip, got %s", ngV1.Spec.NodeType)
	}
}

func TestConvertInstance_V1Alpha1ToV1Alpha2(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{}

	raw := makeV1Alpha1InstanceJSON("worker-1", v1alpha1.InstanceRunning)
	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instance := extractV1Alpha2Instance(t, result)
	if instance.APIVersion != "deckhouse.io/v1alpha2" {
		t.Fatalf("expected apiVersion deckhouse.io/v1alpha2, got %s", instance.APIVersion)
	}
	if instance.Status.Phase != v1alpha2.InstancePhaseRunning {
		t.Fatalf("expected phase Running, got %s", instance.Status.Phase)
	}
	if instance.Spec.NodeRef.Name != "worker-1" {
		t.Fatalf("expected spec.nodeRef.name worker-1, got %s", instance.Spec.NodeRef.Name)
	}
	if instance.Spec.MachineRef == nil || instance.Spec.MachineRef.Name != "worker-1" {
		t.Fatalf("expected spec.machineRef.name worker-1, got %#v", instance.Spec.MachineRef)
	}
	if instance.Spec.ClassReference == nil || instance.Spec.ClassReference.Kind != "DVPInstanceClass" || instance.Spec.ClassReference.Name != "worker" {
		t.Fatalf("unexpected spec.classReference: %#v", instance.Spec.ClassReference)
	}
}

func TestConvertInstance_V1Alpha1ToV1Alpha2_WithoutClassReference(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{}

	raw := makeV1Alpha1InstanceJSONWithoutClassReference("worker-3", v1alpha1.InstanceRunning)
	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instance := extractV1Alpha2Instance(t, result)
	if instance.Spec.ClassReference != nil {
		t.Fatalf("expected spec.classReference to be nil, got %#v", instance.Spec.ClassReference)
	}
}

func TestConvertInstance_V1Alpha1ToV1Alpha2_WithoutMachineRef(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{}

	raw := makeV1Alpha1InstanceJSONWithoutMachineRef("worker-4", v1alpha1.InstanceRunning)
	result, err := h.convertObject(raw, "deckhouse.io/v1alpha2", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instance := extractV1Alpha2Instance(t, result)
	if instance.Spec.MachineRef != nil {
		t.Fatalf("expected spec.machineRef to be nil, got %#v", instance.Spec.MachineRef)
	}
}

func TestConvertInstance_V1Alpha2ToV1Alpha1(t *testing.T) {
	h := &ConversionHandler{}
	cfg := &ProviderClusterConfiguration{}

	raw := makeV1Alpha2InstanceJSON("worker-2", v1alpha2.InstancePhaseTerminating)
	result, err := h.convertObject(raw, "deckhouse.io/v1alpha1", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	instance := extractV1Alpha1Instance(t, result)
	if instance.APIVersion != "deckhouse.io/v1alpha1" {
		t.Fatalf("expected apiVersion deckhouse.io/v1alpha1, got %s", instance.APIVersion)
	}
	if instance.Status.CurrentStatus.Phase != v1alpha1.InstanceTerminating {
		t.Fatalf("expected phase Terminating, got %s", instance.Status.CurrentStatus.Phase)
	}
	if instance.Status.NodeRef.Name != "worker-2" {
		t.Fatalf("expected status.nodeRef.name worker-2, got %s", instance.Status.NodeRef.Name)
	}
	if instance.Status.MachineRef.Name != "worker-2" {
		t.Fatalf("expected status.machineRef.name worker-2, got %s", instance.Status.MachineRef.Name)
	}
	if instance.Status.ClassReference.Kind != "DVPInstanceClass" || instance.Status.ClassReference.Name != "worker" {
		t.Fatalf("unexpected status.classReference: %#v", instance.Status.ClassReference)
	}
}
