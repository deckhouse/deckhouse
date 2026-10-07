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

// Package projectnames reports the projects whose names read like additional namespaces of other
// projects. An additional namespace of project P is named "P-<spec.name>", and only the names that
// ProjectNamespaces claim are reserved, so a project "P-<suffix>" beside P is a separate project that
// looks like a part of P. Nothing is wrong with such a pair, but whoever reads the names can take
// it for one project, so each pair is published as a series for the alert to point at. A pair of a
// project made a short while ago is also reported by an event on that project and its namespace.
package projectnames

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"controller/apis/deckhouse.io/v1alpha3"
)

const (
	metricName = "d8_multitenancy_project_named_like_additional_namespace"
	metricHelp = "A project whose name is the name of another project, a dash and a suffix: " +
		"it reads like an additional namespace of that project but is a separate project."

	// reasonNamedLikeAdditionalNamespace is the reason of the Warning event on a new project named like
	// an additional namespace of another project, and on its namespace.
	reasonNamedLikeAdditionalNamespace = "NamedLikeAdditionalNamespace"

	// recentProject is how young a project has to be for a new pair of it to be reported by an event.
	// The reconciler starts with no pairs published, so after a restart or a change of leader every
	// pair is new, and without the limit the events of projects made long ago would come again.
	recentProject = time.Hour
)

// namedLikeAdditionalNamespace is 1 for every pair of projects where project is named
// "<prefix_project>-<suffix>". The MultitenancyManagerProjectNamedLikeAdditionalNamespace alert
// (monitoring/prometheus-rules/project-names.yaml) fires on it:
//
//	max by (project, prefix_project) (d8_multitenancy_project_named_like_additional_namespace) > 0
//
// The series count is the number of such pairs, bounded by the projects of the cluster.
var namedLikeAdditionalNamespace = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: metricName,
	Help: metricHelp,
}, []string{"project", "prefix_project"})

func init() {
	metrics.Registry.MustRegister(namedLikeAdditionalNamespace)
}

// allProjects is the only request the reconciler works on: a pair is a property of the whole set of
// projects, so every event recounts all of them, and the queue folds a burst of events into one
// recount.
var allProjects = reconcile.Request{NamespacedName: types.NamespacedName{Name: "projects"}}

// Reconciler keeps the pair series in step with the projects.
type Reconciler struct {
	client.Client

	// Recorder reports the new pairs of recent projects on the project and its namespace. Optional.
	Recorder record.EventRecorder

	mu sync.Mutex
	// published holds the pairs the gauge carries, so a recount deletes the series of the pairs that
	// are gone instead of resetting the gauge, which a scrape in between would see empty.
	published map[pair]struct{}
}

// Reconcile recounts the pairs and publishes them.
func (r *Reconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	projects := new(v1alpha3.ProjectList)
	// The list is only read, so the cached objects are not copied: with every namespace of the
	// cluster a project, the copies would be the whole cost of a recount.
	if err := r.List(ctx, projects, client.UnsafeDisableDeepCopy); err != nil {
		return ctrl.Result{}, fmt.Errorf("list projects: %w", err)
	}

	names := make([]string, 0, len(projects.Items))
	for i := range projects.Items {
		if counted(&projects.Items[i]) {
			names = append(names, projects.Items[i].Name)
		}
	}
	added := r.publish(namePairs(names))
	r.report(ctx, added, projects.Items)
	return ctrl.Result{}, nil
}

// publish sets the series of the current pairs and deletes the series of the pairs that are gone. It
// returns the pairs that were not published before.
func (r *Reconciler) publish(pairs []pair) []pair {
	r.mu.Lock()
	defer r.mu.Unlock()

	var added []pair
	current := make(map[pair]struct{}, len(pairs))
	for _, p := range pairs {
		current[p] = struct{}{}
		namedLikeAdditionalNamespace.WithLabelValues(p.project, p.prefixProject).Set(1)
		if _, ok := r.published[p]; !ok {
			added = append(added, p)
		}
	}
	for p := range r.published {
		if _, ok := current[p]; !ok {
			namedLikeAdditionalNamespace.DeleteLabelValues(p.project, p.prefixProject)
		}
	}
	r.published = current
	return added
}

