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

	// HeritageLabel/HeritageValue mark Deckhouse-managed objects.
	HeritageLabel = "heritage"
	HeritageValue = "deckhouse"

	// ModuleLabel/ModuleValue identify the owning module (used by the protective admission policy).
	ModuleLabel = "module"
	ModuleValue = "multitenancy-manager"

	// GrantQuotaName is the fixed name of the per-namespace GrantQuota object.
	GrantQuotaName = "objects"
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
