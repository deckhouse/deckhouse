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

package roles

import (
	"slices"
	"strings"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
)

// skipCheckWildcards is exclusion rules for wildcard verification
//
// the key is the file name
// value is an array of rule names that allow wildcards
var skipCheckWildcards = map[string][]string{
	"admission-policy-engine/templates/rbac-for-us.yaml": {
		"d8:admission-policy-engine:gatekeeper",
	},
}

// ObjectRolesWildcard is a linter for checking the presence
// of a wildcard in a Role and ClusterRole
func ObjectRolesWildcard(object storage.StoreObject) errors.LintRuleError {
	// check only `rbac-for-us.yaml` files
	if !strings.HasSuffix(object.ShortPath(), "rbac-for-us.yaml") {
		return errors.EmptyRuleError
	}

	// check Role and ClusterRole for wildcards
	objectKind := object.Unstructured.GetKind()
	switch objectKind {
	case "Role", "ClusterRole":
		return checkRoles(object)
	default:
		return errors.EmptyRuleError
	}
}

func checkRoles(object storage.StoreObject) errors.LintRuleError {
	// check rules for skip
	for path, rules := range skipCheckWildcards {
		if strings.EqualFold(object.Path, path) {
			if slices.Contains(rules, object.Unstructured.GetName()) {
				return errors.EmptyRuleError
			}
		}
	}

	converter := runtime.DefaultUnstructuredConverter

	role := new(rbac.Role)
	err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), role)
	if err != nil {
		panic(err)
	}

	for _, rule := range role.Rules {
		var objs []string
		if slices.Contains(rule.APIGroups, "*") {
			objs = append(objs, "apiGroups")
		}
		if slices.Contains(rule.Resources, "*") {
			objs = append(objs, "resources")
		}
		if slices.Contains(rule.Verbs, "*") {
			objs = append(objs, "verbs")
		}
		if len(objs) > 0 {
			return errors.NewLintRuleError(
				"WILDCARD001",
				object.Identity(),
				object.Path,
				strings.Join(objs, ", ")+" contains a wildcards",
			)
		}
	}

	return errors.EmptyRuleError
}
