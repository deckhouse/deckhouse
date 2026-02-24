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
	"maps"
	"slices"

	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/schedule/checker"
)

// dump is the serialization envelope for the debug endpoint.
type dump struct {
	Nodes map[string]nodeDump `json:"nodes" yaml:"nodes"`
}

// nodeDump combines status info for a single node.
type nodeDump struct {
	Name      string         `json:"name" yaml:"name"`
	Version   string         `json:"version" yaml:"version"`
	Order     Order          `json:"order" yaml:"order"`
	State     nodeState      `json:"state" yaml:"state"`
	Status    checker.Result `json:"status" yaml:"status"`
	Followees []string       `json:"followees,omitempty" yaml:"followees,omitempty"`
	Followers []string       `json:"followers,omitempty" yaml:"followers,omitempty"`
}

// Dump returns a YAML snapshot of all nodes and their current state.
func (s *Scheduler) Dump() []byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	d := &dump{
		Nodes: make(map[string]nodeDump, len(s.nodes)),
	}

	for _, n := range s.nodes {
		d.Nodes[n.name] = nodeDump{
			Name:      n.name,
			Version:   n.version.String(),
			Order:     n.order,
			State:     n.state,
			Status:    n.status,
			Followees: slices.Collect(maps.Keys(n.followees)),
			Followers: slices.Collect(maps.Keys(n.followers)),
		}
	}

	marshalled, _ := yaml.Marshal(d)
	return marshalled
}
