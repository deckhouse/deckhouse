/*
Copyright 2021 Flant JSC

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
	"fmt"
	"os"
	"strings"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/utils"
)

const (
	serviceAccountNameDelimiter = "-"
	UserAuthzClusterRolePath    = "templates/user-authz-cluster-roles.yaml"
	RootRBACForUsPath           = "templates/rbac-for-us.yaml"
	RootRBACToUsPath            = "templates/rbac-to-us.yaml"
)

func isSystemNamespace(actual string) bool {
	return actual == "default" || actual == "kube-system"
}

func isDeckhouseSystemNamespace(actual string) bool {
	return actual == "d8-monitoring" ||
		actual == "d8-system" ||
		// Temporary code required to ignore existing objects with incorrect naming
		// TODO: remove next lines after RBAC objects naming fixes
		actual == "d8-admission-policy-engine" ||
		actual == "d8-operator-trivy" ||
		actual == "d8-log-shipper"
}

func ObjectRBACPlacement(m utils.Module, object storage.StoreObject) errors.LintRuleError {
	if m.Name == "user-authz" || m.Name == "deckhouse" {
		return errors.EmptyRuleError
	}

	if object.ShortPath() == UserAuthzClusterRolePath {
		return errors.EmptyRuleError
	}

	objectKind := object.Unstructured.GetKind()
	switch object.Unstructured.GetKind() {
	case "ServiceAccount":
		return objectRBACPlacementServiceAccount(m, object)
	case "ClusterRole", "ClusterRoleBinding":
		return objectRBACPlacementClusterRole(objectKind, m, object)
	case "Role", "RoleBinding":
		return objectRBACPlacementRole(objectKind, m, object)
	default:
		shortPath := object.ShortPath()
		if strings.HasSuffix(shortPath, "rbac-for-us.yaml") || strings.HasSuffix(shortPath, "rbac-to-us.yaml") {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				nil,
				"kind %s not allowed in %q", objectKind, shortPath,
			)
		}
		return errors.EmptyRuleError
	}
}

func objectRBACPlacementServiceAccount(m utils.Module, object storage.StoreObject) errors.LintRuleError {
	objectName := object.Unstructured.GetName()
	shortPath := object.ShortPath()
	namespace := object.Unstructured.GetNamespace()

	if shortPath == RootRBACForUsPath {
		if isSystemNamespace(namespace) {
			if objectName != "d8-"+m.Name {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					nil,
					"Name of ServiceAccount in %q in namespace %q should be equal to d8- + Chart Name (d8-%s)",
					RootRBACForUsPath, namespace, m.Name,
				)
			}
			return errors.EmptyRuleError
		}
		if objectName != m.Name {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				nil,
				"Name of ServiceAccount in %q should be equal to Chart Name (%s)",
				RootRBACForUsPath, m.Name,
			)
		}
		if !isDeckhouseSystemNamespace(namespace) && m.Namespace != namespace {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				namespace,
				"ServiceAccount should be deployed to \"d8-system\", \"d8-monitoring\" or %q", m.Namespace,
			)
		}
		return errors.EmptyRuleError
	} else if strings.HasSuffix(shortPath, "rbac-for-us.yaml") {
		parts := strings.Split(
			strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-for-us.yaml"), "templates/"),
			string(os.PathSeparator),
		)

		serviceAccountName := strings.Join(parts, serviceAccountNameDelimiter)
		expectedServiceAccountName := m.Name + serviceAccountNameDelimiter + serviceAccountName

		if isSystemNamespace(namespace) {
			if objectName != "d8-"+expectedServiceAccountName {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					nil,
					"Name of ServiceAccount in %q in namespace %q should be equal to d8-%s",
					shortPath, namespace, expectedServiceAccountName,
				)
			}
			return errors.EmptyRuleError
		}
		if objectName == serviceAccountName {
			if m.Namespace != namespace {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					namespace,
					"ServiceAccount should be deployed to %q", m.Namespace,
				)
			}
			return errors.EmptyRuleError
		} else if objectName == expectedServiceAccountName {
			if !isDeckhouseSystemNamespace(namespace) {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					namespace,
					"ServiceAccount should be deployed to \"d8-system\" or \"d8-monitoring\"",
				)
			}
			return errors.EmptyRuleError
		}
		return errors.NewLintRuleError(
			"MANIFEST053",
			object.Identity(),
			objectName,
			"Name of ServiceAccount should be equal to %q or %q",
			serviceAccountName, expectedServiceAccountName,
		)
	}
	return errors.NewLintRuleError(
		"MANIFEST053",
		object.Identity(),
		shortPath,
		"ServiceAccount should be in %q or \"*/rbac-for-us.yaml\"", RootRBACForUsPath,
	)
}

