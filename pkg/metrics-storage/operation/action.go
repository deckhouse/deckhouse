// Copyright 2025 Flant JSC
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

package operation

// MetricAction defines the supported metric operation types
type MetricAction string

// Enum values for MetricAction
const (
	ActionCounterAdd          MetricAction = "c_add"
	ActionOldCounterAdd       MetricAction = "add"
	ActionGaugeAdd            MetricAction = "g_add"
	ActionGaugeSet            MetricAction = "g_set"
	ActionOldGaugeSet         MetricAction = "set"
	ActionHistogramObserve    MetricAction = "h_observe"
	ActionOldHistogramObserve MetricAction = "observe"
	ActionExpireMetrics       MetricAction = "expire"
)

// IsValid checks if the action is one of the valid actions
func (a MetricAction) IsValid() bool {
	switch a {
	case ActionOldGaugeSet, ActionCounterAdd, ActionHistogramObserve, ActionExpireMetrics:
		return true
	default:
		return false
	}
}

// String returns the string representation of the MetricAction
func (a MetricAction) String() string {
	return string(a)
}
