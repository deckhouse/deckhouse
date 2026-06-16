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
	"testing"
)

func TestModuleConfigSpecProviderParameters(t *testing.T) {
	t.Parallel()

	raw := `{
		"spec": {
			"settings": {
				"provider": {
					"parameters": {
						"namespace": "d8-cloud-provider-dvp"
					}
				}
			}
		}
	}`

	var moduleConfig ModuleConfig
	if err := json.Unmarshal([]byte(raw), &moduleConfig); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if moduleConfig.Spec.Settings.Provider == nil ||
		moduleConfig.Spec.Settings.Provider.Parameters["namespace"] != "d8-cloud-provider-dvp" {
		t.Fatalf("Provider.Parameters = %#v, want namespace parameter", moduleConfig.Spec.Settings.Provider)
	}
}

func TestModuleConfigSpecSubsystemDisabled(t *testing.T) {
	t.Parallel()

	raw := `{
		"spec": {
			"settings": {
				"nodes": {
					"disabled": true
				},
				"storage": {
					"disabled": false
				}
			}
		}
	}`

	var moduleConfig ModuleConfig
	if err := json.Unmarshal([]byte(raw), &moduleConfig); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if moduleConfig.Spec.Settings.Nodes == nil || moduleConfig.Spec.Settings.Nodes.Disabled == nil || !*moduleConfig.Spec.Settings.Nodes.Disabled {
		t.Fatalf("Nodes.Disabled = %#v, want true", moduleConfig.Spec.Settings.Nodes)
	}
	if moduleConfig.Spec.Settings.Storage == nil || moduleConfig.Spec.Settings.Storage.Disabled == nil || *moduleConfig.Spec.Settings.Storage.Disabled {
		t.Fatalf("Storage.Disabled = %#v, want false", moduleConfig.Spec.Settings.Storage)
	}
}