func objectRBACPlacementClusterRole(kind string, m utils.Module, object storage.StoreObject) errors.LintRuleError {
	objectName := object.Unstructured.GetName()
	shortPath := object.ShortPath()

	name := "d8:" + m.Name
	switch {
	case shortPath == RootRBACForUsPath:
		if !strings.HasPrefix(objectName, name) {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				objectName,
				"Name of %s in %q should start with %q",
				kind, RootRBACForUsPath, name,
			)
		}
	case strings.HasSuffix(shortPath, "rbac-for-us.yaml"):
		parts := strings.Split(
			strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-for-us.yaml"), "templates/"),
			string(os.PathSeparator),
		)
		name := name + ":" + strings.Join(parts, ":")
		if !strings.HasPrefix(objectName, name) {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				objectName,
				"Name of %s should start with %q",
				kind, name,
			)
		}
	default:
		return errors.NewLintRuleError(
			"MANIFEST053",
			object.Identity(),
			shortPath,
			"%s should be in %q or \"*/rbac-for-us.yaml\"",
			kind, RootRBACForUsPath,
		)
	}

	return errors.EmptyRuleError
}

func objectRBACPlacementRole(kind string, m utils.Module, object storage.StoreObject) errors.LintRuleError {
	objectName := object.Unstructured.GetName()
	shortPath := object.ShortPath()
	namespace := object.Unstructured.GetNamespace()

	switch {
	case shortPath == RootRBACForUsPath:
		return handleRootRBACForUs(m, object, objectName, kind)
	case shortPath == RootRBACToUsPath:
		return handleRootRBACToUs(m, object, objectName, kind)
	case strings.HasSuffix(shortPath, "rbac-for-us.yaml"):
		return handleNestedRBACForUs(m, object, shortPath, objectName, namespace, kind)
	case strings.HasSuffix(shortPath, "rbac-to-us.yaml"):
		return handleNestedRBACToUs(m, object, shortPath, objectName, kind)
	default:
		msgTemplate := `%s should be in "templates/rbac-for-us.yaml", "templates/rbac-to-us.yaml", ".*/rbac-to-us.yaml" or ".*/rbac-for-us.yaml"`
		return errors.NewLintRuleError(
			"MANIFEST053",
			object.Identity(),
			shortPath,
			msgTemplate,
			kind,
		)
	}
}

// handleRootRBACForUs applies to templates/rbac-for-us.yaml file's objects
func handleRootRBACForUs(m utils.Module, object storage.StoreObject, objectName, kind string) errors.LintRuleError {
	prefix := "d8:" + m.Name
	namespace := object.Unstructured.GetNamespace()

	switch {
	case objectName == m.Name && namespace != m.Namespace:
		if !isDeckhouseSystemNamespace(namespace) {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				namespace,
				"%s in %q should be deployed in namespace \"d8-monitoring\", \"d8-system\" or %q",
				kind, RootRBACForUsPath, m.Namespace,
			)
		}
	case strings.HasPrefix(objectName, prefix):
		if !isSystemNamespace(namespace) {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				namespace,
				"%s in %q should be deployed in namespace \"default\" or \"kube-system\"",
				kind, RootRBACForUsPath,
			)
		}
	case !strings.HasPrefix(objectName, prefix):
		if !isDeckhouseSystemNamespace(namespace) {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				namespace,
				"%s in %q should be deployed in namespace %q",
				kind, RootRBACForUsPath, m.Namespace,
			)
		}
	}

	return errors.EmptyRuleError
}

