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
	"context"
	"encoding/json"
	"net/http"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1 "github.com/deckhouse/node-controller/api/deckhouse.io/v1"
)

// --- helpers ---

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = corev1.AddToScheme(s)
	_ = v1.AddToScheme(s)
	return s
}

func makeAdmissionRequest(t *testing.T, op admissionv1.Operation, ng *v1.NodeGroup, oldNG *v1.NodeGroup) admission.Request {
	t.Helper()
	raw, err := json.Marshal(ng)
	if err != nil {
		t.Fatal(err)
	}
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: op,
			Name:      ng.Name,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
	if oldNG != nil {
		oldRaw, err := json.Marshal(oldNG)
		if err != nil {
			t.Fatal(err)
		}
		req.OldObject = runtime.RawExtension{Raw: oldRaw}
	}
	return req
}

func baseNodeGroup(name string, nodeType v1.NodeType) *v1.NodeGroup {
	return &v1.NodeGroup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "deckhouse.io/v1",
			Kind:       "NodeGroup",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeGroupSpec{NodeType: nodeType},
	}
}

func clusterConfigSecret(clusterType, prefix, defaultCRI string, podSubnetPrefix int) *corev1.Secret {
	yaml := ""
	if clusterType != "" {
		yaml += "clusterType: " + clusterType + "\n"
	}
	if prefix != "" {
		yaml += "prefix: " + prefix + "\n"
	}
	if defaultCRI != "" {
		yaml += "defaultCRI: " + defaultCRI + "\n"
	}
	if podSubnetPrefix > 0 {
		yaml += "podSubnetNodeCIDRPrefix: " + string(rune('0'+podSubnetPrefix/10)) + string(rune('0'+podSubnetPrefix%10)) + "\n"
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-cluster-configuration", Namespace: "kube-system"},
		Data:       map[string][]byte{"cluster-configuration.yaml": []byte(yaml)},
	}
}

func providerConfigSecret(zones []string) *corev1.Secret {
	zonesJSON, _ := json.Marshal(struct {
		Zones []string `json:"zones"`
	}{Zones: zones})
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "d8-provider-cluster-configuration", Namespace: "kube-system"},
		Data:       map[string][]byte{"cloud-provider-discovery-data.json": zonesJSON},
	}
}

func moduleConfigGlobal(customTolerationKeys []string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "deckhouse.io", Version: "v1alpha1", Kind: "ModuleConfig",
	})
	obj.SetName("global")
	if len(customTolerationKeys) > 0 {
		keys := make([]interface{}, len(customTolerationKeys))
		for i, k := range customTolerationKeys {
			keys[i] = k
		}
		_ = unstructured.SetNestedSlice(obj.Object, keys, "spec", "settings", "modules", "placement", "customTolerationKeys")
	}
	return obj
}

// --- Tests ---

func TestValidation_NodeTypeImmutability(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	oldNG := baseNodeGroup("worker", v1.NodeTypeStatic)
	newNG := baseNodeGroup("worker", v1.NodeTypeCloudEphemeral)

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "UPDATE", newNG, oldNG))
	if resp.Allowed {
		t.Fatal("expected denied: nodeType change should be forbidden")
	}
	if resp.Result == nil || resp.Result.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got: %v", resp.Result)
	}
}

func TestValidation_NodeTypeImmutability_SameType(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	oldNG := baseNodeGroup("worker", v1.NodeTypeStatic)
	newNG := baseNodeGroup("worker", v1.NodeTypeStatic)

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "UPDATE", newNG, oldNG))
	if !resp.Allowed {
		t.Fatalf("expected allowed for same nodeType, got denied: %s", resp.Result.Message)
	}
}

func TestValidation_MinPerZoneGreaterThanMaxPerZone(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("ephemeral", v1.NodeTypeCloudEphemeral)
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		MinPerZone:     5,
		MaxPerZone:     2,
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: minPerZone > maxPerZone")
	}
}

func TestValidation_MinPerZoneLessOrEqualMaxPerZone(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("ephemeral", v1.NodeTypeCloudEphemeral)
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		MinPerZone:     2,
		MaxPerZone:     5,
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if !resp.Allowed {
		t.Fatalf("expected allowed: minPerZone <= maxPerZone, got: %s", resp.Result.Message)
	}
}

func TestValidation_DockerCRIForbidden(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.CRI = &v1.CRISpec{Type: v1.CRITypeDocker}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: Docker CRI should be forbidden")
	}
}

