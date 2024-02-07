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
	"github.com/deckhouse/deckhouse/go_lib/set"
	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

type selectByNode struct {
	nodes []snapshot.Node
}

func (s *selectByNode) Select(state State) (string, error) {
	// Not deployed sts
	for x, sts := range state {
		if sts.Node == "" {
			return x, nil
		}
	}

	// Collect nodes
	allNodes := set.New()
	unschedNodes := set.New()
	for _, node := range s.nodes {
		allNodes.Add(node.Name)
		if !node.Schedulable {
			unschedNodes.Add(node.Name)
		}
	}

	// Find sts placed on non-existent node
	for x, sts := range state {
		if !allNodes.Has(sts.Node) {
			return x, nil
		}
	}

	// Find sts placed on unavailable node
	for x, sts := range state {
		if unschedNodes.Has(sts.Node) {
			return x, nil
		}
	}

	return "", errNext
}
