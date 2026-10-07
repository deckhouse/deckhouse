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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
)

// A release rendered from a Helm text holds whatever the text said: rules and bindings, quotas,
// network policies, workloads and their volumes. A copy of a former built-in Helm text, and the
// built-ins themselves before they became structured, rendered an AuthorizationRule with the Admin
// access level for every administrator and the all-pods ResourceQuota from the quota, next to which
// the standard fields add the d8-administrators ProjectRoleBinding (role d8:project:admin) and the
// d8-project-quota ResourceQuota from the same spec fields. An upgrade that drops such a rule or quota
// from the release -- a built-in rewritten with structured fields, a template switched to them, a
// text edit that removes the block -- would delete it, and leave the administrators without access
// and the namespace without a quota until the standard fields work. The handover closes that gap:
//
//   - A dropped object the standard fields replace is kept for the handover: the project gets the
//     HandoverPending condition, before any object is marked, so no mark is ever off the record; the
//     object gets helm.sh/resource-policy: keep, which Helm reads before it deletes an object that
//     left the release, and the kept-by-handover mark; and the standard fields are applied before the
//     upgrade. A dropped rule that names no current administrator is not kept, and loses the mark of
//     an earlier pass, so the upgrade deletes it; a kept rule loses the subjects that are no current
//     administrators. Such an object is an AuthorizationRule the
//     release created that grants the Admin access level and nothing else to User and Group subjects
//     that are all administrators, current or of the revision, or a ResourceQuota of the main
//     namespace without scopes whose every limit spec.quota sets, to the same value or a lower one.
//     Nothing else is: the rules a text renders from other parameters, such as the per-role rules of
//     a projectRoleBindings parameter, grant to subjects that are not administrators, and a quota that
//     spec.quota does not cover would lift limits.
//   - The kept objects are deleted once d8-administrators names the administrators and is Ready for
//     its generation, the d8:project:admin ClusterRole has its aggregated rules, d8-project-quota has
//     the limits of spec.quota and is calculated, and a grace period has passed after that, so every
//     API server has seen the role bindings. Until then the administrators hold both roles and the
//     tighter of the quotas applies. A kept rule that names no current administrator any more grants
//     only what was revoked, and it goes at once; a subject removed from the administrators leaves a
//     kept rule that names others at once.
//   - On a switch from a Helm text to structured fields, every other object of the release that the
//     native render drops is left in place for good instead of being deleted: it gets the keep
//     annotation too, loses the heritage label that makes it the module's, gets the
//     projects.deckhouse.io/kept-from-helm-template label and is listed in status.keptObjects of the
//     project. Under that label the d8-multitenancy-manager-kept-objects admission policy lets only
//     a requester who may make the same change across the cluster change or delete it, so a cluster
//     administrator may and the users of the project may not, until the label is removed. The list, its series and the
//     MultitenancyManagerProjectKeepsHelmTemplateObjects alert stay until each object is deleted,
//     loses the label or is rendered by the release again. A Namespace is never left: the release
//     always holds the main one. When the project is deleted, a left object in its namespaces goes
//     with them, and a cluster-scoped one that still carries the label is deleted with the project,
//     as the uninstall of the release would have done.
//
// An edit of a Helm text, or a move to another Helm text, deletes the other objects it drops as
// before: the text still says what the project holds.
//
// The HandoverPending condition lives in the status, which a replace of the Project does not touch,
// and the d8_multitenancy_project_handover_pending series follows it once the handover has had time
// to finish, so a handover that does not finish is seen in the MultitenancyManagerProjectHandoverStuck
// alert, and a project controller busy after an update raises none.

const (
	// helmResourcePolicy is the annotation Helm reads before it deletes an object that left the
	// release; with helmResourcePolicyKeep it leaves the object in place.
	helmResourcePolicy     = "helm.sh/resource-policy"
	helmResourcePolicyKeep = "keep"

	// keptByHandoverAnnotation marks an object a handover kept, next to the keep annotation. The Helm
	// text of a template may have asked Helm to keep an object for good; only what carries this mark
	// is deleted when the handover finishes.
	keptByHandoverAnnotation = "projects.deckhouse.io/kept-by-handover"

	// keptObjectsRecheck is how soon a project with kept objects looks again whether their
	// replacements work. The project controller is woken by d8-administrators and d8-project-quota,
	// but not by the aggregation of the ClusterRole, and an event can be missed.
	keptObjectsRecheck = time.Minute

	// handoverGrace is how long the kept objects stay after their replacements work: the API servers
	// authorize from an informer of the role bindings, and an HA control plane has several.
	handoverGrace = 2 * time.Minute

	// handoverExpected is how long a handover may take before its series says that it has not
	// finished. The standard fields work within seconds, and the grace period follows; but after an
	// update every built-in project starts a handover in its first reconcile, and the project
	// controller, with one worker, looks at a project again only once the first reconcile of every
	// other one is done. A look that finds the standard fields working finishes the handover.
	handoverExpected = 10 * time.Minute

	// leftObjectsRecheck is how soon a project that lists objects left from a Helm text release looks
	// again whether they are gone: the project controller does not watch them. keptObjectsMinCheck is
	// how often at most it looks, whatever wakes the project.
	leftObjectsRecheck  = 10 * time.Minute
	keptObjectsMinCheck = time.Minute
)

// authorizationRuleGVK is the kind of the rules a Helm text rendered. user-authz serves it; the
// controller scheme does not know it, so the rules are handled as unstructured objects.
var authorizationRuleGVK = schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "AuthorizationRule"}

// handoverPendingProjects is the series the MultitenancyManagerProjectHandoverStuck alert reads: one
// per project whose handover began more than handoverExpected ago and has not finished.
//
//	max by (project) (d8_multitenancy_project_handover_pending) > 0
var handoverPendingProjects = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "d8_multitenancy_project_handover_pending",
	Help: "A project whose AuthorizationRule and ResourceQuota objects kept from a Helm text release have waited for more than 10 minutes for the standard fields that replace them.",
}, []string{"project"})

