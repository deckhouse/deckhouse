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

package projectrolebinding

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authnv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebinding "controller/internal/rolebinding"
)

func newValidator(t *testing.T, objs ...client.Object) *validator {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{rbacv1.AddToScheme, v1alpha3.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &validator{client: c}
}

func binding(role string) *v1alpha3.ProjectRoleBinding {
	return &v1alpha3.ProjectRoleBinding{
		Spec: v1alpha3.ProjectRoleBindingSpec{
			Subjects: []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: "User", Name: "alice"}},
			RoleRef:  v1alpha3.RoleRef{Kind: "ClusterRole", Name: role},
		},
	}
}

func createRequest(t *testing.T, namespace, user string, prb *v1alpha3.ProjectRoleBinding) admission.Request {
	t.Helper()
	raw, err := json.Marshal(prb)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Namespace: namespace,
		UserInfo:  authnv1.UserInfo{Username: user},
		Object:    runtime.RawExtension{Raw: raw},
	}}
}

func realProject(name string) *v1alpha3.Project {
	return &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func virtualProject(name string) *v1alpha3.Project {
	return &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{
		Name:   name,
		Labels: map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"},
	}}
}

func TestHandle_RejectsVirtualProjectNamespaceByName(t *testing.T) {
	t.Parallel()
	v := newValidator(t)
	resp := v.Handle(context.Background(), createRequest(t, "default", rolebinding.ControllerServiceAccount, binding("d8:project:viewer")))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "virtual project namespace")
}

func TestHandle_RejectsMissingProject(t *testing.T) {
	t.Parallel()
	v := newValidator(t)
	resp := v.Handle(context.Background(), createRequest(t, "ghost", rolebinding.ControllerServiceAccount, binding("d8:project:viewer")))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "is not the main namespace of a project")
}

func TestHandle_RejectsVirtualProjectByLabel(t *testing.T) {
	t.Parallel()
	v := newValidator(t, virtualProject("space"))
	resp := v.Handle(context.Background(), createRequest(t, "space", rolebinding.ControllerServiceAccount, binding("d8:project:viewer")))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "virtual project namespace")
}

func TestHandle_AllowsBindingInRealProject(t *testing.T) {
	t.Parallel()
	// real, non-virtual project; the controller binding an allowed (but absent) role is allowed.
	v := newValidator(t, realProject("team"))
	resp := v.Handle(context.Background(), createRequest(t, "team", rolebinding.ControllerServiceAccount, binding("d8:project:viewer")))
	assert.True(t, resp.Allowed)
}

func TestHandle_DeleteSkipsProjectExistenceChecks(t *testing.T) {
	t.Parallel()
	// delete reads the OldObject and skips the project-existence/virtual checks.
	v := newValidator(t)
	raw, err := json.Marshal(binding("d8:project:viewer"))
	require.NoError(t, err)
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
		Namespace: "ghost",
		UserInfo:  authnv1.UserInfo{Username: rolebinding.ControllerServiceAccount},
		OldObject: runtime.RawExtension{Raw: raw},
	}}
	resp := v.Handle(context.Background(), req)
	assert.True(t, resp.Allowed)
}