// report sends a Warning event on the project of each pair just published, and on its namespace,
// when the project is recent. A namespace created directly becomes a project of its own through the
// controller, which no admission webhook checks, so the event is where its author learns that the
// namespace is not an additional namespace of the other project. The projects are the listed ones and
// are only read.
func (r *Reconciler) report(ctx context.Context, added []pair, projects []v1alpha3.Project) {
	if r.Recorder == nil || len(added) == 0 {
		return
	}
	recent := make(map[string]*v1alpha3.Project, len(added))
	for i := range projects {
		if time.Since(projects[i].CreationTimestamp.Time) < recentProject {
			recent[projects[i].Name] = &projects[i]
		}
	}
	for _, p := range added {
		project, ok := recent[p.project]
		if !ok {
			continue
		}
		message := fmt.Sprintf("Namespace %s became a separate project. It is not an additional namespace of project %s. "+
			"To add a namespace to project %s, create a ProjectNamespace in the %s namespace.",
			p.project, p.prefixProject, p.prefixProject, p.prefixProject)
		r.Recorder.Event(project, corev1.EventTypeWarning, reasonNamedLikeAdditionalNamespace, message)

		// A project a user creates gets its namespace only when the controller gets to it, and the
		// admission warning has told its author already.
		namespace := new(corev1.Namespace)
		if err := r.Get(ctx, client.ObjectKey{Name: p.project}, namespace); err != nil {
			if !apierrors.IsNotFound(err) {
				ctrllog.FromContext(ctx).Error(err, "get the namespace of a project named like an additional namespace", "project", p.project)
			}
			continue
		}
		r.Recorder.Event(namespace, corev1.EventTypeWarning, reasonNamedLikeAdditionalNamespace, message)
	}
}

// SetupWithManager wires the reconciler and its watch.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("project-names").
		Watches(&v1alpha3.Project{},
			handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
				return []reconcile.Request{allProjects}
			}),
			builder.WithPredicates(pairsMayChange())).
		Complete(r)
}

// counted reports whether the project takes part in the pairs. A virtual project has no additional
// namespaces to be confused with, and a project being deleted is on its way out.
func counted(project *v1alpha3.Project) bool {
	return !project.IsVirtual() && project.DeletionTimestamp.IsZero()
}

// pairsMayChange lets through the project events that can change the pairs. A name never changes,
// so an update matters only when it changes whether the project is counted.
func pairsMayChange() predicate.Predicate {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldProject, okOld := e.ObjectOld.(*v1alpha3.Project)
			newProject, okNew := e.ObjectNew.(*v1alpha3.Project)
			if !okOld || !okNew {
				return true
			}
			return counted(oldProject) != counted(newProject)
		},
	}
}

// pair is a project named "<prefixProject>-<suffix>" beside the project prefixProject.
type pair struct {
	project       string
	prefixProject string
}

// namePairs returns every pair among the names, ordered by the prefix project and then by the
// project. The names that start with a given string follow one another in sorted order, starting
// where that string would be inserted, so each name costs one search and the pairs it has.
func namePairs(names []string) []pair {
	sorted := slices.Clone(names)
	slices.Sort(sorted)

	var pairs []pair
	for _, prefixProject := range sorted {
		prefix := prefixProject + "-"
		start, _ := slices.BinarySearch(sorted, prefix)
		for _, name := range sorted[start:] {
			if !strings.HasPrefix(name, prefix) {
				break
			}
			pairs = append(pairs, pair{project: name, prefixProject: prefixProject})
		}
	}
	return pairs
}
