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

package processed_status

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

func TestGetTimestamp_UsesEnvOverride(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", "2021-01-01T13:30:00Z")
	if got := GetTimestamp(); got != "2021-01-01T13:30:00Z" {
		t.Fatalf("GetTimestamp() = %q, want env-overridden value", got)
	}
}

func TestCalculateChecksum(t *testing.T) {
	t.Run("env override wins", func(t *testing.T) {
		t.Setenv("TEST_CONDITIONS_CALC_CHKSUM", "fixed")
		if got := CalculateChecksum("a", "b"); got != "fixed" {
			t.Fatalf("expected env override, got %q", got)
		}
	})

	t.Run("deterministic and order-independent", func(t *testing.T) {
		c1 := CalculateChecksum("a", "b", "c")
		c2 := CalculateChecksum("c", "b", "a")
		if c1 != c2 {
			t.Fatalf("checksum should be order-independent: %q vs %q", c1, c2)
		}
		if CalculateChecksum("a") == CalculateChecksum("b") {
			t.Fatal("different inputs should produce different checksums")
		}
	})
}

func TestApplyNodeGroupCRDFilter(t *testing.T) {
	u := newNodeGroupUnstructured("worker")
	u.SetAnnotations(map[string]string{"manual-rollout-id": "abc"})
	if err := unstructured.SetNestedField(u.Object, "Static", "spec", "nodeType"); err != nil {
		t.Fatalf("set nodeType: %v", err)
	}

	res, err := ApplyNodeGroupCRDFilter(u)
	if err != nil {
		t.Fatalf("ApplyNodeGroupCRDFilter: %v", err)
	}

	info, ok := res.(common.NodeGroupCRDInfo)
	if !ok {
		t.Fatalf("expected NodeGroupCRDInfo, got %T", res)
	}
	if info.Name != "worker" {
		t.Errorf("Name = %q, want worker", info.Name)
	}
	if info.ManualRolloutID != "abc" {
		t.Errorf("ManualRolloutID = %q, want abc", info.ManualRolloutID)
	}
	if string(info.Spec.NodeType) != "Static" {
		t.Errorf("Spec.NodeType = %q, want Static", info.Spec.NodeType)
	}
}

func TestApplyNodeGroupCRDFilter_ConversionError(t *testing.T) {
	u := newNodeGroupUnstructured("worker")
	// nodeType must be a string; an int triggers a conversion error.
	if err := unstructured.SetNestedField(u.Object, int64(5), "spec", "nodeType"); err != nil {
		t.Fatalf("set bad nodeType: %v", err)
	}
	if _, err := ApplyNodeGroupCRDFilter(u); err == nil {
		t.Fatal("expected conversion error for non-string nodeType")
	}
}

func newNodeGroupUnstructured(name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"})
	u.SetName(name)
	return u
}

type fakeClientHolder struct {
	client client.Client
}

func newFakeClientWithNG(t *testing.T, objs ...*unstructured.Unstructured) *fakeClientHolder {
	t.Helper()
	// Register NodeGroup as an unstructured type so the fake client preserves
	// the arbitrary status.deckhouse subtree (mirrors the real CRD, which the
	// typed v1.NodeGroup status would otherwise strip).
	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(gvk.GroupVersion().WithKind("NodeGroupList"), &unstructured.UnstructuredList{})

	builder := fake.NewClientBuilder().WithScheme(scheme)
	for _, o := range objs {
		builder = builder.WithObjects(o)
		builder = builder.WithStatusSubresource(o)
	}
	return &fakeClientHolder{client: builder.Build()}
}

func TestPatchProcessedStatus_NotFound(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", "2021-01-01T13:30:00Z")
	h := newFakeClientWithNG(t)
	s := &Service{Client: h.client}
	if err := s.PatchProcessedStatus(context.Background(), "missing"); err == nil {
		t.Fatal("expected error for missing nodegroup")
	}
}

func TestPatchProcessedStatus_SetsSyncedFalseWhenChecksumDiffers(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", "2021-01-01T13:30:00Z")
	t.Setenv("TEST_CONDITIONS_CALC_CHKSUM", "newsum")

	ng := newNodeGroupUnstructured("worker")
	if err := unstructured.SetNestedField(ng.Object, "oldsum", "status", "deckhouse", "observed", "checkSum"); err != nil {
		t.Fatalf("seed observed checksum: %v", err)
	}

	h := newFakeClientWithNG(t, ng)
	s := &Service{Client: h.client}
	if err := s.PatchProcessedStatus(context.Background(), "worker"); err != nil {
		t.Fatalf("PatchProcessedStatus: %v", err)
	}

	got := fetchNG(t, h, "worker")
	synced, _, _ := unstructured.NestedString(got.Object, "status", "deckhouse", "synced")
	if synced != "False" {
		t.Fatalf("expected synced=False, got %q", synced)
	}
	checkSum, _, _ := unstructured.NestedString(got.Object, "status", "deckhouse", "processed", "checkSum")
	if checkSum != "newsum" {
		t.Fatalf("expected processed checkSum=newsum, got %q", checkSum)
	}
	ts, _, _ := unstructured.NestedString(got.Object, "status", "deckhouse", "processed", "lastTimestamp")
	if ts != "2021-01-01T13:30:00Z" {
		t.Fatalf("expected lastTimestamp from env, got %q", ts)
	}
}

