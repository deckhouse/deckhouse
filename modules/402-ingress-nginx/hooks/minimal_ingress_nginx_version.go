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

// this hook figure out minimal ingress controller version at the beginning and on IngressNginxController creation
// this version is used on requirements check on Deckhouse update
// Deckhouse would not update minor version before pod is ready, so this hook will execute at least once (on sync)

package hooks

import (
	"github.com/Masterminds/semver/v3"
	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	"github.com/flant/addon-operator/sdk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
)

var _ = sdk.RegisterFunc(&go_hook.HookConfig{
	Kubernetes: []go_hook.KubernetesConfig{
		{
			Name:                         "ingressControllers",
			ApiVersion:                   "deckhouse.io/v1",
			Kind:                         "IngressNginxController",
			WaitForSynchronization:       pointer.BoolPtr(true),
			ExecuteHookOnEvents:          pointer.BoolPtr(true),
			ExecuteHookOnSynchronization: pointer.BoolPtr(true),
			FilterFunc:                   applySpecControllerFilter,
		},
	},
}, discoverMinimalNginxVersion)

const (
	minVersionValuesKey     = "ingressNginx.internal.minimalControllerVersion"
	incompatibleVersionsKey = "ingressNginx.internal.hasIncompatibleIngressClass"
)

func applySpecControllerFilter(obj *unstructured.Unstructured) (go_hook.FilterResult, error) {
	version, _, err := unstructured.NestedString(obj.Object, "spec", "controllerVersion")
	if err != nil {
		return nil, err
	}

	ingressClass, ok, err := unstructured.NestedString(obj.Object, "spec", "ingressClass")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	return ingressNginxController{
		Version:      version,
		IngressClass: ingressClass,
	}, nil
}

func discoverMinimalNginxVersion(input *go_hook.HookInput) error {
	snap := input.Snapshots["ingressControllers"]
	isIncompatible := false

	var minVersion *semver.Version
	classVersionMap := make(map[string]*semver.Version)

	for _, s := range snap {
		if s == nil {
			continue
		}

		ctrl := s.(ingressNginxController)
		if ctrl.Version == "" {
			ctrl.Version = input.Values.Get("ingressNginx.defaultControllerVersion").String()
		}
		ctrlVersion, err := semver.NewVersion(ctrl.Version)
		if err != nil {
			return err
		}

		if v, ok := classVersionMap[ctrl.IngressClass]; ok {
			if versionsIncompatible(v, ctrlVersion) {
				isIncompatible = true
			}
		}
		classVersionMap[ctrl.IngressClass] = ctrlVersion

		if minVersion == nil || ctrlVersion.LessThan(minVersion) {
			minVersion = ctrlVersion
		}
	}

	input.Values.Set(incompatibleVersionsKey, isIncompatible)

	if minVersion == nil {
		input.Values.Remove(minVersionValuesKey)
		return nil
	}

	input.Values.Set(minVersionValuesKey, minVersion.String())

	return nil
}

var (
	borderVersion = semver.MustParse("1.0.0")
)

func versionsIncompatible(v1, v2 *semver.Version) bool {
	if v1.GreaterThan(borderVersion) && v2.LessThan(borderVersion) {
		return true
	}

	if v1.LessThan(borderVersion) && v2.GreaterThan(borderVersion) {
		return true
	}

	return false
}

type ingressNginxController struct {
	Version      string
	IngressClass string
}
