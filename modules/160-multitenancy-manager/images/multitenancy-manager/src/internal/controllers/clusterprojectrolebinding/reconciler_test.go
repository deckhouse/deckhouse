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
	"testing"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/rolebinding"
)

func newReconciler(t *testing.T, objs ...client.Object) (*Reconciler, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		rbacv1.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&v1alpha3.ClusterProjectRoleBinding{}).
		Build()
	return &Reconciler{Client: c}, c
}

func project(name string, virtual bool, namespaces ...string) *v1alpha3.Project {
	p := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if virtual {
		p.Labels = map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"}
	}
	for _, ns := range namespaces {
		kind := v1alpha3.NamespaceKindAdditional
		if ns == name {
			kind = v1alpha3.NamespaceKindMain
		}
		p.Status.Namespaces = append(p.Status.Namespaces, v1alpha3.NamespaceStatus{Name: ns, Kind: kind})
	}
	return p
}

func cprb(name, role string) *v1alpha3.ClusterProjectRoleBinding {
	return &v1alpha3.ClusterProjectRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1alpha3.ClusterProjectRoleBindingSpec{
			Subjects: []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: "Group", Name: "platform"}},
			RoleRef:  v1alpha3.RoleRef{Kind: "ClusterRole", Name: role},
		},
	}
}

func TestReconcile_FansOutToAllNonVirtualProjects(t *testing.T) {
	r, c := newReconciler(t,
		cprb("global-viewer", "d8:project:viewer"),
		project("alpha", false, "alpha", "alpha-extra"),
		project("beta", false, "beta"),
		project("default", true, "default"),
	)

	_, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "global-viewer"}})
	assert.NoError(t, err)

	name := rolebinding.CPRBServiceName("global-viewer")
	for _, ns := range []string{"alpha", "alpha-extra", "beta"} {
		assert.NoErrorf(t, c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: name}, &rbacv1.RoleBinding{}),
			"RoleBinding must exist in namespace %s", ns)
	}

	// the virtual project must NOT receive the binding
	err = c.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &rbacv1.RoleBinding{})
	assert.Error(t, err)

	// status reflects the bound non-virtual projects
	got := &v1alpha3.ClusterProjectRoleBinding{}
	assert.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "global-viewer"}, got))
	assert.Equal(t, int32(2), got.Status.BoundProjects)
	assert.Contains(t, got.Finalizers, v1alpha3.ClusterProjectRoleBindingFinalizer)
}
