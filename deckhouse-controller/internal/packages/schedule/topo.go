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
	"cmp"
	"slices"
)

// topoSort returns nodes in topological order respecting dependency edges,
// with Order as the primary tiebreaker and name as the secondary tiebreaker
// for nodes at the same topological level.
//
// Predecessor edges come from n.dependencies (Mandatory + Conditional + AnyOf
// members), NOT from n.followees — followees carry subscription edges only
// (used by Trigger) and may be cyclic, which is incompatible with topo sort.
//
// Nodes involved in cycles in the dep graph are silently omitted from the
// result; their callers (compute) will not visit them this pass.
func topoSort(nodes map[string]*node) []*node {
	if len(nodes) == 0 {
		return nil
	}

	// Build the reverse dep map locally so we know whose in-degree to decrement
	// when a node is processed. Walking n.dependencies directly avoids relying
	// on the (subscription-only) followees set.
	dependents := make(map[string][]string, len(nodes))
	inDegree := make(map[string]int, len(nodes))
	for name, n := range nodes {
		deg := 0
		for dep := range dependencyNames(n) {
			if _, ok := nodes[dep]; ok {
				deg++
				dependents[dep] = append(dependents[dep], name)
			}
		}
		inDegree[name] = deg
	}

	// Collect initial zero-in-degree nodes.
	var ready []*node
	for name, deg := range inDegree {
		if deg == 0 {
			ready = append(ready, nodes[name])
		}
	}

	var result []*node
	for len(ready) > 0 {
		// Sort ready nodes: Order ASC, then name ASC for determinism.
		slices.SortFunc(ready, func(a, b *node) int {
			if c := cmp.Compare(a.order, b.order); c != 0 {
				return c
			}
			return cmp.Compare(a.name, b.name)
		})

		// Take the highest-priority node.
		n := ready[0]
		ready = ready[1:]
		result = append(result, n)

		// Decrement in-degree for everyone that depends on n.
		for _, dependentName := range dependents[n.name] {
			inDegree[dependentName]--
			if inDegree[dependentName] == 0 {
				if dn, ok := nodes[dependentName]; ok {
					ready = append(ready, dn)
				}
			}
		}
	}

	return result
}

// dependencyNames returns the deduped union of module names this node depends
// on across the three buckets (Mandatory, Conditional, AnyOf members). Used as
// topological predecessors.
func dependencyNames(n *node) map[string]struct{} {
	out := make(map[string]struct{}, len(n.dependencies.Mandatory)+len(n.dependencies.Conditional))
	for name := range n.dependencies.Mandatory {
		out[name] = struct{}{}
	}

	for name := range n.dependencies.Conditional {
		out[name] = struct{}{}
	}

	for _, g := range n.dependencies.AnyOf {
		for name := range g.Modules {
			out[name] = struct{}{}
		}
	}

	return out
}