func TestValidation_ContainerdCRIAllowed(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.CRI = &v1.CRISpec{Type: v1.CRITypeContainerd}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if !resp.Allowed {
		t.Fatalf("expected allowed for Containerd CRI, got: %s", resp.Result.Message)
	}
}

func TestValidation_CRIConfigMismatch_ContainerdConfigWithDockerType(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	maxDl := 10
	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.CRI = &v1.CRISpec{
		Type:       v1.CRITypeDocker,
		Containerd: &v1.ContainerdSpec{MaxConcurrentDownloads: &maxDl},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: containerd config with Docker type")
	}
}

func TestValidation_RollingUpdateOnlyForCloudEphemeral(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.Disruptions = &v1.DisruptionsSpec{
		ApprovalMode: v1.DisruptionApprovalModeRollingUpdate,
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: RollingUpdate should only be allowed for CloudEphemeral")
	}
}

func TestValidation_RollingUpdateAllowedForCloudEphemeral(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("ephemeral", v1.NodeTypeCloudEphemeral)
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		MinPerZone: 1, MaxPerZone: 3,
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}
	ng.Spec.Disruptions = &v1.DisruptionsSpec{
		ApprovalMode: v1.DisruptionApprovalModeRollingUpdate,
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if !resp.Allowed {
		t.Fatalf("expected allowed: RollingUpdate for CloudEphemeral, got: %s", resp.Result.Message)
	}
}

func TestValidation_DuplicateTaints(t *testing.T) {
	s := newScheme()

	mc := moduleConfigGlobal([]string{"my-key"})
	c := fake.NewClientBuilder().WithScheme(s).WithObjects().WithRuntimeObjects(mc).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.NodeTemplate = &v1.NodeTemplate{
		Taints: []corev1.Taint{
			{Key: "my-key", Value: "a", Effect: corev1.TaintEffectNoSchedule},
			{Key: "my-key", Value: "b", Effect: corev1.TaintEffectNoSchedule},
		},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: duplicate taints (same key+effect)")
	}
}

func TestValidation_TaintsNotInCustomTolerationKeys(t *testing.T) {
	s := newScheme()

	mc := moduleConfigGlobal([]string{})
	c := fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(mc).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.NodeTemplate = &v1.NodeTemplate{
		Taints: []corev1.Taint{
			{Key: "custom-key", Value: "val", Effect: corev1.TaintEffectNoSchedule},
		},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: taint key not in customTolerationKeys")
	}
}

func TestValidation_TaintsStandardKeysAllowed(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.NodeTemplate = &v1.NodeTemplate{
		Taints: []corev1.Taint{
			{Key: "dedicated", Value: "monitoring", Effect: corev1.TaintEffectNoSchedule},
			{Key: "dedicated.deckhouse.io", Value: "monitoring", Effect: corev1.TaintEffectNoExecute},
		},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if !resp.Allowed {
		t.Fatalf("expected allowed for standard taint keys, got: %s", resp.Result.Message)
	}
}

func TestValidation_CloudNameLength(t *testing.T) {
	s := newScheme()

	// prefix "my-long-cluster-prefix" = 22 chars → max ng name = 63-22-1-21 = 19
	sec := clusterConfigSecret("Cloud", "my-long-cluster-prefix", "", 0)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sec).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("this-name-is-way-too-long-for-this-cluster", v1.NodeTypeCloudEphemeral)
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		MinPerZone: 1, MaxPerZone: 3,
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: cluster prefix + node group name too long")
	}
}

func TestValidation_CloudNameLengthOK(t *testing.T) {
	s := newScheme()

	sec := clusterConfigSecret("Cloud", "short", "", 0)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sec).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeCloudEphemeral)
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		MinPerZone: 1, MaxPerZone: 3,
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if !resp.Allowed {
		t.Fatalf("expected allowed: name is short enough, got: %s", resp.Result.Message)
	}
}

func TestValidation_UnknownZone(t *testing.T) {
	s := newScheme()

	provSec := providerConfigSecret([]string{"eu-west-1a", "eu-west-1b"})
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(provSec).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("ephemeral", v1.NodeTypeCloudEphemeral)
	ng.Spec.CloudInstances = &v1.CloudInstancesSpec{
		MinPerZone: 1, MaxPerZone: 3,
		Zones:          []string{"eu-west-1a", "us-east-1a"},
		ClassReference: v1.ClassReference{Kind: "AWSInstanceClass", Name: "worker"},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: unknown zone us-east-1a")
	}
}

