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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/csaupgrade"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/helm"
	"controller/internal/naming"
	"controller/internal/rolebinding"
)

// The objects that spec.manifests of a template renders are applied with the rights of a user of the
// project, not with the rights of the controller, which is bound to cluster-admin:
//
//   - The user is system:multitenancy-manager:project:<project> (naming.ProjectUser). The d8-manifests
//     ProjectRoleBinding, which the controller makes and keeps, grants it the d8:project:admin role,
//     and the project role binding controller fans it out into every namespace of the project. The
//     d8-multitenancy-manager-reserved-users admission policy refuses every other binding that names
//     a user under the prefix, unless the controller writes it, and the Project webhook refuses such a
//     user as an administrator, whom the controller would bind. So nothing else grants that user
//     anything, short of a Helm text that names it, which the controller applies with its own rights
//     and which can grant anything anyway.
//   - The objects are applied only once the binding is Ready for its generation and the role has its
//     aggregated rules, with server-side apply through a client that impersonates the user
//     (internal/impersonate), with the multitenancy-manager/manifests field manager and the force that
//     takes over the fields another manager set. Nothing else here acts as the user.
//   - Every object is namespaced and lands in a namespace of the project: the main one when it names
//     none. A cluster-scoped object, an object in another namespace, and an object with the heritage
//     label of the module or of Deckhouse are refused before anything is applied: the admission
//     policies of those labels refuse a write by anybody else than the controller or Deckhouse, and
//     the refusal says so better than a 403 would. Every object gets the projects.deckhouse.io/project
//     label and projects.deckhouse.io/source: manifests.
//   - An object that exists already is written only when it is the project's: status.manifestObjects
//     lists it, a switch from a Helm text left it, or it has the labels of the manifests of the
//     project. Any other one is left alone and named in the condition, since the apply would
//     overwrite it and a later render would delete it. Helm refuses an object of another release for
//     the same reason.
//   - status.manifestObjects of the project lists what was applied. What is about to be applied goes on
//     the list before the apply, and an object that the template does not render any more is deleted,
//     as the user, once everything else is applied. One that lost the labels, or whose namespace left
//     the project, leaves the list and stays.
//   - An object that a switch from a Helm text left in place (status.keptObjects) and that the
//     manifests render under the same kind, namespace and name is taken over: the fields that the
//     release set go to the field manager of the manifests first, so the apply leaves the object as
//     the manifests render it, and it then loses the kept-from-helm-template label and leaves
//     status.keptObjects.
//
// The ManifestsApplied condition says what the objects wait for, why they are not applied, or which
// of them the API server refused, with the code of the refusal.

const (
	// manifestsFieldManager is the field manager of the server-side apply of the manifests.
	manifestsFieldManager = "multitenancy-manager/manifests"

	// releaseFieldManager is the field manager of the other writes of the controller, the ones of a
	// Helm release among them. Helm names it after the binary, multitenancy-manager, as the controller
	// sets no kube.ManagedFieldsManager, and the API server names the manager of a write of the
	// controller-runtime client after its user agent, which starts with the binary name too.
	releaseFieldManager = "multitenancy-manager"

	// manifestsBindingRecheck is how soon a project whose manifests wait for their binding looks
	// again. A change of the binding wakes the project as well; the role it binds does not.
	manifestsBindingRecheck = 30 * time.Second

	// manifestsKindRecheck is how soon a project whose manifests hold a kind the API server does not
	// serve looks again: the kind may come with a module that is being enabled.
	manifestsKindRecheck = time.Minute

	// manifestsForeignRecheck is how soon a project whose manifests render an object that exists and is
	// not theirs looks again: the project controller does not watch the object, and somebody may delete
	// it or hand it over.
	manifestsForeignRecheck = time.Minute

	// manifestsReapply is how long an unchanged render is not applied again. A project is reconciled
	// on every change of the usage of its quota, and an apply is a request per object. After this time
	// the fields the render sets are put back on an object that somebody changed, and an object that
	// somebody deleted is created again; a field that somebody added stays, as the apply does not own
	// it. The objects are not guarded otherwise: whoever may write them in the namespaces of the
	// project, its administrators among them, may change them in between.
	manifestsReapply = 10 * time.Minute

	// manifestsRefusalsShown is how many refused objects the condition names at most.
	manifestsRefusalsShown = 10
)

// errNoManifestsClients is the error of a manager that has no clients to act as the user of a project.
var errNoManifestsClients = errors.New("the controller has no client to apply the manifests of a template with")

