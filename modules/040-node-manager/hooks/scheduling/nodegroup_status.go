/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scheduling

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

func patchNodeGroupStatus(patcher go_hook.PatchCollector, nodeGroupName string, patch interface{}) {
	patcher.PatchWithMerge(patch, "deckhouse.io/v1", "NodeGroup", "", nodeGroupName, object_patch.WithSubresource("/status"))
}

func setNodeGroupStatus(patcher go_hook.PatchCollector, nodeGroupName string, statusField string, value interface{}) {
	statusPatch := map[string]interface{}{
		"status": map[string]interface{}{
			statusField: value,
		},
	}
	patchNodeGroupStatus(patcher, nodeGroupName, statusPatch)
}
