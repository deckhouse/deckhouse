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
	"strings"

	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/strings/slices"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
)

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
	converter := runtime.DefaultUnstructuredConverter

	role := new(rbac.Role)
	err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), role)
	if err != nil {
		panic(err)
	}

	for _, rule := range role.Rules {
		if slices.Contains(rule.APIGroups, "*") {
			return newWildCardError(object, "apiGroups contained a wildcard rule")
		}
		if slices.Contains(rule.Resources, "*") {
			return newWildCardError(object, "resources contained a wildcard rule")
		}
		if slices.Contains(rule.Verbs, "*") {
			return newWildCardError(object, "verbs contained a wildcard rule")
		}
	}

	return errors.EmptyRuleError
}

func newWildCardError(object storage.StoreObject, message string) errors.LintRuleError {
	return errors.NewLintRuleError(
		"WILDCARD001",
		object.Identity(),
		object.Path,
		message,
	)
}
