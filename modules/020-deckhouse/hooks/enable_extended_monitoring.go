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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, enableExtendedMonitoring)

func enableExtendedMonitoring(input *go_hook.HookInput) error {
	jsonPatch := []byte(`{"metadata":{"annotations":{"extended-monitoring.flant.com/enabled": ""}}}`)

	err := input.ObjectPatcher().MergePatchObject(jsonPatch, "v1", "namespace", "", "d8-system", "")
	if err != nil {
		return err
	}

	err = input.ObjectPatcher().MergePatchObject(jsonPatch, "v1", "namespace", "", "kube-system", "")
	if err != nil {
		return err
	}

	return nil
}
