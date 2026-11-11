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
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

// An object that spec.manifests of a template applied can move into the release of the project in
// one change of the template: a NetworkPolicy named isolated or a PodLoggingConfig named default that
// a structured field renders from then on, or any object that the template of the project renders
// after a switch to another template. Helm creates the objects that an upgrade adds, and refuses one
// that exists without its ownership marks, so such an upgrade would fail on every reconcile and the
// stale manifests would never be cleaned up.
//
// The upgrade does not run with Helm's TakeOwnership: that takes over any object of the same kind and
// name, somebody else's included. Neither is the object deleted before the upgrade: the project would
// be without it until the upgrade creates it again, without its network isolation or its log shipping,
// and an object that holds data would lose it. The controller hands it over instead, before the
// upgrade, and only an object that the manifests of the project applied:
//
//   - It goes off status.manifestObjects first, and into status.handingOver with the revision the
//     upgrade is expected to deploy, in the same status write. A pass that stops after that finds
//     the object again from the journal, not by its labels. The journal is the only list of objects
//     this handover may touch.
//   - The object is applied as the release writes it, with the ownership marks of Helm, by the field
//     manager of the manifests. A server-side apply by the manager that owns the fields drops every
//     field that the manifests set and the release does not, the labels of the manifests among them,
//     and leaves the fields of other managers alone. Helm then takes the object over, as its marks are
//     there, and its own patch has nothing left to change. Helm alone would not do it: an upgrade
//     patches an object it takes over from the rendered object as if that were the previous revision,
//     so every field that only the live object has stays, the labels of the manifests too, and the
//     cleanup of the manifests would delete the object from the release.
//
// The apply runs with the identity of the controller: the object gets the heritage label, which only
// the controller may write. An object that the manifests did not apply is left to the upgrade, which
// refuses it as before.

// handOverManifestObjects hands every object that the upgrade adds to the release and that the
// manifests of the project applied over to the release (see the comment above).
func (m *Manager) handOverManifestObjects(ctx context.Context, project *v1alpha3.Project, change helm.ReleaseChange) error {
	release := helm.ReleaseName(project.Name)
	var keys, conflicts []releaseKey
	var journal []v1alpha3.HandingOverObject
	for _, object := range addedObjects(project.Name, change) {
		key := keyOf(object, project.Name)
		live, err := m.liveObject(ctx, object, key.namespace)
		if err != nil {
			return fmt.Errorf("get the %s '%s/%s': %w", object.Kind, key.namespace, object.Name, err)
		}
		switch {
		case live == nil:
		case listedManifestObject(project, key) || isManifestObjectOf(live, project.Name):
			keys = append(keys, key)
			journal = append(journal, handingOverOf(object, key.namespace, change.NextRevision))
		case !ownedByReleaseMarks(live, release):
			// The key pins a cluster-scoped object to the main namespace, as the post-renderer does; the
			// error names the object by the namespace it has, none for such an object.
			key.namespace = live.GetNamespace()
			conflicts = append(conflicts, key)
		}
	}
	if len(keys) == 0 {
		return nil
	}
	// Helm refuses an object that exists without the ownership marks of the release, so an upgrade that
	// adds one fails whatever the handover does. The objects of the manifests are left alone then: a
	// handover would be rolled back by the same pass, and every retry would move them to the release
	// and back. The upgrade does not run either: Helm would report the first object without the marks
	// in its own order, the objects of the manifests among them, while only the other objects keep
	// the handover from running.
	if len(conflicts) > 0 {
		return handoverBlockedError(keys, conflicts)
	}

	rendered, err := releaseObjectsOf(change.Manifest, project.Name, keys)
	if err != nil {
		return fmt.Errorf("read the objects the upgrade writes: %w", err)
	}

	err = m.recordInStatus(ctx, project, func(status *v1alpha3.ProjectStatus) bool {
		left := slices.DeleteFunc(slices.Clone(status.ManifestObjects), func(entry v1alpha3.ManifestObject) bool {
			return slices.Contains(keys, manifestObjectKey(entry, project.Name))
		})
		// Both writes have to run: a change to the list must not skip the journal.
		manifestsChanged := setManifestObjects(status, left)
		journalChanged := setHandingOver(status, mergeHandingOver(status.HandingOver, journal))
		return manifestsChanged || journalChanged
	})
	if err != nil {
		return fmt.Errorf("record the handover of the objects of the manifests on the project: %w", err)
	}

	for _, key := range keys {
		object := withReleaseOwnership(rendered[key], key.namespace, release)
		err := m.client.Patch(ctx, object, client.Apply, client.FieldOwner(manifestsFieldManager), client.ForceOwnership)
		if err != nil {
			return fmt.Errorf("hand the %s '%s/%s' of the manifests over to the release: %w", key.kind, key.namespace, key.name, err)
		}
	}
	return nil
}

