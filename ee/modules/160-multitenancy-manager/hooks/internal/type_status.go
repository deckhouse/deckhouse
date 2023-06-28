/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package internal

import "github.com/flant/shell-operator/pkg/kube/object_patch"

func SetProjectTypeStatus(patcher *object_patch.PatchCollector, ptName string, status bool, message string) {
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			"statusSummary": map[string]interface{}{
				"status":  status,
				"message": stringOrNil(message),
			},
		},
	}

	patchStatus(patcher, ProjectTypeKind, ptName, statusPatch)
}
