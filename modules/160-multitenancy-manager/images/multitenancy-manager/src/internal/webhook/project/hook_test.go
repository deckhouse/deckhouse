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

package project

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authnv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

func TestValidateStandardFields(t *testing.T) {
	cases := []struct {
		name    string
		project *v1alpha3.Project
		denied  bool
	}{
		{
			name:    "empty is valid",
			project: &v1alpha3.Project{},
		},
		{
			name: "valid administrators and quota",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: "alice"}, {Kind: "Group", Name: "team"}},
				Quota:          corev1.ResourceList{"requests.cpu": resource.MustParse("2")},
			}},
		},
		{
			name: "invalid administrator kind",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "ServiceAccount", Name: "robot"}},
			}},
			denied: true,
		},
		{
			name: "empty administrator name",
			project: &v1alpha3.Project{Spec: v1alpha3.ProjectSpec{
				Administrators: []v1alpha3.Administrator{{Kind: "User", Name: ""}},
			}},
			denied: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			msg := validateStandardFields(tc.project)
			if tc.denied {
				assert.NotEmpty(t, msg)
			} else {
				assert.Empty(t, msg)
			}
		})
	}
}

func newValidator(t *testing.T, objs ...client.Object) *validator {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1alpha3.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &validator{client: c}
}

func managedProject(parameters map[string]any) *v1alpha3.Project {
	return &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "foo",
			Labels: map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace},
		},
		Spec: v1alpha3.ProjectSpec{Parameters: parameters},
	}
}

func updateRequest(t *testing.T, user string, old, updated *v1alpha3.Project) admission.Request {
	t.Helper()
	oldRaw, err := json.Marshal(old)
	require.NoError(t, err)
	newRaw, err := json.Marshal(updated)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		UserInfo:  authnv1.UserInfo{Username: user},
		Object:    runtime.RawExtension{Raw: newRaw},
		OldObject: runtime.RawExtension{Raw: oldRaw},
	}}
}

func TestHandle_ManagedByNamespaceEditProtection(t *testing.T) {
	v := newValidator(t)
	ctx := context.Background()

	t.Run("user spec edit is denied", func(t *testing.T) {
		old := managedProject(map[string]any{"namespace": map[string]any{"labels": map[string]any{"a": "1"}}})
		updated := managedProject(map[string]any{"namespace": map[string]any{"labels": map[string]any{"a": "2"}}})
		resp := v.Handle(ctx, updateRequest(t, "alice", old, updated))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "managed by its namespace")
	})

	t.Run("controller spec edit is allowed", func(t *testing.T) {
		old := managedProject(map[string]any{"namespace": map[string]any{"labels": map[string]any{"a": "1"}}})
		updated := managedProject(map[string]any{"namespace": map[string]any{"labels": map[string]any{"a": "2"}}})
		resp := v.Handle(ctx, updateRequest(t, rolebindingwebhook.ControllerServiceAccount, old, updated))
		assert.True(t, resp.Allowed)
	})

	t.Run("detach by removing the label is allowed", func(t *testing.T) {
		old := managedProject(map[string]any{"namespace": map[string]any{"labels": map[string]any{"a": "1"}}})
		updated := managedProject(map[string]any{"namespace": map[string]any{"labels": map[string]any{"a": "2"}}})
		updated.Labels = nil // detach
		resp := v.Handle(ctx, updateRequest(t, "alice", old, updated))
		assert.True(t, resp.Allowed)
	})

	t.Run("metadata-only edit keeping the label is allowed", func(t *testing.T) {
		old := managedProject(nil)
		updated := managedProject(nil)
		updated.Annotations = map[string]string{"note": "hi"}
		resp := v.Handle(ctx, updateRequest(t, "alice", old, updated))
		assert.True(t, resp.Allowed)
	})
}

func TestHandle_ManagedByNamespaceCreateBypass(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	v := newValidator(t, ns)
	ctx := context.Background()

	createReq := func(user string, labels map[string]string) admission.Request {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: labels}}
		raw, err := json.Marshal(project)
		require.NoError(t, err)
		return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			UserInfo:  authnv1.UserInfo{Username: user},
			Object:    runtime.RawExtension{Raw: raw},
		}}
	}

	// the controller may auto-wrap an existing namespace into a managed-by-namespace project
	resp := v.Handle(ctx, createReq(rolebindingwebhook.ControllerServiceAccount, map[string]string{v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace}))
	assert.True(t, resp.Allowed)

	// a regular user creating a project that collides with an existing namespace is denied
	resp = v.Handle(ctx, createReq("alice", nil))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "a namespace with its name exists")
}
