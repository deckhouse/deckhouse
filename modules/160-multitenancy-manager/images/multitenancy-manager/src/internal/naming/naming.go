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

// Package naming holds the service label/annotation conventions and well-known object names
// shared across the controller and webhooks.
package naming

import (
	"fmt"
	"slices"
	"strings"
)

const (
	// ProjectLabel marks a namespace (and every controller-managed object) with its project.
	ProjectLabel = "projects.deckhouse.io/project"
	// NamespaceRoleLabel distinguishes the control namespace from workload namespaces.
	NamespaceRoleLabel = "projects.deckhouse.io/namespace-role"

	// HeritageLabel/HeritageValue mark the objects this module owns. The value is the module's own,
	// not "deckhouse": the protective admission policy of the module binds on it, so the catalog
	// objects are refused to everyone but the controller the way every other module-owned object
	// in a project namespace is, and no longer depend on the /protect webhook alone.
	HeritageLabel = "heritage"
	HeritageValue = "multitenancy-manager"

	// ModuleLabel/ModuleValue identify the owning module (used by the protective admission policy).
	ModuleLabel = "module"
	ModuleValue = "multitenancy-manager"
)

// ManagedLabels returns the ownership labels every controller-managed object carries.
func ManagedLabels(project string) map[string]string {
	l := map[string]string{
		HeritageLabel: HeritageValue,
		ModuleLabel:   ModuleValue,
	}
	if project != "" {
		l[ProjectLabel] = project
	}
	return l
}

// ManagedNamespaceLabels are the labels on a project namespace that the module owns: the controller
// or its Helm release sets them from the Project and its template, so a direct edit would be
// undone or would lie about the project. Everything else on the namespace belongs to whoever runs
// the cluster, and the protective admission policy lets them change it.
//
// The same list is spelled out in templates/validation.yaml (a CEL list literal, so the policy stays
// within the CEL cost budget without iterating the label map); TestManagedNamespaceKeysMatchThePolicy
// keeps the two in step.
var ManagedNamespaceLabels = []string{
	HeritageLabel,
	"app.kubernetes.io/managed-by",
	"kubernetes.io/metadata.name",
	ProjectLabel,
	"projects.deckhouse.io/project-template",
	"projects.deckhouse.io/project-namespace",
	NamespaceRoleLabel,
	"multitenancy.deckhouse.io/project-managed-by-namespace",
	"security.deckhouse.io/pod-policy",
	"extended-monitoring.deckhouse.io/enabled",
	"security-scanning.deckhouse.io/enabled",
}

// NodeSelectorAnnotation and TolerationsAnnotation carry the default pod placement of a namespace.
// The renderer writes them from the template's nodeSelector/tolerations, and adoption mirrors the
// ones a namespace already had, so they are named rather than spelled out in each place.
const (
	NodeSelectorAnnotation = "scheduler.alpha.kubernetes.io/node-selector"
	TolerationsAnnotation  = "scheduler.alpha.kubernetes.io/defaultTolerations"
)

// TemplateLabelsAnnotation lists, as a JSON object, the labels a project template puts on the
// namespaces of a project: through namespaceMetadata.labels, or, for a Helm resourcesTemplate, on
// the main namespace it renders. The renderer or the Helm post-renderer writes it on the main
// namespace, always, empty when there are none; the ProjectNamespace controller copies those labels
// to every additional namespace and writes the same annotation there as the record of what it set.
const TemplateLabelsAnnotation = "projects.deckhouse.io/template-labels"

// ManagedNamespaceAnnotations are the annotations on a project namespace that the module owns; see
// ManagedNamespaceLabels.
var ManagedNamespaceAnnotations = []string{
	"meta.helm.sh/release-name",
	"meta.helm.sh/release-namespace",
	NodeSelectorAnnotation,
	TolerationsAnnotation,
	TemplateLabelsAnnotation,
}

// IsModuleOwnedLabel reports whether the module owns the label key on a project namespace, so a
// template cannot set it through namespaceMetadata.labels.
func IsModuleOwnedLabel(key string) bool {
	return slices.Contains(ManagedNamespaceLabels, key) ||
		strings.HasPrefix(key, "projects.deckhouse.io/") ||
		strings.HasPrefix(key, "multitenancy.deckhouse.io/")
}

// labelFields are the ProjectTemplate fields that set the module-owned labels a user may choose.
var labelFields = map[string]string{
	"security.deckhouse.io/pod-policy":         "spec.podSecurityStandard",
	"extended-monitoring.deckhouse.io/enabled": "spec.features.monitoring",
	"security-scanning.deckhouse.io/enabled":   "spec.features.vulnerabilityScanning",
}

// ModuleOwnedLabelsIn describes, sorted, the keys of labels that the module owns, each with where it
// is set instead: a field of the template, or the controller. It returns nil when there are none.
// namespaceMetadata.labels cannot set such a key; the renderer would drop it without a word.
func ModuleOwnedLabelsIn(labels map[string]string) []string {
	var out []string
	for key := range labels {
		if !IsModuleOwnedLabel(key) {
			continue
		}
		where := "the controller sets it"
		if field, ok := labelFields[key]; ok {
			where = "set it through " + field
		}
		out = append(out, fmt.Sprintf("%s (%s)", key, where))
	}
	slices.Sort(out)
	return out
}

// SplitModuleOwnedLabels sorts the module-owned labels of labels into those previous does not carry
// with the same value (set) and those it does (kept). An update may keep a module-owned label the
// object already had, so that the object stays editable; only one it sets anew is refused.
func SplitModuleOwnedLabels(labels, previous map[string]string) (map[string]string, map[string]string) {
	set := map[string]string{}
	kept := map[string]string{}
	for key, value := range labels {
		if !IsModuleOwnedLabel(key) {
			continue
		}
		if old, ok := previous[key]; ok && old == value {
			kept[key] = value
			continue
		}
		set[key] = value
	}
	return set, kept
}

// IsGitOpsTrackingLabel reports whether the label key is one a GitOps tool uses to recognise the
// objects it manages. Such a label may reach the main namespace (a GitOps tool puts it there, or the
// template sets it), but it is not inherited by the additional namespaces: the tool would take them for
// its own and could prune them. The list covers the default keys of Argo CD, Flux and kapp; a custom
// tracking key (Argo CD application.instanceLabelKey) is not known here and is inherited.
func IsGitOpsTrackingLabel(key string) bool {
	return key == "app.kubernetes.io/instance" ||
		strings.HasPrefix(key, "argocd.argoproj.io/") ||
		strings.HasPrefix(key, "kustomize.toolkit.fluxcd.io/") ||
		strings.HasPrefix(key, "helm.toolkit.fluxcd.io/") ||
		strings.HasPrefix(key, "kapp.k14s.io/")
}