// keptObjectsProjects is the series the MultitenancyManagerProjectKeepsHelmTemplateObjects alert
// reads: the number of objects in status.keptObjects of a project, while there are any.
//
//	max by (project) (d8_multitenancy_project_kept_objects) > 0
var keptObjectsProjects = prometheus.NewGaugeVec(prometheus.GaugeOpts{
	Name: "d8_multitenancy_project_kept_objects",
	Help: "The number of objects of a release rendered from a Helm text that were left in place, no longer managed.",
}, []string{"project"})

func init() {
	metrics.Registry.MustRegister(handoverPendingProjects, keptObjectsProjects)
}

// Forget drops the series of a project that is gone without the controller seeing its deletion
// through Delete, for example after its finalizer was removed by hand.
func (m *Manager) Forget(project string) {
	handoverPendingProjects.DeleteLabelValues(project)
	keptObjectsProjects.DeleteLabelValues(project)
	m.keptChecks.Delete(project)
}

// beforeUpgrade prepares an upgrade of the project release for what it drops (see helm.BeforeUpgrade).
func (m *Manager) beforeUpgrade(project *v1alpha3.Project) helm.BeforeUpgrade {
	return func(ctx context.Context, change helm.ReleaseChange) error {
		return m.prepareUpgrade(ctx, project, change)
	}
}

// prepareUpgrade keeps for the handover every live object the upgrade drops that the standard fields
// replace, and on a switch from a Helm text to structured fields leaves every other dropped object in
// place. It does nothing for an upgrade that drops no AuthorizationRule or ResourceQuota and is no
// switch. Every dropped object is read and sorted out before anything is written, and the record goes
// to the stored status before the first mark: a pass that stops halfway leaves no mark that is not on
// record, and the next pass, or the cleanup, finds what it marked.
func (m *Manager) prepareUpgrade(ctx context.Context, project *v1alpha3.Project, change helm.ReleaseChange) error {
	switching := change.Deployed.HelmText && change.Native
	dropped := droppedObjects(project.Name, change)
	if len(dropped) == 0 || !switching && !slices.ContainsFunc(dropped, isRuleOrQuota) {
		return nil
	}
	// Without the main namespace the standard fields cannot be created, and the upgrade that brings
	// the namespace back must not wait for them: nothing is handed over then, and on a switch the
	// objects that survived elsewhere are left like any other.
	mainExists, err := m.namespaceExists(ctx, project.Name)
	if err != nil {
		return err
	}

	administrators := handoverAdministrators(project, change.Deployed)
	current := administratorKeys(project.Spec.Administrators)
	var handOver, unmark, leave []*unstructured.Unstructured
	for _, object := range dropped {
		key := keyOf(object, project.Name)
		live, err := m.liveObject(ctx, object, key.namespace)
		if err != nil {
			return fmt.Errorf("get the %s '%s/%s': %w", object.Kind, key.namespace, object.Name, err)
		}
		// An unmanaged object was created once and has not been the release's since.
		if live == nil || isUnmanaged(live) {
			continue
		}
		// An object the Helm text itself asked Helm to keep is not the handover's to delete.
		replaced := mainExists && !keptByText(live) && replacedByStandardFields(project, key, live, administrators)
		switch {
		case replaced && (!isRule(key) || namesAnyOf(live, current)):
			handOver = append(handOver, live)
		case replaced:
			// It grants only to administrators removed from the project, and the upgrade deletes it: a
			// revocation is never delayed, so the mark of an earlier pass goes.
			unmark = append(unmark, live)
		case switching:
			leave = append(leave, live)
		case keptByHandover(live):
			// marked by an earlier pass, and an edit of a Helm text deletes what it drops
			unmark = append(unmark, live)
		}
	}

	left := make([]v1alpha3.KeptObject, 0, len(leave))
	for _, live := range leave {
		left = append(left, keptObjectOf(live))
	}
	handover := len(handOver) > 0
	err = m.recordInStatus(ctx, project, func(status *v1alpha3.ProjectStatus) bool {
		changed := addKeptObjects(status, left)
		if handover {
			changed = v1alpha3.SetCondition(&status.Conditions, v1alpha3.ProjectConditionHandoverPending, corev1.ConditionTrue,
				handoverStartedMessage()) || changed
		}
		return changed
	})
	if err != nil {
		return fmt.Errorf("record the preparation of the upgrade on the project: %w", err)
	}
	m.publishSeries(project)

	for _, live := range unmark {
		if err := m.unmarkKept(ctx, live); err != nil {
			return fmt.Errorf("unmark the %s '%s/%s': %w", live.GetKind(), live.GetNamespace(), live.GetName(), err)
		}
	}
	for _, live := range handOver {
		if err := m.keepForHandover(ctx, live, current); err != nil {
			return fmt.Errorf("keep the %s '%s/%s': %w", live.GetKind(), live.GetNamespace(), live.GetName(), err)
		}
	}
	for _, live := range leave {
		if err := m.leaveObject(ctx, live); err != nil {
			return fmt.Errorf("leave the %s '%s/%s': %w", live.GetKind(), live.GetNamespace(), live.GetName(), err)
		}
	}
	if !handover {
		return nil
	}
	if err := m.reconcileStandardFields(ctx, project); err != nil {
		return fmt.Errorf("apply the standard fields before the handover: %w", err)
	}
	return nil
}

// isRuleOrQuota reports whether the object of a release is an AuthorizationRule or a ResourceQuota,
// the kinds the standard fields replace.
func isRuleOrQuota(object helm.ReleaseObject) bool {
	key := keyOf(object, "")
	return isRule(key) || isQuota(key)
}

func isRule(key releaseKey) bool {
	return key.group == authorizationRuleGVK.Group && key.kind == authorizationRuleGVK.Kind
}

func isQuota(key releaseKey) bool {
	return key.group == "" && key.kind == "ResourceQuota"
}

// handoverAdministrators are the subjects whose AuthorizationRule objects d8-administrators replaces:
// the current administrators, and the ones the deployed revision was rendered with that are no
// administrators any more. One that the project parameters still name is not removed: the parameters
// keep the administrators the conversion could not lift into spec.administrators, d8-administrators
// does not grant them anything, and their rules are the text's.
func handoverAdministrators(project *v1alpha3.Project, deployed *helm.ProjectRelease) map[subjectKey]struct{} {
	administrators := administratorKeys(project.Spec.Administrators)
	named := administratorKeys(helm.ParameterAdministrators(project.Spec.Parameters))
	for _, administrator := range deployed.Administrators {
		key := subjectKey{kind: administrator.Kind, name: administrator.Name}
		if _, ok := named[key]; !ok {
			administrators[key] = struct{}{}
		}
	}
	return administrators
}

