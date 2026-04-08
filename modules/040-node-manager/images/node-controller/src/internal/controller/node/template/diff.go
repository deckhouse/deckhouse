/*
Copyright 2025 Flant JSC

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

package template

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

// applyTemplateMap merges actual map with desired template.
// Keys that are in lastApplied but not in template are removed (excess keys).
// Returns the new map and whether any change occurred.
func applyTemplateMap(actual, template, lastApplied map[string]string) (map[string]string, bool) {
	changed := false
	excess := excessMapKeys(lastApplied, template)

	newMap := make(map[string]string, len(actual))
	for k, v := range actual {
		// Ignore keys removed from template (were in lastApplied but no longer in template).
		if excess[k] {
			changed = true
			continue
		}
		newMap[k] = v
	}

	// Merge with values from template.
	for k, v := range template {
		oldVal, ok := newMap[k]
		if !ok || oldVal != v {
			changed = true
		}
		newMap[k] = v
	}

	return newMap, changed
}

// excessMapKeys returns a set of keys present in a but absent from b.
func excessMapKeys(a, b map[string]string) map[string]bool {
	excess := make(map[string]bool, len(a))
	for k := range a {
		excess[k] = true
	}
	for k := range b {
		delete(excess, k)
	}
	return excess
}

// applyTemplateTaints merges actual taints with desired template.
// Taints whose keys are in lastApplied but not in template are removed (excess).
// Returns new taints slice and whether any change occurred.
func applyTemplateTaints(actual, template, lastApplied []corev1.Taint) ([]corev1.Taint, bool) {
	if template == nil && lastApplied == nil {
		return []corev1.Taint{}, true
	}

	changed := false
	excess := excessTaintKeys(lastApplied, template)

	newTaints := make(map[string]corev1.Taint, len(actual))
	for _, taint := range actual {
		if _, ok := excess[taint.Key]; ok {
			changed = true
			continue
		}
		newTaints[taint.Key] = taint
	}

	for _, taint := range template {
		oldTaint, ok := newTaints[taint.Key]
		if !ok || !taintEqual(oldTaint, taint) {
			changed = true
		}
		newTaints[taint.Key] = taint
	}

	return taintMapToSortedSlice(newTaints), changed
}

// excessTaintKeys returns taint keys present in a but absent from b.
func excessTaintKeys(a, b []corev1.Taint) map[string]struct{} {
	bKeys := make(map[string]struct{}, len(b))
	for _, t := range b {
		bKeys[t.Key] = struct{}{}
	}

	excess := make(map[string]struct{})
	for _, t := range a {
		if _, ok := bKeys[t.Key]; !ok {
			excess[t.Key] = struct{}{}
		}
	}
	return excess
}

// taintEqual compares two taints by key, value, and effect.
func taintEqual(a, b corev1.Taint) bool {
	return a.Key == b.Key && a.Value == b.Value && a.Effect == b.Effect
}

// taintMapToSortedSlice converts a taint map to a deterministically sorted slice.
func taintMapToSortedSlice(m map[string]corev1.Taint) []corev1.Taint {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]corev1.Taint, 0, len(m))
	for _, k := range keys {
		result = append(result, m[k])
	}
	return result
}

// taintsHasKey checks if any taint in the slice has the given key.
func taintsHasKey(taints []corev1.Taint, key string) bool {
	for _, t := range taints {
		if t.Key == key {
			return true
		}
	}
	return false
}

// taintsWithoutKey returns a new slice without the taint that has the given key.
func taintsWithoutKey(taints []corev1.Taint, key string) []corev1.Taint {
	result := make([]corev1.Taint, 0, len(taints))
	for _, t := range taints {
		if t.Key == key {
			continue
		}
		result = append(result, t)
	}
	return result
}
