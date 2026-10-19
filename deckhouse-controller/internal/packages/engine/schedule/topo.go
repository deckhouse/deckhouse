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

// CycleError is a dependency cycle; Members are the participants, sorted.
type CycleError struct {
	Members []string
}

// Error renders the cycle in one line, for admission rejections and logs.
func (e *CycleError) Error() string {
	return fmt.Sprintf("dependency cycle through: %s", strings.Join(e.Members, ", "))
}

// topoSort returns nodes in topological order; among the nodes ready at a time, lower Order and
// then name come first. Hard edges are n.dependencies; soft edges are n.anyOf, kept only while
// they do not close a cycle: when only soft edges hold the rest back, the lowest node ready by its
// hard edges is released. On a hard cycle it returns the partial order and a *CycleError.
func topoSort(nodes map[string]*node) ([]*node, error) {
	if len(nodes) == 0 {
		return nil, nil
	}

	// Reverse edges: whose in-degree to decrement when a node is placed.
	dependents := make(map[string][]string, len(nodes))
	softDependents := make(map[string][]string, len(nodes))
	inDegree := make(map[string]int, len(nodes))
	softDegree := make(map[string]int, len(nodes))
	for name, n := range nodes {
		deg := 0
		for dep := range n.dependencies {
			if _, ok := nodes[dep]; ok {
				deg++
				dependents[dep] = append(dependents[dep], name)
			}
		}
		inDegree[name] = deg

		soft := 0
		for member := range n.anyOf {
			if _, hard := n.dependencies[member]; hard || member == name {
				continue
			}

			if _, ok := nodes[member]; ok {
				soft++
				softDependents[member] = append(softDependents[member], name)
			}
		}
		softDegree[name] = soft
	}

	var ready []*node
	for name, deg := range inDegree {
		if deg == 0 && softDegree[name] == 0 {
			ready = append(ready, nodes[name])
		}
	}

	placed := make(map[string]struct{}, len(nodes))
	result := make([]*node, 0, len(nodes))
	for len(result) < len(nodes) {
		if len(ready) == 0 {
			released := releaseSoft(nodes, inDegree, placed)
			if released == nil {
				break
			}

			softDegree[released.name] = 0
			ready = append(ready, released)
		}

		slices.SortFunc(ready, compareNodes)

		n := ready[0]
		ready = ready[1:]
		result = append(result, n)
		placed[n.name] = struct{}{}

		for _, dependentName := range dependents[n.name] {
			inDegree[dependentName]--
			if inDegree[dependentName] == 0 && softDegree[dependentName] == 0 {
				if dn, ok := nodes[dependentName]; ok {
					ready = append(ready, dn)
				}
			}
		}

		// A released node already has softDegree 0, so it is never queued twice.
		for _, dependentName := range softDependents[n.name] {
			if softDegree[dependentName] == 0 {
				continue
			}

			softDegree[dependentName]--
			if inDegree[dependentName] == 0 && softDegree[dependentName] == 0 {
				ready = append(ready, nodes[dependentName])
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

// releaseSoft picks the lowest unplaced node with no hard predecessor left, dropping its soft
// edges; nil means a hard cycle.
func releaseSoft(nodes map[string]*node, inDegree map[string]int, placed map[string]struct{}) *node {
	var released *node
	for name, deg := range inDegree {
		if _, ok := placed[name]; ok || deg > 0 {
			continue
		}

		if released == nil || compareNodes(nodes[name], released) < 0 {
			released = nodes[name]
		}
	}

	return released
}

func compareNodes(a, b *node) int {
	if c := cmp.Compare(a.order, b.order); c != 0 {
		return c
	}

	return cmp.Compare(a.name, b.name)
}