// maxBlockingObjectsInError caps the objects a blocked handover names, so that the condition of the
// project stays short.
const maxBlockingObjectsInError = 3

// handoverBlockedError says which objects keep the upgrade from taking the objects of the manifests
// over. Both lists are sorted, so the condition of the project changes only when they do. A key
// without a namespace is a cluster-scoped object.
func handoverBlockedError(handed, blocking []releaseKey) error {
	describe := func(keys []releaseKey) []string {
		keys = slices.SortedFunc(slices.Values(keys), compareReleaseKeys)
		names := make([]string, 0, len(keys))
		for _, key := range keys {
			if key.namespace == "" {
				names = append(names, fmt.Sprintf("%s %q", key.kind, key.name))
				continue
			}
			names = append(names, fmt.Sprintf("%s %q in namespace %q", key.kind, key.name, key.namespace))
		}
		return names
	}
	blockers := describe(blocking)
	if len(blockers) > maxBlockingObjectsInError {
		blockers = append(blockers[:maxBlockingObjectsInError], fmt.Sprintf("%d more", len(blocking)-maxBlockingObjectsInError))
	}
	exists := "exists"
	if len(blocking) > 1 {
		exists = "exist"
	}
	return fmt.Errorf("%s %s and cannot be imported into the release of the project: not an object of the manifests, "+
		"without the ownership marks of the release. The upgrade does not run, and the objects of the manifests it would "+
		"take over stay with the manifests: %s",
		strings.Join(blockers, ", "), exists, strings.Join(describe(handed), ", "))
}

// compareReleaseKeys orders release keys by namespace, kind, name and group.
func compareReleaseKeys(a, b releaseKey) int {
	return cmp.Or(cmp.Compare(a.namespace, b.namespace), cmp.Compare(a.kind, b.kind), cmp.Compare(a.name, b.name), cmp.Compare(a.group, b.group))
}

// handingOverOf is the status.handingOver entry of an object the upgrade is about to adopt.
func handingOverOf(object helm.ReleaseObject, namespace string, revision int) v1alpha3.HandingOverObject {
	return v1alpha3.HandingOverObject{
		APIVersion: object.APIVersion, Kind: object.Kind, Namespace: namespace, Name: object.Name, Revision: revision,
	}
}

// finishManifestHandover completes status.handingOver. An empty list costs nothing: the release is
// not read. An entry whose object the deployed revision holds, at the expected revision or a newer
// one, is dropped: the upgrade deployed it. Any other entry is put back under manifests. A zero
// revision was not filled in and is put back rather than treated as deployed. The pass is idempotent,
// and it touches only the objects the journal lists.
func (m *Manager) finishManifestHandover(ctx context.Context, project *v1alpha3.Project) error {
	if len(project.Status.HandingOver) == 0 {
		return nil
	}
	deployed, err := m.helmClient.CurrentRelease(ctx, project.Name)
	if err != nil {
		return fmt.Errorf("read the deployed release of the '%s' project: %w", project.Name, err)
	}
	// Snapshots: recordInStatus applies the change to the project first, which would otherwise hide
	// the list this pass decided about from the write of the stored status.
	processed := slices.Clone(project.Status.HandingOver)
	var back []v1alpha3.HandingOverObject
	for _, entry := range processed {
		if entry.Revision > 0 && deployed != nil && deployed.Revision >= entry.Revision && releaseHolds(deployed, entry, project.Name) {
			continue
		}
		restored, err := m.rollBackHandedObject(ctx, project, entry, deployed)
		if err != nil {
			return err
		}
		if restored {
			back = append(back, entry)
		}
	}
	err = m.recordInStatus(ctx, project, func(status *v1alpha3.ProjectStatus) bool {
		left := slices.DeleteFunc(slices.Clone(status.HandingOver), func(entry v1alpha3.HandingOverObject) bool {
			return slices.ContainsFunc(processed, func(seen v1alpha3.HandingOverObject) bool {
				return handingOverKey(seen) == handingOverKey(entry)
			})
		})
		changed := setHandingOver(status, left)
		if len(back) == 0 {
			return changed
		}
		restored := make([]v1alpha3.ManifestObject, 0, len(back))
		for _, entry := range back {
			restored = append(restored, v1alpha3.ManifestObject{
				APIVersion: entry.APIVersion, Kind: entry.Kind, Namespace: entry.Namespace, Name: entry.Name,
			})
		}
		return setManifestObjects(status, mergeManifestObjects(status.ManifestObjects, restored)) || changed
	})
	if err != nil {
		return fmt.Errorf("record the end of the handover of the manifests on the '%s' project: %w", project.Name, err)
	}
	return nil
}

