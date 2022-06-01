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
	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	OnStartup: &go_hook.OrderedConfig{Order: 20},
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:       "ingressControllers",
			ApiVersion: "deckhouse.io/v1",
			Kind:       "IngressNginxController",
			FilterFunc: applySpecControllerFilter,
		},
	},
}, discoverMinimalNginxVersion)

const (
	minVersionValuesKey = "ingressNginx.internal.minimalControllerVersion"
)

func applySpecControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, ok, err := unstructured.NestedString(obj.Object, "spec", "controllerVersion")
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, nil
	}

	return version, nil
}

func discoverMinimalNginxVersion(input *go_hook.HookInput) error {
	snap := input.Snapshots["ingressControllers"]

	var minVersion *semver.Version

	for _, s := range snap {
		if s == nil {
			continue
		}

		v, err := semver.NewVersion(s.(string))
		if err != nil {
			return err
		}

		if minVersion == nil || v.LessThan(minVersion) {
			minVersion = v
		}
	}

	if minVersion == nil {
		input.Values.Remove(minVersionValuesKey)
		return nil
	}

	input.Values.Set(minVersionValuesKey, minVersion.String())

	return nil
}
