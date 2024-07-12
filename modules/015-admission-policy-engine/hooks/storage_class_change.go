/*
Copyright 2023 Flant JSC

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

	"github.com/deckhouse/deckhouse/go_lib/hooks/storage_class_change"
)

var _ = storage_class_change.RegisterHook(storage_class_change.Args{
	ModuleName:                    "admissionPolicyEngine",
	Namespace:                     "d8-admission-policy-engine",
	LabelSelectorKey:              "app",
	LabelSelectorValue:            "trivy-provider",
	ObjectKind:                    "StatefulSet",
	ObjectName:                    "trivy-provider",
	D8ConfigStorageClassParamName: "denyVulnerableImages.storageClass",
	BeforeHookCheck: func(input *go_hook.HookInput) bool {
		return input.Values.Get("admissionPolicyEngine.denyVulnerableImages.enabled").Bool()
	},
})
