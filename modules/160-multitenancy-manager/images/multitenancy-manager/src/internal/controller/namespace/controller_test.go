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

package template

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"controller/apis/deckhouse.io/v1alpha3"
)

func TestIsAutoWrapCandidate(t *testing.T) {
	cases := []struct {
		name string
		ns   *corev1.Namespace
		want bool
	}{
		{name: "plain user namespace", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}, want: true},
		{name: "default namespace", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, want: false},
		{name: "reserved d8 prefix", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-system"}}, want: false},
		{name: "reserved kube prefix", ns: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}, want: false},
		{
			name: "deckhouse heritage",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "module-ns", Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageDeckhouse}}},
			want: false,
		},
		{
			// A namespace already owned by a project (its main namespace or an additional namespace
			// created by a ProjectNamespace) must never be auto-wrapped into a separate project.
			name: "project-owned namespace is skipped",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "owned", Labels: map[string]string{v1alpha3.ResourceLabelProject: "owned", v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}}},
			want: false,
		},
		{
			name: "additional project namespace is skipped",
			ns:   &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a-backend", Labels: map[string]string{v1alpha3.ResourceLabelProject: "team-a", v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageMultitenancy}}},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isAutoWrapCandidate(tc.ns))
		})
	}
}

func TestPredicateShouldHandle(t *testing.T) {
	adopt := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "adopt-me", Annotations: map[string]string{v1alpha3.NamespaceAnnotationAdopt: ""}}}
	managed := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "d8-managed", Finalizers: []string{v1alpha3.NamespaceFinalizerManagedProject}}}
	orphan := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
	system := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}}

	t.Run("orphan allowed when flag enabled", func(t *testing.T) {
		p := customPredicate[*corev1.Namespace]{logger: logr.Discard(), allowOrphanNamespaces: true}
		assert.True(t, p.shouldHandle(orphan))
		assert.False(t, p.shouldHandle(system))
	})

	t.Run("orphan ignored when flag disabled, adopt still handled", func(t *testing.T) {
		p := customPredicate[*corev1.Namespace]{logger: logr.Discard(), allowOrphanNamespaces: false}
		assert.False(t, p.shouldHandle(orphan))
		assert.True(t, p.shouldHandle(adopt))
	})

	t.Run("finalizer-marked namespace always handled", func(t *testing.T) {
		p := customPredicate[*corev1.Namespace]{logger: logr.Discard(), allowOrphanNamespaces: false}
		assert.True(t, p.shouldHandle(managed))
	})
}
