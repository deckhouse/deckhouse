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

package nodegroup

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
	"github.com/deckhouse/node-controller/internal/register"
)

func cloudScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add corev1: %v", err)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("add v1: %v", err)
	}
	if err := mcmv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add mcm: %v", err)
	}
	if err := capiv1beta2.AddToScheme(scheme); err != nil {
		t.Fatalf("add capi: %v", err)
	}
	// scheme.AddKnownTypeWithName(common.MCMMachineDeploymentGVK, &unstructured.Unstructured{})
	// scheme.AddKnownTypeWithName(common.MCMMachineDeploymentGVK.GroupVersion().WithKind("MachineDeploymentList"), &unstructured.UnstructuredList{})
	// scheme.AddKnownTypeWithName(common.CAPIMachineDeploymentGVK, &unstructured.Unstructured{})
	// scheme.AddKnownTypeWithName(common.CAPIMachineDeploymentGVK.GroupVersion().WithKind("MachineDeploymentList"), &unstructured.UnstructuredList{})
	return scheme
}

func mcmMD(name, ng string, replicas int64, failureMsg string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(common.MCMMachineDeploymentGVK)
	u.SetName(name)
	u.SetNamespace(common.MachineNamespace)
	u.SetLabels(map[string]string{"node-group": ng})
	_ = unstructured.SetNestedField(u.Object, replicas, "spec", "replicas")
	if failureMsg != "" {
		_ = unstructured.SetNestedSlice(u.Object, []interface{}{
			map[string]interface{}{
				"name": "broken-machine",
				"lastOperation": map[string]interface{}{
					"description":    failureMsg,
					"lastUpdateTime": "2025-06-01T00:00:00Z",
				},
			},
		}, "status", "failedMachines")
	}
	return u
}

func TestReconcile_CloudEphemeral_StatusAndFailures(t *testing.T) {
	setEnv(t)
	ng := cloudEphemeralNG("cloud", []string{"a", "b"}, 1, 4)

	scheme := cloudScheme(t)
	cl := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(ng, mcmMD("cloud-md", "cloud", 3, "provider quota exceeded")).
		WithStatusSubresource(&v1.NodeGroup{}).
		Build()
	r := &Status{Base: register.Base{Client: cl, Recorder: record.NewFakeRecorder(10)}}

	doReconcile(t, r, "cloud")

	updated := &v1.NodeGroup{}
	if err := cl.Get(context.Background(), types.NamespacedName{Name: "cloud"}, updated); err != nil {
		t.Fatalf("get nodegroup: %v", err)
	}

	if updated.Status.Min != 2 { // 1 * 2 zones
		t.Errorf("Min = %d, want 2", updated.Status.Min)
	}
	if updated.Status.Max != 8 { // 4 * 2 zones
		t.Errorf("Max = %d, want 8", updated.Status.Max)
	}
	if updated.Status.Desired != 3 {
		t.Errorf("Desired = %d, want 3", updated.Status.Desired)
	}
	if len(updated.Status.LastMachineFailures) != 1 {
		t.Fatalf("expected 1 machine failure, got %d", len(updated.Status.LastMachineFailures))
	}
	if updated.Status.ConditionSummary == nil || updated.Status.ConditionSummary.Ready != "False" {
		t.Fatalf("expected ConditionSummary.Ready=False due to failure, got %#v", updated.Status.ConditionSummary)
	}
}

func cloudEphemeralNG(name string, zones []string, minPerZone, maxPerZone int32) *v1.NodeGroup {
	return &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.NodeGroupSpec{
			NodeType: v1.NodeTypeCloudEphemeral,
			CloudInstances: &v1.CloudInstancesSpec{
				Zones:      zones,
				MinPerZone: minPerZone,
				MaxPerZone: maxPerZone,
			},
		},
	}
}

func TestReconcile_LongErrorTruncatedAndEventEmitted(t *testing.T) {
	setEnv(t)
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
	r, rec := newReconciler(t, ng)

	longErr := strings.Repeat("x", 2000)
	ngWithError := ng.DeepCopy()
	ngWithError.Status.Error = longErr
	if err := r.Client.Status().Update(context.Background(), ngWithError); err != nil {
		t.Fatalf("set initial status: %v", err)
	}

	doReconcile(t, r, "worker")

	select {
	case event := <-rec.Events:
		// Event payload includes prefix "Warning MachineFailed "; ensure the
		// long error was truncated to 1024 bytes before recording.
		idx := strings.LastIndex(event, "xxxx")
		_ = idx
		count := strings.Count(event, "x")
		if count != 1024 {
			t.Fatalf("expected error truncated to 1024 x's, got %d", count)
		}
	default:
		t.Fatal("expected an event")
	}
}

func TestSecretToAllNodeGroups(t *testing.T) {
	setEnv(t)
	r, _ := newReconciler(t,
		&v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "a"}},
		&v1.NodeGroup{ObjectMeta: metav1.ObjectMeta{Name: "b"}},
	)
	reqs := r.secretToAllNodeGroups(context.Background(), &corev1.Secret{})
	if len(reqs) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(reqs))
	}
}
