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

func nodeUserObject(name string, uid int64, nodeGroups []string, passwordHash string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeUser"})
	u.SetName(name)
	groups := make([]any, 0, len(nodeGroups))
	for _, g := range nodeGroups {
		groups = append(groups, g)
	}
	spec := map[string]any{
		"uid":          uid,
		"nodeGroups":   groups,
		"sshPublicKey": "ssh-rsa AAAB",
	}
	if passwordHash != "" {
		spec["passwordHash"] = passwordHash
	}
	u.Object["spec"] = spec
	return u
}

func nodeUserReader(t *testing.T, existing ...*unstructured.Unstructured) client.Reader {
	t.Helper()
	s := runtime.NewScheme()
	s.AddKnownTypeWithName(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeUser"}, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeUserList"}, &unstructured.UnstructuredList{})
	builder := fake.NewClientBuilder().WithScheme(s)
	for _, u := range existing {
		builder = builder.WithObjects(u)
	}
	return builder.Build()
}

func makeNodeUserRequest(t *testing.T, op admissionv1.Operation, user *unstructured.Unstructured) admission.Request {
	t.Helper()
	raw, err := json.Marshal(user.Object)
	if err != nil {
		t.Fatal(err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: op,
			Name:      user.GetName(),
			Kind:      metav1.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeUser"},
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
}

func TestNodeUser_AllowUniqueUID(t *testing.T) {
	w := &NodeUserValidator{Reader: nodeUserReader(t,
		nodeUserObject("existing", 1001, []string{"worker"}, "hash"))}
	req := makeNodeUserRequest(t, admissionv1.Create, nodeUserObject("new-user", 1002, []string{"worker"}, "hash"))
	resp := w.Handle(t.Context(), req)
	if !resp.Allowed {
		t.Fatalf("expected allowed for unique uid, got: %v", resp.Result)
	}
	if len(resp.Warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", resp.Warnings)
	}
}

func TestNodeUser_DenyWhenExistingHasWildcard(t *testing.T) {
	w := &NodeUserValidator{Reader: nodeUserReader(t,
		nodeUserObject("existing", 1001, []string{"*"}, "hash"))}
	req := makeNodeUserRequest(t, admissionv1.Create, nodeUserObject("new-user", 1001, []string{"worker"}, "hash"))
	resp := w.Handle(t.Context(), req)
	if resp.Allowed {
		t.Fatal("expected denied: existing user with same uid in nodeGroup \"*\"")
	}
	expMessage := `The user with the uid: 1001 already exists in the nodeGroup: "*"`
	if resp.Result == nil || resp.Result.Message != expMessage {
		t.Fatalf("expected message %q, got: %v", expMessage, resp.Result)
	}
}

func TestNodeUser_DenyWildcardWhenSameUIDExists(t *testing.T) {
	w := &NodeUserValidator{Reader: nodeUserReader(t,
		nodeUserObject("existing", 1001, []string{"worker"}, "hash"))}
	req := makeNodeUserRequest(t, admissionv1.Create, nodeUserObject("new-user", 1001, []string{"*"}, "hash"))
	resp := w.Handle(t.Context(), req)
	if resp.Allowed {
		t.Fatal("expected denied: new wildcard user conflicts with existing same-uid user")
	}
	expMessage := "The user with the uid: 1001 already exists in the nodeGroup: *"
	if resp.Result == nil || resp.Result.Message != expMessage {
		t.Fatalf("expected message %q, got: %v", expMessage, resp.Result)
	}
}

func TestNodeUser_DenySharedNodeGroup(t *testing.T) {
	w := &NodeUserValidator{Reader: nodeUserReader(t,
		nodeUserObject("existing", 1001, []string{"worker", "front"}, "hash"))}
	req := makeNodeUserRequest(t, admissionv1.Create, nodeUserObject("new-user", 1001, []string{"front"}, "hash"))
	resp := w.Handle(t.Context(), req)
	if resp.Allowed {
		t.Fatal("expected denied: same uid in shared nodeGroup front")
	}
	expMessage := "The user with the uid: 1001 already exists in the nodeGroup: front"
	if resp.Result == nil || resp.Result.Message != expMessage {
		t.Fatalf("expected message %q, got: %v", expMessage, resp.Result)
	}
}

func TestNodeUser_DenySharedNodeGroupMultiGroupUser(t *testing.T) {
	w := &NodeUserValidator{Reader: nodeUserReader(t,
		nodeUserObject("existing", 1001, []string{"front"}, "hash"))}
	req := makeNodeUserRequest(t, admissionv1.Create, nodeUserObject("new-user", 1001, []string{"worker", "front"}, "hash"))
	resp := w.Handle(t.Context(), req)
	if resp.Allowed {
		t.Fatal("expected denied: multi-group user shares nodeGroup front with same-uid user")
	}
	expMessage := "The user with the uid: 1001 already exists in the nodeGroup: front"
	if resp.Result == nil || resp.Result.Message != expMessage {
		t.Fatalf("expected message %q, got: %v", expMessage, resp.Result)
	}
}

func TestNodeUser_AllowSameUIDDisjointNodeGroups(t *testing.T) {
	w := &NodeUserValidator{Reader: nodeUserReader(t,
		nodeUserObject("existing", 1001, []string{"front"}, "hash"))}
	req := makeNodeUserRequest(t, admissionv1.Create, nodeUserObject("new-user", 1001, []string{"worker"}, "hash"))
	resp := w.Handle(t.Context(), req)
	if !resp.Allowed {
		t.Fatalf("expected allowed for same uid in disjoint nodeGroups, got: %v", resp.Result)
	}
}

func TestNodeUser_UpdateExcludesSelf(t *testing.T) {
	self := nodeUserObject("self", 1001, []string{"worker"}, "hash")
	w := &NodeUserValidator{Reader: nodeUserReader(t, self)}
	req := makeNodeUserRequest(t, admissionv1.Update, nodeUserObject("self", 1001, []string{"worker"}, "hash"))
	resp := w.Handle(t.Context(), req)
	if !resp.Allowed {
		t.Fatalf("expected allowed on self-update, got: %v", resp.Result)
	}
}

func TestNodeUser_EmptyPasswordHashWarns(t *testing.T) {
	w := &NodeUserValidator{Reader: nodeUserReader(t)}
	req := makeNodeUserRequest(t, admissionv1.Create, nodeUserObject("new-user", 1001, []string{"worker"}, ""))
	resp := w.Handle(t.Context(), req)
	if !resp.Allowed {
		t.Fatalf("expected allowed with warning, got: %v", resp.Result)
	}
	expWarning := "Password hash is empty. This may not be secure and it may be prohibited by PAM settings."
	if len(resp.Warnings) != 1 || resp.Warnings[0] != expWarning {
		t.Fatalf("expected warning %q, got: %v", expWarning, resp.Warnings)
	}
}
