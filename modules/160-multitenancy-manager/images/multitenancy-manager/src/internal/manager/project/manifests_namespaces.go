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
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/edition"
	"controller/internal/rolebinding"
)

// The manifests of a template may make the additional namespaces of a project themselves, with a
// ProjectNamespace in the main namespace, and put objects into them. On the first render such a
// namespace does not exist yet, so the objects are applied in two steps:
//
//   - A namespace that a ProjectNamespace of the same render makes is a namespace of the project for
//     the check of the objects (pinManifestObject), as the ones of status.namespaces are. An object in
//     any other namespace still refuses the whole render.
//   - The objects of such a namespace wait until the namespace is in status.namespaces and the
//     d8-manifests binding has been fanned out into it, so that the user the objects are applied as
//     has its rights there. The other objects, the ProjectNamespace among them, are applied at once.
//     The ManifestsApplied condition names what the objects wait for: a namespace that is being
//     deleted, one that is not created yet, or the Ready=False message of its ProjectNamespace. The
//     objects that the render no longer holds are still deleted; the ones that wait are kept. A
//     recheck of the wait does not apply the objects that are already applied. The creation of the
//     namespace and the creation of the fanned-out binding in it wake the project, as the project
//     controller watches both, and manifestsNamespaceRecheck is only the fallback.
//   - A ProjectNamespace that the render no longer holds is deleted, and its controller deletes the
//     namespace with everything in it. The objects of the manifests in that namespace leave
//     status.manifestObjects without a delete of their own (prunedNamespaces).

// manifestsNamespaceRecheck is how soon a project whose manifests wait for a namespace looks again
// when no event of the namespace or of the binding in it wakes it before.
const manifestsNamespaceRecheck = 15 * time.Second

// manifestsNamespaceOf returns the namespace that an object of a render of the manifests of the
// project makes, when it is a ProjectNamespace in the main namespace of the project with a name: the
// namespace of a ProjectNamespace is "<project>-<spec.name>", and one may be created only in the main
// namespace of a project.
func manifestsNamespaceOf(project string, object *unstructured.Unstructured) (string, bool) {
	if object.GroupVersionKind().GroupKind() != v1alpha3.Kind(v1alpha3.ProjectNamespaceKind) {
		return "", false
	}
	if namespace := object.GetNamespace(); namespace != "" && namespace != project {
		return "", false
	}
	name, _, err := unstructured.NestedString(object.Object, "spec", "name")
	if err != nil || name == "" {
		return "", false
	}
	return project + "-" + name, true
}

// plannedManifestNamespaces lists the namespaces that the ProjectNamespace objects of a render of the
// manifests of the project make. A render that does not decode is read up to the document that
// fails, and the check of the objects says what is wrong with it.
func plannedManifestNamespaces(project, rendered string) []string {
	var planned []string
	_ = eachManifestObject(rendered, func(object *unstructured.Unstructured) error {
		if namespace, ok := manifestsNamespaceOf(project, object); ok && !slices.Contains(planned, namespace) {
			planned = append(planned, namespace)
		}
		return nil
	})
	return planned
}

// UnknownManifestNamespaces lists, sorted, the namespaces that the objects of a render of the
// manifests of the project are put into and that are neither among known, the namespaces of the
// project, nor made by a ProjectNamespace of the render. The controller refuses such a render. An
// object without a namespace goes to the main namespace, which is always known. A render that does
// not decode is read up to the document that fails.
func UnknownManifestNamespaces(project string, known []string, rendered string) []string {
	planned := plannedManifestNamespaces(project, rendered)
	var unknown []string
	_ = eachManifestObject(rendered, func(object *unstructured.Unstructured) error {
		namespace := object.GetNamespace()
		if namespace == "" || namespace == project || slices.Contains(known, namespace) ||
			slices.Contains(planned, namespace) || slices.Contains(unknown, namespace) {
			return nil
		}
		unknown = append(unknown, namespace)
		return nil
	})
	slices.Sort(unknown)
	return unknown
}

// holdBackForNamespaces splits the objects of a render into the ones to apply now, the ones held
// back, and what those wait for, sorted (see the comment at the top of the file). Only a namespace
// that a ProjectNamespace of the render makes is waited for: any other one is in status.namespaces,
// or the object refused the render. The namespace, the fanned-out binding and the ProjectNamespace
// are read from the cache.
func (m *Manager) holdBackForNamespaces(
	ctx context.Context,
	project *v1alpha3.Project,
	planned []string,
	objects []*unstructured.Unstructured,
) ([]*unstructured.Unstructured, []*unstructured.Unstructured, []string, error) {
	if edition.SingleNamespaceProjects(edition.Current()) {
		ready, held, namespaces := holdBackForGood(planned, objects)
		return ready, held, namespaces, nil
	}
	failures, err := m.projectNamespaceFailures(ctx, project.Name, planned)
	if err != nil {
		return nil, nil, nil, err
	}
	namespaces := rolebinding.ProjectNamespaceNames(project)
	waits := make(map[string]string)
	for _, namespace := range planned {
		live := new(corev1.Namespace)
		switch err := m.client.Get(ctx, client.ObjectKey{Name: namespace}, live); {
		case apierrors.IsNotFound(err):
			live = nil
		case err != nil:
			return nil, nil, nil, fmt.Errorf("get the '%s' namespace: %w", namespace, err)
		}
		switch {
		case live != nil && !live.DeletionTimestamp.IsZero():
			waits[namespace] = namespaceWait(namespace, true, failures[namespace])
			continue
		case live == nil || !slices.Contains(namespaces, namespace):
			waits[namespace] = namespaceWait(namespace, false, failures[namespace])
			continue
		}
		fannedOut := new(rbacv1.RoleBinding)
		key := client.ObjectKey{Namespace: namespace, Name: rolebinding.PRBServiceName(v1alpha3.ProjectManifestsBinding)}
		switch err := m.client.Get(ctx, key, fannedOut); {
		case apierrors.IsNotFound(err):
			waits[namespace] = fmt.Sprintf("the %s ProjectRoleBinding to reach the '%s' namespace", v1alpha3.ProjectManifestsBinding, namespace)
		case err != nil:
			return nil, nil, nil, fmt.Errorf("get the %s RoleBinding in the '%s' namespace: %w", key.Name, namespace, err)
		}
	}

	ready := make([]*unstructured.Unstructured, 0, len(objects))
	var held []*unstructured.Unstructured
	var waiting []string
	for _, object := range objects {
		what, ok := waits[object.GetNamespace()]
		if !ok {
			ready = append(ready, object)
			continue
		}
		held = append(held, object)
		if !slices.Contains(waiting, what) {
			waiting = append(waiting, what)
		}
	}
	slices.Sort(waiting)
	return ready, held, waiting, nil
}

