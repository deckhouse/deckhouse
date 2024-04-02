/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"github.com/flant/shell-operator/pkg/kube/object_patch"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha2"
)

// Project
func ProjectStatusIsDeploying(status v1alpha2.ProjectStatus) bool {
	return status.State == "Deploying"
}

func SetProjectStatusError(patcher *object_patch.PatchCollector, projectName, errMsg string) {
	setProjectStatus(patcher, projectName, "Error", errMsg, false)
}

func SetProjectStatusDeploying(patcher *object_patch.PatchCollector, projectName string) {
	setProjectStatus(patcher, projectName, "Deploying", "Deckhouse is creating the project, see deckhouse logs for more details.", false)
}

func SetSyncStatusProject(patcher *object_patch.PatchCollector, projectName string) {
	setProjectStatus(patcher, projectName, "Sync", "", true)
}

func setProjectStatus(patcher *object_patch.PatchCollector, projectName, status, message string, sync bool) {
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"state":   status,
			"message": stringOrNil(message),
			"sync":    sync,
		},
	}

	patchStatus(patcher, ProjectKind, projectName, statusPatch, ProjectAPIVersion)
}

// ProjectTemplate
func SetProjectTemplateStatusReady(patcher *object_patch.PatchCollector, projectTemplateName string) {
	setProjectTemplateStatus(patcher, projectTemplateName, true, "")
}

func SetProjectTemplateStatusError(patcher *object_patch.PatchCollector, projectTemplateName, message string) {
	setProjectTemplateStatus(patcher, projectTemplateName, false, message)
}

func setProjectTemplateStatus(patcher *object_patch.PatchCollector, projectTemplateName string, ready bool, message string) {
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"ready":   ready,
			"message": stringOrNil(message),
		},
	}

	patchStatus(patcher, ProjectTemplateKind, projectTemplateName, statusPatch, ProjectTemplateAPIVersion)
}

// ProjectType
func SetProjectTypeStatusReady(patcher *object_patch.PatchCollector, ptName string) {
	setProjectTypeStatus(patcher, ptName, true, "")
}

func SetProjectTypeStatusError(patcher *object_patch.PatchCollector, ptName, message string) {
	setProjectTypeStatus(patcher, ptName, false, message)
}

func setProjectTypeStatus(patcher *object_patch.PatchCollector, ptName string, ready bool, message string) {
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"ready":   ready,
			"message": stringOrNil(message),
		},
	}

	patchStatus(patcher, ProjectTypeKind, ptName, statusPatch, ProjectTypeAPIVersion)
}
