/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"encoding/json"
	"testing"
)

func TestPublishAPIConfigUnmarshalJSON_SupportsLegacyEnable(t *testing.T) {
	t.Parallel()

	var config Config
	if err := json.Unmarshal([]byte(`{"enable":true}`), &config); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if config.Enabled == nil {
		t.Fatal("expected enabled value to be populated from legacy enable")
	}
	if !*config.Enabled {
		t.Fatalf("expected enabled=true, got %v", *config.Enabled)
	}
}

func TestPublishAPIConfigUnmarshalJSON_PrefersEnabledOverLegacyEnable(t *testing.T) {
	t.Parallel()

	var config Config
	if err := json.Unmarshal([]byte(`{"enabled":true,"enable":false}`), &config); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if config.Enabled == nil {
		t.Fatal("expected enabled value to be populated")
	}
	if !*config.Enabled {
		t.Fatalf("expected enabled=true to win over legacy enable, got %v", *config.Enabled)
	}
}