// ManifestsClients hands out the clients that act as a user, as impersonate.Clients does.
type ManifestsClients interface {
	For(user string) (client.Client, error)
}

// Option configures a Manager.
type Option func(*Manager)

// WithManifestsClients sets the clients the manager applies spec.manifests of a template with.
func WithManifestsClients(clients ManifestsClients) Option {
	return func(m *Manager) {
		m.manifestsClients = clients
	}
}

// manifestsOutcome is what reconcileManifests found: how soon to look again, and whether the objects
// could not be applied, which puts the project in Error.
type manifestsOutcome struct {
	recheck time.Duration
	failed  bool
}

// appliedManifests is the render that the objects of a project were last applied from, and when.
type appliedManifests struct {
	digest string
	at     time.Time
}

// manifestsRefusedError says why the controller applies none of the objects of a render.
type manifestsRefusedError struct {
	reason string
}

func (e manifestsRefusedError) Error() string {
	return e.reason
}

// reconcileManifests applies the objects that spec.manifests of the template renders for the project,
// and deletes the ones applied before that it does not render any more, as the user of the project
// (see the comment at the top of the file). It sets the ManifestsApplied condition, except for a
// project whose template has no manifests and that has no objects to delete. It returns an error when
// the API server refused something that a retry may get through.
func (m *Manager) reconcileManifests(ctx context.Context, project *v1alpha3.Project) (manifestsOutcome, error) {
	template, err := m.projectTemplateByName(ctx, project.Spec.ProjectTemplateName)
	if err != nil {
		return m.manifestsFailed(project, "The project template is not read: "+err.Error()+"."), err
	}
	// gone since the render of this reconcile; the next one says so
	if template == nil {
		return manifestsOutcome{}, nil
	}
	wanted := template.Spec.HasManifests()
	if !wanted && len(project.Status.ManifestObjects) == 0 {
		m.manifestsApplied.Delete(project.Name)
		return manifestsOutcome{}, m.deleteManifestsBinding(ctx, project)
	}

	// A template renders from its Helm text or from its fields and manifests, never from both. The
	// template webhook refuses such a template, and one that got past it renders from its text.
	if wanted && template.Spec.HasResourcesTemplate() {
		return m.manifestsFailed(project, "The manifests of the template are not applied, because the template renders from its resourcesTemplate."), nil
	}

	var objects []*unstructured.Unstructured
	if wanted {
		rendered, err := m.helmClient.RenderManifests(project, template.Spec.Manifests, template.Spec.ParametersSchema.OpenAPIV3Schema)
		if err != nil {
			return m.manifestsFailed(project, fmt.Sprintf("The manifests of the template do not render: %v.", err)), nil
		}
		objects, err = m.manifestObjects(project, rendered)
		var refused manifestsRefusedError
		switch {
		case errors.As(err, &refused):
			return m.manifestsFailed(project, "The manifests of the template are not applied, because "+refused.Error()+"."), nil
		case meta.IsNoMatchError(err):
			outcome := m.manifestsFailed(project, fmt.Sprintf("The manifests of the template are not applied, because the API server does not serve a kind they hold: %v.", err))
			outcome.recheck = manifestsKindRecheck
			return outcome, nil
		case err != nil:
			return m.manifestsFailed(project, fmt.Sprintf("The manifests of the template are not applied: %v.", err)), err
		}
	}

	user := naming.ProjectUser(project.Name)
	waits, err := m.ensureManifestsBinding(ctx, project, user)
	if err != nil {
		return m.manifestsFailed(project, fmt.Sprintf("The binding of the %s user is not made: %v.", user, err)), err
	}
	if len(waits) > 0 {
		project.SetConditionFalse(v1alpha3.ProjectConditionManifestsApplied, fmt.Sprintf(
			"The objects of the manifests of the template are applied and deleted as the %s user once %s.", user, strings.Join(waits, " and ")))
		return manifestsOutcome{recheck: manifestsBindingRecheck}, nil
	}

	applied := manifestObjectsOf(objects)
	digest, err := manifestsDigest(objects)
	if err != nil {
		return m.manifestsFailed(project, fmt.Sprintf("The manifests of the template are not applied: %v.", err)), err
	}
	if since, recently := m.appliedRecently(project, applied, digest); recently {
		project.SetConditionTrue(v1alpha3.ProjectConditionManifestsApplied)
		return manifestsOutcome{recheck: manifestsReapply - since}, nil
	}
	// The objects are about to change, so the render they were last applied from does not describe
	// them any more: an apply that stops halfway leaves some of them as the new render has them, and a
	// template that goes back to the previous render has to be applied again.
	m.manifestsApplied.Delete(project.Name)

	if m.manifestsClients == nil {
		return m.manifestsFailed(project, "The controller has no client to apply the manifests of the template with."), errNoManifestsClients
	}
	actor, err := m.manifestsClients.For(user)
	if err != nil {
		return m.manifestsFailed(project, fmt.Sprintf("The client of the %s user is not made: %v.", user, err)), err
	}

	owned, foreign, refusals := m.ownManifestObjects(ctx, project, actor, objects)

	// What is about to be applied goes on record first, so a reconcile that stops halfway leaves no
	// applied object off the list.
	err = m.recordInStatus(ctx, project, func(status *v1alpha3.ProjectStatus) bool {
		return setManifestObjects(status, mergeManifestObjects(status.ManifestObjects, manifestObjectsOf(owned)))
	})
	if err != nil {
		return m.manifestsFailed(project, fmt.Sprintf("The objects of the manifests are not recorded on the project: %v.", err)), err
	}

	for _, object := range owned {
		if m.isKeptObject(project, object) {
			if err := takeOverReleaseFields(ctx, actor, object); err != nil {
				refusals = append(refusals, refusalOf(object, err))
				continue
			}
		}
		err := actor.Patch(ctx, object.DeepCopy(), client.Apply, client.FieldOwner(manifestsFieldManager), client.ForceOwnership)
		if err != nil {
			refusals = append(refusals, refusalOf(object, err))
			continue
		}
		if err := m.adoptKeptObject(ctx, project, object); err != nil {
			return m.manifestsFailed(project, fmt.Sprintf("An object left from the Helm text is not taken over: %v.", err)), err
		}
	}
	if len(foreign) > 0 || len(refusals) > 0 {
		var messages []string
		if len(foreign) > 0 {
			messages = append(messages, fmt.Sprintf("The manifests of the template do not take over objects that exist and are not theirs: %s.", joinRefusals(foreign)))
		}
		if len(refusals) == 0 {
			// nothing to retry: the objects wait for somebody to delete or hand them over
			outcome := m.manifestsFailed(project, strings.Join(messages, " "))
			outcome.recheck = manifestsForeignRecheck
			return outcome, nil
		}
		refused := joinRefusals(refusals)
		messages = append(messages, fmt.Sprintf("The API server refused to apply objects of the manifests of the template as the %s user: %s.", user, refused))
		return m.manifestsFailed(project, strings.Join(messages, " ")), fmt.Errorf("apply the manifests of the template as %s: %s", user, refused)
	}

	left, refusals := m.pruneManifestObjects(ctx, project, actor, applied)
	err = m.recordInStatus(ctx, project, func(status *v1alpha3.ProjectStatus) bool {
		return setManifestObjects(status, mergeManifestObjects(applied, left))
	})
	if err != nil {
		return m.manifestsFailed(project, fmt.Sprintf("The objects of the manifests are not recorded on the project: %v.", err)), err
	}
	if len(refusals) > 0 {
		refused := joinRefusals(refusals)
		message := fmt.Sprintf("The API server refused to delete objects that the manifests of the template no longer render, as the %s user: %s.",
			user, refused)
		return m.manifestsFailed(project, message), fmt.Errorf("delete what the manifests of the template no longer render as %s: %s", user, refused)
	}

	if !wanted {
		m.manifestsApplied.Delete(project.Name)
		return manifestsOutcome{}, m.deleteManifestsBinding(ctx, project)
	}
	m.manifestsApplied.Store(project.Name, appliedManifests{digest: digest, at: time.Now()})
	project.SetConditionTrue(v1alpha3.ProjectConditionManifestsApplied)
	return manifestsOutcome{recheck: manifestsReapply}, nil
}

