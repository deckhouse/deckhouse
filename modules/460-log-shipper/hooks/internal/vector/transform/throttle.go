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
		excludeCond, err := processExcludesForDynamicTransform(rl.Excludes, *rl.LinesPerMinute, rl.KeyField)
		if err != nil {
			return nil, err
		}
		throttleTransform.DynamicArgsMap[excludeField] = excludeCond
	}
	return throttleTransform, nil
}

func processExcludesForDynamicTransform(excludes []v1alpha1.Filter, threshold int32, keyField string) (map[string]interface{}, error) {
	throttleTransformExcludes := make([]string, len(excludes))
	for i, filter := range excludes {
		condition, err := excludeThrottleTransformCond(&filter)
		if err != nil {
			return nil, err
		}

		throttleTransformExcludes[i] = *condition
	}
	resultCond := combineThrottleTransformExcludes(throttleTransformExcludes)
	return map[string]interface{}{"type": "vrl", "source": resultCond}, nil
}

func excludeThrottleTransformCond(exclude *v1alpha1.Filter) (*string, error) {
	if exclude == nil {
		return nil, fmt.Errorf("no filter provided for dynamic transform exclude")
	}

	rule := getRuleOutOfFilter(exclude)
	if rule == "" {
		return nil, fmt.Errorf("exclude rule value is empty for dynamic transform")
	}

	condition, err := rule.Render(vrl.Args{"filter": exclude})
	if err != nil {
		return nil, fmt.Errorf("error rendering exclude rule for dynamic transform: %w", err)
	}

	return &condition, nil
}

func combineThrottleTransformExcludes(excludes []string) string {
	resultExcludeConds := make([]string, len(excludes)+1)
	resultExcludeConds[0] = "matchedExcludeCond"
	resultCond := fmt.Sprintf("%s = false;\n", resultExcludeConds[0])

	for i, excludeCond := range excludes {
		resultExcludeConds[i+1] = fmt.Sprintf("matchedExcludeCond%d", i)
		resultCond += fmt.Sprintf("%s = %s;\n", resultExcludeConds[i+1], excludeCond)
	}
	resultCond += strings.Join(resultExcludeConds, " || ")
	return resultCond
}
