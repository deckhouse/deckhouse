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

package instanceprefix

import "github.com/tidwall/gjson"

// Values is the part of go_hook.HookInput.Values the resolver reads.
type Values interface {
	Get(path string) gjson.Result
	GetOk(path string) (gjson.Result, bool)
}

// Resolve returns the prefix node-controller puts in front of MachineDeployment names:
// nodeManager.instancePrefix when set, otherwise global.prefix (the global ModuleConfig),
// otherwise the deprecated global.clusterConfiguration.cloud.prefix.
//
// It reads only config and global values, which are complete before any node-manager hook runs.
// Hooks must call it rather than read nodeManager.internal.instancePrefix: that value is written by
// a beforeHelm hook, so it is absent on the first module run after a restart of deckhouse and stale
// on the run after a prefix change.
func Resolve(values Values) string {
	prefix, exists := values.GetOk("nodeManager.instancePrefix")
	if !exists {
		prefix, exists = values.GetOk("global.prefix")
	}
	if !exists {
		prefix = values.Get("global.clusterConfiguration.cloud.prefix")
	}

	return prefix.String()
}