// replacedByStandardFields reports whether the standard fields of the project replace a live object
// the release created (see the classification at the top of the file).
func replacedByStandardFields(project *v1alpha3.Project, key releaseKey, live *unstructured.Unstructured, administrators map[subjectKey]struct{}) bool {
	switch {
	case !ownedByRelease(live, project.Name):
		// an object of that name the release did not create is not the handover's to delete
		return false
	case isRule(key):
		return isAdminLevelRule(live) && !slices.ContainsFunc(ruleSubjects(live), func(subject subjectKey) bool {
			_, ok := administrators[subject]
			return !ok
		})
	case isQuota(key):
		return key.namespace == project.Name && coveredByProjectQuota(project, live)
	}
	return false
}

// isAdminLevelRule reports whether an AuthorizationRule grants the Admin access level and nothing
// else, to User and Group subjects: no other spec field holds anything but the default of the CRD,
// and no subject is a ServiceAccount or has a namespace.
func isAdminLevelRule(rule *unstructured.Unstructured) bool {
	spec, ok := rule.Object["spec"].(map[string]any)
	if !ok || spec["accessLevel"] != "Admin" {
		return false
	}
	for field, value := range spec {
		switch field {
		case "accessLevel", "subjects":
		case "portForwarding", "allowScale":
			if value != nil && value != false {
				return false
			}
		default:
			return false
		}
	}
	subjects := ruleSubjects(rule)
	return len(subjects) > 0 && !slices.ContainsFunc(subjects, func(subject subjectKey) bool {
		return subject.kind != rbacv1.UserKind && subject.kind != rbacv1.GroupKind || subject.namespace != "" || subject.name == ""
	})
}

// coveredByProjectQuota reports whether d8-project-quota is at least as tight as a ResourceQuota of
// the main namespace: the quota has no scopes and no scope selector, and spec.quota sets every limit
// it has to the same value or a lower one. A quota without limits limits nothing.
func coveredByProjectQuota(project *v1alpha3.Project, live *unstructured.Unstructured) bool {
	quota := new(corev1.ResourceQuota)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(live.Object, quota); err != nil {
		return false
	}
	if len(quota.Spec.Scopes) > 0 || quota.Spec.ScopeSelector != nil {
		return false
	}
	for name, limit := range quota.Spec.Hard {
		own, ok := project.Spec.Quota[name]
		if !ok || own.Cmp(limit) > 0 {
			return false
		}
	}
	return true
}

// ownedByRelease reports whether a live object is one the release of the project created: Helm
// recorded the release in its annotation, and the post-renderer set the heritage label, which the
// d8-multitenancy-manager admission policy lets nobody but the controller set.
func ownedByRelease(object client.Object, project string) bool {
	return object.GetLabels()[v1alpha3.ResourceLabelHeritage] == v1alpha3.ResourceHeritageMultitenancy &&
		object.GetAnnotations()[helm.ResourceAnnotationReleaseName] == helm.ReleaseName(project)
}

// subjectKey identifies a subject the way spec.administrators and an AuthorizationRule both name
// one. Administrators have no namespace, so a ServiceAccount subject never matches one.
type subjectKey struct {
	kind, name, namespace string
}

// administratorKeys is the set of the administrators.
func administratorKeys(administrators []v1alpha3.Administrator) map[subjectKey]struct{} {
	keys := make(map[subjectKey]struct{}, len(administrators))
	for _, administrator := range administrators {
		keys[subjectKey{kind: administrator.Kind, name: administrator.Name}] = struct{}{}
	}
	return keys
}

// ruleSubjects lists the subjects the AuthorizationRule names. A subject of another shape comes back
// as a key that matches no administrator.
func ruleSubjects(rule *unstructured.Unstructured) []subjectKey {
	field, _, _ := unstructured.NestedFieldNoCopy(rule.Object, "spec", "subjects")
	entries, _ := field.([]any)
	subjects := make([]subjectKey, 0, len(entries))
	for _, entry := range entries {
		fields, _ := entry.(map[string]any)
		kind, _ := fields["kind"].(string)
		name, _ := fields["name"].(string)
		namespace, _ := fields["namespace"].(string)
		subjects = append(subjects, subjectKey{kind: kind, name: name, namespace: namespace})
	}
	return subjects
}

// namesAnyOf reports whether the AuthorizationRule names one of the subjects.
func namesAnyOf(rule *unstructured.Unstructured, subjects map[subjectKey]struct{}) bool {
	return slices.ContainsFunc(ruleSubjects(rule), func(subject subjectKey) bool {
		_, ok := subjects[subject]
		return ok
	})
}

// keepForHandover annotates a live object with helm.sh/resource-policy: keep and the handover mark,
// so the upgrade leaves it and the cleanup deletes it once its replacement works. A rule loses the
// subjects that are no current administrators: a revocation is never delayed.
func (m *Manager) keepForHandover(ctx context.Context, live *unstructured.Unstructured, current map[subjectKey]struct{}) error {
	base := live.DeepCopy()
	narrowed := narrowRule(live, current)
	if keptByHandover(live) && !narrowed {
		return nil
	}
	annotations := maps.Clone(live.GetAnnotations())
	if annotations == nil {
		annotations = make(map[string]string, 2)
	}
	annotations[helmResourcePolicy] = helmResourcePolicyKeep
	annotations[keptByHandoverAnnotation] = "true"
	live.SetAnnotations(annotations)
	return m.client.Patch(ctx, live, client.MergeFrom(base))
}

// narrowKeptRule removes from a kept rule the subjects that are no current administrators.
func (m *Manager) narrowKeptRule(ctx context.Context, rule *unstructured.Unstructured, current map[subjectKey]struct{}) error {
	base := rule.DeepCopy()
	if !narrowRule(rule, current) {
		return nil
	}
	if err := m.client.Patch(ctx, rule, client.MergeFrom(base)); err != nil {
		return fmt.Errorf("remove the subjects that are no administrators from the AuthorizationRule '%s/%s': %w",
			rule.GetNamespace(), rule.GetName(), err)
	}
	return nil
}

