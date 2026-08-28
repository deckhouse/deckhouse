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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	projectmanager "controller/internal/manager/project"
)

func TestEnqueueProjectForNamespace(t *testing.T) {
	t.Run("deleted unowned ns refreshes virtual default", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "t-lvlns"}}
		reqs := enqueueProjectForNamespace(context.Background(), ns)
		assert.Equal(t, []string{projectmanager.DefaultProjectName}, requestNames(reqs))
	})

	t.Run("upmeter probe refreshes virtual deckhouse", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "upmeter-probe-namespace-foo"}}
		reqs := enqueueProjectForNamespace(context.Background(), ns)
		assert.Equal(t, []string{projectmanager.DeckhouseProjectName}, requestNames(reqs))
	})

	t.Run("owned ns wakes the real project only", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   "team",
			Labels: map[string]string{v1alpha3.ResourceLabelProject: "team"},
		}}
		reqs := enqueueProjectForNamespace(context.Background(), ns)
		assert.Equal(t, []string{"team"}, requestNames(reqs))
	})
}

func TestNamespaceWatchPredicate_DeleteAlways(t *testing.T) {
	p := namespaceWatchPredicate{}
	assert.True(t, p.Delete(event.DeleteEvent{Object: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "t-lvlns"}}}))
}

func requestNames(reqs []reconcile.Request) []string {
	out := make([]string, 0, len(reqs))
	for _, req := range reqs {
		out = append(out, req.Name)
	}
	return out
}
