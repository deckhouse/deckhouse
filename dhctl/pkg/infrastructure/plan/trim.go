// Copyright 2026 Flant JSC
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

package plan

import "encoding/json"

// Trim returns a copy of the plan keeping only what describes the change
// itself: resource_changes cut down to type, name, actions and a few
// identity fields of before/after, and prior_state cut down to each
// resource's name, type and tags.Name. Everything else a raw OpenTofu plan
// carries (format_version, terraform_version, variables, planned_values,
// output_changes, configuration, resource_changes' "after_unknown", and the
// rest of prior_state/before/after) is static context rather than a change,
// and is dropped.
func (p Plan) Trim() Plan {
	trimmed := Plan{}

	if resourceChanges, ok := trimResourceChanges(p["resource_changes"]); ok {
		trimmed["resource_changes"] = resourceChanges
	}

	if priorState, ok := trimPriorState(p["prior_state"]); ok {
		trimmed["prior_state"] = priorState
	}

	return trimmed
}

type trimmedPriorState struct {
	Values struct {
		RootModule struct {
			Resources []trimmedResource `json:"resources"`
		} `json:"root_module"`
	} `json:"values"`
}

type trimmedResource struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Values struct {
		Tags struct {
			Name string `json:"Name,omitempty"`
		} `json:"tags"`
	} `json:"values"`
}

func trimPriorState(raw any) (trimmedPriorState, bool) {
	if raw == nil {
		return trimmedPriorState{}, false
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return trimmedPriorState{}, false
	}

	var trimmed trimmedPriorState

	// json.Unmarshal is best-effort: a type mismatch on one field (e.g. a
	// provider whose "tags" is a list, not a map) only zeroes that field,
	// not the rest of the resource or array. Ignoring the error is deliberate.
	_ = json.Unmarshal(data, &trimmed)

	return trimmed, true
}

// trimmedResourceChange keeps only what identifies a resource and describes
// its change: type, name, actions, and a few identity fields of before and
// after. Their full attributes and after_unknown duplicate most of a
// resource's own state for every unchanged (no-op) resource too, making
// them the largest part of a plan on a big cluster — dropped entirely
// rather than trimmed further.
type trimmedResourceChange struct {
	Type   string          `json:"type"`
	Name   string          `json:"name"`
	Change trimmedChangeOp `json:"change"`
}

type trimmedChangeOp struct {
	Actions []string             `json:"actions"`
	Before  *trimmedChangeValues `json:"before,omitempty"`
	After   *trimmedChangeValues `json:"after,omitempty"`
}

type trimmedChangeValues struct {
	Name     string                `json:"name,omitempty"`
	Manifest *trimmedManifest      `json:"manifest,omitempty"`
	Metadata []trimmedMetadataName `json:"metadata,omitempty"`
}

type trimmedManifest struct {
	Kind     string               `json:"kind,omitempty"`
	Metadata *trimmedMetadataName `json:"metadata,omitempty"`
}

type trimmedMetadataName struct {
	Name string `json:"name,omitempty"`
}

func trimResourceChanges(raw any) ([]trimmedResourceChange, bool) {
	if raw == nil {
		return nil, false
	}

	data, err := json.Marshal(raw)
	if err != nil {
		return nil, false
	}

	var trimmed []trimmedResourceChange

	// Same best-effort decode as trimPriorState: a shape mismatch on one
	// resource_change's "before" (provider-specific attributes) only zeroes
	// that one field of that one element, not the whole array.
	_ = json.Unmarshal(data, &trimmed)

	return trimmed, true
}
