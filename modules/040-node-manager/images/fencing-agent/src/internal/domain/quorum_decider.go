package domain

import (
	"sync"
	"time"
)

type NodesNumber struct {
	TotalNodes int   `json:"total_nodes"`
	Timestamp  int64 `json:"timestamp"`
}
type QuorumDecider struct {
	NodesNumber NodesNumber
	mtx         sync.RWMutex
}

func NewQuorumDecider(totalNodes int) *QuorumDecider {
	return &QuorumDecider{
		NodesNumber: NodesNumber{
			TotalNodes: totalNodes,
			Timestamp:  time.Now().UnixMilli(),
		},
	}
}

func (qd *QuorumDecider) ShouldFeed(numMembers int) bool {
	qd.mtx.RLock()
	defer qd.mtx.RUnlock()

	quorum := qd.NodesNumber.TotalNodes/2 + 1
	return numMembers >= quorum
}

func (qd *QuorumDecider) SetTotalNodes(nodesNumber NodesNumber) {
	qd.mtx.Lock()
	defer qd.mtx.Unlock()

	if qd.NodesNumber.Timestamp > nodesNumber.Timestamp {
		return
	}

	qd.NodesNumber = nodesNumber
}
