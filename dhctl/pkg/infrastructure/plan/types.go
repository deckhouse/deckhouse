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

package plan

type Action string

const (
	ActionCreate = Action("create")
	ActionDelete = Action("delete")
)

const (
	HasNoChanges = iota
	HasChanges
	HasDestructiveChanges
)

type VMChangeTester func(change ResourceChange) bool

type Plan map[string]any

type DestructiveChanges struct {
	ResourcesDeleted   []ValueChange `json:"resources_deleted,omitempty"`
	ResourcesRecreated []ValueChange `json:"resourced_recreated,omitempty"`
	Provider           string        `json:"provider,omitempty"`
}

type ValueChange struct {
	CurrentValue interface{} `json:"current_value,omitempty"`
	NextValue    interface{} `json:"next_value,omitempty"`
	Type         string      `json:"type,omitempty"`
}

type InfrastructurePlan struct {
	ResourceChanges []ResourceChange `json:"resource_changes"`
}

type ResourceChange struct {
	Change       ChangeOp `json:"change"`
	Type         string   `json:"type"`
	Name         string   `json:"name"`
	ProviderName string   `json:"provider_name,omitempty"`
}

func (r *ResourceChange) HasAction(findAction Action) bool {
	act := string(findAction)
	for _, action := range r.Change.Actions {
		if action == act {
			return true
		}
	}
	return false
}

type ChangeOp struct {
	Actions []string               `json:"actions"`
	Before  map[string]interface{} `json:"before,omitempty"`
	After   map[string]interface{} `json:"after,omitempty"`
}
