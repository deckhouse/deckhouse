/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import (
	"github.com/flant/shell-operator/pkg/kube/object_patch"

	"github.com/deckhouse/deckhouse/ee/modules/160-multitenancy-manager/hooks/apis/deckhouse.io/v1alpha1"
)

func ProjectStatusIsDeploying(status v1alpha1.ProjectStatus) bool {
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

	patchStatus(patcher, ProjectKind, projectName, statusPatch)
}
