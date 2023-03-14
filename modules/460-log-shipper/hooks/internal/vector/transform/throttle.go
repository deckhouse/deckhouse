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
func ThrottleTransform(rl v1alpha1.RateLimitSpec) ([]apis.LogTransform, error) {

	if rl.Excludes != nil && rl.KeyField != "" {
		return processExcludesForDynamicTransform(rl.Excludes, *rl.LinesPerMinute, rl.KeyField)
	}

	return []apis.LogTransform{newDynamicTransform(*rl.LinesPerMinute, rl.KeyField)}, nil
}

func processExcludesForDynamicTransform(excludes []v1alpha1.Filter, threshold int32, keyField string) ([]apis.LogTransform, error) {
	throttleTransforms := make([]apis.LogTransform, len(excludes))
	for i, filter := range excludes {
		throttleTransform := newDynamicTransform(threshold, keyField)
		throttleTransform.CommonTransform.Inputs = set.New()
		if err := setExcludeThrottleTransformDynamicArg(throttleTransform.DynamicArgsMap, &filter); err != nil {
			return nil, err
		}

		throttleTransforms[i] = throttleTransform
	}
	return throttleTransforms, nil
}

func setExcludeThrottleTransformDynamicArg(m map[string]interface{}, exclude *v1alpha1.Filter) error {
	v, err := throttleTransformExclude(exclude)
	if err != nil {
		return err
	}
	m[excludeField] = v
	return nil
}

func throttleTransformExclude(exclude *v1alpha1.Filter) (map[string]interface{}, error) {
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

	return map[string]interface{}{"type": "vrl", "source": condition}, nil
}

func newDynamicTransform(threshold int32, keyField string) *DynamicTransform {
	throttleTransform := DynamicTransform{
		CommonTransform: CommonTransform{
			Name:   "ratelimit",
			Type:   "throttle",
			Inputs: set.New(),
		},
		DynamicArgsMap: map[string]interface{}{
			"threshold":   threshold,
			"window_secs": 60,
			excludeField:  "null",
		},
	}
	if keyField != "" {
		throttleTransform.DynamicArgsMap[keyFieldField] = keyField
	}
	return &throttleTransform
}
