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

package validation

import (
	"testing"

	cpapi "github.com/deckhouse/deckhouse/go_lib/cloud-provider/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func moduleConfigState(moduleConfig *cpapi.ModuleConfig, legacy map[string]any) *State {
	return &State{
		ModuleName:                  "cloud-provider-dvp",
		ModuleConfig:                moduleConfig,
		LegacyProviderClusterConfig: legacy,
	}
}

func TestValidateModuleConfigAllowsDisabledSubsystems(t *testing.T) {
	t.Parallel()

	state := moduleConfigState(&cpapi.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cloud-provider-dvp"},
		Spec: cpapi.ModuleConfigSpec{
			Enabled: ptr.To(true),
			Version: 2,
			Settings: cpapi.ModuleConfigSpecSettings{
				Storage: &cpapi.ModuleConfigSpecSubsystemSettings{Disabled: ptr.To(true)},
				Nodes:   &cpapi.ModuleConfigSpecSubsystemSettings{Disabled: ptr.To(true)},
			},
		},
	}, nil)

	if result := ValidateModuleConfig(state); result.HasErrors() {
		t.Fatalf("ValidateModuleConfig() unexpected errors: %s", result.Error())
	}
}

func TestValidateModuleConfigIgnoresSensitiveSettings(t *testing.T) {
	t.Parallel()

	moduleConfig := &cpapi.ModuleConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cloud-provider-dvp"},
		Spec: cpapi.ModuleConfigSpec{
			Settings: cpapi.ModuleConfigSpecSettings{
				Provider: &cpapi.ModuleConfigSpecProviderSettings{
					Parameters: map[string]any{
						"token": "must-not-fail",
					},
				},
			},
		},
	}

	if result := ValidateModuleConfig(moduleConfigState(moduleConfig, nil)); result.HasErrors() {
		t.Fatalf("ValidateModuleConfig() unexpected errors: %s", result.Error())
	}
}

func TestValidateModuleConfigRequiredWithoutLegacyPCC(t *testing.T) {
	t.Parallel()

	result := ValidateModuleConfig(&State{ModuleName: "cloud-provider-dvp"})
	if !hasViolationCode(result, "module_config_required") {
		t.Fatalf("ValidateModuleConfig() = %q", result.Error())
	}
}

func TestValidateModuleConfigAllowsLegacyPCCWithoutModuleConfig(t *testing.T) {
	t.Parallel()

	state := moduleConfigState(nil, map[string]any{"masterNodeGroup": map[string]any{}})
	if result := ValidateModuleConfig(state); result.HasErrors() {
		t.Fatalf("ValidateModuleConfig() = %q, want no errors", result.Error())
	}
}

func TestValidateModuleConfigRejectsWrongName(t *testing.T) {
	t.Parallel()

	result := ValidateModuleConfig(moduleConfigState(
		&cpapi.ModuleConfig{ObjectMeta: metav1.ObjectMeta{Name: "wrong-name"}},
		nil,
	))
	if !hasViolationCode(result, "invalid_module_config_name") {
		t.Fatalf("ValidateModuleConfig() = %q", result.Error())
	}
}
