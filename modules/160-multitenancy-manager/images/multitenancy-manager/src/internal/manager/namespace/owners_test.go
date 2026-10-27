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

package namespace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
)

func ownerRef(kind, name, uid string) metav1.OwnerReference {
	return metav1.OwnerReference{APIVersion: "example.com/v1", Kind: kind, Name: name, UID: k8stypes.UID(uid)}
}

func TestMirrorNamespaceOwners(t *testing.T) {
	tenant := ownerRef("Tenant", "team", "uid-tenant")
	own := ownerRef("Release", "gitops", "uid-own")

	t.Run("a namespace without owners leaves a user-created Project as it is", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj", OwnerReferences: []metav1.OwnerReference{own}}}
		assert.False(t, MirrorNamespaceOwners(project, namespace("proj", nil, nil)))
		assert.Equal(t, []metav1.OwnerReference{own}, project.OwnerReferences)
		assert.NotContains(t, project.Annotations, v1alpha3.ProjectAnnotationNamespaceOwners)
	})

	t.Run("the owners of the namespace are copied without controller and blockOwnerDeletion", func(t *testing.T) {
		ns := namespace("proj", nil, nil)
		controlling := tenant
		controlling.Controller = ptr.To(true)
		controlling.BlockOwnerDeletion = ptr.To(true)
		ns.OwnerReferences = []metav1.OwnerReference{controlling}
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj", OwnerReferences: []metav1.OwnerReference{own}}}

		assert.True(t, MirrorNamespaceOwners(project, ns))
		assert.Equal(t, []metav1.OwnerReference{own, tenant}, project.OwnerReferences, "the Project keeps an owner of its own")
		assert.Equal(t, "uid-tenant", project.Annotations[v1alpha3.ProjectAnnotationNamespaceOwners])
		assert.False(t, MirrorNamespaceOwners(project, ns), "a second pass changes nothing")
	})

	t.Run("an owner removed from the namespace is removed from the Project", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{
			Name:            "proj",
			OwnerReferences: []metav1.OwnerReference{own, tenant},
			Annotations:     map[string]string{v1alpha3.ProjectAnnotationNamespaceOwners: "uid-tenant"},
		}}
		assert.True(t, MirrorNamespaceOwners(project, namespace("proj", nil, nil)))
		assert.Equal(t, []metav1.OwnerReference{own}, project.OwnerReferences)
		assert.NotContains(t, project.Annotations, v1alpha3.ProjectAnnotationNamespaceOwners)
	})

	t.Run("an owner the Project has of its own is neither copied over nor recorded", func(t *testing.T) {
		ns := namespace("proj", nil, nil)
		ns.OwnerReferences = []metav1.OwnerReference{ownerRef("Tenant", "renamed", "uid-tenant"), own}
		mine := tenant
		mine.Controller = ptr.To(true)
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "proj", OwnerReferences: []metav1.OwnerReference{mine}}}

		assert.True(t, MirrorNamespaceOwners(project, ns))
		assert.Equal(t, []metav1.OwnerReference{mine, own}, project.OwnerReferences, "the Project's own reference stays as it is")
		assert.Equal(t, "uid-own", project.Annotations[v1alpha3.ProjectAnnotationNamespaceOwners])

		assert.True(t, MirrorNamespaceOwners(project, namespace("proj", nil, nil)))
		assert.Equal(t, []metav1.OwnerReference{mine}, project.OwnerReferences, "an owner removed from the namespace leaves the Project's own reference")
		assert.NotContains(t, project.Annotations, v1alpha3.ProjectAnnotationNamespaceOwners)
	})

	t.Run("a namespace that is gone changes nothing", func(t *testing.T) {
		project := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{
			Name:            "proj",
			OwnerReferences: []metav1.OwnerReference{tenant},
			Annotations:     map[string]string{v1alpha3.ProjectAnnotationNamespaceOwners: "uid-tenant"},
		}}
		assert.False(t, MirrorNamespaceOwners(project, nil))
		assert.Equal(t, []metav1.OwnerReference{tenant}, project.OwnerReferences)
	})
}

// An adopted namespace owned by another object gives its owners to the Project, so that the garbage
// collector deletes the Project with it.
func TestAdopt_CopiesTheOwnersOfTheNamespace(t *testing.T) {
	ns := namespace("foo", nil, nil)
	ns.OwnerReferences = []metav1.OwnerReference{ownerRef("Tenant", "team", "uid-tenant")}
	m, c := newManager(t, ns)

	_, err := m.Adopt(context.Background(), ns)
	require.NoError(t, err)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, []metav1.OwnerReference{ownerRef("Tenant", "team", "uid-tenant")}, got.OwnerReferences)
	assert.Equal(t, "uid-tenant", got.Annotations[v1alpha3.ProjectAnnotationNamespaceOwners])

	plain := namespace("bar", nil, nil)
	m, c = newManager(t, plain)
	_, err = m.Adopt(context.Background(), plain)
	require.NoError(t, err)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "bar"}, got))
	assert.Empty(t, got.OwnerReferences, "a namespace without owners gives the Project none")
	assert.NotContains(t, got.Annotations, v1alpha3.ProjectAnnotationNamespaceOwners)
}
