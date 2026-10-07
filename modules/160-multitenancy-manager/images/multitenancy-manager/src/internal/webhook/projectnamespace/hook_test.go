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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	return &validator{client: c, reader: c}
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

func updateRequest(t *testing.T, namespace string, pns *v1alpha3.ProjectNamespace) admission.Request {
	t.Helper()
	pns.Namespace = namespace
	raw, err := json.Marshal(pns)
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		Namespace: namespace,
		Object:    runtime.RawExtension{Raw: raw},
		OldObject: runtime.RawExtension{Raw: raw},
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

	// A GitOps sync applies the Project right before its ProjectNamespace, so the project may be in
	// the API server while the cache of the webhook lacks it.
	t.Run("a new project the cache has not seen yet is read from the API server", func(t *testing.T) {
		cached := newValidator(t)
		live := newValidator(t, projectObj("team-a", false))
		v := &validator{client: cached.client, reader: live.client}
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.True(t, resp.Allowed)
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

	// A project makes its namespace only when the controller gets to it, so the project itself is
	// what holds the name until then.
	t.Run("denied when a project has the resulting name", func(t *testing.T) {
		v := newValidator(t, projectObj("team-a", false), projectObj("team-a-backend", false))
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `the resulting namespace name "team-a-backend" is the name of a project`)
	})

	// "team" with "a-backend" and "team-a" with "backend" arrive at the same name from two projects.
	t.Run("denied when another ProjectNamespace claims the resulting name", func(t *testing.T) {
		other := pns("a-backend")
		other.Namespace = "team"
		v := newValidator(t, projectObj("team", false), projectObj("team-a", false), other)
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `the resulting namespace "team-a-backend" is already claimed by the "a-backend" ProjectNamespace of project "team"`)
	})

	t.Run("denied when another ProjectNamespace of the same project claims the resulting name", func(t *testing.T) {
		other := pns("cache")
		other.Namespace = "team-a"
		other.Spec.Name = "backend"
		v := newValidator(t, projectObj("team-a", false), other)
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `already claimed by the "cache" ProjectNamespace of project "team-a"`)
	})

	// A GitOps sync can apply a Project or another ProjectNamespace right before this one, so the
	// object that takes the name may be in the API server while the cache of the webhook lacks it.
	t.Run("a project the cache has not seen yet is read from the API server", func(t *testing.T) {
		cached := newValidator(t, projectObj("team-a", false))
		live := newValidator(t, projectObj("team-a", false), projectObj("team-a-backend", false))
		v := &validator{client: cached.client, reader: live.client}
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `the resulting namespace name "team-a-backend" is the name of a project`)
	})

	t.Run("a claim the cache has not seen yet is read from the API server", func(t *testing.T) {
		other := pns("a-backend")
		other.Namespace = "team"
		cached := newValidator(t, projectObj("team", false), projectObj("team-a", false))
		live := newValidator(t, projectObj("team", false), projectObj("team-a", false), other)
		v := &validator{client: cached.client, reader: live.client}
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `the resulting namespace "team-a-backend" is already claimed by the "a-backend" ProjectNamespace of project "team"`)
	})

	t.Run("a namespace the cache has not seen yet is read from the API server", func(t *testing.T) {
		other := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   "team-a-backend",
			Labels: map[string]string{v1alpha3.ResourceLabelProject: "team-b"},
		}}
		cached := newValidator(t, projectObj("team-a", false))
		live := newValidator(t, projectObj("team-a", false), other)
		v := &validator{client: cached.client, reader: live.client}
		resp := v.Handle(ctx, createRequest(t, "team-a", pns("backend")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `namespace "team-a-backend" already exists and is not owned by project "team-a"`)
	})

	// spec.name is immutable, so a claim that was free at CREATE stays this object's. Checking it
	// again on UPDATE would only refuse the finalizer removal that tears the object down.
	t.Run("the name checks do not run again on update", func(t *testing.T) {
		v := newValidator(t, projectObj("team-a-backend", false))
		resp := v.Handle(ctx, updateRequest(t, "team-a", pns("backend")))
		assert.True(t, resp.Allowed, resp.Result)
	})
}

