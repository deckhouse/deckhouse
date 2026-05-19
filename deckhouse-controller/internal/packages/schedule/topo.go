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
	"fmt"
	"slices"
	"strings"
)

// CycleError reports a topological cycle in the dependency graph. Members are
// the participating node names, sorted alphabetically for deterministic output.
type CycleError struct {
	Members []string
}

// Error renders the cycle members in a single line suitable for K8s admission
// rejections and operator-facing logs.
func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle through: %s", strings.Join(e.Members, ", "))
}

// topoSort returns nodes in topological order respecting dependency edges,
// with Order as the primary tiebreaker and name as the secondary tiebreaker
// for nodes at the same topological level.
//
// Predecessor edges come from n.dependencies (Mandatory + Conditional via
// dependencyNames), NOT from n.followees — followees carry subscription edges
// only and may be cyclic, which is incompatible with topo sort.
//
// On cycle, returns the partial sort plus a *CycleError naming the
// participants. Callers (CheckConstraints) use this to reject configurations
// that introduce cycles before they hit the live scheduler graph. compute()
// falls back gracefully if a cycle ever slips through, relying on its
// disabled-mark-active walk over s.nodes to unblock higher-tier packages.
func topoSort(nodes map[string]*node) ([]*node, error) {
	if len(nodes) == 0 {
		return nil, nil
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

	result := make([]*node, 0, len(nodes))
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

	// Any node still carrying positive in-degree participates in a cycle.
	if len(result) < len(nodes) {
		members := make([]string, 0, len(nodes)-len(result))
		for name, deg := range inDegree {
			if deg > 0 {
				members = append(members, name)
			}
		}

		slices.Sort(members)

		return result, &CycleError{Members: members}
	}

	return result, nil
}

// dependencyNames returns the deduped union of names this node depends on for
// topological ordering: Mandatory + Conditional only.
//
// AnyOf candidates are deliberately excluded. AnyOf is a soft cardinality
// constraint ("≥1 of these satisfies"), so authors legitimately write mutual
// anyOf references as fallback chains — those would be reported as cycles by
// topoSort and silently dropped. The anyOf checker resolves group satisfaction
// against the dep getter directly; topological precedence buys nothing for the
// "≥1 satisfies" predicate beyond cascade speed, and a one-pass cascade lag is
// acceptable for AnyOf (next pass converges).
func dependencyNames(n *node) map[string]struct{} {
	out := make(map[string]struct{}, len(n.dependencies.Mandatory)+len(n.dependencies.Conditional))
	for name := range n.dependencies.Mandatory {
		out[name] = struct{}{}
	}

	for name := range n.dependencies.Conditional {
		out[name] = struct{}{}
	}

	return out
}
