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

package hooks

import (
	"context"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnBeforeHelm: &go_hook.OrderedConfig{Order: 100},
}, handleSetInstancePrefix)

// The value is published for templates only: they render after every beforeHelm hook. Hooks call
// resolveInstancePrefix themselves, since they may run before this one.
func handleSetInstancePrefix(_ context.Context, input *go_hook.HookInput) error {
	input.Values.Set("nodeManager.internal.instancePrefix", resolveInstancePrefix(input))

	return nil
}

// resolveInstancePrefix returns the prefix put in front of MachineDeployment names:
// nodeManager.instancePrefix when set, otherwise global.clusterConfiguration.cloud.prefix.
//
// It reads only config and global values, which are complete before any node-manager hook runs.
// Hooks must call it rather than read nodeManager.internal.instancePrefix: that value is written by
// a beforeHelm hook, so it is absent on the first module run after a restart of deckhouse and stale
// on the run after a prefix change.
func resolveInstancePrefix(input *go_hook.HookInput) string {
	prefix, exists := input.Values.GetOk("nodeManager.instancePrefix")
	if !exists {
		prefix = input.Values.Get("global.clusterConfiguration.cloud.prefix")
	}

	return prefix.String()
}
