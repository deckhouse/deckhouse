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

package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

// testSettings is a minimal ModuleSettingsObject implementation for decoding tests.
type testSettings struct {
	Provider testSection `json:"provider,omitempty"`
	Nodes    testSection `json:"nodes,omitempty"`
	Storage  testSection `json:"storage,omitempty"`
	CCM      testSection `json:"ccm,omitempty"`
}

type testSection struct {
	Disabled   bool              `json:"disabled,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

func (s *testSettings) HasProviderSection() bool {
	return s != nil && !reflect.DeepEqual(s.Provider, testSection{})
}

func (s *testSettings) HasNodesSection() bool {
	return s != nil && !reflect.DeepEqual(s.Nodes, testSection{})
}

func (s *testSettings) HasStorageSection() bool {
	return s != nil && !reflect.DeepEqual(s.Storage, testSection{})
}

func (s *testSettings) HasCCMSection() bool {
	return s != nil && !reflect.DeepEqual(s.CCM, testSection{})
}

var _ ModuleSettingsObject = (*testSettings)(nil)

func TestModuleConfigDecodesTypedSettings(t *testing.T) {
	t.Parallel()

	raw := `{
		"metadata": {"name": "cloud-provider-test"},
		"spec": {
			"enabled": true,
			"version": 2,
			"settings": {
				"provider": {"parameters": {"namespace": "d8-cloud-provider-test"}},
				"nodes": {"disabled": true}
			}
		}
	}`

	var moduleConfig ModuleConfig[*testSettings]
	if err := json.Unmarshal([]byte(raw), &moduleConfig); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if moduleConfig.Name != "cloud-provider-test" {
		t.Fatalf("Name = %q, want cloud-provider-test", moduleConfig.Name)
	}
	if moduleConfig.Spec.Version != 2 {
		t.Fatalf("Spec.Version = %d, want 2", moduleConfig.Spec.Version)
	}
	if moduleConfig.Spec.Enabled == nil || !*moduleConfig.Spec.Enabled {
		t.Fatalf("Spec.Enabled = %v, want true", moduleConfig.Spec.Enabled)
	}
	if got := moduleConfig.Spec.Settings.Provider.Parameters["namespace"]; got != "d8-cloud-provider-test" {
		t.Fatalf("Settings.Provider.Parameters[namespace] = %q, want d8-cloud-provider-test", got)
	}
}

func TestModuleConfigSettingsSectionPresence(t *testing.T) {
	t.Parallel()

	raw := `{"spec": {"settings": {"provider": {"parameters": {"namespace": "ns"}}, "storage": {"disabled": true}}}}`

	var moduleConfig ModuleConfig[*testSettings]
	if err := json.Unmarshal([]byte(raw), &moduleConfig); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	settings := moduleConfig.Spec.Settings
	if !settings.HasProviderSection() {
		t.Fatal("HasProviderSection() = false, want true")
	}
	if !settings.HasStorageSection() {
		t.Fatal("HasStorageSection() = false, want true")
	}
	if settings.HasNodesSection() {
		t.Fatal("HasNodesSection() = true, want false")
	}
	if settings.HasCCMSection() {
		t.Fatal("HasCCMSection() = true, want false")
	}
}

func TestModuleConfigNilSettingsIsSafe(t *testing.T) {
	t.Parallel()

	moduleConfig := ModuleConfig[*testSettings]{}

	if moduleConfig.Spec.Settings.HasProviderSection() {
		t.Fatal("HasProviderSection() on nil settings = true, want false")
	}
	if moduleConfig.Spec.Settings.HasNodesSection() ||
		moduleConfig.Spec.Settings.HasStorageSection() ||
		moduleConfig.Spec.Settings.HasCCMSection() {
		t.Fatal("section predicates on nil settings must be false")
	}
}
