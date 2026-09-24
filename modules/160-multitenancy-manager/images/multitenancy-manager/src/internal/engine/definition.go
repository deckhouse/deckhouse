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

package engine

import (
	"fmt"

	"controller/api/v1alpha1"
	"controller/internal/jsonpath"
)

// This file holds the validity rules of the catalogFields of a GrantableClusterResourceDefinition. They
// are shared by the GrantableClusterResourceDefinition validating webhook, which applies them with
// ratcheting on UPDATE, and by the definition reconciler, which applies all of them to the stored
// object and reports the result as the CatalogFieldsValid condition. Both print the same problem
// strings. The names, their uniqueness and the number of entries are bounded by the CRD schema.

// DefinitionProblems returns every catalogFields problem of a definition, empty when it is valid: the
// value-backed problem first, then the path problems of each entry.
func DefinitionProblems(factory jsonpath.Factory, def *v1alpha1.GrantableClusterResourceDefinition) []string {
	var problems []string
	if p := ValueBackedCatalogFieldsProblem(def); p != "" {
		problems = append(problems, p)
	}
	for i, f := range def.Spec.CatalogFields {
		if p := CatalogFieldProblem(factory, i, f); p != "" {
			problems = append(problems, p)
		}
	}
	return problems
}

// ValueBackedCatalogFieldsProblem reports catalogFields on a value-backed definition: it has no
// objects to read the fields from, so the declaration would never show anything.
func ValueBackedCatalogFieldsProblem(def *v1alpha1.GrantableClusterResourceDefinition) string {
	if !def.IsValueBacked() || len(def.Spec.CatalogFields) == 0 {
		return ""
	}
	return "'spec.catalogFields' is set, but the definition is value-backed (no 'spec.grantedResource.kind'): " +
		"there are no objects to read the fields from. Remove 'spec.catalogFields' or set 'spec.grantedResource'"
}

// CatalogFieldProblem reports the path of one spec.catalogFields entry that the catalog projection
// refuses (see CompileCatalogPath), empty when the entry is usable.
func CatalogFieldProblem(factory jsonpath.Factory, i int, f v1alpha1.CatalogField) string {
	if _, err := CompileCatalogPath(factory, f.Path); err != nil {
		return fmt.Sprintf("'spec.catalogFields[%d].path' %q: %v", i, f.Path, err)
	}
	return ""
}
