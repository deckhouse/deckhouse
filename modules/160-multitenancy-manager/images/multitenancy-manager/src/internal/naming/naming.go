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

// ManagedNamespaceAnnotations are the annotations on a project namespace that the module owns; see
// ManagedNamespaceLabels.
var ManagedNamespaceAnnotations = []string{
	"meta.helm.sh/release-name",
	"meta.helm.sh/release-namespace",
	"scheduler.alpha.kubernetes.io/node-selector",
	"scheduler.alpha.kubernetes.io/defaultTolerations",
}
