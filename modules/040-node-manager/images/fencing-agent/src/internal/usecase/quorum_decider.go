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
