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

package namespaces

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/edition"
)

// OfProject lists the namespaces of a project, the ones with its projects.deckhouse.io/project label.
//
// In an edition where a project has only its main namespace (edition.SingleNamespaceProjects), only
// the namespace named after the project is one of its namespaces. Any other namespace with the label,
// one left by a ProjectNamespace made in another edition before a switch or one labelled by hand, is
// left out, and the controller puts nothing of the project into it: no bindings, no quota, no objects
// of the template. Its labels stay as they are, and so does what the controller put into it before
// (Dropped), so a switch back to an edition with additional namespaces brings it back into the
// project. Every place that decides what goes into the namespaces
// of a project reads them here.
func OfProject(ctx context.Context, reader client.Reader, project string) ([]corev1.Namespace, error) {
	list := new(corev1.NamespaceList)
	if err := reader.List(ctx, list, client.MatchingLabels{v1alpha3.ResourceLabelProject: project}); err != nil {
		return nil, fmt.Errorf("list the namespaces of the %q project: %w", project, err)
	}
	if !edition.SingleNamespaceProjects(edition.Current()) {
		return list.Items, nil
	}
	for i := range list.Items {
		if list.Items[i].Name == project {
			return list.Items[i : i+1], nil
		}
	}
	return []corev1.Namespace{}, nil
}

// InProject reports whether the namespace is a namespace of the project, as OfProject lists them.
func InProject(ns *corev1.Namespace, project string) bool {
	if ns.Labels[v1alpha3.ResourceLabelProject] != project {
		return false
	}
	return ns.Name == project || !edition.SingleNamespaceProjects(edition.Current())
}

// Dropped returns the namespaces that carry the label of the project but are not namespaces of it, as
// OfProject lists them: in an edition where a project has only its main namespace, the additional
// namespaces left from another edition. The controller stops managing them but leaves what it put into
// them in place: the objects of the template stay as they were, and the RoleBindings of the project
// bindings are neither made nor widened there, only narrowed by a revocation. A switch back to an
// edition with additional namespaces takes them into the project again.
// In the other editions there are none, and nothing is read. Namespaces that are being deleted are
// left out.
func Dropped(ctx context.Context, reader client.Reader, project string) (map[string]struct{}, error) {
	return dropped(ctx, reader, client.MatchingLabels{v1alpha3.ResourceLabelProject: project})
}

// DroppedOfAnyProject is Dropped for every project at once.
func DroppedOfAnyProject(ctx context.Context, reader client.Reader) (map[string]struct{}, error) {
	return dropped(ctx, reader, client.HasLabels{v1alpha3.ResourceLabelProject})
}

func dropped(ctx context.Context, reader client.Reader, selector client.ListOption) (map[string]struct{}, error) {
	names := map[string]struct{}{}
	if !edition.SingleNamespaceProjects(edition.Current()) {
		return names, nil
	}
	list := new(corev1.NamespaceList)
	if err := reader.List(ctx, list, selector); err != nil {
		return nil, fmt.Errorf("list the namespaces of projects: %w", err)
	}
	for i := range list.Items {
		ns := &list.Items[i]
		if ns.DeletionTimestamp.IsZero() && !InProject(ns, ns.Labels[v1alpha3.ResourceLabelProject]) {
			names[ns.Name] = struct{}{}
		}
	}
	return names, nil
}

// LabelledBy returns the namespaces that carry the label of one of the projects, whatever the edition.
// The labels are what membership follows, while Project.status.namespaces catches up with them only at
// the next reconcile of the project: a namespace that comes back into a project after a switch of the
// edition is labelled before the status lists it again.
func LabelledBy(ctx context.Context, reader client.Reader, projects map[string]struct{}) (map[string]struct{}, error) {
	names := map[string]struct{}{}
	if len(projects) == 0 {
		return names, nil
	}
	list := new(corev1.NamespaceList)
	if err := reader.List(ctx, list, client.HasLabels{v1alpha3.ResourceLabelProject}); err != nil {
		return nil, fmt.Errorf("list the namespaces of projects: %w", err)
	}
	for i := range list.Items {
		if _, ok := projects[list.Items[i].Labels[v1alpha3.ResourceLabelProject]]; ok {
			names[list.Items[i].Name] = struct{}{}
		}
	}
	return names, nil
}
