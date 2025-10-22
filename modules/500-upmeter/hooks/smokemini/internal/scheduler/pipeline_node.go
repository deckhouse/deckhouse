/*
Copyright 2021 Flant JSC

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

package scheduler

import (
	"math/rand"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

// NewNodeSelector crates the node fuiltering pipeline. The resulting node list is the result of all
// filters in the sequence from top to bottom.
func NewNodeSelector(state State) NodeFilterPipe {
	return NodeFilterPipe{
		&filterByAvailability{},
		&filterByZone{state: state},
		&filterByMinSts{state: state},
		&nodeShuffler{},
	}
}

type NodeFilter interface {
	Filter([]snapshot.Node, string) []snapshot.Node
}

// NodeFilterPipe is the sequential wrapper for other node filters. A result of each filter is
// passed to the next one.  The filters do not share knowledge about each other. Filters are
// responsible to handle empty input their own way.
type NodeFilterPipe []NodeFilter

func (pipe NodeFilterPipe) Filter(nodes []snapshot.Node, x string) []snapshot.Node {
	for _, filter := range pipe {
		nodes = filter.Filter(nodes, x)
	}
	return nodes
}

// applyFilter implements the boilerplate of filtering a Node slice
func applyFilter(nodes []snapshot.Node, filter func(snapshot.Node) bool) []snapshot.Node {
	filtered := make([]snapshot.Node, 0)
	for _, node := range nodes {
		if !filter(node) {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered
}

// nodeShuffler shuffles the list of nodes, and does not filter any of
type nodeShuffler struct{}

func (f nodeShuffler) Filter(nodes []snapshot.Node, _ string) []snapshot.Node {
	rand.Shuffle(len(nodes), func(i, j int) { nodes[i], nodes[j] = nodes[j], nodes[i] })
	return nodes
}

// filterByAvailability filters nodes that are available for scheduling
type filterByAvailability struct{}

func (f *filterByAvailability) Filter(nodes []snapshot.Node, _ string) []snapshot.Node {
	return applyFilter(nodes, func(node snapshot.Node) bool {
		return node.Schedulable
	})
}