// ownManifestObjects splits the objects of a render into the ones the apply may write and the ones
// it leaves alone, as Helm refuses an object that exists and is not its release's. An object that
// status.manifestObjects lists, or that a switch from a Helm text left (status.keptObjects), is the
// project's. Any other one is read as the user first: one that does not exist is created, and one
// that has the labels of the manifests of the project is the project's too, as when the list was lost.
// Every other object that exists is somebody else's, which the apply with force would overwrite and a
// later render would delete, and it is named in the second list. A read the API server refuses is
// named in the third one and retried.
func (m *Manager) ownManifestObjects(
	ctx context.Context,
	project *v1alpha3.Project,
	actor client.Client,
	objects []*unstructured.Unstructured,
) ([]*unstructured.Unstructured, []string, []string) {
	var owned []*unstructured.Unstructured
	var foreign, refusals []string
	for _, object := range objects {
		key := keyOf(releaseObjectOf(object), project.Name)
		listed := slices.ContainsFunc(project.Status.ManifestObjects, func(entry v1alpha3.ManifestObject) bool {
			return manifestObjectKey(entry, project.Name) == key
		})
		if listed || m.isKeptObject(project, object) {
			owned = append(owned, object)
			continue
		}

		live := new(unstructured.Unstructured)
		live.SetGroupVersionKind(object.GroupVersionKind())
		switch err := actor.Get(ctx, client.ObjectKeyFromObject(object), live); {
		case apierrors.IsNotFound(err):
			owned = append(owned, object)
		case err != nil:
			refusals = append(refusals, refusalOf(object, err))
		case isManifestObjectOf(live, project.Name):
			owned = append(owned, object)
		default:
			foreign = append(foreign, fmt.Sprintf(
				"%s %s/%s exists, and the manifests take over only the objects they applied and the ones a switch from a Helm text left; "+
					"delete it, or give it the %s: %s and %s: %s labels to hand it over",
				object.GetKind(), object.GetNamespace(), object.GetName(),
				v1alpha3.ResourceLabelProject, project.Name, v1alpha3.ResourceLabelSource, v1alpha3.ResourceSourceManifests))
		}
	}
	return owned, foreign, refusals
}

