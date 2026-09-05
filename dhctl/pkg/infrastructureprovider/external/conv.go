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

package external

import (
	"encoding/json"
	"fmt"

	validatev1 "github.com/deckhouse/deckhouse/go_lib/dhctl-provider-protocol/api/validate/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
)

func toWireInput(input config.ProviderInput) (validatev1.Input, error) {
	pcc := make(map[string]any, len(input.ProviderClusterConfig))

	for key, raw := range input.ProviderClusterConfig {
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			return validatev1.Input{}, fmt.Errorf("unmarshal provider cluster config key %q: %w", key, err)
		}

		pcc[key] = value
	}

	return validatev1.Input{
		ProviderName:          input.ProviderName,
		ClusterPrefix:         input.ClusterPrefix,
		Layout:                input.Layout,
		Operation:             validatev1.Operation(input.Operation),
		ProviderClusterConfig: pcc,
		CloudProviderVars:     input.CloudProviderVars,
	}, nil
}

func violationsToStrings(violations []*validatev1.ViolationResponse) []string {
	lines := make([]string, 0, len(violations))
	for _, violation := range violations {
		if violation.Path == "" {
			lines = append(lines, violation.Message)
		} else {
			lines = append(lines, fmt.Sprintf("%s: %s", violation.Path, violation.Message))
		}
	}
	return lines
}
