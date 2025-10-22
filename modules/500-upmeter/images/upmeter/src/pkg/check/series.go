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

package check

import (
	"fmt"
)

var ErrIndexTooBig = fmt.Errorf("index is too big")

// StatusSeries contains the list of statuses of a check for current episode (30s).
// The series slice is to be merged with other checks using the series of the statuses.
type StatusSeries struct {
	nextIndex int
	series    []Status
}

func NewStatusSeries(size int) *StatusSeries {
	return &StatusSeries{
		series: make([]Status, size),
	}
}

// Add adds a status to the series at specified position
func (ss *StatusSeries) AddI(i int, status Status) error {
	if len(ss.series) <= i {
		return ErrIndexTooBig
	}
	ss.series[i] = status
	ss.nextIndex = i + i
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
	ss.nextIndex = 0
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

// MergeStatusSeries merges status series according to the merge strategy: Down is preferred to Up,
// Up is preferred to Unknown, anything is preferred to nodata. If no status series are passed to
// merge, empty status series is returned. No data measn no data.
func MergeStatusSeries(size int, sss []*StatusSeries) (*StatusSeries, error) {
	acc := NewStatusSeries(size)

	for _, ss := range sss {
		err := acc.Merge(ss)
		if err != nil {
			return nil, fmt.Errorf("merging status series: %v", err)
		}
	}

	return acc, nil
}
