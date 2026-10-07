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
	corev1 "k8s.io/api/core/v1"
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
	for _, add := range []func(*runtime.Scheme) error{rbacv1.AddToScheme, corev1.AddToScheme, v1alpha3.AddToScheme} {
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

func updateRequest(t *testing.T, namespace, user string, prb *v1alpha3.ProjectRoleBinding) admission.Request {
	t.Helper()
	raw, err := json.Marshal(prb)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		Namespace: namespace,
		UserInfo:  authnv1.UserInfo{Username: user},
		Object:    runtime.RawExtension{Raw: raw},
		OldObject: runtime.RawExtension{Raw: raw},
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

func TestHandle_AllowsFinalizerRemovalDuringProjectTeardown(t *testing.T) {
	t.Parallel()
	// During project teardown the controller removes the PRB finalizer with an Update, by which
	// time the owning project is already gone. The placement check must not run on Update, or the
	// finalizer removal (and the project's main-namespace cleanup) would deadlock.
	v := newValidator(t)
	resp := v.Handle(context.Background(), updateRequest(t, "ghost", rolebinding.ControllerServiceAccount, binding("d8:project:viewer")))
	assert.True(t, resp.Allowed)
}

// The subjects of the stored binding are handed to the shared validator on UPDATE, so a
// ServiceAccount it already has is not refused again, as the controller's finalizer updates carry it.
// The handler passes on the warning that the ServiceAccount gets no rights from the binding.
func TestHandle_KeepsTheStoredServiceAccountSubjectsOnUpdate(t *testing.T) {
	t.Parallel()
	prb := binding("d8:project:viewer")
	prb.Spec.Subjects = []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "deployer", Namespace: "team-other"}}
	v := newValidator(t, realProject("team"))

	resp := v.Handle(context.Background(), createRequest(t, "team", rolebinding.ControllerServiceAccount, prb))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, `must belong to project "team"`)

	resp = v.Handle(context.Background(), updateRequest(t, "team", rolebinding.ControllerServiceAccount, prb))
	assert.True(t, resp.Allowed, resp.Result)
	assert.Contains(t, resp.Warnings, `ServiceAccount team-other/deployer is no longer in a namespace of project "team" and gets no rights from this binding`)
}

// The administrators binding as the controller writes it from spec.administrators passes, though one
// of them names the ServiceAccounts of another project: the handler gives the shared validator the
// name of the binding. The same object from Deckhouse is held to the project, and a user may not
// write a binding the controller manages at all.
func TestHandle_AdministratorsBindingOfTheController(t *testing.T) {
	t.Parallel()
	administrators := &v1alpha3.ProjectRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha3.ProjectAdministratorsBinding,
			Namespace: "team",
			Labels: map[string]string{
				v1alpha3.ResourceLabelHeritage:  v1alpha3.ResourceHeritageMultitenancy,
				v1alpha3.ResourceLabelProject:   "team",
				v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController,
			},
		},
		Spec: v1alpha3.ProjectRoleBindingSpec{
			Subjects: []rbacv1.Subject{
				{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "system:serviceaccount:ci:runner"},
				{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "alice@example.com"},
			},
			RoleRef: v1alpha3.RoleRef{Kind: "ClusterRole", Name: v1alpha3.ProjectAdministratorsRoleName},
		},
	}
	ci := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ci", Labels: map[string]string{v1alpha3.ResourceLabelProject: "ci"}}}
	v := newValidator(t, realProject("team"), ci)

	resp := v.Handle(context.Background(), createRequest(t, "team", rolebinding.ControllerServiceAccount, administrators))
	assert.True(t, resp.Allowed, resp.Result)

	resp = v.Handle(context.Background(), createRequest(t, "team", rolebinding.DeckhouseServiceAccount, administrators))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, `names ServiceAccounts of namespace "ci", which must belong to project "team"`)

	resp = v.Handle(context.Background(), createRequest(t, "team", "alice@example.com", administrators))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "is managed by the controller")
}

func TestHandle_RejectsCreateInNonMainNamespaceForUser(t *testing.T) {
	t.Parallel()
	// A normal user must not be able to create a PRB in a namespace that is not a project's main
	// namespace; the placement check still runs on Create regardless of the caller.
	v := newValidator(t)
	resp := v.Handle(context.Background(), createRequest(t, "ghost", "alice@example.com", binding("d8:project:viewer")))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "is not the main namespace of a project")
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