// narrowRule drops from spec.subjects of an AuthorizationRule the subjects that are not among current,
// and reports whether it dropped any. Another object is left as it is.
func narrowRule(object *unstructured.Unstructured, current map[subjectKey]struct{}) bool {
	if object.GroupVersionKind().GroupKind() != authorizationRuleGVK.GroupKind() {
		return false
	}
	field, _, _ := unstructured.NestedFieldNoCopy(object.Object, "spec", "subjects")
	entries, _ := field.([]any)
	subjects := ruleSubjects(object)
	kept := make([]any, 0, len(entries))
	for i, entry := range entries {
		if _, ok := current[subjects[i]]; ok {
			kept = append(kept, entry)
		}
	}
	if len(kept) == len(entries) {
		return false
	}
	return unstructured.SetNestedSlice(object.Object, kept, "spec", "subjects") == nil
}

// unmarkKept removes the handover mark from a live object, and the keep annotation the mark says the
// handover added, so the upgrade deletes the object.
func (m *Manager) unmarkKept(ctx context.Context, live *unstructured.Unstructured) error {
	annotations := maps.Clone(live.GetAnnotations())
	delete(annotations, keptByHandoverAnnotation)
	delete(annotations, helmResourcePolicy)
	base := live.DeepCopy()
	live.SetAnnotations(annotations)
	return m.client.Patch(ctx, live, client.MergeFrom(base))
}

// isUnmanaged reports whether the object carries the unmanaged label, which the post-renderer reads
// by its key, whatever its value.
func isUnmanaged(object client.Object) bool {
	_, unmanaged := object.GetLabels()[v1alpha3.ResourceLabelUnmanaged]
	return unmanaged
}

// keptByHandover reports whether the object is one a handover kept.
func keptByHandover(object client.Object) bool {
	return object.GetAnnotations()[keptByHandoverAnnotation] == "true"
}

// keptByText reports whether the Helm text itself asked Helm to keep the object: it carries
// helm.sh/resource-policy: keep without the handover mark.
func keptByText(object client.Object) bool {
	return object.GetAnnotations()[helmResourcePolicy] == helmResourcePolicyKeep && !keptByHandover(object)
}

// droppedObjects lists the objects of the deployed revision that the upgrade takes out of the
// release, without the Namespace objects.
func droppedObjects(project string, change helm.ReleaseChange) []helm.ReleaseObject {
	stays := make(map[releaseKey]struct{}, len(change.Targets))
	for _, target := range change.Targets {
		stays[keyOf(target, project)] = struct{}{}
	}
	var dropped []helm.ReleaseObject
	for _, object := range change.Deployed.Objects {
		key := keyOf(object, project)
		if _, ok := stays[key]; ok || key.group == "" && key.kind == "Namespace" {
			continue
		}
		dropped = append(dropped, object)
	}
	return dropped
}

// releaseKey identifies an object of a release across API versions of its kind, the way Helm matches
// the objects of two revisions.
type releaseKey struct {
	group, kind, namespace, name string
}

// keyOf is the key of an object of a release of the project; an object without a namespace is in the
// main namespace, where the post-renderer pins it.
func keyOf(object helm.ReleaseObject, project string) releaseKey {
	namespace := cmp.Or(object.Namespace, project)
	group := ""
	if gv, err := schema.ParseGroupVersion(object.APIVersion); err == nil {
		group = gv.Group
	}
	return releaseKey{group: group, kind: object.Kind, namespace: namespace, name: object.Name}
}

// releaseObjectOf identifies a live object the way a release manifest does.
func releaseObjectOf(live *unstructured.Unstructured) helm.ReleaseObject {
	return helm.ReleaseObject{APIVersion: live.GetAPIVersion(), Kind: live.GetKind(), Namespace: live.GetNamespace(), Name: live.GetName()}
}

// keptObjectOf is the status.keptObjects entry of a live object.
func keptObjectOf(live *unstructured.Unstructured) v1alpha3.KeptObject {
	return v1alpha3.KeptObject{APIVersion: live.GetAPIVersion(), Kind: live.GetKind(), Namespace: live.GetNamespace(), Name: live.GetName()}
}

// liveObject reads the object of a release as it is in the cluster, or returns nil when it is gone or
// its kind is not served any more. The read is unstructured, so it goes to the API server rather
// than to a cache that may not have seen the latest write yet. The namespace is used only for a
// namespaced kind: a manifest pins every object to a namespace of the project, a cluster-scoped one
// as well.
func (m *Manager) liveObject(ctx context.Context, object helm.ReleaseObject, namespace string) (*unstructured.Unstructured, error) {
	live := new(unstructured.Unstructured)
	live.SetAPIVersion(object.APIVersion)
	live.SetKind(object.Kind)
	// A kind the mapper does not know is read as a namespaced one, and the read says whether it is
	// served at all.
	namespaced, err := m.client.IsObjectNamespaced(live)
	if err != nil && !meta.IsNoMatchError(err) {
		return nil, fmt.Errorf("find the scope of the %s kind: %w", object.Kind, err)
	}
	if err == nil && !namespaced {
		namespace = ""
	}
	if err := m.client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: object.Name}, live); err != nil {
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, err
	}
	return live, nil
}

// leaveObject leaves a live object of a Helm text release in place for good: helm.sh/resource-policy:
// keep, so the upgrade does not delete it; no handover mark, so the cleanup does not either; no
// heritage label, so it is not the module's any more; and the kept-from-helm-template label, which
// lists it and under which the d8-multitenancy-manager-kept-objects admission policy lets only a
// requester who may make the same change across the cluster change or delete it. The
// ownership annotations of Helm stay, so a later render of an object of the same kind and name takes
// it over again.
func (m *Manager) leaveObject(ctx context.Context, live *unstructured.Unstructured) error {
	annotations := maps.Clone(live.GetAnnotations())
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	labels := maps.Clone(live.GetLabels())
	if labels == nil {
		labels = make(map[string]string, 1)
	}
	annotations[helmResourcePolicy] = helmResourcePolicyKeep
	delete(annotations, keptByHandoverAnnotation)
	delete(labels, v1alpha3.ResourceLabelHeritage)
	labels[v1alpha3.ResourceLabelKeptFromHelmTemplate] = "true"
	if maps.Equal(annotations, live.GetAnnotations()) && maps.Equal(labels, live.GetLabels()) {
		return nil
	}

	base := live.DeepCopy()
	live.SetAnnotations(annotations)
	live.SetLabels(labels)
	return m.client.Patch(ctx, live, client.MergeFrom(base))
}

