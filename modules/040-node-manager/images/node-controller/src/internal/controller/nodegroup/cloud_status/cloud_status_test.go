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

package cloud_status

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	capiv1beta2 "github.com/deckhouse/node-controller/api/cluster.x-k8s.io/v1beta2"
	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
	mcmv1alpha1 "github.com/deckhouse/node-controller/api/machine.sapcloud.io/v1alpha1"
	"github.com/deckhouse/node-controller/internal/controller/nodegroup/common"
)

func newScheme(t *testing.T) *runtime.Scheme {
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

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().WithScheme(newScheme(t)).WithObjects(objs...).Build()
}

func mcmMachineDeployment(name, ngName string, replicas int64) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(common.MCMMachineDeploymentGVK)
	u.SetName(name)
	u.SetNamespace(common.MachineNamespace)
	u.SetLabels(map[string]string{"node-group": ngName})
	_ = unstructured.SetNestedField(u.Object, replicas, "spec", "replicas")
	return u
}

func capiMachineDeployment(name, ngName string, replicas int64) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(common.CAPIMachineDeploymentGVK)
	u.SetName(name)
	u.SetNamespace(common.MachineNamespace)
	u.SetLabels(map[string]string{"node-group": ngName})
	_ = unstructured.SetNestedField(u.Object, replicas, "spec", "replicas")
	return u
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

func mcmMachine(name, ngName string) *mcmv1alpha1.Machine {
	m := &mcmv1alpha1.Machine{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: common.MachineNamespace},
	}
	m.Spec.NodeTemplateSpec.Labels = map[string]string{common.NodeGroupLabel: ngName}
	return m
}

func capiMachine(name, ngName string) *capiv1beta2.Machine {
	return &capiv1beta2.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: common.MachineNamespace,
			Labels:    map[string]string{"node-group": ngName},
		},
	}
}

func TestCompute_NonCloudEphemeralReturnsEmpty(t *testing.T) {
	ng := &v1.NodeGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeStatic},
	}
	s := &Service{Client: newClient(t)}
	res := s.Compute(context.Background(), ng)
	if res.Desired != 0 || res.Min != 0 || res.Max != 0 || res.Instances != 0 ||
		res.IsFrozen || res.LatestError != "" || len(res.Failures) != 0 {
		t.Fatalf("expected empty result for static NG, got %#v", res)
	}
}

func TestCompute_MinMaxFromZonesAndReplicas(t *testing.T) {
	ng := cloudEphemeralNG("worker", []string{"a", "b"}, 1, 3)
	s := &Service{Client: newClient(t,
		mcmMachineDeployment("worker-md", "worker", 4),
		mcmMachine("m1", "worker"),
		mcmMachine("m2", "worker"),
	)}

	res := s.Compute(context.Background(), ng)

	if res.Min != 2 { // minPerZone(1) * zones(2)
		t.Errorf("Min = %d, want 2", res.Min)
	}
	if res.Max != 6 { // maxPerZone(3) * zones(2)
		t.Errorf("Max = %d, want 6", res.Max)
	}
	if res.Desired != 4 { // replicas, above min
		t.Errorf("Desired = %d, want 4", res.Desired)
	}
	if res.Instances != 2 {
		t.Errorf("Instances = %d, want 2", res.Instances)
	}
}

func TestCompute_DesiredBumpedToMin(t *testing.T) {
	ng := cloudEphemeralNG("worker", []string{"a", "b", "c"}, 2, 5)
	s := &Service{Client: newClient(t,
		mcmMachineDeployment("worker-md", "worker", 1),
	)}

	res := s.Compute(context.Background(), ng)

	if res.Min != 6 {
		t.Errorf("Min = %d, want 6", res.Min)
	}
	if res.Desired != 6 { // bumped from replicas(1) up to Min(6)
		t.Errorf("Desired = %d, want 6 (bumped to min)", res.Desired)
	}
}

func TestCompute_CombinesMCMAndCAPIReplicasAndMachines(t *testing.T) {
	ng := cloudEphemeralNG("worker", []string{"a"}, 0, 10)
	s := &Service{Client: newClient(t,
		mcmMachineDeployment("worker-mcm", "worker", 2),
		capiMachineDeployment("worker-capi", "worker", 3),
		mcmMachine("m1", "worker"),
		capiMachine("cm1", "worker"),
		capiMachine("cm2", "worker"),
	)}

	res := s.Compute(context.Background(), ng)

	if res.Desired != 5 { // 2 (mcm) + 3 (capi)
		t.Errorf("Desired = %d, want 5", res.Desired)
	}
	if res.Instances != 3 { // 1 mcm + 2 capi
		t.Errorf("Instances = %d, want 3", res.Instances)
	}
}

