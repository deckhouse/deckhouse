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
type MetricAction int

// Enum values for MetricAction
const (
	ActionCounterAdd MetricAction = iota
	ActionGaugeAdd
	ActionGaugeSet
	ActionHistogramObserve
	ActionExpireMetrics
)

// IsValid checks if the action is one of the valid actions
func (a MetricAction) IsValid() bool {
	switch a {
	case ActionCounterAdd, ActionGaugeAdd, ActionGaugeSet,
		ActionHistogramObserve, ActionExpireMetrics:
		return true
	default:
		return false
	}
}

var actionStrings = map[MetricAction]string{
	ActionCounterAdd:       "CounterAdd",
	ActionGaugeAdd:         "GaugeAdd",
	ActionGaugeSet:         "GaugeSet",
	ActionHistogramObserve: "HistogramObserve",
	ActionExpireMetrics:    "ExpireMetrics",
}

func (a MetricAction) String() string {
	if str, ok := actionStrings[a]; ok {
		return str
	}

	return "Unknown"
}