// takeBackObject finishes an object the release holds again, because the template renders an object
// of its kind and name: it loses the kept-from-helm-template label and the handover mark, and gets the
// heritage label and the keep annotation as the manifest of the release has them. Helm puts the
// labels of the manifest back on an object of a built-in kind it takes over, but not on a custom
// resource, whose manifests do not differ; and a keep that the Helm text sets itself stays, so the
// release does not delete the object when the template stops rendering it.
func (m *Manager) takeBackObject(ctx context.Context, live *unstructured.Unstructured, rendered helm.ReleaseObject) error {
	labels := maps.Clone(live.GetLabels())
	if labels == nil {
		labels = make(map[string]string, 1)
	}
	annotations := maps.Clone(live.GetAnnotations())
	if annotations == nil {
		annotations = make(map[string]string, 1)
	}
	delete(labels, v1alpha3.ResourceLabelKeptFromHelmTemplate)
	delete(annotations, keptByHandoverAnnotation)
	if rendered.Heritage {
		labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
	}
	switch {
	case rendered.Keep:
		annotations[helmResourcePolicy] = helmResourcePolicyKeep
	case annotations[helmResourcePolicy] == helmResourcePolicyKeep:
		delete(annotations, helmResourcePolicy)
	}
	if maps.Equal(labels, live.GetLabels()) && maps.Equal(annotations, live.GetAnnotations()) {
		return nil
	}
	base := live.DeepCopy()
	live.SetLabels(labels)
	live.SetAnnotations(annotations)
	return m.client.Patch(ctx, live, client.MergeFrom(base))
}

// addKeptObjects adds the objects to status.keptObjects and reports whether the list changed.
func addKeptObjects(status *v1alpha3.ProjectStatus, objects []v1alpha3.KeptObject) bool {
	merged := mergeKeptObjects(status.KeptObjects, objects)
	if slices.Equal(merged, status.KeptObjects) {
		return false
	}
	status.KeptObjects = merged
	return true
}

// mergeKeptObjects is the union of the two lists, sorted.
func mergeKeptObjects(list, more []v1alpha3.KeptObject) []v1alpha3.KeptObject {
	merged := slices.Concat(list, more)
	slices.SortFunc(merged, func(a, b v1alpha3.KeptObject) int {
		return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Name, b.Name), cmp.Compare(a.APIVersion, b.APIVersion))
	})
	return slices.Compact(merged)
}

// refreshKeptObjects drops from status.keptObjects of the project the objects that are gone, no
// longer carry the kept-from-helm-template label, or that the release holds again, and publishes what
// is left. Whether the release holds an object is read from its deployed revision, not from the
// heritage label: Helm does not put the label back on a custom resource it takes over, and a rollback
// puts it back on an object of a built-in kind the upgrade then drops again. A left object that has
// the label back without being the release's loses it again. It returns how soon to look again while
// something is left: nothing the project controller watches says when one goes. The status is
// written by the reconcile that calls it.
func (m *Manager) refreshKeptObjects(ctx context.Context, project *v1alpha3.Project) (time.Duration, error) {
	if len(project.Status.KeptObjects) == 0 {
		keptObjectsProjects.DeleteLabelValues(project.Name)
		m.keptChecks.Delete(project.Name)
		return 0, nil
	}
	// A project is reconciled on every change of the usage of its quota, and every look reads the
	// release and each object from the API server; once a minute is enough to notice what goes.
	if last, ok := m.keptChecks.Load(project.Name); ok {
		if checked, _ := last.(time.Time); time.Since(checked) < keptObjectsMinCheck {
			keptObjectsProjects.WithLabelValues(project.Name).Set(float64(len(project.Status.KeptObjects)))
			return leftObjectsRecheck, nil
		}
	}
	m.keptChecks.Store(project.Name, time.Now())
	held, err := m.releaseHolds(ctx, project)
	if err != nil {
		return 0, err
	}
	var still []v1alpha3.KeptObject
	for _, kept := range project.Status.KeptObjects {
		object := helm.ReleaseObject{APIVersion: kept.APIVersion, Kind: kept.Kind, Namespace: kept.Namespace, Name: kept.Name}
		live, err := m.liveObject(ctx, object, kept.Namespace)
		if err != nil {
			return 0, fmt.Errorf("get the %s '%s/%s' left from the Helm text release: %w", kept.Kind, kept.Namespace, kept.Name, err)
		}
		if live == nil {
			continue
		}
		if rendered, ok := held[keyOf(object, project.Name)]; ok {
			if err := m.takeBackObject(ctx, live, rendered); err != nil {
				return 0, fmt.Errorf("take back the %s '%s/%s': %w", kept.Kind, kept.Namespace, kept.Name, err)
			}
			continue
		}
		if live.GetLabels()[v1alpha3.ResourceLabelKeptFromHelmTemplate] != "true" {
			continue
		}
		if _, ok := live.GetLabels()[v1alpha3.ResourceLabelHeritage]; ok {
			if err := m.leaveObject(ctx, live); err != nil {
				return 0, fmt.Errorf("leave the %s '%s/%s': %w", kept.Kind, kept.Namespace, kept.Name, err)
			}
		}
		still = append(still, kept)
	}
	project.Status.KeptObjects = still
	if len(still) == 0 {
		keptObjectsProjects.DeleteLabelValues(project.Name)
		return 0, nil
	}
	keptObjectsProjects.WithLabelValues(project.Name).Set(float64(len(still)))
	return leftObjectsRecheck, nil
}

