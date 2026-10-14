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

package webhook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	// StaticMachine / StaticInstance are consumed as Unstructured.
	s.AddKnownTypeWithName(schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1alpha1", Kind: "StaticMachine"}, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1alpha1", Kind: "StaticMachineList"}, &unstructured.UnstructuredList{})
	s.AddKnownTypeWithName(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "StaticInstance"}, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "StaticInstanceList"}, &unstructured.UnstructuredList{})
	return s
}

func staticMachineForNG(name, ngName string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "infrastructure.cluster.x-k8s.io", Version: "v1alpha1", Kind: "StaticMachine"})
	u.SetName(name)
	u.SetNamespace(MachineNamespace)
	u.SetLabels(map[string]string{NodeGroupLabel: ngName})
	return u
}

func staticInstanceBoundTo(name, staticMachineName string, lbls map[string]string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "StaticInstance"})
	u.SetName(name)
	if lbls != nil {
		u.SetLabels(lbls)
	}
	u.Object["status"] = map[string]any{
		"machineRef": map[string]any{
			"name":      staticMachineName,
			"namespace": MachineNamespace,
		},
	}
	return u
}

// nodeGroupObject renders a minimal NodeGroup JSON suitable for admission
// requests — only the fields the validator reads.
func nodeGroupObject(name, nodeType string, ls *metav1.LabelSelector) runtime.RawExtension {
	obj := map[string]any{
		"metadata": map[string]any{"name": name},
		"spec":     map[string]any{"nodeType": nodeType},
	}
	if ls != nil {
		obj["spec"].(map[string]any)["staticInstances"] = map[string]any{
			"labelSelector": ls,
		}
	}
	raw, _ := json.Marshal(obj)
	return runtime.RawExtension{Raw: raw}
}

func makeNGRequest(op admissionv1.Operation, name string, newObj, oldObj runtime.RawExtension) admission.Request {
	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: op,
			Name:      name,
			Object:    newObj,
			OldObject: oldObj,
		},
	}
	return req
}

func TestNodeGroup_LabelSelectorImmutability_ModifyDenied(t *testing.T) {
	s := testScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	v := &NodeGroupValidator{Reader: c}

	oldLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "worker"}}
	newLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "infra"}}

	resp := v.Handle(context.Background(),
		makeNGRequest(admissionv1.Update, "worker",
			nodeGroupObject("worker", "Static", newLS),
			nodeGroupObject("worker", "Static", oldLS)))
	if resp.Allowed {
		t.Fatal("expected denied: labelSelector is immutable once set")
	}
}

func TestNodeGroup_LabelSelectorImmutability_RemoveDenied(t *testing.T) {
	s := testScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	v := &NodeGroupValidator{Reader: c}

	oldLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "worker"}}

	resp := v.Handle(context.Background(),
		makeNGRequest(admissionv1.Update, "worker",
			nodeGroupObject("worker", "Static", nil),
			nodeGroupObject("worker", "Static", oldLS)))
	if resp.Allowed {
		t.Fatal("expected denied: labelSelector cannot be removed once set")
	}
}

func TestNodeGroup_LabelSelectorAdd_AllowedWhenNoBoundSIs(t *testing.T) {
	s := testScheme()
	// No StaticMachine/StaticInstance for this NG — guard has nothing to
	// inspect and must allow.
	c := fake.NewClientBuilder().WithScheme(s).Build()
	v := &NodeGroupValidator{Reader: c}

	newLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "worker"}}

	resp := v.Handle(context.Background(),
		makeNGRequest(admissionv1.Update, "worker",
			nodeGroupObject("worker", "Static", newLS),
			nodeGroupObject("worker", "Static", nil)))
	if !resp.Allowed {
		t.Fatalf("expected allowed: no bound StaticInstances, got: %s", resp.Result.Message)
	}
}

