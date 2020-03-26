package roles

import (
	"strings"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

func ObjectDeckhouseClusterRoles(m types.Module, object storage.StoreObject) errors.LintRuleError {
	if m.Name != "deckhouse" {
		return errors.EmptyRuleError
	}

	// deckhouse module should contain only global cluster roles
	objectKind := object.Unstructured.GetKind()
	if strings.HasPrefix(object.Path, "deckhouse/templates/common/rbac/") {
		if objectKind != "ClusterRole" {
			return errors.NewLintRuleError(
				"MANIFEST052",
				object.Identity(),
				nil,
				"Only ClusterRoles can be specified in \"deckhouse/templates/common/rbac/\"",
			)
		}
		objectName := object.Unstructured.GetName()
		if !strings.HasPrefix(objectName, "d8:") {
			return errors.NewLintRuleError(
				"MANIFEST052",
				object.Identity(),
				objectName,
				"Name of ClusterRoles in \"deckhouse/templates/common/rbac/\" should start with \"d8:\"",
			)
		}
	} else if objectKind == "ClusterRole" || objectKind == "ClusterRoleBinding" {
		return errors.NewLintRuleError(
			"MANIFEST052",
			object.Identity(),
			nil,
			"ClusterRoles and ClusterRoleBindings could be specified only in \"deckhouse/templates/common/rbac/\"",
		)
	}
	return errors.EmptyRuleError
}
