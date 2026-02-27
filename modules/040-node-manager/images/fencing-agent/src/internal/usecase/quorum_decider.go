/*
Copyright 2026 Flant JSC

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

package usecase

import (
	"fencing-agent/internal/domain"
	"sync"
	"time"
)

type QuorumDecider struct {
	nodesNumber domain.NodeGroupState
	mtx         sync.RWMutex
}

func NewQuorumDecider(totalNodes int) *QuorumDecider {
	return &QuorumDecider{
		nodesNumber: domain.NodeGroupState{
			TotalNodes: totalNodes,
			Timestamp:  time.Now().UnixMilli(),
		},
	}
}

func (qd *QuorumDecider) ShouldFeed(numMembers int) bool {
	qd.mtx.RLock()
	defer qd.mtx.RUnlock()

	quorum := qd.nodesNumber.TotalNodes/2 + 1
	return numMembers >= quorum
}

func (qd *QuorumDecider) SetTotalNodes(nodesNumber domain.NodeGroupState) {
	qd.mtx.Lock()
	defer qd.mtx.Unlock()

	if qd.nodesNumber.Timestamp > nodesNumber.Timestamp {
		return
	}

	qd.nodesNumber = nodesNumber
}
