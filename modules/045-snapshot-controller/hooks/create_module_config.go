/*
Copyright 2025 Flant JSC

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnAfterHelm: &go_hook.OrderedConfig{Order: 10},
}, createModuleConfig)

func createModuleConfig(input *go_hook.HookInput) error {
	desiredModuleConfig := &v1alpha1.ModuleConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ModuleConfig",
			APIVersion: "deckhouse.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "snapshot-controller",
		},
		Spec: v1alpha1.ModuleConfigSpec{
			Enabled:  ptr.To(true),
			Version:  1,
			Settings: v1alpha1.SettingsValues{},
		},
	}

	input.PatchCollector.CreateOrUpdate(desiredModuleConfig)
	return nil
}