// isManifestObjectOf reports whether the object has the labels of an object applied from the manifests
// of the project.
func isManifestObjectOf(object *unstructured.Unstructured, project string) bool {
	labels := object.GetLabels()
	return labels[v1alpha3.ResourceLabelProject] == project && labels[v1alpha3.ResourceLabelSource] == v1alpha3.ResourceSourceManifests
}

// manifestsFailed sets ManifestsApplied to False with the message and says that the project is in Error.
func (m *Manager) manifestsFailed(project *v1alpha3.Project, message string) manifestsOutcome {
	project.SetConditionFalse(v1alpha3.ProjectConditionManifestsApplied, message)
	return manifestsOutcome{failed: true}
}

// manifestObjects reads the objects of a render of the manifests, in the order the render has them,
// List objects unwrapped. Each of them is pinned to a namespace of the project and gets the labels of
// the objects of the manifests; the fields a server sets are dropped, so that an object copied from a
// cluster applies. It returns a manifestsRefusedError for an object it does not apply, and a NoMatch
// error for a kind the API server does not serve.
func (m *Manager) manifestObjects(project *v1alpha3.Project, rendered string) ([]*unstructured.Unstructured, error) {
	documents := releaseutil.SplitManifests(rendered)
	names := slices.Collect(maps.Keys(documents))
	sort.Sort(releaseutil.BySplitManifestsOrder(names))

	namespaces := rolebinding.ProjectNamespaceNames(project)
	seen := make(map[releaseKey]struct{})
	var objects []*unstructured.Unstructured
	var add func(object *unstructured.Unstructured) error
	add = func(object *unstructured.Unstructured) error {
		if object.IsList() {
			return object.EachListItem(func(item runtime.Object) error {
				nested, ok := item.(*unstructured.Unstructured)
				if !ok {
					return nil
				}
				return add(nested)
			})
		}
		if err := m.pinManifestObject(project, namespaces, object); err != nil {
			return err
		}
		key := keyOf(releaseObjectOf(object), project.Name)
		if _, ok := seen[key]; ok {
			return manifestsRefusedError{reason: fmt.Sprintf("they hold the %s '%s/%s' twice", object.GetKind(), object.GetNamespace(), object.GetName())}
		}
		seen[key] = struct{}{}
		objects = append(objects, object)
		return nil
	}

	for _, name := range names {
		var fields map[string]any
		if err := yaml.Unmarshal([]byte(documents[name]), &fields); err != nil {
			return nil, manifestsRefusedError{reason: fmt.Sprintf("a rendered object is not valid YAML: %v", err)}
		}
		// a document of comments alone
		if len(fields) == 0 {
			continue
		}
		object := new(unstructured.Unstructured)
		if err := yaml.Unmarshal([]byte(documents[name]), object); err != nil || object.GetAPIVersion() == "" {
			return nil, manifestsRefusedError{reason: "a rendered object has no apiVersion or kind"}
		}
		if err := add(object); err != nil {
			return nil, err
		}
	}
	return objects, nil
}