// A binding can still name the ServiceAccounts of a namespace that is gone. A ProjectNamespace that
// makes a namespace under that name for another project would hand them to it, and is refused until
// the subjects are removed. Recreating a deleted ProjectNamespace of the project the binding is of
// brings the ServiceAccounts back to where they belong, and is allowed.
func TestHandle_NameAStoredBindingGrantsTo(t *testing.T) {
	ctx := context.Background()
	serviceAccount := func(namespace string) rbacv1.Subject {
		return rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "deployer", Namespace: namespace}
	}
	projectBinding := func(namespace string, subjects ...rbacv1.Subject) *v1alpha3.ProjectRoleBinding {
		return &v1alpha3.ProjectRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "leaky", Namespace: namespace},
			Spec:       v1alpha3.ProjectRoleBindingSpec{Subjects: subjects},
		}
	}

	t.Run("a binding of another project", func(t *testing.T) {
		v := newValidator(t, projectObj("proj", false), projectObj("proj-a", false), projectBinding("proj", serviceAccount("proj-a-x")))
		resp := v.Handle(ctx, createRequest(t, "proj-a", pns("x")))
		assert.False(t, resp.Allowed)
		assert.Equal(t, `the resulting namespace "proj-a-x" cannot be created, a ProjectRoleBinding of another project still grants rights to `+
			"ServiceAccounts of a namespace with this name. These subjects have to be removed from that binding first", resp.Result.Message)
	})

	t.Run("a cluster-wide binding", func(t *testing.T) {
		cluster := &v1alpha3.ClusterProjectRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "leaky"},
			Spec: v1alpha3.ClusterProjectRoleBindingSpec{Subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: "system:serviceaccount:tools-ci:runner"},
			}},
		}
		v := newValidator(t, projectObj("tools", false), cluster)
		resp := v.Handle(ctx, createRequest(t, "tools", pns("ci")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, `the resulting namespace "tools-ci" cannot be created, a ClusterProjectRoleBinding still grants rights`)
		assert.NotContains(t, resp.Result.Message, "leaky")
	})

	t.Run("a binding of the project itself", func(t *testing.T) {
		v := newValidator(t, projectObj("proj", false), projectBinding("proj",
			serviceAccount("proj-extra"),
			rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:proj-extra"},
		))
		resp := v.Handle(ctx, createRequest(t, "proj", pns("extra")))
		assert.True(t, resp.Allowed, resp.Result)
	})

	// No name changes hands when the namespace is there and the project's: a create of its
	// ProjectNamespace again is let through whatever a cluster-wide binding names. A namespace being
	// deleted is on its way out, and the name is checked as a new one.
	cluster := &v1alpha3.ClusterProjectRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "ci"},
		Spec:       v1alpha3.ClusterProjectRoleBindingSpec{Subjects: []rbacv1.Subject{serviceAccount("proj-extra")}},
	}
	own := func() *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj-extra", Labels: map[string]string{v1alpha3.ResourceLabelProject: "proj"}}}
	}
	t.Run("the namespace of the project exists", func(t *testing.T) {
		v := newValidator(t, projectObj("proj", false), own(), cluster)
		resp := v.Handle(ctx, createRequest(t, "proj", pns("extra")))
		assert.True(t, resp.Allowed, resp.Result)
	})
	t.Run("the namespace of the project is being deleted", func(t *testing.T) {
		deleting := own()
		deleting.Finalizers = []string{"kubernetes"}
		deleting.DeletionTimestamp = &metav1.Time{Time: time.Now()}
		v := newValidator(t, projectObj("proj", false), deleting, cluster)
		resp := v.Handle(ctx, createRequest(t, "proj", pns("extra")))
		assert.False(t, resp.Allowed)
		assert.Contains(t, resp.Result.Message, "a ClusterProjectRoleBinding still grants rights")
	})

	t.Run("bindings of other namespaces", func(t *testing.T) {
		v := newValidator(t, projectObj("proj", false), projectObj("proj-a", false), projectBinding("proj", serviceAccount("proj-a-y")))
		resp := v.Handle(ctx, createRequest(t, "proj-a", pns("x")))
		assert.True(t, resp.Allowed, resp.Result)
	})
}

func TestClaiming(t *testing.T) {
	ctx := context.Background()
	claim := func(namespace, name, suffix string) *v1alpha3.ProjectNamespace {
		return &v1alpha3.ProjectNamespace{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       v1alpha3.ProjectNamespaceSpec{Name: suffix},
		}
	}

	tests := []struct {
		name     string
		objects  []client.Object
		lookup   string
		expected string
	}{
		{name: "no claims", lookup: "team-a-backend"},
		{name: "a name without a dash is never claimed", objects: []client.Object{claim("team", "x", "a")}, lookup: "team"},
		{name: "claimed at the first dash", objects: []client.Object{claim("team", "x", "a-backend")}, lookup: "team-a-backend", expected: "team/x"},
		{name: "claimed at a later dash", objects: []client.Object{claim("team-a", "x", "backend")}, lookup: "team-a-backend", expected: "team-a/x"},
		{name: "another suffix is not a claim", objects: []client.Object{claim("team-a", "x", "frontend")}, lookup: "team-a-backend"},
		{name: "a longer suffix is not a claim", objects: []client.Object{claim("team-a", "x", "backend-2")}, lookup: "team-a-backend"},
		{name: "a shorter suffix is not a claim", objects: []client.Object{claim("team-a", "x", "backend")}, lookup: "team-a-backend-2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := newValidator(t, tt.objects...)
			got, err := Claiming(ctx, v.client, tt.lookup)
			require.NoError(t, err)
			if tt.expected == "" {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.expected, got.Namespace+"/"+got.Name)
		})
	}
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

func TestHandle_AllowsFinalizerRemovalDuringProjectTeardown(t *testing.T) {
	// During project teardown the controller removes the ProjectNamespace finalizer with an Update,
	// by which time the owning project (and its main namespace) is already gone. The placement
	// check must not run on Update, or the finalizer removal - and the additional-namespace
	// cleanup - would deadlock exactly like the ProjectRoleBinding regression (R2).
	v := newValidator(t)
	resp := v.Handle(context.Background(), updateRequest(t, "ghost", pns("backend")))
	assert.True(t, resp.Allowed)
}

func TestHandle_CreateInNonMainNamespaceStillDenied(t *testing.T) {
	// The placement gate remains enforced on Create regardless of the caller: a real create in a
	// namespace that is not a project's main namespace is rejected.
	v := newValidator(t)
	resp := v.Handle(context.Background(), createRequest(t, "ghost", pns("backend")))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "is not the main namespace of a project")
}