// deleteLeftClusterObjects deletes, with the project, the cluster-scoped objects of its
// status.keptObjects that are still left: they carry the kept-from-helm-template label, the project
// label and the release name of the project. The uninstall of the release would have deleted them,
// and nothing else does, as they are in no namespace of the project. An object whose label was
// removed is its owners' and stays.
func (m *Manager) deleteLeftClusterObjects(ctx context.Context, project *v1alpha3.Project) error {
	for _, kept := range project.Status.KeptObjects {
		if kept.Namespace != "" {
			continue
		}
		object := helm.ReleaseObject{APIVersion: kept.APIVersion, Kind: kept.Kind, Name: kept.Name}
		live, err := m.liveObject(ctx, object, "")
		if err != nil {
			return fmt.Errorf("get the %s '%s' left from the Helm text release: %w", kept.Kind, kept.Name, err)
		}
		if live == nil || live.GetNamespace() != "" ||
			live.GetLabels()[v1alpha3.ResourceLabelKeptFromHelmTemplate] != "true" ||
			live.GetLabels()[v1alpha3.ResourceLabelProject] != project.Name ||
			live.GetAnnotations()[helm.ResourceAnnotationReleaseName] != helm.ReleaseName(project.Name) {
			continue
		}
		if err := m.deleteKept(ctx, live); err != nil {
			return err
		}
	}
	return nil
}

// namespaceExists reports whether the main namespace of the project exists.
func (m *Manager) namespaceExists(ctx context.Context, name string) (bool, error) {
	err := m.client.Get(ctx, client.ObjectKey{Name: name}, new(corev1.Namespace))
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return false, fmt.Errorf("get the '%s' namespace: %w", name, err)
	}
}

// finishHandover removes the record of a finished handover: the condition, on the project and in its
// stored status, and the series of the project.
func (m *Manager) finishHandover(ctx context.Context, project *v1alpha3.Project) error {
	err := m.recordInStatus(ctx, project, func(status *v1alpha3.ProjectStatus) bool {
		return v1alpha3.RemoveCondition(&status.Conditions, v1alpha3.ProjectConditionHandoverPending)
	})
	if err != nil {
		return fmt.Errorf("remove the record of the handover from the project: %w", err)
	}
	m.publishSeries(project)
	return nil
}

// recordInStatus applies change to the status of the project and to its stored status, and writes the
// stored status when change reports that it changed it. The records that span reconciles, of a
// handover and of the objects left from a Helm text release, are written this way at once, so they do
// not depend on the rest of the reconcile. The project is read from the cache, which can lag behind a
// write of the same reconcile, so a conflict is retried with a growing backoff.
func (m *Manager) recordInStatus(ctx context.Context, project *v1alpha3.Project, change func(*v1alpha3.ProjectStatus) bool) error {
	change(&project.Status)
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		stored := new(v1alpha3.Project)
		if err := m.client.Get(ctx, client.ObjectKey{Name: project.Name}, stored); err != nil {
			return fmt.Errorf("get the '%s' project: %w", project.Name, err)
		}
		if !change(&stored.Status) {
			return nil
		}
		return m.client.Status().Update(ctx, stored)
	})
}

// publishSeries publishes the series of the project from its status: one while a handover that
// began more than handoverExpected ago is on record, and the number of the objects in
// status.keptObjects while there are any.
func (m *Manager) publishSeries(project *v1alpha3.Project) {
	pending := project.IsConditionTrue(v1alpha3.ProjectConditionHandoverPending)
	if pending && time.Since(handoverStart(project)) >= handoverExpected {
		handoverPendingProjects.WithLabelValues(project.Name).Set(1)
	} else {
		handoverPendingProjects.DeleteLabelValues(project.Name)
	}
	if len(project.Status.KeptObjects) > 0 {
		keptObjectsProjects.WithLabelValues(project.Name).Set(float64(len(project.Status.KeptObjects)))
	} else {
		keptObjectsProjects.DeleteLabelValues(project.Name)
	}
}

// handoverStartedMessage is the HandoverPending message of a handover that has just kept the
// objects of the release.
func handoverStartedMessage() string {
	return fmt.Sprintf("The AuthorizationRule and ResourceQuota objects of the Helm text release that the standard fields "+
		"replace are kept until the %s ProjectRoleBinding names the administrators of the project and is Ready for its "+
		"current generation, the %s ClusterRole has its aggregated rules and the %s ResourceQuota has the limits of the "+
		"project and is calculated, and for at least %d minutes after the binding became Ready and the handover began.",
		v1alpha3.ProjectAdministratorsBinding, v1alpha3.ProjectAdministratorsRoleName, v1alpha3.ProjectQuotaName,
		int(handoverGrace/time.Minute))
}

// handoverWaitMessage is the HandoverPending message of a handover whose kept objects wait for the
// given reasons.
func handoverWaitMessage(waits []string) string {
	return "The AuthorizationRule and ResourceQuota objects kept from the Helm text release are not deleted yet, because " +
		strings.Join(waits, " and ") + ". Until then the kept AuthorizationRule objects keep granting access and the kept " +
		"quotas still apply."
}

// handoverGraceMessage is the HandoverPending message of a handover whose kept objects are deleted
// once the grace period ends at the given time. The message names the time rather than what is left
// of the period, so it does not change, and does not write the status, on every look.
func handoverGraceMessage(deadline time.Time) string {
	return fmt.Sprintf("The standard fields that replace the AuthorizationRule and ResourceQuota objects kept from the Helm "+
		"text release work, and the kept objects are deleted after %s.", deadline.UTC().Format(time.RFC3339))
}

