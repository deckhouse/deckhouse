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

package schedule

import (
	"encoding/json"
	"maps"
	"slices"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
)

// dump is the JSON-serializable snapshot of a single node, used by Dump.
type dump struct {
	Status    checker.Result `json:"status"`
	Name      string         `json:"name"`
	Version   string         `json:"version"`
	Order     Order          `json:"order"`
	State     nodeState      `json:"state"`
	Followees []string       `json:"followees,omitempty"`
	Followers []string       `json:"followers,omitempty"`
}

// Dump returns a JSON snapshot of all nodes in topological order.
func (s *Scheduler) Dump() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sorted := topoSort(s.nodes)

	res := make([]dump, len(sorted))
	for _, n := range sorted {
		res = append(res, dump{
			Name:      n.name,
			Version:   n.version.String(),
			Order:     n.order,
			State:     n.state,
			Status:    n.status,
			Followees: slices.Collect(maps.Keys(n.followees)),
			Followers: slices.Collect(maps.Keys(n.followers)),
		})
	}

	marshalled, _ := json.MarshalIndent(res, "", "  ")
	return marshalled
}
