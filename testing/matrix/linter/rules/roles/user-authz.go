package roles

import (
	"fmt"

	"github.com/iancoleman/strcase"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

/*
ObjectUserAuthzClusterRolePath validates that files for user-authz contains only cluster roles.
Also, it validates that role names equals to d8:user-authz:<ChartName>:<AccessLevel>
*/
func ObjectUserAuthzClusterRolePath(m types.Module, object storage.StoreObject) errors.LintRuleError {
	objectKind := object.Unstructured.GetKind()

	shortPath := object.ShortPath()
	if shortPath == UserAuthzClusterRolePath {
		if objectKind != "ClusterRole" {
			return errors.NewLintRuleError(
				"MANIFEST051",
				object.Identity(),
				nil,
				"Only ClusterRoles can be specified in \"templates/user-authz-cluster-roles.yaml\"",
			)
		}

		objectName := object.Unstructured.GetName()
		accessLevel, ok := object.Unstructured.GetAnnotations()["user-authz.deckhouse.io/access-level"]
		if !ok {
			return errors.NewLintRuleError(
				"MANIFEST051",
				object.Identity(),
				nil,
				"User-authz access ClusterRoles should have annotation \"user-authz.deckhouse.io/access-level\"",
			)
		}

		expectedName := fmt.Sprintf("d8:user-authz:%s:%s", m.Name, strcase.ToKebab(accessLevel))
		if objectName != expectedName {
			return errors.NewLintRuleError(
				"MANIFEST051",
				object.Identity(),
				nil,
				"Name of user-authz ClusterRoles should be %q", expectedName,
			)
		}
	}
	return errors.EmptyRuleError
}
