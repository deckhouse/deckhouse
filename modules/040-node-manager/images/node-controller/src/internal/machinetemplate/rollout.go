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
	"sort"
	"strings"
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
	data, err := json.Marshal(v)
	if err != nil {
		return "<unprintable>"
	}
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
	oldProjection, err := project(oldSpec, fields)
	if err != nil {
		return nil, fmt.Errorf("project stored InstanceClass: %w", err)
	}
	newProjection, err := project(newSpec, fields)
	if err != nil {
		return nil, fmt.Errorf("project current InstanceClass: %w", err)
	}

	changes := make([]Change, 0)
	for _, field := range fields {
		oldValue := oldProjection[field]
		newValue := newProjection[field]
		if reflect.DeepEqual(oldValue, newValue) {
			continue
		}
		changes = append(changes, Change{Path: field, Old: oldValue, New: newValue})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, nil
}

// project extracts the rolloutFields from a spec into a flat path→value map. A field that is
// absent is left out of the map, which makes "absent" and "explicit null" compare equal — the
// provider CRDs default them the same way, and the v1 checksums treated them alike too.
func project(spec map[string]any, fields []string) (map[string]any, error) {
	normalized, err := normalize(spec)
	if err != nil {
		return nil, err
	}
	root, _ := normalized.(map[string]any)

	out := make(map[string]any, len(fields))
	for _, field := range fields {
		value, found := lookupPath(root, field)
		if !found || value == nil {
			continue
		}
		out[field] = value
	}
	return out, nil
}

func lookupPath(root map[string]any, path string) (any, bool) {
	current := any(root)
	for _, segment := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// normalize converts any nested value into what json.Unmarshal would produce, so that values
// coming from the API (int64 inside unstructured) and values coming back from the snapshot
// annotation (float64 inside JSON) compare equal.
func normalize(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal for comparison: %w", err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal for comparison: %w", err)
	}
	return out, nil
}

// FormatChanges renders the changes for a NodeGroup event.
func FormatChanges(changes []Change) string {
	parts := make([]string, 0, len(changes))
	for _, change := range changes {
		parts = append(parts, change.String())
	}
	return strings.Join(parts, ", ")
}
