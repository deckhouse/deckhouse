/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import "github.com/flant/shell-operator/pkg/kube/object_patch"

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

	patchStatus(patcher, ProjectTypeKind, ptName, statusPatch)
}
