package roles

import (
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/deckhouse/deckhouse/testing/matrix/linter/rules/errors"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/storage"
	"github.com/deckhouse/deckhouse/testing/matrix/linter/types"
)

func ObjectBindingSubjectServiceAccountCheck(m types.Module, object storage.StoreObject, objectStore *storage.UnstructuredObjectStore) errors.LintRuleError {
	if m.Name == "user-authz" {
		return errors.EmptyRuleError
	}
	converter := runtime.DefaultUnstructuredConverter

	var subjects []v1.Subject

	// deckhouse module should contain only global cluster roles
	objectKind := object.Unstructured.GetKind()
	switch objectKind {
	case "ClusterRoleBinding":
		clusterRoleBinding := new(v1.ClusterRoleBinding)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), clusterRoleBinding)
		if err != nil {
			panic(err)
		}
		subjects = clusterRoleBinding.Subjects
	case "RoleBinding":
		roleBinding := new(v1.RoleBinding)
		err := converter.FromUnstructured(object.Unstructured.UnstructuredContent(), roleBinding)
		if err != nil {
			panic(err)
		}
		subjects = roleBinding.Subjects

	default:
		return errors.EmptyRuleError
	}

	for _, subject := range subjects {
		if subject.Kind != "ServiceAccount" {
			continue
		}
		if subject.Namespace == m.Namespace && !objectStore.Exists(storage.ResourceIndex{
			Name: subject.Name, Kind: subject.Kind, Namespace: subject.Namespace,
		}) {
			return errors.NewLintRuleError(
				"MANIFEST054",
				object.Identity(),
				subject.Name,
				"%s bind to the wrong ServiceAccount (doesn't exist in the store)", objectKind,
			)
		}
	}

	return errors.EmptyRuleError
}