func TestNodeGroup_LabelSelectorAdd_DeniedWhenBoundSIMismatch(t *testing.T) {
	s := testScheme()
	sm := staticMachineForNG("worker-abcd", "worker")
	si := staticInstanceBoundTo("host-1", "worker-abcd", map[string]string{"role": "infra"})
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sm, si).Build()
	v := &NodeGroupValidator{Reader: c}

	newLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "worker"}}

	resp := v.Handle(context.Background(),
		makeNGRequest(admissionv1.Update, "worker",
			nodeGroupObject("worker", "Static", newLS),
			nodeGroupObject("worker", "Static", nil)))
	if resp.Allowed {
		t.Fatal("expected denied: a bound StaticInstance would fall outside the new selector")
	}
	if resp.Result == nil || !strings.Contains(resp.Result.Message, "host-1") {
		t.Fatalf("expected denial to name the offending StaticInstance, got: %v", resp.Result)
	}
}

func TestNodeGroup_LabelSelectorAdd_AllowedWhenBoundSIsMatch(t *testing.T) {
	s := testScheme()
	sm := staticMachineForNG("worker-abcd", "worker")
	si := staticInstanceBoundTo("host-1", "worker-abcd", map[string]string{"role": "worker"})
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sm, si).Build()
	v := &NodeGroupValidator{Reader: c}

	newLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "worker"}}

	resp := v.Handle(context.Background(),
		makeNGRequest(admissionv1.Update, "worker",
			nodeGroupObject("worker", "Static", newLS),
			nodeGroupObject("worker", "Static", nil)))
	if !resp.Allowed {
		t.Fatalf("expected allowed: bound StaticInstance already matches the new selector, got: %s", resp.Result.Message)
	}
}

func TestNodeGroup_LabelSelectorAdd_IgnoresOtherNodeGroups(t *testing.T) {
	s := testScheme()
	// A StaticInstance bound to a different NG's StaticMachine must not
	// affect admission on 'worker'.
	sm := staticMachineForNG("other-abcd", "other-ng")
	si := staticInstanceBoundTo("host-1", "other-abcd", map[string]string{"role": "infra"})
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(sm, si).Build()
	v := &NodeGroupValidator{Reader: c}

	newLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "worker"}}

	resp := v.Handle(context.Background(),
		makeNGRequest(admissionv1.Update, "worker",
			nodeGroupObject("worker", "Static", newLS),
			nodeGroupObject("worker", "Static", nil)))
	if !resp.Allowed {
		t.Fatalf("expected allowed: other NG's bound StaticInstance must be ignored, got: %s", resp.Result.Message)
	}
}

func TestNodeGroup_SkipsNonStaticNodeTypes(t *testing.T) {
	s := testScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	v := &NodeGroupValidator{Reader: c}

	oldLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "worker"}}
	newLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "infra"}}

	// A CloudEphemeral NodeGroup must not go through the guard — it does
	// not use staticInstances at all.
	resp := v.Handle(context.Background(),
		makeNGRequest(admissionv1.Update, "worker",
			nodeGroupObject("worker", "CloudEphemeral", newLS),
			nodeGroupObject("worker", "CloudEphemeral", oldLS)))
	if !resp.Allowed {
		t.Fatalf("expected allowed for CloudEphemeral: %s", resp.Result.Message)
	}
}

func TestNodeGroup_SkipsCreate(t *testing.T) {
	s := testScheme()
	c := fake.NewClientBuilder().WithScheme(s).Build()
	v := &NodeGroupValidator{Reader: c}

	newLS := &metav1.LabelSelector{MatchLabels: map[string]string{"role": "worker"}}
	resp := v.Handle(context.Background(),
		makeNGRequest(admissionv1.Create, "worker",
			nodeGroupObject("worker", "Static", newLS),
			runtime.RawExtension{}))
	if !resp.Allowed {
		t.Fatalf("expected allowed on CREATE (immutability only applies on UPDATE), got: %s", resp.Result.Message)
	}
}
