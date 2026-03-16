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
// Nodes involved in cycles are silently omitted from the result.
func topoSort(nodes map[string]*node) []*node {
	if len(nodes) == 0 {
		return nil
	}

	// Compute in-degree: count of followees that actually exist in nodes.
	inDegree := make(map[string]int, len(nodes))
	for name, n := range nodes {
		deg := 0
		for dep := range n.followees {
			if _, ok := nodes[dep]; ok {
				deg++
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

		// Decrement in-degree for all followers.
		for followerName := range n.followers {
			inDegree[followerName]--
			if inDegree[followerName] == 0 {
				if fn, ok := nodes[followerName]; ok {
					ready = append(ready, fn)
				}
			}
		}
	}

	return result
}
