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

package projectnamespace

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
)

func newValidator(t *testing.T, objs ...client.Object) *validator {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{corev1.AddToScheme, v1alpha3.AddToScheme} {
		require.NoError(t, add(scheme))
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	return &validator{client: c}
}

func createRequest(t *testing.T, namespace string, pns *v1alpha3.ProjectNamespace) admission.Request {
	t.Helper()
	pns.Namespace = namespace
	raw, err := json.Marshal(pns)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Create,
		Namespace: namespace,
		Object:    runtime.RawExtension{Raw: raw},
	}}
}

func pns(name string) *v1alpha3.ProjectNamespace {
	return &v1alpha3.ProjectNamespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1alpha3.ProjectNamespaceSpec{Name: name},
	}
}

func projectObj(name string, virtual bool) *v1alpha3.Project {
	p := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if virtual {
		p.Labels = map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"}
	}
	return p
}

func TestHandle_MainNamespaceOnly(t *testing.T) {
	ctx := context.Background()

	t.Run("allowed in a project main namespace", func(t *testing.T) {
		v := newValidator(t, projectObj("team-a", false))
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.True(t, resp.Allowed)
	})

	t.Run("denied when namespace is not a project main namespace", func(t *testing.T) {
		v := newValidator(t)
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "is not the main namespace of a project")
	})

	t.Run("denied in an additional namespace (no recursion)", func(t *testing.T) {
		// "team-a-backend" is an additional namespace: there is no Project with that name.
		v := newValidator(t, projectObj("team-a", false))
		resp := v.Handle(ctx, createRequest(t, "team-a-backend", pns("inner")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "is not the main namespace of a project")
	})

	t.Run("denied in a virtual project namespace", func(t *testing.T) {
		v := newValidator(t, projectObj("default", true))
		resp := v.Handle(ctx, createRequest(t, "default", pns("backend")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "virtual project namespace")
	})
}

func TestHandle_Length(t *testing.T) {
	ctx := context.Background()
	v := newValidator(t, projectObj("team-a", false))

	// "team-a-" (7) + suffix; suffix of 57 chars makes 64 > 63.
	long := strings.Repeat("a", 57)
	resp := v.Handle(ctx, createRequest(t, "team-a", pns(long)))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "exceeds the 63-character limit")

	// 56 chars => 63 exactly, allowed.
	ok := strings.Repeat("a", 56)
	resp = v.Handle(ctx, createRequest(t, "team-a", pns(ok)))
	assert.True(t, resp.Allowed)
}

func TestHandle_Collision(t *testing.T) {
	ctx := context.Background()

	t.Run("denied when target namespace is owned by another project", func(t *testing.T) {
		other := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   "team-a-backend",
			Labels: map[string]string{v1alpha3.ResourceLabelProject: "team-b"},
		}}
		v := newValidator(t, projectObj("team-a", false), other)
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "not owned by project")
	})

	t.Run("allowed when target namespace already owned by this project", func(t *testing.T) {
		owned := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   "team-a-backend",
			Labels: map[string]string{v1alpha3.ResourceLabelProject: "team-a"},
		}}
		v := newValidator(t, projectObj("team-a", false), owned)
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.True(t, resp.Allowed)
	})
}

func TestHandle_FeaturesPassthrough(t *testing.T) {
	ctx := context.Background()
	v := newValidator(t, projectObj("team-a", false))

	p := pns("backend")
	p.Spec.Features = []string{"monitoring", "vulnerabilityScanning"}
	// The "subset of project features" check is a no-op placeholder, so features pass through.
	resp := v.Handle(ctx, createRequest(t, "team-a", p))
	assert.True(t, resp.Allowed)
}

func TestHandle_DeleteAllowed(t *testing.T) {
	v := newValidator(t)
	resp := v.Handle(context.Background(), admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
		Namespace: "team-a",
	}})
	assert.True(t, resp.Allowed)
}
