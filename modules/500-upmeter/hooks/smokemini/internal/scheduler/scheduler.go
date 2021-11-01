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
	"fmt"

	"github.com/deckhouse/deckhouse/modules/500-upmeter/hooks/smokemini/internal/snapshot"
)

func New(indexSelector IndexSelector, nodeFilter NodeFilter, cleaner Cleaner, image, storageClass string) *Scheduler {
	return &Scheduler{
		indexSelector: indexSelector,
		nodeFilter:    nodeFilter,
		cleaner:       cleaner,

		image:        image,
		storageClass: storageClass,
	}
}

type Scheduler struct {
	indexSelector IndexSelector
	nodeFilter    NodeFilter
	cleaner       Cleaner
	image         string
	storageClass  string
}

func (s *Scheduler) Schedule(state State, nodes []snapshot.Node) (string, *XState, error) {
	// Select sts
	x, err := s.indexSelector.Select(state)
	if err != nil {
		return "", nil, fmt.Errorf("%w: no smoke-mini StatefulSet chosen", err)
	}

	// Find suitable nodes
	bestNodes := s.nodeFilter.Filter(nodes, x)
	if len(bestNodes) == 0 {
		return x, nil, fmt.Errorf("%w: no suitable node found", ErrSkip)
	}

	node := bestNodes[0]
	newSts := &XState{
		Node:         node.Name,
		Zone:         node.Zone,
		Image:        s.image,
		StorageClass: s.storageClass,
	}

	s.cleaner.Clean(x, state[x], newSts)
	return x, newSts, nil
}