// serverFields are the fields of an object that a server sets and an apply cannot.
var serverFields = [][]string{
	{"metadata", "resourceVersion"},
	{"metadata", "uid"},
	{"metadata", "generation"},
	{"metadata", "creationTimestamp"},
	{"metadata", "deletionTimestamp"},
	{"metadata", "deletionGracePeriodSeconds"},
	{"metadata", "managedFields"},
	{"metadata", "selfLink"},
	{"status"},
}

// pinManifestObject checks one object of the manifests and makes it ready to apply (see
// manifestObjects).
func (m *Manager) pinManifestObject(project *v1alpha3.Project, namespaces []string, object *unstructured.Unstructured) error {
	describe := func() string {
		return fmt.Sprintf("the %s '%s'", object.GetKind(), object.GetName())
	}
	if object.GetName() == "" {
		return manifestsRefusedError{reason: fmt.Sprintf("a %s %s has no metadata.name, and an object is applied by its name", object.GetAPIVersion(), object.GetKind())}
	}
	namespaced, err := m.client.IsObjectNamespaced(object)
	if err != nil {
		return fmt.Errorf("find the scope of %s: %w", describe(), err)
	}
	if !namespaced {
		return manifestsRefusedError{reason: describe() + " is cluster-scoped, and the manifests of a template create objects in the namespaces of the project only"}
	}
	switch namespace := object.GetNamespace(); {
	case namespace == "":
		object.SetNamespace(project.Name)
	case !slices.Contains(namespaces, namespace):
		return manifestsRefusedError{reason: fmt.Sprintf("%s is in the '%s' namespace, which is not a namespace of the project", describe(), namespace)}
	}
	labels := object.GetLabels()
	switch heritage := labels[v1alpha3.ResourceLabelHeritage]; heritage {
	case v1alpha3.ResourceHeritageMultitenancy, v1alpha3.ResourceHeritageDeckhouse:
		return manifestsRefusedError{reason: fmt.Sprintf(
			"the %s '%s/%s' has the %s: %s label, which marks the objects that only the platform writes",
			object.GetKind(), object.GetNamespace(), object.GetName(), v1alpha3.ResourceLabelHeritage, heritage)}
	}

	for _, field := range serverFields {
		unstructured.RemoveNestedField(object.Object, field...)
	}
	if labels == nil {
		labels = make(map[string]string, 2)
	}
	labels[v1alpha3.ResourceLabelProject] = project.Name
	labels[v1alpha3.ResourceLabelSource] = v1alpha3.ResourceSourceManifests
	object.SetLabels(labels)
	return nil
}

// ensureManifestsBinding makes the d8-manifests ProjectRoleBinding, which grants the user the
// d8:project:admin role in every namespace of the project, and lists what keeps it from working yet:
// the binding has to be Ready for its generation, which its controller says once it has fanned the
// binding out, and the role has to have its aggregated rules, as an aggregated ClusterRole is created
// empty. Both are read from the cache. An empty list means the user has its rights.
func (m *Manager) ensureManifestsBinding(ctx context.Context, project *v1alpha3.Project, user string) ([]string, error) {
	binding := &v1alpha3.ProjectRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: v1alpha3.ProjectManifestsBinding, Namespace: project.Name}}
	_, err := controllerutil.CreateOrUpdate(ctx, m.client, binding, func() error {
		if binding.Labels == nil {
			binding.Labels = make(map[string]string)
		}
		maps.Copy(binding.Labels, standardFieldLabels(project.Name))
		binding.Spec.Subjects = []rbacv1.Subject{{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: user}}
		binding.Spec.RoleRef = v1alpha3.RoleRef{Kind: "ClusterRole", Name: v1alpha3.ProjectAdministratorsRoleName}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("upsert the %s binding: %w", v1alpha3.ProjectManifestsBinding, err)
	}

	var waits []string
	ready := readyCondition(binding.Status.Conditions)
	if binding.Status.ObservedGeneration != binding.Generation || ready == nil || ready.Status != corev1.ConditionTrue {
		waits = append(waits, fmt.Sprintf("the %s ProjectRoleBinding that grants it the %s role is Ready for its current generation",
			v1alpha3.ProjectManifestsBinding, v1alpha3.ProjectAdministratorsRoleName))
	}
	role := new(rbacv1.ClusterRole)
	switch err := m.client.Get(ctx, client.ObjectKey{Name: v1alpha3.ProjectAdministratorsRoleName}, role); {
	case apierrors.IsNotFound(err):
		waits = append(waits, fmt.Sprintf("the %s ClusterRole exists", v1alpha3.ProjectAdministratorsRoleName))
	case err != nil:
		return nil, fmt.Errorf("get the %s ClusterRole: %w", v1alpha3.ProjectAdministratorsRoleName, err)
	case len(role.Rules) == 0:
		waits = append(waits, fmt.Sprintf("the %s ClusterRole has its aggregated rules", v1alpha3.ProjectAdministratorsRoleName))
	}
	return waits, nil
}

