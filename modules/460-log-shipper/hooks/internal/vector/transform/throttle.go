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
	"github.com/deckhouse/deckhouse/modules/460-log-shipper/hooks/internal/v1alpha1"
)

// ThrottleTransform adds throttling to event's flow.
func ThrottleTransform(rl v1alpha1.RateLimitSpec) *DynamicTransform {
	throttleTransform := DynamicTransform{
		CommonTransform: CommonTransform{
			Name: "ratelimit",
			Type: "throttle",
		},
		DynamicArgsMap: map[string]interface{}{
			"exclude":     "null",
			"threshold":   *rl.LinesPerMinute,
			"window_secs": 60,
		},
	}

	return &throttleTransform
}
