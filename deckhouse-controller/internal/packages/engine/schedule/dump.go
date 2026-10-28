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
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/engine/schedule/rule"
)

// dump is the serialization envelope for the scheduler endpoint.
type dump struct {
	Paused bool                `json:"paused" yaml:"paused"`
	Nodes  map[string]nodeDump `json:"nodes" yaml:"nodes"`
}

// nodeDump is one node: what was registered, its facts and the last computation.
type nodeDump struct {
	Version        string                `json:"version" yaml:"version"`
	Module         bool                  `json:"module" yaml:"module"`
	Order          Order                 `json:"order" yaml:"order"`
	EffectiveOrder Order                 `json:"effectiveOrder" yaml:"effectiveOrder"`
	Decision       rule.Decision         `json:"decision" yaml:"decision"`
	Dependencies   map[string]Dependency `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Subscriptions  []string              `json:"subscriptions,omitempty" yaml:"subscriptions,omitempty"`
	Published      bool                  `json:"published" yaml:"published"`
	OwnInputs      string                `json:"ownInputs,omitempty" yaml:"ownInputs,omitempty"`
	Applied        string                `json:"applied,omitempty" yaml:"applied,omitempty"`
	Output         string                `json:"output,omitempty" yaml:"output,omitempty"`
	Inputs         string                `json:"inputs" yaml:"inputs"`
	Converged      bool                  `json:"converged" yaml:"converged"`
	Gate           bool                  `json:"gate" yaml:"gate"`
}

func sortedKeys(set map[string]struct{}) []string {
	return slices.Sorted(maps.Keys(set))
}

// dumpNode: s.mu held.
func dumpNode(n *node) nodeDump {
	return nodeDump{
		Version:        n.revision,
		Module:         n.module,
		Order:          n.order,
		EffectiveOrder: n.effectiveOrder,
		Decision:       n.decision,
		Dependencies:   maps.Clone(n.dependencies),
		Subscriptions:  sortedKeys(n.subscriptions),
		Published:      n.facts.Version != "",
		OwnInputs:      n.facts.OwnInputs,
		Applied:        n.facts.Applied,
		Output:         n.facts.Output,
		Inputs:         n.inputs,
		Converged:      n.converged,
		Gate:           n.gate,
	}
}

// Dump returns a snapshot of all nodes.
func (s *Scheduler) Dump() any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := &dump{
		Paused: s.paused,
		Nodes:  make(map[string]nodeDump, len(s.nodes)),
	}

	for _, n := range s.nodes {
		snapshot.Nodes[n.name] = dumpNode(n)
	}

	return snapshot
}

// DumpByName returns a snapshot of one node; nil when it is unknown.
func (s *Scheduler) DumpByName(name string) any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n, ok := s.nodes[name]
	if !ok {
		return nil
	}

	return dumpNode(n)
}

// Unconverged is what Converged waits for in the scheduler.
type Unconverged struct {
	Paused bool                       // Paused: paused or stopped, nothing is computed.
	Nodes  map[string]UnconvergedNode // Nodes are the unconverged global and module nodes; empty while Paused.
}

// UnconvergedNode is one unconverged node; WaitsFor is empty while its gate is open.
type UnconvergedNode struct {
	Revision string // Revision is the node's revision, the one its Current runs.
	Order    Order  // Order is the effective order the tier rule gates by.
	// WaitsFor is global, the nodes of the lowest unconverged tier, or a dependency or AnyOf member.
	WaitsFor string
}

// Unconverged returns the global and module nodes Converged waits for; applications never hold it.
func (s *Scheduler) Unconverged() Unconverged {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := Unconverged{Paused: s.paused || s.stopped, Nodes: make(map[string]UnconvergedNode)}
	if res.Paused {
		return res
	}

	unconverged := s.lowestUnconverged()
	front := s.tierFront(unconverged)

	for _, n := range s.nodes {
		if (n.module || n.name == GlobalNode) && !n.converged {
			waitsFor := s.blockerOf(n, unconverged)
			// The tier rule held it (blockerOf checks it right after the barrier): name the nodes
			// that must converge first instead of the tier.
			if waitsFor != GlobalNode && unconverged < n.effectiveOrder {
				waitsFor = front
			}

			res.Nodes[n.name] = UnconvergedNode{Revision: n.revision, Order: n.effectiveOrder, WaitsFor: waitsFor}
		}
	}

	return res
}

// frontLimit bounds the names tierFront lists; the rest are counted.
const frontLimit = 3

// tierFront names the unconverged nodes of the lowest unconverged tier, sorted: what every node of a
// higher tier waits for now.
func (s *Scheduler) tierFront(unconverged Order) string {
	var names []string

	for _, n := range s.nodes {
		if !n.converged && n.effectiveOrder == unconverged {
			names = append(names, n.name)
		}
	}

	slices.Sort(names)

	if len(names) > frontLimit {
		return fmt.Sprintf("%s +%d", strings.Join(names[:frontLimit], ", "), len(names)-frontLimit)
	}

	return strings.Join(names, ", ")
}
