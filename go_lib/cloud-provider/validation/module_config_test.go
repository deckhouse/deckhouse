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
	testprovider "github.com/deckhouse/deckhouse/go_lib/cloud-provider/validation/internal/testprovider"
	"k8s.io/utils/ptr"
)

func moduleConfigState(moduleConfig *cpapi.ModuleConfig[*testprovider.Settings], pcc *testprovider.ProviderClusterConfig) *testState {
	return &testState{
		ModuleName:            "cloud-provider-dvp",
		ModuleConfig:          moduleConfig,
		ProviderClusterConfig: pcc,
	}
}

func TestValidateModuleConfigAllowsDisabledSubsystems(t *testing.T) {
	t.Parallel()

	state := moduleConfigState(&cpapi.ModuleConfig[*testprovider.Settings]{
		ObjectMeta: cpapi.ObjectMeta{Name: "cloud-provider-dvp"},
		Spec: cpapi.ModuleConfigSpec[*testprovider.Settings]{
			Enabled: ptr.To(true),
			Version: 2,
			Settings: &testprovider.Settings{
				Provider: testprovider.Section{Parameters: map[string]string{"namespace": "d8-cloud-provider-test"}},
				Storage:  testprovider.Section{Disabled: true},
				Nodes:    testprovider.Section{Disabled: true},
			},
		},
	}, nil)

	if result := ValidateModuleConfig(state); result.HasErrors() {
		t.Fatalf("ValidateModuleConfig() unexpected errors: %s", result.Error())
	}
}

func TestValidateModuleConfigIgnoresSensitiveSettings(t *testing.T) {
	t.Parallel()

	moduleConfig := &cpapi.ModuleConfig[*testprovider.Settings]{
		ObjectMeta: cpapi.ObjectMeta{Name: "cloud-provider-dvp"},
		Spec: cpapi.ModuleConfigSpec[*testprovider.Settings]{
			Settings: &testprovider.Settings{
				Provider: testprovider.Section{Parameters: map[string]string{"token": "must-not-fail"}},
			},
		},
	}

	if result := ValidateModuleConfig(moduleConfigState(moduleConfig, nil)); result.HasErrors() {
		t.Fatalf("ValidateModuleConfig() unexpected errors: %s", result.Error())
	}
}

func TestValidateModuleConfigRequiredWithoutLegacyPCC(t *testing.T) {
	t.Parallel()

	result := ValidateModuleConfig(&testState{ModuleName: "cloud-provider-dvp"})
	if !hasViolationCode(result, "module_config_required") {
		t.Fatalf("ValidateModuleConfig() = %q", result.Error())
	}
}

func TestValidateModuleConfigAllowsLegacyPCCWithoutModuleConfig(t *testing.T) {
	t.Parallel()

	state := moduleConfigState(nil, &testprovider.ProviderClusterConfig{MasterNodeGroup: &testprovider.MasterNodeGroup{}})
	if result := ValidateModuleConfig(state); result.HasErrors() {
		t.Fatalf("ValidateModuleConfig() = %q, want no errors", result.Error())
	}
}

func TestValidateModuleConfigRejectsWrongName(t *testing.T) {
	t.Parallel()

	result := ValidateModuleConfig(moduleConfigState(
		&cpapi.ModuleConfig[*testprovider.Settings]{ObjectMeta: cpapi.ObjectMeta{Name: "wrong-name"}},
		nil,
	))
	if !hasViolationCode(result, "invalid_module_config_name") {
		t.Fatalf("ValidateModuleConfig() = %q", result.Error())
	}
}
