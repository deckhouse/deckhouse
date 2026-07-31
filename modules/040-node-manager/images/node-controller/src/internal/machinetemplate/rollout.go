/*
Copyright 2026 Flant JSC

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

package machinetemplate

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Change is one difference between the InstanceClass a generation was built from and the one in
// the cluster now. It is what the operator reads in the NodeGroup event explaining a rollout —
// the question "why did my machines just roll?" had no answer at all under the checksum naming.
type Change struct {
	Path string
	Old  any
	New  any
}

func (c Change) String() string {
	return fmt.Sprintf("%s %s → %s", c.Path, formatValue(c.Old), formatValue(c.New))
}

// maxValueLen keeps one field from filling the event message. Events are for orientation; the
// full snapshot stays on the object.
const maxValueLen = 64

func formatValue(v any) string {
	if v == nil {
		return "<none>"
	}
	// The value already survived a json.Marshal in fieldValue, so this one cannot fail.
	data, _ := json.Marshal(v)
	s := string(data)
	if len(s) > maxValueLen {
		return s[:maxValueLen] + "…"
	}
	return s
}

// Changes reports which of the contract's rolloutFields differ between two InstanceClass specs.
//
// Comparison is by value, after both sides are normalized through JSON: 50 read from the API as
// int64 and 50 read back from the snapshot annotation as float64 are the same number. That is the
// whole point of v2 — the v1 name was a hash over the bytes of a YAML serialization, so a change
// in how a number is printed renamed every template in every cluster and rolled every machine.
//
// The fields are applied here, at comparison time, and are not stored in the snapshot: a provider
// release that adds or removes a rolloutField therefore changes only the criterion, never the
// recorded facts, and so cannot roll machines by itself.
func Changes(oldSpec, newSpec map[string]any, fields []string) ([]Change, error) {
	changes := make([]Change, 0)
	for _, field := range slices.Sorted(slices.Values(fields)) {
		oldValue, err := fieldValue(oldSpec, field)
		if err != nil {
			return nil, fmt.Errorf("read %s of the stored InstanceClass: %w", field, err)
		}
		newValue, err := fieldValue(newSpec, field)
		if err != nil {
			return nil, fmt.Errorf("read %s of the current InstanceClass: %w", field, err)
		}
		if reflect.DeepEqual(oldValue, newValue) {
			continue
		}
		changes = append(changes, Change{Path: field, Old: oldValue, New: newValue})
	}
	return changes, nil
}

// fieldValue reads one rolloutField out of a spec and normalizes it: an absent field and an
// explicit null both come back as nil (the provider CRDs default them the same way, and the v1
// checksums treated them alike), and a number is whatever json.Unmarshal would make of it, so that
// int64 from the API and float64 from the snapshot annotation compare equal.
//
// Only the field is normalized, not the whole spec: this runs for every zone of every reconcile,
// and the specs are much bigger than the handful of values being compared.
func fieldValue(spec map[string]any, field string) (any, error) {
	value, found, err := unstructured.NestedFieldNoCopy(spec, strings.Split(field, ".")...)
	if err != nil || !found || value == nil {
		// err means the path runs through a non-map, which is the same "the field is not there"
		// answer as !found — the provider named a path its CRD does not have.
		return nil, nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal for comparison: %w", err)
	}
	var normalized any
	if err := json.Unmarshal(data, &normalized); err != nil {
		return nil, fmt.Errorf("unmarshal for comparison: %w", err)
	}
	return normalized, nil
}

// FormatChanges renders the changes for a NodeGroup event.
func FormatChanges(changes []Change) string {
	parts := make([]string, 0, len(changes))
	for _, change := range changes {
		parts = append(parts, change.String())
	}
	return strings.Join(parts, ", ")
}
