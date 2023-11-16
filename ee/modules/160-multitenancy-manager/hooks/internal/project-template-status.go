/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import "github.com/flant/shell-operator/pkg/kube/object_patch"

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

	patchStatus(patcher, ProjectTemplateKind, projectTemplateName, statusPatch)
}
