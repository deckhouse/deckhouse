package check

import (
	"fmt"
)

var (
	ErrLimitReached = fmt.Errorf("limit reached")
)

type StatusSeries struct {
	index  int
	series []Status
}

func NewStatusSeries(size int) *StatusSeries {
	return &StatusSeries{
		series: make([]Status, size),
	}
}

// Add adds a status to the series
func (ss *StatusSeries) Add(status Status) error {
	if len(ss.series) == ss.index {
		return ErrLimitReached
	}
	ss.series[ss.index] = status
	ss.index++
	return nil
}

// Merge updates current series with the source series according to the merging strategy: least non-zero status rule,
// i.e.: Down < Up < Unknown.
func (ss *StatusSeries) Merge(src *StatusSeries) error {
	if ss.size() != src.size() {
		return fmt.Errorf("the capacity of status series must be equal, got %d and %d", ss.size(), src.size())
	}

	for i := 0; i < ss.size(); i++ {
		ss.series[i] = mergeStrategy(ss.series[i], src.series[i])
	}

	return nil
}

func (ss *StatusSeries) Stats() Stats {
	stats := Stats{
		Expected: ss.size(),
	}

	for _, status := range ss.series {
		switch status {
		case Up:
			stats.Up++
		case Down:
			stats.Down++
		case Unknown:
			stats.Unknown++
		}
	}

	return stats
}

func (ss *StatusSeries) Clean() {
	ss.index = 0
	ss.series = make([]Status, len(ss.series))
}

func (ss *StatusSeries) size() int {
	return len(ss.series)
}

// mergeStrategy prefers new information when it is more valuable:
// Down more than Up, Up more than Unknown, anything more that nodata which is just a zero.
func mergeStrategy(dst, src Status) Status {
	if src == nodata {
		return dst
	}
	if dst == nodata {
		return src
	}
	if dst > src {
		return src
	}
	return dst
}
