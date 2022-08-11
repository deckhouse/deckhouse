/*
Copyright 2022 Flant JSC
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

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

// this hook is needed only for release 1.34.12
// after that release bashible-apiserver is updated and works without this

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Queue: "/modules/node-manager/remove_bashible_apiserver",
	OnBeforeHelm: &go_hook.OrderedConfig{
		Order: 5,
	},
}, removeBashibleHandler)

func removeBashibleHandler(input *go_hook.HookInput) error {
	input.PatchCollector.Delete("apps/v1", "Deployment", "d8-cloud-instance-manager", "bashible-apiserver")

	return nil
}
