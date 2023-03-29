/*
Copyright 2021 Flant JSC

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

package transform

import (
	"fmt"
	"strings"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/apis/v1alpha1"
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/vrl"
)

const (
	excludeField  = "exclude"
	keyFieldField = "key_field"
)

// ThrottleTransform adds throttling to event's flow.
func ThrottleTransform(rl v1alpha1.RateLimitSpec) (apis.LogTransform, error) {
	throttleTransform := &DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "ratelimit",
			Type:   "throttle",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			excludeField:  "null",
			"threshold":   *rl.LinesPerMinute,
			"window_secs": 60,
		},
	}

	if rl.KeyField != "" {
		throttleTransform.DynamicArgsMap[keyFieldField] = rl.KeyField
	}

	if rl.Excludes != nil {
		excludeCond, err := generateExcludesForDynamicTransform(rl.Excludes)
		if err != nil {
			return nil, err
		}
		throttleTransform.DynamicArgsMap[excludeField] = excludeCond
	}
	return throttleTransform, nil
}

func generateExcludesForDynamicTransform(excludes []v1alpha1.Filter) (map[string]interface{}, error) {
	var trottleResult strings.Builder
	trottleResult.WriteString("{ false }")

	for _, filter := range excludes {
		rule := getRuleOutOfFilter(&filter)
		if rule == "" {
			return nil, fmt.Errorf("exclude rule value is empty for dynamic transform")
		}
		condition, err := rule.Render(vrl.Args{"filter": filter})
		if err != nil {
			return nil, fmt.Errorf("error rendering exclude rule for dynamic transform: %w", err)
		}

		trottleResult.WriteString("|| { " + condition + " }")
	}

	return map[string]interface{}{"type": "vrl", "source": trottleResult.String()}, nil
}
