// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/kube/object_patch"
)

// it's important to run the hook before any cloud-provider try to deploy it's templates and consider to remove the d8-cni-configuration Secret which it used to handle
var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 3},
	Queue:     "main",
}, unhelmD8CNIConfiguration)

func unhelmD8CNIConfiguration(input *go_hook.HookInput) error {
	patch := map[string]any{
		"metadata": map[string]any{
			"annotations": map[string]any{
				"meta.helm.sh/release-name":      nil,
				"meta.helm.sh/release-namespace": nil,
				"helm.sh/resource-policy":        "keep",
			},
			"labels": map[string]interface{}{
				"app.kubernetes.io/managed-by": nil,
				"heritage":                     nil,
				"module":                       nil,
			},
		},
	}

	input.PatchCollector.MergePatch(
		patch,
		"v1",
		"Secret",
		"kube-system",
		"d8-cni-configuration",
		object_patch.IgnoreMissingObject(),
	)
	return nil
}
