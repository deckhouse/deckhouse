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
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	v1core "k8s.io/api/core/v1"
	"github.com/flant/shell-operator/pkg/kube_events_manager/types"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "default_sc_name",
			ApiVersion: "v1",
			Kind:       "ConfigMap",
			NameSelector: &types.NameSelector{
				MatchNames: []string{"d8-default-storage-class"},
			},
			NamespaceSelector: &types.NamespaceSelector{
				NameSelector: &types.NameSelector{
					MatchNames: []string{"d8-system"},
				},
			},
			FilterFunc: applyDefaultStorageClassNameFilter,
		},
	},
}, discoveryDefaultStorageClassName)

func applyDefaultStorageClassNameFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	// slightly modified code from go_lib/filter/extract.go/KeyFromConfigMap
	const key = "default-storage-class-name"

	var cm v1core.ConfigMap
	err := sdk.FromUnstructured(obj, &cm)
	if err != nil {
		// if no configmap - no problem
		return "", err
	}

	val, ok := cm.Data[key]
	if !ok {
		// if no key in configmap - no problem
		return "", nil
	}

	return val, nil
}

func discoveryDefaultStorageClassName(input *go_hook.HookInput) error {
	defaultStorageClassNameSnap :=	input.Snapshots["default_sc_name"]

	const valuePath = "global.discovery.defaultStorageClassName"

	if len(defaultStorageClassNameSnap) == 0 || defaultStorageClassNameSnap[0] == "" {
		input.LogEntry.Warnln("Default storage class name not found or empty. Cleaning current value.")
		input.Values.Remove(valuePath)
		return nil
	}

	input.LogEntry.Infof("Set %s to `%s`", valuePath, defaultStorageClassNameSnap[0])
	input.Values.Set(valuePath, defaultStorageClassNameSnap[0])

	return nil
}
