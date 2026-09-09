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

package clusterprojectrolebinding

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func newValidator(t *testing.T, objs ...client.Object) *validator {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		rbacv1.AddToScheme, authorizationv1.AddToScheme, v1alpha3.AddToScheme,
	} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &validator{client: c}
}

func clusterBinding(role string, labels map[string]string) *v1alpha3.ClusterProjectRoleBinding {
	return &v1alpha3.ClusterProjectRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Labels: labels},
		Spec: v1alpha3.ClusterProjectRoleBindingSpec{
			Subjects: []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: "Group", Name: "platform"}},
			RoleRef:  v1alpha3.RoleRef{Kind: role, Name: "d8:project:viewer"},
		},
	}
}

func request(t *testing.T, op admissionv1.Operation, user string, cprb *v1alpha3.ClusterProjectRoleBinding) admission.Request {
	t.Helper()
	raw, err := json.Marshal(cprb)
	require.NoError(t, err)
	ext := runtime.RawExtension{Raw: raw}
	ar := admissionv1.AdmissionRequest{
		Operation: op,
		UserInfo:  authnv1.UserInfo{Username: user},
	}
	if op == admissionv1.Delete {
		ar.OldObject = ext
	} else {
		ar.Object = ext
	}
	return admission.Request{AdmissionRequest: ar}
}

func TestHandle_AllowsValidBinding(t *testing.T) {
	t.Parallel()
	// the controller binding an allowed (but absent) role is allowed (delegates to shared Validate).
	v := newValidator(t)
	resp := v.Handle(context.Background(), request(t, admissionv1.Create, rolebinding.ControllerServiceAccount, clusterBinding("ClusterRole", nil)))
	assert.True(t, resp.Allowed)
}

func TestHandle_DeniesNonClusterRole(t *testing.T) {
	t.Parallel()
	v := newValidator(t)
	resp := v.Handle(context.Background(), request(t, admissionv1.Create, rolebinding.ControllerServiceAccount, clusterBinding("Role", nil)))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "ClusterRole")
}

func TestHandle_DeniesManagedByForUser(t *testing.T) {
	t.Parallel()
	// a controller-managed binding cannot be modified by a regular user (managed-by protection,
	// read from the object's labels by the hook).
	v := newValidator(t)
	managed := clusterBinding("ClusterRole", map[string]string{v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController})
	resp := v.Handle(context.Background(), request(t, admissionv1.Create, "alice", managed))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "managed by the controller")
}

func TestHandle_DeleteReadsManagedByFromOldObject(t *testing.T) {
	t.Parallel()
	// on delete the hook unmarshals OldObject; a managed-by binding deleted by a user is denied.
	v := newValidator(t)
	managed := clusterBinding("ClusterRole", map[string]string{v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController})
	resp := v.Handle(context.Background(), request(t, admissionv1.Delete, "alice", managed))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "managed by the controller")
}
