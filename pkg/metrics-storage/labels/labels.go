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

package labels

import (
	"sort"
)

// MergeLabels merges several maps into one. Last map keys overrides keys from first maps.
//
// Can be used to copy a map if just one argument is used.
func MergeLabels(labelsMaps ...map[string]string) map[string]string {
	labels := make(map[string]string)
	for _, labelsMap := range labelsMaps {
		for k, v := range labelsMap {
			labels[k] = v
		}
	}
	return labels
}

// LabelNames returns sorted label keys
func LabelNames(labels map[string]string) []string {
	names := make([]string, 0)
	for labelName := range labels {
		names = append(names, labelName)
	}
	sort.Strings(names)
	return names
}

func LabelValues(labels map[string]string, labelNames []string) []string {
	values := make([]string, 0)
	for _, name := range labelNames {
		values = append(values, labels[name])
	}
	return values
}

// IsSubset checks if a set contains b subset
func IsSubset(a, b []string) bool {
	aMap := make(map[string]struct{}, len(a))
	for _, v := range a {
		aMap[v] = struct{}{}
	}

	for _, v := range b {
		if _, found := aMap[v]; !found {
			return false
		}
	}
	return true
}
