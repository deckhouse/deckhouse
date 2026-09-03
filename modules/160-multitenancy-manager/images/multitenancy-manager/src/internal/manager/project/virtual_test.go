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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
)

func TestVirtualProjectName(t *testing.T) {
	cases := []struct {
		name string
		ns   *corev1.Namespace
		want string
	}{
		{
			name: "owned by a real project",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team", Labels: map[string]string{v1alpha3.ResourceLabelProject: "team"}}},
			want: "",
		},
		{
			name: "d8 prefix",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-upmeter"}},
			want: DeckhouseProjectName,
		},
		{
			name: "upmeter probe",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "upmeter-probe-namespace-foo"}},
			want: DeckhouseProjectName,
		},
		{
			name: "heritage deckhouse leftover",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "deny-exec-heritage-test", Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageDeckhouse}}},
			want: DeckhouseProjectName,
		},
		{
			name: "heritage upmeter",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "obsv", Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageUpmeter}}},
			want: DeckhouseProjectName,
		},
		{
			name: "plain default namespace",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			want: DefaultProjectName,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, VirtualProjectName(tc.ns))
		})
	}
}

func TestHandleVirtual_DropsDeadNamespaces(t *testing.T) {
	virtual := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: DefaultProjectName},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: VirtualTemplate},
		Status: v1alpha3.ProjectStatus{
			Namespaces: []v1alpha3.NamespaceStatus{{Name: "t-lvlns"}, {Name: "default"}},
			State:      v1alpha3.ProjectStateDeployed,
		},
	}
	live := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	m, c := newManager(t, virtual, live)

	_, err := m.HandleVirtual(context.Background(), virtual)
	require.NoError(t, err)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: DefaultProjectName}, got))
	require.Len(t, got.Status.Namespaces, 1)
	assert.Equal(t, "default", got.Status.Namespaces[0].Name)
}

func TestHandleVirtual_SkipsNamespaceWithSameNameProject(t *testing.T) {
	virtual := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:   DefaultProjectName,
			Labels: map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"},
		},
		Spec: v1alpha3.ProjectSpec{ProjectTemplateName: VirtualTemplate},
	}
	claimed := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	foo := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
	plain := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	m, c := newManager(t, virtual, claimed, foo, plain)

	_, err := m.HandleVirtual(context.Background(), virtual)
	require.NoError(t, err)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: DefaultProjectName}, got))
	require.Len(t, got.Status.Namespaces, 1)
	assert.Equal(t, "default", got.Status.Namespaces[0].Name)
}

func TestHandleVirtual_PutsSystemNamespacesOnDeckhouse(t *testing.T) {
	virtual := &v1alpha3.Project{
		ObjectMeta: metav1.ObjectMeta{Name: DeckhouseProjectName},
		Spec:       v1alpha3.ProjectSpec{ProjectTemplateName: VirtualTemplate},
	}
	probe := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "upmeter-probe-namespace-foo"}}
	heritage := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name:   "deny-exec-heritage-test",
		Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageDeckhouse},
	}}
	m, c := newManager(t, virtual, probe, heritage)

	_, err := m.HandleVirtual(context.Background(), virtual)
	require.NoError(t, err)

	got := new(v1alpha3.Project)
	require.NoError(t, c.Get(context.Background(), client.ObjectKey{Name: DeckhouseProjectName}, got))
	names := make([]string, 0, len(got.Status.Namespaces))
	for _, ns := range got.Status.Namespaces {
		names = append(names, ns.Name)
	}
	assert.ElementsMatch(t, []string{"upmeter-probe-namespace-foo", "deny-exec-heritage-test"}, names)
}
