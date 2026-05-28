// Copyright 2026 Flant JSC
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
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var moduleConfigGVR = schema.ParseGroupResource("moduleconfigs.deckhouse.io").WithVersion("v1alpha1")

type moduleConfigEnabledResult struct {
	enabledFlagExists bool
	enabled           bool
}

func moduleEnabledByModuleConfig(mc *unstructured.Unstructured) (*moduleConfigEnabledResult, error) {
	moduleEnabled, exists, err := unstructured.NestedBool(mc.UnstructuredContent(), "spec", "enabled")
	if err != nil {
		return nil, fmt.Errorf("nested for check enabled module config %s: %w", mc.GetName(), err)
	}

	res := &moduleConfigEnabledResult{
		enabledFlagExists: false,
		enabled:           false,
	}

	if !exists {
		return res, nil
	}

	res.enabledFlagExists = true
	res.enabled = moduleEnabled

	return res, nil
}

func moduleConfigSettingFullPath(path ...string) []string {
	fullPath := []string{"spec", "settings"}
	return append(fullPath, path...)
}

func moduleConfigSettingString(mc *unstructured.Unstructured, path ...string) (string, bool, error) {
	fullPath := moduleConfigSettingFullPath(path...)

	res, exists, err := unstructured.NestedString(mc.UnstructuredContent(), fullPath...)
	if err != nil {
		return "", false, fmt.Errorf("cannot get string for mc %s for path %v: %w", mc.GetName(), fullPath, err)
	}

	if !exists {
		return "", false, nil
	}

	return res, true, nil
}
