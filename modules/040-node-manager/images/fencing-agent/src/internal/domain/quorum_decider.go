package domain

import "sync/atomic"

type QuorumDecider struct {
	totalNodes atomic.Int64
}

func NewQuorumDecider(totalNodes int) *QuorumDecider {
	qd := &QuorumDecider{}
	qd.totalNodes.Store(int64(totalNodes))

	return qd
}

func (qd *QuorumDecider) ShouldFeed(numMembers int) bool {
	quorum := qd.totalNodes.Load()/2 + 1
	return numMembers >= int(quorum)
}
