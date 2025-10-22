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
	"math"

	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

// filterByMinSts returns nodes with minimal sts count
type filterByMinSts struct {
	state State
}

func (f *filterByMinSts) Filter(nodes []snapshot.Node, x string) []snapshot.Node {
	destinations := f.selectNodes(nodes, x)
	return applyFilter(nodes, func(node snapshot.Node) bool {
		return destinations.Has(node.Name)
	})
}

func (f *filterByMinSts) selectNodes(nodes []snapshot.Node, x string) set.Set {
	var (
		stsPerNode  = map[string]int{}
		currentNode = f.state[x].Node
	)

	// Collect nodes of interest
	for _, n := range nodes {
		if n.Name == currentNode {
			// Our goal is to probe new node if possible
			continue
		}
		stsPerNode[n.Name] = 0
	}

	if len(stsPerNode) == 0 {
		// No new nodes to consider
		return set.New(currentNode)
	}

	// Count sts, skip current one from consideration
	for _, sts := range f.state {
		if sts.Node == "" || sts.Node == currentNode {
			continue
		}
		stsPerNode[sts.Node]++
	}

	dests := selectKeysByMinValue(stsPerNode)

	return set.New(dests...)
}

func selectKeysByMinValue(kv map[string]int) []string {
	// Find minimum value
	minv := math.MaxInt32
	for _, v := range kv {
		if minv > v {
			minv = v
		}
	}

	// Collect keys
	ks := make([]string, 0)
	for k, v := range kv {
		if v == minv {
			ks = append(ks, k)
		}
	}

	return ks
}
