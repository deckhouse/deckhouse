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
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

func TestIsLeftoverWrap(t *testing.T) {
	t.Run("nil project", func(t *testing.T) {
		assert.False(t, IsLeftoverWrap(nil))
	})
	t.Run("handmade empty template", func(t *testing.T) {
		assert.False(t, IsLeftoverWrap(project("foo", nil, "")))
	})
	t.Run("wrap label", func(t *testing.T) {
		assert.True(t, IsLeftoverWrap(project("foo", map[string]string{
			v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		}, "")))
	})
}

func TestCompleteLeftover_SkipsInferWhenForeignHelm(t *testing.T) {
	ns := namespace("foo", map[string]string{
		v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
	}, map[string]string{
		helm.ResourceAnnotationReleaseName:      "foo",
		helm.ResourceAnnotationReleaseNamespace: "foo",
	})
	wrap := project("foo", map[string]string{
		v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
	}, "")
	m, c := newManager(t, ns, wrap)

	deleted, err := m.CompleteLeftover(context.Background(), wrap)
	require.NoError(t, err)
	assert.False(t, deleted)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Empty(t, got.Spec.ProjectTemplateName)
	assert.Equal(t, v1alpha3.ManagedByNamespace, got.Labels[v1alpha3.ProjectLabelManagedByNamespace])

	updated := new(corev1.Namespace)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, updated))
	assert.Equal(t, "foo", updated.Annotations[helm.ResourceAnnotationReleaseName])
	assert.Equal(t, "foo", updated.Annotations[helm.ResourceAnnotationReleaseNamespace])
	assert.NotContains(t, updated.Labels, helm.ResourceLabelManagedBy)
}

func TestCompleteLeftover_InfersWhenNamespaceFree(t *testing.T) {
	ns := namespace("foo", map[string]string{
		v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
		labelPodPolicy:                          "restricted",
	}, nil)
	wrap := project("foo", map[string]string{
		v1alpha3.ProjectLabelManagedByNamespace: v1alpha3.ManagedByNamespace,
	}, "")
	m, c := newManager(t, ns, wrap)

	deleted, err := m.CompleteLeftover(context.Background(), wrap)
	require.NoError(t, err)
	assert.False(t, deleted)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: "foo"}, got))
	assert.Equal(t, TemplateDefault, got.Spec.ProjectTemplateName)
	assert.NotContains(t, got.Labels, v1alpha3.ProjectLabelManagedByNamespace)
	assert.Equal(t, false, got.Spec.Parameters["requiredRequests"])
}
