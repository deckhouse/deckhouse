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
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	projectmanager "controller/internal/manager/project"
)

func TestEnqueueProjectForNamespace(t *testing.T) {
	t.Run("deleted unowned ns refreshes virtual default and the same-name project", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "t-lvlns"}}
		reqs := namespaceProjectRequests(ns)
		assert.ElementsMatch(t, []string{projectmanager.DefaultProjectName, "t-lvlns"}, requestNames(reqs))
	})

	t.Run("upmeter probe refreshes virtual deckhouse and the same-name project", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "upmeter-probe-namespace-foo"}}
		reqs := namespaceProjectRequests(ns)
		assert.ElementsMatch(t, []string{projectmanager.DeckhouseProjectName, "upmeter-probe-namespace-foo"}, requestNames(reqs))
	})

	t.Run("owned ns wakes the real project only", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   "team",
			Labels: map[string]string{v1alpha3.ResourceLabelProject: "team"},
		}}
		reqs := namespaceProjectRequests(ns)
		assert.Equal(t, []string{"team"}, requestNames(reqs))
	})

	t.Run("adopt wakes virtual default and the new project", func(t *testing.T) {
		oldNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team"}}
		newNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   "team",
			Labels: map[string]string{v1alpha3.ResourceLabelProject: "team"},
		}}
		reqs := namespaceProjectRequests(oldNS, newNS)
		assert.ElementsMatch(t, []string{projectmanager.DefaultProjectName, "team"}, requestNames(reqs))
	})

	t.Run("heritage change wakes both virtual projects and the same-name project", func(t *testing.T) {
		oldNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "probe"}}
		newNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name:   "probe",
			Labels: map[string]string{v1alpha3.ResourceLabelHeritage: v1alpha3.ResourceHeritageDeckhouse},
		}}
		reqs := namespaceProjectRequests(oldNS, newNS)
		assert.ElementsMatch(t, []string{projectmanager.DefaultProjectName, projectmanager.DeckhouseProjectName, "probe"}, requestNames(reqs))
	})

	t.Run("unlabeled ns wakes virtual default and the same-name error project", func(t *testing.T) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
		reqs := namespaceProjectRequests(ns)
		assert.ElementsMatch(t, []string{projectmanager.DefaultProjectName, "foo"}, requestNames(reqs))
	})
}

func TestNamespaceWatchPredicate_DeleteAlways(t *testing.T) {
	p := namespaceWatchPredicate{}
	assert.True(t, p.Delete(event.DeleteEvent{Object: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "t-lvlns"}}}))
}

func TestNamespaceWatchPredicate_HelmOwnershipChange(t *testing.T) {
	p := namespaceWatchPredicate{}
	oldNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "foo",
		Annotations: map[string]string{
			helm.ResourceAnnotationReleaseName:      "foo",
			helm.ResourceAnnotationReleaseNamespace: "foo",
		},
	}}
	newNS := oldNS.DeepCopy()
	delete(newNS.Annotations, helm.ResourceAnnotationReleaseName)
	delete(newNS.Annotations, helm.ResourceAnnotationReleaseNamespace)
	assert.True(t, p.Update(event.UpdateEvent{ObjectOld: oldNS, ObjectNew: newNS}))

	same := oldNS.DeepCopy()
	same.Annotations["note"] = "unrelated"
	assert.False(t, p.Update(event.UpdateEvent{ObjectOld: oldNS, ObjectNew: same}))
}

func requestNames(reqs []reconcile.Request) []string {
	out := make([]string, 0, len(reqs))
	for _, req := range reqs {
		out = append(out, req.Name)
	}
	return out
}

// TestCustomPredicate_StatusOnlyWriteDoesNotRequeue: a parked project (TemplateRequiresRewrite=False)
// ends every reconcile with a status write. If that write re-entered the queue the project would
// spin forever. The update predicate lets a Project through only on a generation change or the
// require-sync annotation, so a status-only update is dropped.
func TestCustomPredicate_StatusOnlyWriteDoesNotRequeue(t *testing.T) {
	p := customPredicate[client.Object]{logger: logr.Discard()}
	old := &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "parked", Generation: 3}}
	old.Status.State = v1alpha3.ProjectStateDeployed
	cur := old.DeepCopy()
	cur.Status.State = v1alpha3.ProjectStateError
	cur.Status.Conditions = []v1alpha3.Condition{{Type: v1alpha3.ProjectConditionProjectTemplateUsable, Status: corev1.ConditionFalse}}
	if p.Update(event.TypedUpdateEvent[client.Object]{ObjectOld: old, ObjectNew: cur}) {
		t.Fatal("a status-only update must not requeue the project")
	}

	// A spec edit (generation bump) and the require-sync annotation still do.
	bumped := cur.DeepCopy()
	bumped.Generation = 4
	if !p.Update(event.TypedUpdateEvent[client.Object]{ObjectOld: cur, ObjectNew: bumped}) {
		t.Fatal("a generation change must requeue the project")
	}
	synced := cur.DeepCopy()
	synced.Annotations = map[string]string{v1alpha3.ProjectAnnotationRequireSync: "true"}
	if !p.Update(event.TypedUpdateEvent[client.Object]{ObjectOld: cur, ObjectNew: synced}) {
		t.Fatal("the require-sync annotation must requeue the project")
	}
}