func TestPatchProcessedStatus_SetsSyncedTrueWhenChecksumMatches(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", "2021-01-01T13:30:00Z")
	t.Setenv("TEST_CONDITIONS_CALC_CHKSUM", "samesum")

	ng := newNodeGroupUnstructured("worker")
	if err := unstructured.SetNestedField(ng.Object, "samesum", "status", "deckhouse", "observed", "checkSum"); err != nil {
		t.Fatalf("seed observed checksum: %v", err)
	}

	h := newFakeClientWithNG(t, ng)
	s := &Service{Client: h.client}
	if err := s.PatchProcessedStatus(context.Background(), "worker"); err != nil {
		t.Fatalf("PatchProcessedStatus: %v", err)
	}

	got := fetchNG(t, h, "worker")
	synced, _, _ := unstructured.NestedString(got.Object, "status", "deckhouse", "synced")
	if synced != "True" {
		t.Fatalf("expected synced=True, got %q", synced)
	}
}

func TestPatchProcessedStatus_FilterError(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", "2021-01-01T13:30:00Z")

	ng := newNodeGroupUnstructured("worker")
	// non-string nodeType makes ApplyNodeGroupCRDFilter fail inside PatchProcessedStatus.
	if err := unstructured.SetNestedField(ng.Object, int64(5), "spec", "nodeType"); err != nil {
		t.Fatalf("set bad nodeType: %v", err)
	}

	h := newFakeClientWithNG(t, ng)
	s := &Service{Client: h.client}
	if err := s.PatchProcessedStatus(context.Background(), "worker"); err == nil {
		t.Fatal("expected filter error to propagate")
	}
}

func TestPatchProcessedStatus_DeckhouseStatusNotMap(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", "2021-01-01T13:30:00Z")

	ng := newNodeGroupUnstructured("worker")
	// status.deckhouse as a scalar makes the nested status writes fail.
	if err := unstructured.SetNestedField(ng.Object, "broken", "status", "deckhouse"); err != nil {
		t.Fatalf("set bad deckhouse status: %v", err)
	}

	h := newFakeClientWithNG(t, ng)
	s := &Service{Client: h.client}
	if err := s.PatchProcessedStatus(context.Background(), "worker"); err == nil {
		t.Fatal("expected error when status.deckhouse is not a map")
	}
}

func TestPatchProcessedStatus_ObservedChecksumNotString(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", "2021-01-01T13:30:00Z")
	t.Setenv("TEST_CONDITIONS_CALC_CHKSUM", "newsum")

	ng := newNodeGroupUnstructured("worker")
	if err := unstructured.SetNestedField(ng.Object, int64(7), "status", "deckhouse", "observed", "checkSum"); err != nil {
		t.Fatalf("seed bad observed checksum: %v", err)
	}

	h := newFakeClientWithNG(t, ng)
	s := &Service{Client: h.client}
	if err := s.PatchProcessedStatus(context.Background(), "worker"); err == nil {
		t.Fatal("expected error when observed checkSum is not a string")
	}
}

func TestPatchProcessedStatus_NoObservedChecksumSetsSyncedFalse(t *testing.T) {
	t.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", "2021-01-01T13:30:00Z")
	t.Setenv("TEST_CONDITIONS_CALC_CHKSUM", "newsum")

	ng := newNodeGroupUnstructured("worker")

	h := newFakeClientWithNG(t, ng)
	s := &Service{Client: h.client}
	if err := s.PatchProcessedStatus(context.Background(), "worker"); err != nil {
		t.Fatalf("PatchProcessedStatus: %v", err)
	}

	got := fetchNG(t, h, "worker")
	synced, _, _ := unstructured.NestedString(got.Object, "status", "deckhouse", "synced")
	if synced != "False" {
		t.Fatalf("expected synced=False when no observed checksum present, got %q", synced)
	}
}

func fetchNG(t *testing.T, h *fakeClientHolder, name string) *unstructured.Unstructured {
	t.Helper()
	got := newNodeGroupUnstructured(name)
	if err := h.client.Get(context.Background(), types.NamespacedName{Name: name}, got); err != nil {
		t.Fatalf("get nodegroup %s: %v", name, err)
	}
	return got
}