func TestCompute_FrozenAndFailuresSortedLatestError(t *testing.T) {
	ng := cloudEphemeralNG("worker", []string{"a"}, 0, 5)

	md := mcmMachineDeployment("worker-md", "worker", 1)
	_ = unstructured.SetNestedSlice(md.Object, []interface{}{
		map[string]interface{}{"type": "Frozen", "status": "True"},
	}, "status", "conditions")
	_ = unstructured.SetNestedSlice(md.Object, []interface{}{
		map[string]interface{}{
			"name": "older",
			"lastOperation": map[string]interface{}{
				"description":    "older error",
				"lastUpdateTime": "2025-01-01T00:00:00Z",
				"state":          "Failed",
				"type":           "Create",
			},
		},
		map[string]interface{}{
			"name": "newer",
			"lastOperation": map[string]interface{}{
				"description":    "newer error",
				"lastUpdateTime": "2025-06-01T00:00:00Z",
			},
		},
	}, "status", "failedMachines")

	s := &Service{Client: newClient(t, md)}
	res := s.Compute(context.Background(), ng)

	if !res.IsFrozen {
		t.Error("expected IsFrozen=true")
	}
	if len(res.Failures) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(res.Failures))
	}
	if res.LatestError != "newer error" {
		t.Errorf("LatestError = %q, want newer error (sorted by time)", res.LatestError)
	}
}

func TestGetZonesCount(t *testing.T) {
	tests := []struct {
		name   string
		ng     *v1.NodeGroup
		secret *corev1.Secret
		want   int32
	}{
		{
			name: "zones from spec",
			ng:   cloudEphemeralNG("worker", []string{"a", "b", "c"}, 0, 1),
			want: 3,
		},
		{
			name: "no spec zones, fall back to provider secret",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, CloudInstances: &v1.CloudInstancesSpec{}},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: common.CloudProviderSecretName},
				Data:       map[string][]byte{"zones": []byte(`["z1","z2"]`)},
			},
			want: 2,
		},
		{
			name: "no spec zones, no secret",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, CloudInstances: &v1.CloudInstancesSpec{}},
			},
			want: 0,
		},
		{
			name: "secret with empty zones array",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, CloudInstances: &v1.CloudInstancesSpec{}},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: common.CloudProviderSecretName},
				Data:       map[string][]byte{"zones": []byte(`[]`)},
			},
			want: 0,
		},
		{
			name: "secret with malformed zones",
			ng: &v1.NodeGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "worker"},
				Spec:       v1.NodeGroupSpec{NodeType: v1.NodeTypeCloudEphemeral, CloudInstances: &v1.CloudInstancesSpec{}},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: common.CloudProviderSecretName},
				Data:       map[string][]byte{"zones": []byte(`not-json`)},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object
			if tt.secret != nil {
				objs = append(objs, tt.secret)
			}
			s := &Service{Client: newClient(t, objs...)}
			if got := s.getZonesCount(context.Background(), tt.ng); got != tt.want {
				t.Fatalf("getZonesCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestParseMCMFailedMachines(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"failedMachines": []interface{}{
				map[string]interface{}{
					"name":       "m1",
					"providerID": "pid1",
					"ownerRef":   "owner1",
					"lastOperation": map[string]interface{}{
						"description":    "boom",
						"lastUpdateTime": "2025-03-03T03:03:03Z",
						"state":          "Failed",
						"type":           "Create",
					},
				},
				// no lastOperation description -> skipped
				map[string]interface{}{"name": "m2"},
				// not a map -> skipped
				"garbage",
			},
		},
	}

	failures := parseMCMFailedMachines(obj)
	if len(failures) != 1 {
		t.Fatalf("expected 1 failure (others skipped), got %d", len(failures))
	}
	f := failures[0]
	if f.MachineName != "m1" || f.ProviderID != "pid1" || f.OwnerRef != "owner1" {
		t.Errorf("unexpected failure fields: %#v", f)
	}
	if f.Message != "boom" || f.State != "Failed" || f.Type != "Create" {
		t.Errorf("unexpected lastOperation fields: %#v", f)
	}
	if f.Time.Format("2006-01-02") != "2025-03-03" {
		t.Errorf("unexpected time: %v", f.Time)
	}
}

func TestParseMCMFailedMachines_NoFailedMachines(t *testing.T) {
	if got := parseMCMFailedMachines(map[string]interface{}{}); got != nil {
		t.Fatalf("expected nil for object without failedMachines, got %#v", got)
	}
}

func TestGetMachineDeploymentInfo_NoDeployments(t *testing.T) {
	s := &Service{Client: newClient(t)}
	info := s.getMachineDeploymentInfo(context.Background(), "worker")
	if info.Desired != 0 || info.IsFrozen || len(info.Failures) != 0 {
		t.Fatalf("expected empty info, got %#v", info)
	}
}

func TestGetMachinesCount_FiltersByNodeGroup(t *testing.T) {
	s := &Service{Client: newClient(t,
		mcmMachine("m1", "worker"),
		mcmMachine("m2", "other"),
		capiMachine("cm1", "worker"),
	)}
	if got := s.getMachinesCount(context.Background(), "worker"); got != 2 {
		t.Fatalf("getMachinesCount() = %d, want 2", got)
	}
}
