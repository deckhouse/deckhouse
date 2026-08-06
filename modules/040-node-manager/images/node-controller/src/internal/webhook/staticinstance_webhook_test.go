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
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func staticInstanceObject(name, address string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "StaticInstance"})
	u.SetName(name)
	u.Object["spec"] = map[string]any{
		"address":        address,
		"credentialsRef": map[string]any{"name": "creds"},
	}
	return u
}

func staticInstanceReader(t *testing.T, existing ...*unstructured.Unstructured) client.Reader {
	t.Helper()
	s := runtime.NewScheme()
	s.AddKnownTypeWithName(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "StaticInstance"}, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "StaticInstanceList"}, &unstructured.UnstructuredList{})
	builder := fake.NewClientBuilder().WithScheme(s)
	for _, u := range existing {
		builder = builder.WithObjects(u)
	}
	return builder.Build()
}

func makeStaticInstanceRequest(t *testing.T, op admissionv1.Operation, instance *unstructured.Unstructured) admission.Request {
	t.Helper()
	raw, err := json.Marshal(instance.Object)
	if err != nil {
		t.Fatal(err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: op,
			Name:      instance.GetName(),
			Kind:      metav1.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "StaticInstance"},
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
}

func TestStaticInstance_AllowUniqueAddress(t *testing.T) {
	w := &StaticInstanceValidator{Reader: staticInstanceReader(t,
		staticInstanceObject("existing", "10.0.0.1"))}
	req := makeStaticInstanceRequest(t, admissionv1.Create, staticInstanceObject("new-instance", "10.0.0.2"))
	resp := w.Handle(t.Context(), req)
	if !resp.Allowed {
		t.Fatalf("expected allowed for unique address, got: %v", resp.Result)
	}
}

func TestStaticInstance_DenyDuplicateAddress(t *testing.T) {
	w := &StaticInstanceValidator{Reader: staticInstanceReader(t,
		staticInstanceObject("existing", "10.0.0.1"))}
	req := makeStaticInstanceRequest(t, admissionv1.Create, staticInstanceObject("new-instance", "10.0.0.1"))
	resp := w.Handle(t.Context(), req)
	if resp.Allowed {
		t.Fatal("expected denied: address already in use")
	}
	expMessage := `staticinstances.deckhouse.io "new-instance", static instance "existing" is already using the address "10.0.0.1"`
	if resp.Result == nil || resp.Result.Message != expMessage {
		t.Fatalf("expected message %q, got: %v", expMessage, resp.Result)
	}
}

func TestStaticInstance_UpdateExcludesSelf(t *testing.T) {
	w := &StaticInstanceValidator{Reader: staticInstanceReader(t,
		staticInstanceObject("self", "10.0.0.1"))}
	req := makeStaticInstanceRequest(t, admissionv1.Update, staticInstanceObject("self", "10.0.0.1"))
	resp := w.Handle(t.Context(), req)
	if !resp.Allowed {
		t.Fatalf("expected allowed on self-update with own address, got: %v", resp.Result)
	}
}