// rollBackHandedObject puts one journalled object back under manifests: the manifests field manager
// applies it with the manifests labels and without the labels of the release (heritage, managed-by and
// the template label), which the controller may remove. The Helm ownership marks the handover added are removed only when the deployed revision
// does not hold the object. A missing object has nothing to apply; the journal entry is still dropped.
func (m *Manager) rollBackHandedObject(ctx context.Context, project *v1alpha3.Project, entry v1alpha3.HandingOverObject, deployed *helm.ProjectRelease) (bool, error) {
	live, err := m.liveObject(ctx, helm.ReleaseObject{APIVersion: entry.APIVersion, Kind: entry.Kind, Name: entry.Name}, entry.Namespace)
	if err != nil {
		return false, fmt.Errorf("get the %s '%s/%s': %w", entry.Kind, entry.Namespace, entry.Name, err)
	}
	if live == nil {
		return false, nil
	}
	object := live.DeepCopy()
	object.SetResourceVersion("")
	object.SetUID("")
	object.SetManagedFields(nil)
	labels := maps.Clone(object.GetLabels())
	if labels == nil {
		labels = map[string]string{}
	}
	delete(labels, v1alpha3.ResourceLabelHeritage)
	delete(labels, helm.ResourceLabelManagedBy)
	delete(labels, v1alpha3.ResourceLabelTemplate)
	labels[v1alpha3.ResourceLabelProject] = project.Name
	labels[v1alpha3.ResourceLabelSource] = v1alpha3.ResourceSourceManifests
	object.SetLabels(labels)
	if !releaseHolds(deployed, entry, project.Name) {
		annotations := maps.Clone(object.GetAnnotations())
		delete(annotations, helm.ResourceAnnotationReleaseName)
		delete(annotations, helm.ResourceAnnotationReleaseNamespace)
		object.SetAnnotations(annotations)
	}
	if err := m.client.Patch(ctx, object, client.Apply, client.FieldOwner(manifestsFieldManager), client.ForceOwnership); err != nil {
		return false, fmt.Errorf("put the %s '%s/%s' back under manifests: %w", entry.Kind, entry.Namespace, entry.Name, err)
	}
	return true, nil
}

// releaseHolds reports whether the deployed revision's manifest lists the object.
func releaseHolds(deployed *helm.ProjectRelease, entry v1alpha3.HandingOverObject, project string) bool {
	if deployed == nil {
		return false
	}
	key := handingOverKey(entry)
	return slices.ContainsFunc(deployed.Objects, func(object helm.ReleaseObject) bool {
		return keyOf(object, project) == key
	})
}

// handingOverKey identifies an entry the way a release object is identified, without pinning an
// empty namespace to a project: the entry already stores the namespace.
func handingOverKey(entry v1alpha3.HandingOverObject) releaseKey {
	group := ""
	if gv, err := schema.ParseGroupVersion(entry.APIVersion); err == nil {
		group = gv.Group
	}
	return releaseKey{group: group, kind: entry.Kind, namespace: entry.Namespace, name: entry.Name}
}

func mergeHandingOver(list, more []v1alpha3.HandingOverObject) []v1alpha3.HandingOverObject {
	merged := slices.Clone(more)
	for _, entry := range list {
		if !slices.ContainsFunc(more, func(other v1alpha3.HandingOverObject) bool {
			return handingOverKey(other) == handingOverKey(entry)
		}) {
			merged = append(merged, entry)
		}
	}
	slices.SortFunc(merged, func(a, b v1alpha3.HandingOverObject) int {
		return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Name, b.Name), cmp.Compare(a.APIVersion, b.APIVersion))
	})
	return merged
}

func setHandingOver(status *v1alpha3.ProjectStatus, entries []v1alpha3.HandingOverObject) bool {
	if len(entries) == 0 {
		entries = nil
	}
	if slices.Equal(status.HandingOver, entries) {
		return false
	}
	status.HandingOver = entries
	return true
}

