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

package projectnames

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"controller/apis/deckhouse.io/v1alpha3"
)

func project(name string) *v1alpha3.Project {
	return &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func virtualProject(name string) *v1alpha3.Project {
	p := project(name)
	p.Labels = map[string]string{v1alpha3.ProjectLabelVirtualProject: "true"}
	return p
}

func newReconciler(t *testing.T, objs ...client.Object) *Reconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha3.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	return &Reconciler{Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()}
}

// eventRecorder keeps the events it is given, with the kind and name of the object each is on.
type eventRecorder struct {
	events []string
}

func (e *eventRecorder) Event(object runtime.Object, eventType, reason, message string) {
	on := "unknown"
	switch obj := object.(type) {
	case *v1alpha3.Project:
		on = "Project " + obj.Name
	case *corev1.Namespace:
		on = "Namespace " + obj.Name
	}
	e.events = append(e.events, on+": "+eventType+" "+reason+" "+message)
}

func (e *eventRecorder) Eventf(object runtime.Object, eventType, reason, messageFmt string, args ...any) {
	e.Event(object, eventType, reason, fmt.Sprintf(messageFmt, args...))
}

func (e *eventRecorder) AnnotatedEventf(object runtime.Object, _ map[string]string, eventType, reason, messageFmt string, args ...any) {
	e.Eventf(object, eventType, reason, messageFmt, args...)
}

// takeEvents returns the events recorded so far and forgets them.
func (e *eventRecorder) takeEvents() []string {
	events := e.events
	e.events = nil
	return events
}

// seriesOf renders the expected exposition of the metric for the given pairs.
func seriesOf(pairs ...[2]string) string {
	var b strings.Builder
	b.WriteString("# HELP " + metricName + " " + metricHelp + "\n")
	b.WriteString("# TYPE " + metricName + " gauge\n")
	for _, p := range pairs {
		b.WriteString(metricName + `{prefix_project="` + p[1] + `",project="` + p[0] + `"} 1` + "\n")
	}
	return b.String()
}

// The gauge is package state shared with the other tests of the package, so this test does not run
// in parallel and starts from an empty gauge.
func TestReconciler_Reconcile(t *testing.T) {
	namedLikeAdditionalNamespace.Reset()
	ctx := context.Background()

	terminating := project("shop-old")
	terminating.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
	terminating.Finalizers = []string{"test"}

	r := newReconciler(t,
		project("shop"),
		project("shop-review"),
		project("shop-review-17"),
		project("shopping"),
		terminating,
		virtualProject("default"),
		project("default-apps"),
	)

	_, err := r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	require.NoError(t, testutil.CollectAndCompare(namedLikeAdditionalNamespace, strings.NewReader(seriesOf(
		[2]string{"shop-review", "shop"},
		[2]string{"shop-review-17", "shop"},
		[2]string{"shop-review-17", "shop-review"},
	)), metricName))

	// Once one side of a pair is gone its series goes too, and the others stay.
	require.NoError(t, r.Delete(ctx, project("shop-review")))
	_, err = r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	require.NoError(t, testutil.CollectAndCompare(namedLikeAdditionalNamespace, strings.NewReader(seriesOf(
		[2]string{"shop-review-17", "shop"},
	)), metricName))

	require.NoError(t, r.Delete(ctx, project("shop")))
	_, err = r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.Equal(t, 0, testutil.CollectAndCount(namedLikeAdditionalNamespace, metricName))
}

// A new pair of a recent project is reported once, on the project and on its namespace. A pair of a
// project made long ago is not, as after a restart or a change of leader every pair is new again.
func TestReconciler_ReportsNewPairsOfRecentProjects(t *testing.T) {
	namedLikeAdditionalNamespace.Reset()
	ctx := context.Background()
	createdAgo := func(name string, age time.Duration) *v1alpha3.Project {
		p := project(name)
		p.CreationTimestamp = metav1.NewTime(time.Now().Add(-age))
		return p
	}
	namespace := func(name string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}
	const message = "Namespace shop-review-17 became a separate project. It is not an additional namespace of project shop. " +
		"To add a namespace to project shop, create a ProjectNamespace in the shop namespace."

	r := newReconciler(t,
		createdAgo("shop", 48*time.Hour),
		createdAgo("shop-review-17", 5*time.Minute),
		namespace("shop-review-17"),
		createdAgo("shop-legacy", 2*time.Hour),
		namespace("shop-legacy"),
	)
	recorder := &eventRecorder{}
	r.Recorder = recorder

	_, err := r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		"Project shop-review-17: Warning NamedLikeAdditionalNamespace " + message,
		"Namespace shop-review-17: Warning NamedLikeAdditionalNamespace " + message,
	}, recorder.takeEvents())

	_, err = r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	assert.Empty(t, recorder.takeEvents(), "a pair already published is not reported again")

	// A project a user creates has no namespace until the controller gets to it, and the event goes
	// on the project alone.
	require.NoError(t, r.Create(ctx, createdAgo("shop-review-18", time.Minute)))
	_, err = r.Reconcile(ctx, ctrl.Request{})
	require.NoError(t, err)
	events := recorder.takeEvents()
	require.Len(t, events, 1)
	assert.True(t, strings.HasPrefix(events[0], "Project shop-review-18: Warning NamedLikeAdditionalNamespace Namespace shop-review-18 became"), events[0])
}

func TestPairsMayChange(t *testing.T) {
	deleting := project("shop")
	deleting.DeletionTimestamp = &metav1.Time{Time: metav1.Now().Time}
	labelled := project("shop")
	labelled.Labels = map[string]string{"team": "a"}
	onVirtual := project("shop")
	onVirtual.Spec.ProjectTemplateName = v1alpha3.VirtualProjectTemplateName

	tests := []struct {
		name     string
		updated  *v1alpha3.Project
		expected bool
	}{
		{name: "the deletion starts", updated: deleting, expected: true},
		{name: "the project turns virtual", updated: onVirtual, expected: true},
		{name: "a label changes", updated: labelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pairsMayChange().Update(event.UpdateEvent{ObjectOld: project("shop"), ObjectNew: tt.updated})
			assert.Equal(t, tt.expected, got)
		})
	}

	assert.True(t, pairsMayChange().Create(event.CreateEvent{Object: project("shop")}), "create")
	assert.True(t, pairsMayChange().Delete(event.DeleteEvent{Object: project("shop")}), "delete")
}

func TestNamePairs(t *testing.T) {
	tests := []struct {
		name     string
		names    []string
		expected []pair
	}{
		{name: "no projects"},
		{name: "unrelated names", names: []string{"shop", "shopping", "shop2", "api"}},
		{
			name:     "a project and one named after it",
			names:    []string{"shop-review", "shop"},
			expected: []pair{{project: "shop-review", prefixProject: "shop"}},
		},
		{
			name:  "every level of a chain",
			names: []string{"a-b-c", "a", "a-b"},
			expected: []pair{
				{project: "a-b", prefixProject: "a"},
				{project: "a-b-c", prefixProject: "a"},
				{project: "a-b-c", prefixProject: "a-b"},
			},
		},
		{
			name:  "all the names under a project, whatever sorts between them",
			names: []string{"shop0", "shop-b", "shop", "shop-a-b", "shop-a", "shopa"},
			expected: []pair{
				{project: "shop-a", prefixProject: "shop"},
				{project: "shop-a-b", prefixProject: "shop"},
				{project: "shop-b", prefixProject: "shop"},
				{project: "shop-a-b", prefixProject: "shop-a"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, namePairs(tt.names))
		})
	}
}
