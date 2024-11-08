// Copyright 2021 Flant JSC
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
	"strings"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "default_sc",
			ApiVersion: "storage.k8s.io/v1",
			Kind:       "Storageclass",
			FilterFunc: applyDefaultStorageClassFilter,
		},
	},
}, discoveryDefaultStorageClass)

type storageClass struct {
	Name      string
	IsDefault bool
}

func applyDefaultStorageClassFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	annotations := obj.GetAnnotations()

	annotToCheck := []string{
		"storageclass.beta.kubernetes.io/is-default-class",
		"storageclass.kubernetes.io/is-default-class",
	}

	isDefault := false
	for _, annot := range annotToCheck {
		if v, ok := annotations[annot]; ok && strings.ToLower(v) == "true" {
			isDefault = true
			break
		}
	}

	return storageClass{
		IsDefault: isDefault,
		Name:      obj.GetName(),
	}, nil
}

func discoveryDefaultStorageClass(input *go_hook.HookInput) error {
	storageClassesSnap := input.Snapshots["default_sc"]

	defaultStorageClass := ""
	for _, scRaw := range storageClassesSnap {
		sc := scRaw.(storageClass)
		if sc.IsDefault {
			defaultStorageClass = sc.Name
			break
		}
	}

	const valuePath = "global.discovery.defaultStorageClass"

	if defaultStorageClass == "" {
		input.Logger.Warn("Default storage class not found. Cleaning current value.")
		input.Values.Remove(valuePath)
		return nil
	}

	input.Values.Set(valuePath, defaultStorageClass)

	return nil
}