// addedObjects lists the objects that the upgrade adds to the release: the targets the deployed
// revision does not hold, without the Namespace objects.
func addedObjects(project string, change helm.ReleaseChange) []helm.ReleaseObject {
	deployed := make(map[releaseKey]struct{}, len(change.Deployed.Objects))
	for _, object := range change.Deployed.Objects {
		deployed[keyOf(object, project)] = struct{}{}
	}
	var added []helm.ReleaseObject
	for _, target := range change.Targets {
		key := keyOf(target, project)
		if _, ok := deployed[key]; ok || key.group == "" && key.kind == "Namespace" {
			continue
		}
		added = append(added, target)
	}
	return added
}

// ownedByReleaseMarks reports whether a live object carries the marks Helm adopts an object of the
// release by: the managed-by label and the release name. The release namespace is not compared, so an
// object Helm might still refuse counts as the release's; the handover is then attempted and rolled
// back as before, rather than skipped for an object Helm would adopt.
func ownedByReleaseMarks(live *unstructured.Unstructured, release string) bool {
	return live.GetLabels()[helm.ResourceLabelManagedBy] == helm.ManagedByHelm &&
		live.GetAnnotations()[helm.ResourceAnnotationReleaseName] == release
}

// listedManifestObject reports whether status.manifestObjects of the project lists the object.
func listedManifestObject(project *v1alpha3.Project, key releaseKey) bool {
	return slices.ContainsFunc(project.Status.ManifestObjects, func(entry v1alpha3.ManifestObject) bool {
		return manifestObjectKey(entry, project.Name) == key
	})
}

// releaseObjectsOf reads the objects of a post-rendered release manifest that have the keys.
func releaseObjectsOf(manifest, project string, keys []releaseKey) (map[releaseKey]*unstructured.Unstructured, error) {
	objects := make(map[releaseKey]*unstructured.Unstructured, len(keys))
	err := eachManifestObject(manifest, func(object *unstructured.Unstructured) error {
		key := keyOf(releaseObjectOf(object), project)
		if slices.Contains(keys, key) {
			objects[key] = object
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, key := range keys {
		if objects[key] == nil {
			return nil, fmt.Errorf("the manifest has no %s '%s/%s'", key.kind, key.namespace, key.name)
		}
	}
	return objects, nil
}

// withReleaseOwnership is a copy of the rendered object in its namespace, with the ownership marks
// that let Helm take it over into the release: the managed-by label and the release annotations, as
// helm.ApplyReleaseOwnership writes them on the main namespace.
func withReleaseOwnership(rendered *unstructured.Unstructured, namespace, release string) *unstructured.Unstructured {
	object := rendered.DeepCopy()
	object.SetNamespace(namespace)
	labels := maps.Clone(object.GetLabels())
	if labels == nil {
		labels = make(map[string]string, 1)
	}
	labels[helm.ResourceLabelManagedBy] = helm.ManagedByHelm
	object.SetLabels(labels)
	annotations := maps.Clone(object.GetAnnotations())
	if annotations == nil {
		annotations = make(map[string]string, 2)
	}
	annotations[helm.ResourceAnnotationReleaseName] = release
	annotations[helm.ResourceAnnotationReleaseNamespace] = ""
	object.SetAnnotations(annotations)
	return object
}

// manifestsKeys lists by key the objects that the manifests of the template of the project render. A
// template without manifests, or with a Helm text, which the manifests do not apply with, lists none.
// So does a render that fails: the reconcile of the manifests reports it, and the upgrade goes on as
// it would without the manifests. The objects of a render that fails halfway are listed up to the
// document that fails.
func (m *Manager) manifestsKeys(ctx context.Context, project *v1alpha3.Project) (map[releaseKey]struct{}, error) {
	template, err := m.projectTemplateByName(ctx, project.Spec.ProjectTemplateName)
	if err != nil {
		return nil, fmt.Errorf("get the '%s' project template: %w", project.Spec.ProjectTemplateName, err)
	}
	if template == nil || !template.Spec.HasManifests() || template.Spec.HasResourcesTemplate() {
		return nil, nil
	}
	rendered, err := m.helmClient.RenderManifests(project, template.Spec.Manifests, template.Spec.ParametersSchema.OpenAPIV3Schema)
	if err != nil {
		return nil, nil //nolint:nilerr // the reconcile of the manifests reports the render error in ManifestsApplied
	}
	keys := make(map[releaseKey]struct{})
	// The error is the one the reconcile of the manifests reports, and the objects before it count.
	_ = eachManifestObject(rendered, func(object *unstructured.Unstructured) error {
		keys[keyOf(releaseObjectOf(object), project.Name)] = struct{}{}
		return nil
	})
	return keys, nil
}