// handleRootRBACToUs applies to templates/rbac-to-us.yaml file's objects
func handleRootRBACToUs(m utils.Module, object storage.StoreObject, objectName, kind string) errors.LintRuleError {
	prefix := "access-to-" + m.Name
	if !strings.HasPrefix(objectName, prefix) {
		return errors.NewLintRuleError(
			"MANIFEST053",
			object.Identity(),
			objectName,
			"%s in %q should start with %q",
			kind, RootRBACToUsPath, prefix,
		)
	}

	namespace := object.Unstructured.GetNamespace()
	if !isDeckhouseSystemNamespace(namespace) && namespace != m.Namespace {
		return errors.NewLintRuleError(
			"MANIFEST053",
			object.Identity(),
			namespace,
			"%s in %q should be deployed in namespace \"d8-system\", \"d8-monitoring\" or %q",
			kind, RootRBACToUsPath, m.Namespace,
		)
	}

	return errors.EmptyRuleError
}

// handleNestedRBACForUs applies to templates/**/rbac-for-us.yaml file's objects
func handleNestedRBACForUs(m utils.Module, object storage.StoreObject, shortPath, objectName, namespace, kind string) errors.LintRuleError {
	parts := strings.Split(
		strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-for-us.yaml"), "templates/"),
		string(os.PathSeparator),
	)
	localPrefix := strings.Join(parts, ":")
	globalPrefix := fmt.Sprintf("%s:%s", m.Name, strings.Join(parts, ":"))
	systemPrefix := fmt.Sprintf("d8:%s", globalPrefix)

	switch {
	case strings.HasPrefix(objectName, localPrefix):
		if namespace != m.Namespace {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				namespace,
				"%s with prefix %q should be deployed in namespace %q",
				kind, localPrefix, m.Namespace,
			)
		}
	case strings.HasPrefix(objectName, globalPrefix):
		if !isDeckhouseSystemNamespace(namespace) {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				namespace,
				"%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"",
				kind, globalPrefix,
			)
		}
	case strings.HasPrefix(objectName, systemPrefix):
		if !isSystemNamespace(namespace) {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				namespace,
				"%s with prefix %q should be deployed in namespace \"default\" or \"kube-system\"",
				kind, systemPrefix,
			)
		}
	default:
		return errors.NewLintRuleError(
			"MANIFEST053",
			object.Identity(),
			objectName,
			"%s in %q should start with %q or %q",
			kind, shortPath, localPrefix, globalPrefix,
		)
	}

	return errors.EmptyRuleError
}

// handleNestedRBACToUs applies to templates/**/rbac-to-us.yaml file's objects
func handleNestedRBACToUs(m utils.Module, object storage.StoreObject, shortPath, objectName, kind string) errors.LintRuleError {
	parts := strings.Split(
		strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-to-us.yaml"), "templates/"),
		string(os.PathSeparator),
	)

	localPrefix := fmt.Sprintf("access-to-%s-", strings.Join(parts, "-"))
	globalPrefix := fmt.Sprintf("access-to-%s-%s-", m.Name, strings.Join(parts, "-"))
	namespace := object.Unstructured.GetNamespace()

	switch {
	case strings.HasPrefix(objectName, localPrefix):
		if namespace != m.Namespace {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				namespace,
				"%s with prefix %q should be deployed in namespace %q",
				kind, globalPrefix, m.Namespace,
			)
		}
	case strings.HasPrefix(objectName, globalPrefix):
		if !isDeckhouseSystemNamespace(namespace) {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				namespace,
				"%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"",
				kind, globalPrefix,
			)
		}
	default:
		return errors.NewLintRuleError(
			"MANIFEST053",
			object.Identity(),
			objectName,
			"%s should start with %q or %q", kind, localPrefix, globalPrefix,
		)
	}

	return errors.EmptyRuleError
}