// deleteKeptObjects finishes the handover of the project, if one is on record. A kept rule that names
// no current administrator is deleted at once, and one that names others too loses them at once. The
// other kept objects are deleted once the standard
// fields that replace them work and the grace period has passed, rules first and quotas last, and then
// the record goes. Until then it returns how soon to look again and says in the HandoverPending
// condition what the objects wait for. A kept object that the release renders again is the release's
// once more and loses the mark; one that the standard fields no longer replace, a quota that
// spec.quota no longer covers for example, is left in place for good. The release is read only when
// something is to be done, so a handover that waits costs no read of the Helm history.
func (m *Manager) deleteKeptObjects(ctx context.Context, project *v1alpha3.Project) (time.Duration, error) {
	if !project.IsConditionTrue(v1alpha3.ProjectConditionHandoverPending) {
		return 0, nil
	}

	kept, err := m.listKeptByHandover(ctx, project)
	if err != nil {
		return 0, err
	}
	current := administratorKeys(project.Spec.Administrators)
	var stale, revoked, waitingRules, waitingQuotas []*unstructured.Unstructured
	limited := false
	for _, object := range kept {
		key := keyOf(releaseObjectOf(object), project.Name)
		switch {
		case !ownedByRelease(object, project.Name) ||
			isRule(key) && !isAdminLevelRule(object) ||
			isQuota(key) && (key.namespace != project.Name || !coveredByProjectQuota(project, object)):
			stale = append(stale, object)
		case isRule(key) && !namesAnyOf(object, current):
			revoked = append(revoked, object)
		case isRule(key):
			waitingRules = append(waitingRules, object)
		default:
			waitingQuotas = append(waitingQuotas, object)
			hard, _, _ := unstructured.NestedMap(object.Object, "spec", "hard")
			limited = limited || len(hard) > 0
		}
	}

	var recheck time.Duration
	var message string
	if len(waitingRules) > 0 || len(waitingQuotas) > 0 {
		waits, since, err := m.standardFieldsWaits(ctx, project, len(waitingRules) > 0, limited)
		if err != nil {
			return 0, err
		}
		if started := handoverStart(project); started.After(since) {
			since = started
		}
		deadline := since.Add(handoverGrace)
		switch {
		case len(waits) > 0:
			m.logger.Info("the objects kept from the Helm text release wait for the standard fields to work",
				"project", project.Name, "waits", waits)
			recheck, message = keptObjectsRecheck, handoverWaitMessage(waits)
		case time.Now().Before(deadline):
			recheck, message = time.Until(deadline), handoverGraceMessage(deadline)
		}
	}
	remove := revoked
	if recheck == 0 {
		// rules first and quotas last: a reconcile that fails in between leaves a quota, which only limits
		remove = slices.Concat(revoked, waitingRules, waitingQuotas)
	}

	// A kept object the release renders again is the release's once more. The release is read only
	// when something is to be done, so a handover that waits costs no read of the Helm history.
	var held map[releaseKey]helm.ReleaseObject
	if len(stale) > 0 || len(remove) > 0 {
		if held, err = m.releaseHolds(ctx, project); err != nil {
			return 0, err
		}
	}
	var takeBack, leave, deletes []*unstructured.Unstructured
	for _, object := range stale {
		if _, ok := held[keyOf(releaseObjectOf(object), project.Name)]; ok {
			takeBack = append(takeBack, object)
		} else {
			leave = append(leave, object)
		}
	}
	for _, object := range remove {
		if _, ok := held[keyOf(releaseObjectOf(object), project.Name)]; ok {
			takeBack = append(takeBack, object)
		} else {
			deletes = append(deletes, object)
		}
	}

	// What is left is recorded before it loses the mark, and the record of the handover goes only
	// after the last delete: a reconcile that fails halfway leaves nothing that is not on record.
	left := make([]v1alpha3.KeptObject, 0, len(leave))
	for _, object := range leave {
		left = append(left, keptObjectOf(object))
	}
	err = m.recordInStatus(ctx, project, func(status *v1alpha3.ProjectStatus) bool {
		changed := addKeptObjects(status, left)
		if recheck > 0 {
			changed = v1alpha3.SetCondition(&status.Conditions, v1alpha3.ProjectConditionHandoverPending, corev1.ConditionTrue,
				message) || changed
		}
		return changed
	})
	if err != nil {
		return 0, fmt.Errorf("record the handover on the project: %w", err)
	}
	m.publishSeries(project)

	if recheck > 0 {
		for _, rule := range waitingRules {
			if err := m.narrowKeptRule(ctx, rule, current); err != nil {
				return 0, err
			}
		}
	}
	for _, object := range takeBack {
		if err := m.takeBackObject(ctx, object, held[keyOf(releaseObjectOf(object), project.Name)]); err != nil {
			return 0, fmt.Errorf("take back the %s '%s/%s': %w", object.GetKind(), object.GetNamespace(), object.GetName(), err)
		}
	}
	for _, object := range leave {
		if err := m.leaveObject(ctx, object); err != nil {
			return 0, fmt.Errorf("leave the %s '%s/%s': %w", object.GetKind(), object.GetNamespace(), object.GetName(), err)
		}
	}
	for _, object := range deletes {
		if err := m.deleteKept(ctx, object); err != nil {
			return 0, err
		}
	}
	if recheck > 0 {
		return recheck, nil
	}
	if err := m.finishHandover(ctx, project); err != nil {
		return 0, err
	}
	m.logger.Info("finished the handover of the objects kept from the Helm text release", "project", project.Name,
		"deleted", len(deletes), "left", len(leave), "takenBack", len(takeBack))
	return 0, nil
}

// listKeptByHandover lists the AuthorizationRule and ResourceQuota objects a handover of the project
// kept, namespace by namespace of the project. The lists are unstructured, so they are read from the
// API server: in the reconcile that starts the handover, a cache may not have seen the mark yet. A
// cluster that does not serve AuthorizationRule has none of them.
func (m *Manager) listKeptByHandover(ctx context.Context, project *v1alpha3.Project) ([]*unstructured.Unstructured, error) {
	namespaces := []string{project.Name}
	for _, namespace := range project.Status.Namespaces {
		if namespace.Name != project.Name {
			namespaces = append(namespaces, namespace.Name)
		}
	}
	var kept []*unstructured.Unstructured
	for _, kind := range []schema.GroupVersionKind{
		authorizationRuleGVK.GroupVersion().WithKind(authorizationRuleGVK.Kind + "List"),
		corev1.SchemeGroupVersion.WithKind("ResourceQuotaList"),
	} {
		for _, namespace := range namespaces {
			list := new(unstructured.UnstructuredList)
			list.SetGroupVersionKind(kind)
			err := m.client.List(ctx, list, client.InNamespace(namespace), client.MatchingLabels{v1alpha3.ResourceLabelProject: project.Name})
			if meta.IsNoMatchError(err) {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("list the %s objects in the '%s' namespace: %w", strings.TrimSuffix(kind.Kind, "List"), namespace, err)
			}
			for i := range list.Items {
				if keptByHandover(&list.Items[i]) {
					kept = append(kept, &list.Items[i])
				}
			}
		}
	}
	return kept, nil
}

