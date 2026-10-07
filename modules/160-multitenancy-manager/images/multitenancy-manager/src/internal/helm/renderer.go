/*
Copyright 2024 Flant JSC

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

package helm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"

	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/naming"
)

var (
	ErrNamespaceOverride = errors.New("objects that defined in different namespaces will still be deployed to project namespace")
)

// BindingRoleRef records the role referenced by a binding object rendered from a project template,
// together with the binding kind/name for diagnostics.
type BindingRoleRef struct {
	BindingKind string
	BindingName string
	RoleKind    string
	RoleName    string
}

type postRenderer struct {
	project        *v1alpha3.Project
	versions       map[string]struct{}
	logger         logr.Logger
	warning        error
	isFirstInstall bool
	// referencedRoles collects the roleRefs of every binding object the template renders.
	referencedRoles []BindingRoleRef
	// manifests, when non-empty, is the source the post-renderer processes instead of the chart's
	// rendered output. Schema-based (v1alpha2) templates are rendered natively in Go
	// (controller/internal/render) and applied through this override, so no user data ever passes
	// through Helm's template engine while the release lifecycle (install/upgrade/prune/history) is
	// still driven by Helm.
	manifests string
}

func newPostRenderer(project *v1alpha3.Project, versions map[string]struct{}, logger logr.Logger, isFirstInstall bool) *postRenderer {
	return &postRenderer{
		project:        project,
		versions:       versions,
		logger:         logger.WithName("post-renderer"),
		isFirstInstall: isFirstInstall,
	}
}

// Run post renderer which will remove all namespaces except the project one
// or will add a project namespace if it does not exist in manifests
func (r *postRenderer) Run(renderedManifests *bytes.Buffer) (*bytes.Buffer, error) {
	// clear resources
	r.project.Status.Resources = make(map[string]map[string]v1alpha3.ResourceKind)

	// A schema-based template renders its objects natively; use them as the source of truth instead
	// of the (empty) chart output.
	source := renderedManifests.String()
	if r.manifests != "" {
		source = r.manifests
	}

	var core *unstructured.Unstructured
	builder := strings.Builder{}
	for _, manifest := range releaseutil.SplitManifests(source) {
		object := new(unstructured.Unstructured)
		if err := yaml.Unmarshal([]byte(manifest), object); err != nil {
			r.logger.Info("failed to unmarshal manifest", "project", r.project.Name, "manifest", manifest, "error", err.Error())
			return renderedManifests, err
		}

		// skip empty manifests
		if object.GetAPIVersion() == "" || object.GetKind() == "" {
			continue
		}

		if err := r.processObject(object, &core, &builder); err != nil {
			return renderedManifests, err
		}
	}

	buf := bytes.NewBuffer(nil)

	// ensure core namespace
	if core == nil {
		buf.WriteString("\n---\n" + string(r.newNamespace(r.project.Name)))
	} else {
		data, err := yaml.Marshal(core.Object)
		if err != nil {
			return renderedManifests, fmt.Errorf("marshal core namespace: %w", err)
		}
		buf.WriteString("\n---\n" + string(data))
	}

	buf.WriteString(builder.String())

	return buf, nil
}

// processObject renders a single object into builder, applying the label injection and the
// namespace rules. List wrappers (kind: List) are expanded recursively because Helm flattens lists
// only after post-rendering: without this, a binding inside a List would escape the role check, and
// the items would keep the namespace they name and go without the project labels.
func (r *postRenderer) processObject(object *unstructured.Unstructured, core **unstructured.Unstructured, builder *strings.Builder) error {
	if object.IsList() {
		return object.EachListItem(func(item runtime.Object) error {
			nested, ok := item.(*unstructured.Unstructured)
			if !ok {
				return nil
			}
			return r.processObject(nested, core, builder)
		})
	}

	r.collectRoleRef(object)

	// skip resource that not present in the cluster
	if r.versions != nil {
		version := fmt.Sprintf("%s/%s", object.GetAPIVersion(), object.GetKind())
		if _, ok := r.versions[version]; !ok {
			r.project.AddResource(object, false)
			r.logger.Info("the resource skipped during render project", "project", r.project.Name, "resource", object.GetName(), "version", version)
			return nil
		}
	}

	labels := object.GetLabels()
	if len(labels) == 0 {
		labels = make(map[string]string)
	}

	// check if resource should be excluded from management
	isUnmanaged := false
	if _, ok := labels[v1alpha3.ResourceLabelUnmanaged]; ok {
		isUnmanaged = true
		// Include unmanaged resources only on first install to create them once
		// On subsequent upgrades, skip them so they won't be updated
		if !r.isFirstInstall {
			r.logger.Info("the resource is unmanaged and will be skipped (not first install)", "project", r.project.Name, "resource", object.GetName(), "kind", object.GetKind())
			return nil
		}
		// On first install, include unmanaged resources but mark them to not be tracked
		r.logger.Info("the resource is unmanaged but will be created on first install", "project", r.project.Name, "resource", object.GetName(), "kind", object.GetKind())
		// Add Helm resource-policy annotation to prevent deletion on release uninstall
		annotations := object.GetAnnotations()
		if len(annotations) == 0 {
			annotations = make(map[string]string)
		}
		annotations["helm.sh/resource-policy"] = "keep"
		object.SetAnnotations(annotations)
	}

	// inject multitenancy-manager labels
	// For unmanaged resources, only add project and template labels, not heritage
	// For other resources, skip heritage label if ResourceLabelSkipHeritage is set
	if !isUnmanaged {
		if _, skipHeritage := labels[v1alpha3.ResourceLabelSkipHeritage]; !skipHeritage {
			labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
		}
	}
	labels[v1alpha3.ResourceLabelProject] = r.project.Name
	labels[v1alpha3.ResourceLabelTemplate] = r.project.Spec.ProjectTemplateName

	object.SetLabels(labels)

	if object.GetKind() == "Namespace" {
		// skip other namespaces
		if object.GetName() == r.project.Name {
			// The native render writes the record of a structured template itself; the main
			// namespace of a Helm text gets it here.
			if r.manifests == "" {
				if err := recordTemplateLabels(object); err != nil {
					return err
				}
			}
			r.project.AddResource(object, true)
			*core = object
		}

		return nil
	}

	// Namespaced objects may target ANY namespace of the project (main + additional): schema-based
	// templates render NetworkPolicy/PodLoggingConfig once per project namespace. The render and this
	// post-renderer derive the project namespace set from the same project.Status.Namespaces, so every
	// rendered target is allowed here (no duplicates). An empty namespace defaults to main; a namespace
	// outside the project (e.g. authored by a legacy resourcesTemplate) is forced to main with a warning.
	switch ns := object.GetNamespace(); {
	case ns == "":
		object.SetNamespace(r.project.Name)
	case !r.isProjectNamespace(ns):
		r.warning = ErrNamespaceOverride
		object.SetNamespace(r.project.Name)
	}

	// Track resource in project status only if it's not unmanaged
	// Unmanaged resources are created once but not tracked/updated
	if _, isUnmanaged := labels[v1alpha3.ResourceLabelUnmanaged]; !isUnmanaged {
		r.project.AddResource(object, true)
	}

	data, err := yaml.Marshal(object.Object)
	if err != nil {
		return fmt.Errorf("marshal rendered object %s/%s: %w", object.GetKind(), object.GetName(), err)
	}
	builder.WriteString("\n---\n" + string(data))
	return nil
}

// collectRoleRef records the roleRef of a binding object (native RoleBinding/ClusterRoleBinding or
// a ProjectRoleBinding/ClusterProjectRoleBinding) so the controller can flag templates that grant
// disabled or otherwise forbidden roles.
func (r *postRenderer) collectRoleRef(object *unstructured.Unstructured) {
	var roleRef map[string]string
	switch object.GetKind() {
	case "RoleBinding", "ClusterRoleBinding":
		if !strings.HasPrefix(object.GetAPIVersion(), "rbac.authorization.k8s.io/") {
			return
		}
		roleRef, _, _ = unstructured.NestedStringMap(object.Object, "roleRef")
	case v1alpha3.ProjectRoleBindingKind, v1alpha3.ClusterProjectRoleBindingKind:
		if !strings.HasPrefix(object.GetAPIVersion(), "deckhouse.io/") {
			return
		}
		roleRef, _, _ = unstructured.NestedStringMap(object.Object, "spec", "roleRef")
	default:
		return
	}
	if roleRef == nil {
		return
	}
	r.referencedRoles = append(r.referencedRoles, BindingRoleRef{
		BindingKind: object.GetKind(),
		BindingName: object.GetName(),
		RoleKind:    roleRef["kind"],
		RoleName:    roleRef["name"],
	})
}

// recordTemplateLabels writes the template-labels record on the main namespace a Helm text rendered:
// the labels the text put on it, without the ones the module owns or GitOps tools track objects by,
// which is the set the native render records from namespaceMetadata.labels. The ProjectNamespace
// controller copies these labels to every additional namespace and creates none while the record is
// missing. A record the text wrote itself is replaced.
func recordTemplateLabels(namespace *unstructured.Unstructured) error {
	labels := map[string]string{}
	for key, value := range namespace.GetLabels() {
		if !naming.IsModuleOwnedLabel(key) && !naming.IsGitOpsTrackingLabel(key) {
			labels[key] = value
		}
	}
	// encoding/json sorts the keys, so the value is stable
	record, err := json.Marshal(labels)
	if err != nil {
		return fmt.Errorf("marshal template labels: %w", err)
	}

	annotations := namespace.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[naming.TemplateLabelsAnnotation] = string(record)
	namespace.SetAnnotations(annotations)
	return nil
}

// isProjectNamespace reports whether ns is the project's main namespace or one of its additional
// namespaces (from status.namespaces). Namespaced objects targeting such a namespace are kept as-is;
// anything else is forced to the main namespace.
func (r *postRenderer) isProjectNamespace(ns string) bool {
	if ns == r.project.Name {
		return true
	}
	for _, s := range r.project.Status.Namespaces {
		if s.Name == ns {
			return true
		}
	}
	return false
}

func (r *postRenderer) newNamespace(name string) []byte {
	obj := corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{},
			// nothing to inherit, and recorded all the same: see recordTemplateLabels
			Annotations: map[string]string{naming.TemplateLabelsAnnotation: "{}"},
		},
	}

	obj.Labels[v1alpha3.ResourceLabelHeritage] = v1alpha3.ResourceHeritageMultitenancy
	obj.Labels[v1alpha3.ResourceLabelProject] = r.project.Name
	obj.Labels[v1alpha3.ResourceLabelTemplate] = r.project.Spec.ProjectTemplateName

	data, _ := yaml.Marshal(obj) //nolint:errcheck // marshaling a static in-memory corev1.Namespace cannot fail
	return data
}