// holdBackForGood splits the objects of a render in an edition where a project has only its main
// namespace: no ProjectNamespace makes a namespace there, so the objects of a namespace one of the
// render names are never applied. The third list names, sorted and quoted, the namespaces that hold
// objects back, for manifestsWaitingMessage.
func holdBackForGood(planned []string, objects []*unstructured.Unstructured) ([]*unstructured.Unstructured, []*unstructured.Unstructured, []string) {
	ready := make([]*unstructured.Unstructured, 0, len(objects))
	var held []*unstructured.Unstructured
	var namespaces []string
	for _, object := range objects {
		namespace := object.GetNamespace()
		if !slices.Contains(planned, namespace) {
			ready = append(ready, object)
			continue
		}
		held = append(held, object)
		if quoted := "'" + namespace + "'"; !slices.Contains(namespaces, quoted) {
			namespaces = append(namespaces, quoted)
		}
	}
	slices.Sort(namespaces)
	return ready, held, namespaces
}

// namespaceWait is what the objects of a namespace that a ProjectNamespace makes wait for. A
// namespace that is being deleted is not one that is still to be created. failure is the
// Ready=False message of the ProjectNamespace, when it has one.
func namespaceWait(namespace string, terminating bool, failure string) string {
	switch {
	case terminating && failure != "":
		return fmt.Sprintf("the '%s' namespace to finish terminating: %s", namespace, failure)
	case terminating:
		return fmt.Sprintf("the '%s' namespace to finish terminating", namespace)
	case failure != "":
		return fmt.Sprintf("the '%s' namespace to be created: %s", namespace, failure)
	default:
		return fmt.Sprintf("the '%s' namespace to be created", namespace)
	}
}

// projectNamespaceFailures maps each planned namespace to the Ready=False message of the
// ProjectNamespace that makes it, when that object says why the namespace is not there.
func (m *Manager) projectNamespaceFailures(ctx context.Context, project string, planned []string) (map[string]string, error) {
	if len(planned) == 0 {
		return map[string]string{}, nil
	}
	list := &v1alpha3.ProjectNamespaceList{}
	if err := m.client.List(ctx, list, client.InNamespace(project)); err != nil {
		return nil, fmt.Errorf("list the ProjectNamespace objects of the '%s' project: %w", project, err)
	}
	failures := make(map[string]string, len(planned))
	for i := range list.Items {
		item := &list.Items[i]
		if item.Spec.Name == "" {
			continue
		}
		namespace := project + "-" + item.Spec.Name
		if !slices.Contains(planned, namespace) {
			continue
		}
		for _, cond := range item.Status.Conditions {
			if cond.Type == v1alpha3.ProjectNamespaceConditionReady && cond.Status == corev1.ConditionFalse && cond.Message != "" {
				failures[namespace] = cond.Message
				break
			}
		}
	}
	return failures, nil
}

// prunedNamespaces lists the namespaces of the ProjectNamespace objects of status.manifestObjects
// that the render no longer holds, read as the user of the project. The prune deletes such a
// ProjectNamespace, and its controller deletes the namespace with every object in it, so the objects
// of the manifests there leave the list without a delete of their own, which the API server could
// refuse while the namespace terminates. A ProjectNamespace that is gone, cannot be read, or is not
// the manifests' adds nothing: its namespace is left to the prune as any other.
func prunedNamespaces(ctx context.Context, actor client.Client, project *v1alpha3.Project, keep map[releaseKey]struct{}) map[string]struct{} {
	pruned := make(map[string]struct{})
	for _, listed := range project.Status.ManifestObjects {
		if _, ok := keep[manifestObjectKey(listed, project.Name)]; ok {
			continue
		}
		live := new(unstructured.Unstructured)
		live.SetAPIVersion(listed.APIVersion)
		live.SetKind(listed.Kind)
		if live.GroupVersionKind().GroupKind() != v1alpha3.Kind(v1alpha3.ProjectNamespaceKind) {
			continue
		}
		if err := actor.Get(ctx, client.ObjectKey{Namespace: listed.Namespace, Name: listed.Name}, live); err != nil {
			continue
		}
		if namespace, ok := manifestsNamespaceOf(project.Name, live); ok && isManifestObjectOf(live, project.Name) {
			pruned[namespace] = struct{}{}
		}
	}
	return pruned
}
