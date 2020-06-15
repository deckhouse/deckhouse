package roles

import (
	"fmt"
	"os"
	"strings"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
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
	return actual == "d8-monitoring" || actual == "d8-system"
}

func ObjectRBACPlacement(m types.Module, object storage.StoreObject) errors.LintRuleError {
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
	}
	return errors.EmptyRuleError
}

func objectRBACPlacementServiceAccount(m types.Module, object storage.StoreObject) errors.LintRuleError {
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

func objectRBACPlacementClusterRole(kind string, m types.Module, object storage.StoreObject) errors.LintRuleError {
	objectName := object.Unstructured.GetName()
	shortPath := object.ShortPath()

	name := "d8:" + m.Name
	if shortPath == RootRBACForUsPath {
		if !strings.HasPrefix(objectName, name) {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				objectName,
				"Name of %s in %q should start with %q",
				kind, RootRBACForUsPath, name,
			)
		}
	} else if strings.HasSuffix(shortPath, "rbac-for-us.yaml") {
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
	} else {
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

func objectRBACPlacementRole(kind string, m types.Module, object storage.StoreObject) errors.LintRuleError {
	objectName := object.Unstructured.GetName()
	shortPath := object.ShortPath()
	namespace := object.Unstructured.GetNamespace()

	if shortPath == RootRBACForUsPath {
		prefix := "d8:" + m.Name
		namespace := object.Unstructured.GetNamespace()

		if objectName == m.Name {
			if namespace != m.Namespace && !isDeckhouseSystemNamespace(namespace) {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					namespace,
					"%s in %q should be deployed in namespace \"d8-monitoring\", \"d8-system\" or %q",
					kind, RootRBACForUsPath, m.Namespace,
				)
			}
		} else if strings.HasPrefix(objectName, prefix) {
			if !isSystemNamespace(namespace) {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					namespace,
					"%s in %q should be deployed in namespace \"default\" or \"kube-system\"",
					kind, RootRBACForUsPath,
				)
			}
		} else {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				objectName,
				"%s in %q should equal %q or start with %q", kind, RootRBACForUsPath, m.Name, prefix,
			)
		}
		return errors.EmptyRuleError
	} else if shortPath == RootRBACToUsPath {
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
	} else if strings.HasSuffix(shortPath, "rbac-for-us.yaml") {
		parts := strings.Split(
			strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-for-us.yaml"), "templates/"),
			string(os.PathSeparator),
		)
		localPrefix := strings.Join(parts, ":")
		globalPrefix := fmt.Sprintf("%s:%s", m.Name, strings.Join(parts, ":"))

		if strings.HasPrefix(objectName, localPrefix) {
			if namespace != m.Namespace {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					namespace,
					"%s with prefix %q should be deployed in namespace %q",
					kind, globalPrefix, m.Namespace,
				)
			}
		} else if strings.HasPrefix(objectName, globalPrefix) {
			if !isDeckhouseSystemNamespace(namespace) {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					namespace,
					"%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"",
					kind, globalPrefix,
				)
			}
		} else {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				objectName,
				"%s in %q should start with %q or %q",
				kind, shortPath, localPrefix, globalPrefix,
			)
		}
	} else if strings.HasSuffix(shortPath, "rbac-to-us.yaml") {
		parts := strings.Split(
			strings.TrimPrefix(strings.TrimSuffix(shortPath, "/rbac-to-us.yaml"), "templates/"),
			string(os.PathSeparator),
		)

		localPrefix := fmt.Sprintf("access-to-%s-", strings.Join(parts, "-"))
		globalPrefix := fmt.Sprintf("access-to-%s-%s-", m.Name, strings.Join(parts, "-"))
		namespace := object.Unstructured.GetNamespace()

		if strings.HasPrefix(objectName, localPrefix) {
			if namespace != m.Namespace {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					namespace,
					"%s with prefix %q should be deployed in namespace %q",
					kind, globalPrefix, m.Namespace,
				)
			}
		} else if strings.HasPrefix(objectName, globalPrefix) {
			if !isDeckhouseSystemNamespace(namespace) {
				return errors.NewLintRuleError(
					"MANIFEST053",
					object.Identity(),
					namespace,
					"%s with prefix %q should be deployed in namespace \"d8-system\" or \"d8-monitoring\"",
					kind, globalPrefix,
				)
			}
		} else {
			return errors.NewLintRuleError(
				"MANIFEST053",
				object.Identity(),
				objectName,
				"%s should start with %q or %q", kind, localPrefix, globalPrefix,
			)
		}
	} else {
		return errors.NewLintRuleError(
			"MANIFEST053",
			object.Identity(),
			shortPath,
			"%s should be in \"templates/rbac-for-us.yaml\", \"templates/rbac-to-us.yaml\", \".*/rbac-to-us.yaml\" "+
				"or \".*/rbac-for-us.yaml\"", kind,
		)
	}
	return errors.EmptyRuleError
}