// deleteManifestsBinding deletes the d8-manifests ProjectRoleBinding of a project that has no
// manifests and nothing applied from them left: its user is to have no rights then. The binding is
// looked up in the cache first, so a project without one costs no request.
func (m *Manager) deleteManifestsBinding(ctx context.Context, project *v1alpha3.Project) error {
	binding := new(v1alpha3.ProjectRoleBinding)
	key := client.ObjectKey{Namespace: project.Name, Name: v1alpha3.ProjectManifestsBinding}
	switch err := m.client.Get(ctx, key, binding); {
	case apierrors.IsNotFound(err):
		return nil
	case err != nil:
		return fmt.Errorf("get the %s binding: %w", key.Name, err)
	}
	if err := m.client.Delete(ctx, binding); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete the %s binding: %w", key.Name, err)
	}
	return nil
}

// appliedRecently reports whether the objects of the same render were applied less than
// manifestsReapply ago, and how long ago. A render whose objects are not exactly the ones
// status.manifestObjects lists, or that holds an object left from a Helm text, is applied again.
func (m *Manager) appliedRecently(project *v1alpha3.Project, applied []v1alpha3.ManifestObject, digest string) (time.Duration, bool) {
	value, _ := m.manifestsApplied.Load(project.Name)
	last, ok := value.(appliedManifests)
	if !ok || last.digest != digest || !slices.Equal(project.Status.ManifestObjects, applied) {
		return 0, false
	}
	since := time.Since(last.at)
	if since >= manifestsReapply {
		return 0, false
	}
	for _, object := range applied {
		if slices.ContainsFunc(project.Status.KeptObjects, func(kept v1alpha3.KeptObject) bool {
			return keptObjectKey(kept, project.Name) == manifestObjectKey(object, project.Name)
		}) {
			return 0, false
		}
	}
	return since, true
}

// isKeptObject reports whether status.keptObjects of the project holds the object.
func (m *Manager) isKeptObject(project *v1alpha3.Project, object *unstructured.Unstructured) bool {
	key := keyOf(releaseObjectOf(object), project.Name)
	return slices.ContainsFunc(project.Status.KeptObjects, func(kept v1alpha3.KeptObject) bool {
		return keptObjectKey(kept, project.Name) == key
	})
}

// takeOverReleaseFields hands the fields that the Helm release and the controller set on an object a
// switch from a Helm text left in place over to the field manager of the manifests, before the object
// is applied. A server-side apply removes a field it no longer sets only when no other manager owns
// it, and the release wrote with Update operations, so without this every field that the Helm text
// set and the manifests leave out, a label or annotation of Helm among them, would stay on the object
// for good. After it the object is what the manifests render, as for any other object of theirs. The
// object is read and patched as the user of the project, at the resourceVersion read, so a change in
// between makes the patch fail and the apply is retried.
func takeOverReleaseFields(ctx context.Context, actor client.Client, object *unstructured.Unstructured) error {
	live := new(unstructured.Unstructured)
	live.SetGroupVersionKind(object.GroupVersionKind())
	switch err := actor.Get(ctx, client.ObjectKeyFromObject(object), live); {
	case apierrors.IsNotFound(err):
		return nil
	case err != nil:
		return err
	}
	patch, err := csaupgrade.UpgradeManagedFieldsPatch(live, sets.New(releaseFieldManager), manifestsFieldManager)
	if err != nil {
		return fmt.Errorf("hand the fields of %s over to %s: %w", releaseFieldManager, manifestsFieldManager, err)
	}
	if patch == nil {
		return nil
	}
	return actor.Patch(ctx, live, client.RawPatch(types.JSONPatchType, patch))
}

