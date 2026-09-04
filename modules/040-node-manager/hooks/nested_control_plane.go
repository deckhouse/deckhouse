/*
Copyright 2026 Flant JSC
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

import "github.com/flant/addon-operator/pkg/module_manager/go_hook"

// nestedControlPlane reports whether Deckhouse manages this cluster from a parent
// one (a virtual control plane tenant). There the node lifecycle objects belong to
// control-plane-manager, so hooks that create them here would fight it.
func nestedControlPlane(input *go_hook.HookInput) bool {
	return !input.Values.Get("global.deckhouseSelfHosted").Bool()
}