// releaseHolds is the objects the deployed revision of the project release holds, by their key.
func (m *Manager) releaseHolds(ctx context.Context, project *v1alpha3.Project) (map[releaseKey]helm.ReleaseObject, error) {
	release, err := m.helmClient.CurrentRelease(ctx, project.Name)
	if err != nil {
		return nil, fmt.Errorf("read the release of the project: %w", err)
	}
	if release == nil {
		return map[releaseKey]helm.ReleaseObject{}, nil
	}
	held := make(map[releaseKey]helm.ReleaseObject, len(release.Objects))
	for _, object := range release.Objects {
		held[keyOf(object, project.Name)] = object
	}
	return held, nil
}

// deleteKept deletes a kept object as it was read: an object that changed since is not deleted, the
// conflict fails the reconcile, and the next one reads the object again.
func (m *Manager) deleteKept(ctx context.Context, object *unstructured.Unstructured) error {
	uid, version := object.GetUID(), object.GetResourceVersion()
	err := m.client.Delete(ctx, object, client.Preconditions{UID: &uid, ResourceVersion: &version})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete the %s '%s/%s': %w", object.GetKind(), object.GetNamespace(), object.GetName(), err)
	}
	return nil
}

// standardFieldsWaits lists what keeps the standard fields from replacing what a handover kept, and
// tells since when they have worked. For kept rules, the administrators binding has to name the
// administrators of the project and be fanned out for that spec, and the d8:project:admin role it
// binds has to have its aggregated rules: an aggregated ClusterRole is created empty and filled by
// the aggregation controller. For kept quotas with limits, the project quota has to have the limits
// of spec.quota, and the resourcequota controller has to have calculated it: until it has, the quota
// limits nothing. The binding and the quota are read from the cache, which may not have seen the
// spec the same reconcile has just written. An empty list means the replacements work.
func (m *Manager) standardFieldsWaits(ctx context.Context, project *v1alpha3.Project, rules, quotas bool) ([]string, time.Time, error) {
	var waits []string
	var since time.Time
	if rules {
		binding := new(v1alpha3.ProjectRoleBinding)
		key := client.ObjectKey{Namespace: project.Name, Name: v1alpha3.ProjectAdministratorsBinding}
		switch err := m.client.Get(ctx, key, binding); {
		case apierrors.IsNotFound(err):
			waits = append(waits, fmt.Sprintf("the %s ProjectRoleBinding does not exist", key.Name))
		case err != nil:
			return nil, since, fmt.Errorf("get the administrators binding: %w", err)
		case !bindsTheAdministrators(binding, project.Spec.Administrators):
			waits = append(waits, fmt.Sprintf("the %s ProjectRoleBinding does not name the administrators of the project yet", key.Name))
		default:
			ready := readyCondition(binding.Status.Conditions)
			if binding.Status.ObservedGeneration != binding.Generation || ready == nil || ready.Status != corev1.ConditionTrue {
				waits = append(waits, fmt.Sprintf("the %s ProjectRoleBinding is not Ready for its current generation", key.Name))
			} else {
				since = ready.LastTransitionTime.Time
			}
		}

		role := new(rbacv1.ClusterRole)
		switch err := m.client.Get(ctx, client.ObjectKey{Name: v1alpha3.ProjectAdministratorsRoleName}, role); {
		case apierrors.IsNotFound(err):
			waits = append(waits, fmt.Sprintf("the %s ClusterRole does not exist", v1alpha3.ProjectAdministratorsRoleName))
		case err != nil:
			return nil, since, fmt.Errorf("get the %s ClusterRole: %w", v1alpha3.ProjectAdministratorsRoleName, err)
		case len(role.Rules) == 0:
			waits = append(waits, fmt.Sprintf("the %s ClusterRole has no aggregated rules yet", v1alpha3.ProjectAdministratorsRoleName))
		}
	}

	if quotas {
		quota := new(corev1.ResourceQuota)
		key := client.ObjectKey{Namespace: project.Name, Name: v1alpha3.ProjectQuotaName}
		switch err := m.client.Get(ctx, key, quota); {
		case apierrors.IsNotFound(err):
			waits = append(waits, fmt.Sprintf("the %s ResourceQuota does not exist", key.Name))
		case err != nil:
			return nil, since, fmt.Errorf("get the project quota: %w", err)
		case !sameQuantities(quota.Spec.Hard, project.Spec.Quota):
			waits = append(waits, fmt.Sprintf("the %s ResourceQuota does not have the limits of the project yet", key.Name))
		case !sameQuantities(quota.Status.Hard, quota.Spec.Hard):
			waits = append(waits, fmt.Sprintf("the %s ResourceQuota is not calculated", key.Name))
		}
	}
	return waits, since, nil
}

// readyCondition is the Ready condition of a ProjectRoleBinding, nil without one.
func readyCondition(conditions []v1alpha3.Condition) *v1alpha3.Condition {
	for i := range conditions {
		if conditions[i].Type == v1alpha3.ProjectRoleBindingConditionReady {
			return &conditions[i]
		}
	}
	return nil
}

// handoverStart is when the handover of the project began, the transition of its HandoverPending
// condition to True.
func handoverStart(project *v1alpha3.Project) time.Time {
	for _, cond := range project.Status.Conditions {
		if cond.Type == v1alpha3.ProjectConditionHandoverPending {
			return cond.LastTransitionTime.Time
		}
	}
	return time.Time{}
}

// bindsTheAdministrators reports whether the binding names exactly the administrators.
func bindsTheAdministrators(binding *v1alpha3.ProjectRoleBinding, administrators []v1alpha3.Administrator) bool {
	subjects := make(map[subjectKey]struct{}, len(binding.Spec.Subjects))
	for _, subject := range binding.Spec.Subjects {
		subjects[subjectKey{kind: subject.Kind, name: subject.Name, namespace: subject.Namespace}] = struct{}{}
	}
	return maps.Equal(subjects, administratorKeys(administrators))
}

// sameQuantities reports whether the two lists set the same resources to equal quantities; the
// resourcequota controller has calculated a quota when its status.hard says what its spec.hard says.
func sameQuantities(a, b corev1.ResourceList) bool {
	if len(a) != len(b) {
		return false
	}
	for name, quantity := range a {
		other, ok := b[name]
		if !ok || other.Cmp(quantity) != 0 {
			return false
		}
	}
	return true
}
