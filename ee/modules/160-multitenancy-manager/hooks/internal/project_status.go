package internal

import (
	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

func SetErrorStatusProject(patcher *object_patch.PatchCollector, projectName, errMsg string, conditions []v1alpha1.Condition) {
	conditions = append(conditions, v1alpha1.Condition{
		Name:    "Error",
		Message: errMsg,
		Status:  false,
	})

	setProjectStatus(patcher, projectName, false, errMsg, conditions)
}

func SetDeployingStatusProject(patcher *object_patch.PatchCollector, projectName string, conditions []v1alpha1.Condition) {
	conditions = append(conditions, v1alpha1.Condition{
		Name:   "Deploying",
		Status: false,
	})
	setProjectStatus(patcher, projectName, false, "", conditions)
}

func SetSyncStatusProject(patcher *object_patch.PatchCollector, projectName string, conditions []v1alpha1.Condition) {
	conditions = append(conditions, v1alpha1.Condition{
		Name:   "Sync",
		Status: true,
	})
	setProjectStatus(patcher, projectName, true, "", conditions)
}

func setProjectStatus(patcher *object_patch.PatchCollector, projectName string, status bool, message string, conditions []v1alpha1.Condition) {
	uniqueConds := uniqueConditions(conditions)
	newConditions := make([]map[string]interface{}, 0, len(uniqueConds))
	for _, cond := range uniqueConds {
		newCond := map[string]interface{}{
			"name":    cond.Name,
			"status":  cond.Status,
			"message": stringOrNil(cond.Message),
		}

		newConditions = append(newConditions, newCond)
	}

	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"statusSummary": map[string]interface{}{
				"status":  status,
				"message": stringOrNil(message),
			},
			"conditions": newConditions,
		},
	}

	patchStatus(patcher, ProjectKind, projectName, statusPatch)
}

func uniqueConditions(conds []v1alpha1.Condition) []v1alpha1.Condition {
	uniqueConds := make(map[v1alpha1.Condition]bool)
	for _, c := range conds {
		if uniqueConds[c] {
			continue
		}
		uniqueConds[c] = true
	}

	result := make([]v1alpha1.Condition, 0, len(uniqueConds))
	for c := range uniqueConds {
		result = append(result, c)
	}
	return result
}
