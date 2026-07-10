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

package rolebinding

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authnv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebinding "controller/internal/rolebinding"
)

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		rbacv1.AddToScheme, authorizationv1.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func request(op admissionv1.Operation, user string) admission.Request {
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: op,
		UserInfo:  authnv1.UserInfo{Username: user},
	}}
}

func clusterRole(name string, labels, annotations map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels, Annotations: annotations}}
}

func TestValidate_ManagedByProtection(t *testing.T) {
	c := newClient(t)
	in := Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer", ManagedBy: v1alpha3.ManagedByController}

	// a regular user cannot touch a controller-managed binding
	resp := Validate(context.Background(), c, request(admissionv1.Update, "alice"), in)
	assert.False(t, resp.Allowed)

	// the controller can
	resp = Validate(context.Background(), c, request(admissionv1.Update, ControllerServiceAccount), in)
	assert.True(t, resp.Allowed)
}

func TestValidate_Delete(t *testing.T) {
	c := newClient(t)
	// delete of a non-managed binding is always allowed (no role checks)
	resp := Validate(context.Background(), c, request(admissionv1.Delete, "alice"), Input{})
	assert.True(t, resp.Allowed)
}

func TestValidate_RoleRefKind(t *testing.T) {
	c := newClient(t)
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "Role", RoleRefName: "d8:project:viewer"})
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "ClusterRole")
}

func TestValidate_RoleNotAllowed(t *testing.T) {
	c := newClient(t)
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "cluster-admin"})
	assert.False(t, resp.Allowed)
}

func TestValidate_ClusterRoleMissing(t *testing.T) {
	c := newClient(t)
	// privileged user, allowed prefix, role does not exist -> allowed with a warning
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer"})
	assert.True(t, resp.Allowed)
	assert.NotEmpty(t, resp.Warnings)
}

func TestValidate_ClusterRoleMissingFailsClosedForUser(t *testing.T) {
	c := newClient(t)
	// non-privileged user, allowed prefix, role does not exist -> denied (fail closed)
	resp := Validate(context.Background(), c, request(admissionv1.Create, "alice"),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer"})
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "does not exist")
}

func TestValidate_Subjects(t *testing.T) {
	c := newClient(t, clusterRole("d8:project:viewer", nil, nil))
	base := func(subjects ...rbacv1.Subject) Input {
		return Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer", Namespace: "proj", Subjects: subjects}
	}

	// invalid subject kind is rejected
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		base(rbacv1.Subject{Kind: "Robot", Name: "x"}))
	assert.False(t, resp.Allowed)

	// ServiceAccount without namespace is rejected
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		base(rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "sa"}))
	assert.False(t, resp.Allowed)

	// ServiceAccount from a foreign namespace is rejected
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		base(rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "sa", Namespace: "other"}))
	assert.False(t, resp.Allowed)

	// ServiceAccount from the project's main / additional namespace is accepted
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		base(rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "sa", Namespace: "proj-extra"}))
	assert.True(t, resp.Allowed)
}

func TestValidate_DisabledForProjects(t *testing.T) {
	c := newClient(t, clusterRole("d8:project:viewer", nil, map[string]string{rolebinding.AnnotationDisabledForProjects: "true"}))
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer"})
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "disabled")
}

func TestValidate_CustomRoleLabels(t *testing.T) {
	// custom role without the required kind label is rejected
	c := newClient(t, clusterRole("d8:custom:bad", nil, nil))
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:custom:bad"})
	assert.False(t, resp.Allowed)

	// custom role with a system scope is rejected
	c = newClient(t, clusterRole("d8:custom:sys", map[string]string{LabelRBACKind: "custom-role", LabelRBACScope: "system"}, nil))
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:custom:sys"})
	assert.False(t, resp.Allowed)

	// a well-formed custom role for the controller is allowed
	c = newClient(t, clusterRole("d8:custom:good", map[string]string{LabelRBACKind: "custom-role", LabelRBACScope: "namespace"}, nil))
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:custom:good"})
	assert.True(t, resp.Allowed)
}

func TestValidate_PrivilegeEscalationDeniedForUser(t *testing.T) {
	// a non-privileged user with a valid role still needs bind permission; the fake SAR returns
	// Allowed=false, so the request is denied.
	c := newClient(t, clusterRole("d8:project:viewer", nil, nil))
	resp := Validate(context.Background(), c, request(admissionv1.Create, "alice"),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer"})
	assert.False(t, resp.Allowed)
}