// adoptKeptObject finishes the takeover of an object that a switch from a Helm text left in place and
// that the apply has just taken over: it loses the kept-from-helm-template label and a handover mark,
// with the identity of the controller, as these are the marks of the controller, and leaves
// status.keptObjects. Usually the apply has removed both marks already, together with the other fields
// of the release (takeOverReleaseFields).
func (m *Manager) adoptKeptObject(ctx context.Context, project *v1alpha3.Project, object *unstructured.Unstructured) error {
	key := keyOf(releaseObjectOf(object), project.Name)
	isKept := func(kept v1alpha3.KeptObject) bool { return keptObjectKey(kept, project.Name) == key }
	if !m.isKeptObject(project, object) {
		return nil
	}

	live, err := m.liveObject(ctx, releaseObjectOf(object), object.GetNamespace())
	if err != nil {
		return fmt.Errorf("get the %s '%s/%s': %w", object.GetKind(), object.GetNamespace(), object.GetName(), err)
	}
	if live != nil {
		labels := maps.Clone(live.GetLabels())
		annotations := maps.Clone(live.GetAnnotations())
		delete(labels, v1alpha3.ResourceLabelKeptFromHelmTemplate)
		delete(annotations, keptByHandoverAnnotation)
		if !maps.Equal(labels, live.GetLabels()) || !maps.Equal(annotations, live.GetAnnotations()) {
			base := live.DeepCopy()
			live.SetLabels(labels)
			live.SetAnnotations(annotations)
			if err := m.client.Patch(ctx, live, client.MergeFrom(base)); err != nil {
				return fmt.Errorf("unmark the %s '%s/%s': %w", object.GetKind(), object.GetNamespace(), object.GetName(), err)
			}
		}
	}

	err = m.recordInStatus(ctx, project, func(status *v1alpha3.ProjectStatus) bool {
		before := len(status.KeptObjects)
		status.KeptObjects = slices.DeleteFunc(status.KeptObjects, isKept)
		return len(status.KeptObjects) != before
	})
	if err != nil {
		return fmt.Errorf("take the %s '%s/%s' off status.keptObjects: %w", object.GetKind(), object.GetNamespace(), object.GetName(), err)
	}
	m.publishSeries(project)
	return nil
}

// pruneManifestObjects deletes, as the user of the project, the objects of status.manifestObjects that
// the render does not hold. One whose API version is not served any more is read through the version
// the API server serves its kind at. One that is gone, whose kind is not served at all any more, that
// lost the labels of the objects of the manifests, or whose namespace is not a namespace of the
// project any more, leaves the list without a delete. It returns the objects that stay on the list,
// those the API server refused to read or delete, and what it answered for them.
func (m *Manager) pruneManifestObjects(
	ctx context.Context,
	project *v1alpha3.Project,
	actor client.Client,
	applied []v1alpha3.ManifestObject,
) ([]v1alpha3.ManifestObject, []string) {
	keep := make(map[releaseKey]struct{}, len(applied))
	for _, object := range applied {
		keep[manifestObjectKey(object, project.Name)] = struct{}{}
	}
	namespaces := rolebinding.ProjectNamespaceNames(project)

	var left []v1alpha3.ManifestObject
	var refusals []string
	for _, listed := range project.Status.ManifestObjects {
		if _, ok := keep[manifestObjectKey(listed, project.Name)]; ok || !slices.Contains(namespaces, listed.Namespace) {
			continue
		}
		live := new(unstructured.Unstructured)
		live.SetAPIVersion(listed.APIVersion)
		live.SetKind(listed.Kind)
		live.SetNamespace(listed.Namespace)
		live.SetName(listed.Name)
		err := actor.Get(ctx, client.ObjectKeyFromObject(live), live)
		if meta.IsNoMatchError(err) {
			// The version it was applied with may not be served any more while the kind is, under
			// another version: the object is still there and is read through that one.
			mapping, mappingErr := actor.RESTMapper().RESTMapping(live.GroupVersionKind().GroupKind())
			if mappingErr == nil {
				live.SetGroupVersionKind(mapping.GroupVersionKind)
				listed.APIVersion = mapping.GroupVersionKind.GroupVersion().String()
				err = actor.Get(ctx, client.ObjectKeyFromObject(live), live)
			}
		}
		switch {
		case apierrors.IsNotFound(err) || meta.IsNoMatchError(err):
			continue
		case err != nil:
			left = append(left, listed)
			refusals = append(refusals, refusalOf(live, err))
			continue
		}
		if !isManifestObjectOf(live, project.Name) {
			continue
		}
		uid := live.GetUID()
		if err := actor.Delete(ctx, live, client.Preconditions{UID: &uid}); err != nil && !apierrors.IsNotFound(err) {
			left = append(left, listed)
			refusals = append(refusals, refusalOf(live, err))
		}
	}
	return left, refusals
}

