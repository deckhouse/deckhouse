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

package taints

import (
	v1 "k8s.io/api/core/v1"
)

type Slice []v1.Taint

func (s Slice) Array() []v1.Taint {
	return s
}

func (s Slice) HasKey(key string) bool {
	for _, t := range s {
		if t.Key == key {
			return true
		}
	}
	return false
}

func (s Slice) WithoutKey(key string) Slice {
	res := make(Slice, 0)
	for _, t := range s {
		if t.Key == key {
			continue
		}
		res = append(res, t)
	}
	return res
}

// Merge returns new merged slice.
func (s Slice) Merge(in []v1.Taint) Slice {
	resMap := make(Map)
	for _, t := range s {
		resMap[t.ToString()] = t
	}

	for _, t := range in {
		resMap[t.ToString()] = t
	}

	// Sort keys and return taints as an array.
	return resMap.Slice()
}

// Equal returns true if all taints in slice are equal to taints in "in" slice.
func (s Slice) Equal(in []v1.Taint) bool {
	aIndex := make(map[string]struct{})
	for _, t := range s {
		aIndex[t.ToString()] = struct{}{}
	}

	bIndex := make(map[string]struct{})
	for _, t := range in {
		bIndex[t.ToString()] = struct{}{}
	}

	if len(aIndex) != len(bIndex) {
		return false
	}

	for k := range aIndex {
		if _, ok := bIndex[k]; !ok {
			return false
		}
	}

	return true
}

// ApplyTemplate use "template" slice to add new taints and update existin.
// lastApplied slice is used to delete excess taints.
func (s Slice) ApplyTemplate(template []v1.Taint, lastApplied []v1.Taint) (Slice, bool) {
	if template == nil && lastApplied == nil {
		return Slice{}, true
	}

	changed := false
	excess := Slice(lastApplied).ExcessKeys(template)

	newTaints := make(Map)
	for _, taint := range s {
		// Ignore keys removed from template.
		if _, ok := excess[taint.Key]; ok {
			changed = true
			continue
		}
		newTaints[taint.Key] = taint
	}

	for _, taint := range template {
		// Check if taint on node is different from taint in template.
		oldTaint, ok := newTaints[taint.Key]
		if !ok || oldTaint.ToString() != taint.ToString() {
			changed = true
		}
		newTaints[taint.Key] = taint
	}

	// Sort keys and return taints as an array.
	return newTaints.Slice(), changed
}

// ExcessKeys returns taint keys without equal keys from "in" taints.
func (s Slice) ExcessKeys(in []v1.Taint) map[string]struct{} {
	bIdx := make(map[string]struct{})
	for _, taint := range in {
		bIdx[taint.Key] = struct{}{}
	}

	res := make(map[string]struct{})
	for _, taint := range s {
		if _, ok := bIdx[taint.Key]; ok {
			continue
		}
		res[taint.Key] = struct{}{}
	}
	return res
}