func TestValidation_TopologyManagerWithoutResourceReservation(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.Kubelet = &v1.KubeletSpec{
		TopologyManager: &v1.TopologyManagerSpec{Policy: "single-numa-node"},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: topologyManager requires resourceReservation")
	}
}

func TestValidation_TopologyManagerStaticWithoutCPU(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.Kubelet = &v1.KubeletSpec{
		TopologyManager:     &v1.TopologyManagerSpec{Policy: "single-numa-node"},
		ResourceReservation: &v1.ResourceReservationSpec{Mode: "Static", Static: &v1.StaticResourceReservation{}},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: topologyManager + Static mode requires cpu")
	}
}

func TestValidation_LabelSelectorImmutability(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	oldNG := baseNodeGroup("worker", v1.NodeTypeStatic)
	oldNG.Spec.StaticInstances = &v1.StaticInstancesSpec{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"role": "worker"},
		},
	}

	newNG := baseNodeGroup("worker", v1.NodeTypeStatic)
	newNG.Spec.StaticInstances = &v1.StaticInstancesSpec{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"role": "infra"},
		},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "UPDATE", newNG, oldNG))
	if resp.Allowed {
		t.Fatal("expected denied: labelSelector is immutable once set")
	}
}

func TestValidation_LabelSelectorCanBeAdded(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	oldNG := baseNodeGroup("worker", v1.NodeTypeStatic)

	newNG := baseNodeGroup("worker", v1.NodeTypeStatic)
	newNG.Spec.StaticInstances = &v1.StaticInstancesSpec{
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"role": "worker"},
		},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "UPDATE", newNG, oldNG))
	if !resp.Allowed {
		t.Fatalf("expected allowed: adding labelSelector to existing NG, got: %s", resp.Result.Message)
	}
}

func TestValidation_DisruptionWindowsInvalidTime(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.Disruptions = &v1.DisruptionsSpec{
		ApprovalMode: v1.DisruptionApprovalModeAutomatic,
		Automatic: &v1.AutomaticDisruptionSpec{
			Windows: []v1.DisruptionWindow{
				{From: "25:00", To: "06:00", Days: []string{"Mon"}},
			},
		},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: invalid time format 25:00")
	}
}

func TestValidation_DisruptionWindowsInvalidDay(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.Disruptions = &v1.DisruptionsSpec{
		ApprovalMode: v1.DisruptionApprovalModeAutomatic,
		Automatic: &v1.AutomaticDisruptionSpec{
			Windows: []v1.DisruptionWindow{
				{From: "01:00", To: "06:00", Days: []string{"Monday"}},
			},
		},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if resp.Allowed {
		t.Fatal("expected denied: invalid day Monday (should be Mon)")
	}
}

func TestValidation_ValidDisruptionWindows(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.Disruptions = &v1.DisruptionsSpec{
		ApprovalMode: v1.DisruptionApprovalModeAutomatic,
		Automatic: &v1.AutomaticDisruptionSpec{
			Windows: []v1.DisruptionWindow{
				{From: "1:00", To: "6:00", Days: []string{"Mon", "Fri"}},
			},
		},
	}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if !resp.Allowed {
		t.Fatalf("expected allowed for valid disruption window, got: %s", resp.Result.Message)
	}
}

func TestValidation_MaxPodsWarning(t *testing.T) {
	s := newScheme()

	sec := clusterConfigSecret("", "", "", 24)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sec).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	maxPods := int32(500)
	ng := baseNodeGroup("worker", v1.NodeTypeStatic)
	ng.Spec.Kubelet = &v1.KubeletSpec{MaxPods: &maxPods}

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if !resp.Allowed {
		t.Fatalf("maxPods warning should not deny, got: %s", resp.Result.Message)
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("expected a warning about maxPods being too high")
	}
}

func TestValidation_SimpleCreateAllowed(t *testing.T) {
	s := newScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	w := &NodeGroupValidator{Client: c, decoder: admission.NewDecoder(s)}

	ng := baseNodeGroup("worker", v1.NodeTypeStatic)

	resp := w.Handle(context.Background(), makeAdmissionRequest(t, "CREATE", ng, nil))
	if !resp.Allowed {
		t.Fatalf("expected simple create to be allowed, got: %s", resp.Result.Message)
	}
}