// refusalOf describes an object and what the API server answered for it, with the code of the
// answer, such as 403 Forbidden.
func refusalOf(object *unstructured.Unstructured, err error) string {
	what := fmt.Sprintf("%s %s/%s", object.GetKind(), object.GetNamespace(), object.GetName())
	var status apierrors.APIStatus
	if errors.As(err, &status) {
		answer := status.Status()
		return fmt.Sprintf("%s: %d %s: %s", what, answer.Code, answer.Reason, answer.Message)
	}
	return fmt.Sprintf("%s: %v", what, err)
}

// joinRefusals joins the refusals, sorted, naming at most manifestsRefusalsShown of them.
func joinRefusals(refusals []string) string {
	sorted := slices.Sorted(slices.Values(refusals))
	if len(sorted) <= manifestsRefusalsShown {
		return strings.Join(sorted, "; ")
	}
	return fmt.Sprintf("%s; and %d more", strings.Join(sorted[:manifestsRefusalsShown], "; "), len(sorted)-manifestsRefusalsShown)
}

// manifestsDigest is the digest of the objects of a render, in their order.
func manifestsDigest(objects []*unstructured.Unstructured) (string, error) {
	hash := sha256.New()
	for _, object := range objects {
		// encoding/json sorts the keys of a map, so the same object has the same digest
		data, err := json.Marshal(object.Object)
		if err != nil {
			return "", fmt.Errorf("marshal the %s '%s/%s': %w", object.GetKind(), object.GetNamespace(), object.GetName(), err)
		}
		hash.Write(data)
		hash.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// manifestObjectsOf is the status.manifestObjects entries of the objects, sorted.
func manifestObjectsOf(objects []*unstructured.Unstructured) []v1alpha3.ManifestObject {
	entries := make([]v1alpha3.ManifestObject, 0, len(objects))
	for _, object := range objects {
		entries = append(entries, v1alpha3.ManifestObject{
			APIVersion: object.GetAPIVersion(),
			Kind:       object.GetKind(),
			Namespace:  object.GetNamespace(),
			Name:       object.GetName(),
		})
	}
	return sortManifestObjects(entries)
}

// mergeManifestObjects is the union of the two lists, sorted. An object both name is taken from more,
// which holds the API version it was last applied with.
func mergeManifestObjects(list, more []v1alpha3.ManifestObject) []v1alpha3.ManifestObject {
	merged := slices.Clone(more)
	for _, entry := range list {
		if !slices.ContainsFunc(more, func(other v1alpha3.ManifestObject) bool {
			return manifestObjectKey(other, "") == manifestObjectKey(entry, "")
		}) {
			merged = append(merged, entry)
		}
	}
	return sortManifestObjects(merged)
}

func sortManifestObjects(entries []v1alpha3.ManifestObject) []v1alpha3.ManifestObject {
	slices.SortFunc(entries, func(a, b v1alpha3.ManifestObject) int {
		return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Kind, b.Kind),
			cmp.Compare(a.Name, b.Name), cmp.Compare(a.APIVersion, b.APIVersion))
	})
	return entries
}

// setManifestObjects sets status.manifestObjects and reports whether it changed.
func setManifestObjects(status *v1alpha3.ProjectStatus, entries []v1alpha3.ManifestObject) bool {
	if len(entries) == 0 {
		entries = nil
	}
	if slices.Equal(status.ManifestObjects, entries) {
		return false
	}
	status.ManifestObjects = entries
	return true
}

// manifestObjectKey identifies an entry of status.manifestObjects across API versions of its kind.
func manifestObjectKey(object v1alpha3.ManifestObject, project string) releaseKey {
	return keyOf(helm.ReleaseObject{APIVersion: object.APIVersion, Kind: object.Kind, Namespace: object.Namespace, Name: object.Name}, project)
}

// keptObjectKey identifies an entry of status.keptObjects the same way.
func keptObjectKey(object v1alpha3.KeptObject, project string) releaseKey {
	return keyOf(helm.ReleaseObject{APIVersion: object.APIVersion, Kind: object.Kind, Namespace: object.Namespace, Name: object.Name}, project)
}
