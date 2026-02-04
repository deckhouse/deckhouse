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
	totalNodes          int
	lastUpdateTimestamp int64
	mtx                 sync.RWMutex
}

func NewQuorumDecider(totalNodes int) *QuorumDecider {
	return &QuorumDecider{
		totalNodes:          totalNodes,
		lastUpdateTimestamp: time.Now().UnixMilli(),
	}
}

func (qd *QuorumDecider) ShouldFeed(numMembers int) bool {
	qd.mtx.RLock()
	defer qd.mtx.RUnlock()

	quorum := qd.totalNodes/2 + 1
	return numMembers >= quorum
}

func (qd *QuorumDecider) SetTotalNodes(nodesNumber NodesNumber) {
	qd.mtx.Lock()
	defer qd.mtx.Unlock()
	if qd.lastUpdateTimestamp > nodesNumber.Timestamp {
		return
	}
	qd.lastUpdateTimestamp = nodesNumber.Timestamp
	qd.totalNodes = nodesNumber.TotalNodes
}
