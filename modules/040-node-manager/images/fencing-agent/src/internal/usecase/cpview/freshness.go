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

package cpview

import "fencing-agent/internal/domain"

// Fresh is the result of the rule of freshness for one SEE answer.
type Fresh int

const (
	// FreshWait: no result yet.
	FreshWait Fresh = iota
	// FreshWitness: two probes after the answer failed. The answer is a witness.
	FreshWitness
	// FreshCanceled: a probe after the answer succeeded. The answer is not a witness.
	FreshCanceled
)

// Freshness applies the rule of freshness to one SEE answer that is fresher
// by more than witnessFresherBy.
// The number of a probe cycle is the time of its start by CLOCK_BOOTTIME in
// milliseconds, for the proxy and for the own probes. Only the cycles that
// started after the answer count.
type Freshness struct {
	// last is the number of the last cycle it took, at first the time of the answer.
	last   uint64
	fails  int
	result Fresh
}

// NewFreshness starts the rule. at is the time of the answer by BootMillis.
func NewFreshness(at uint64) *Freshness {
	return &Freshness{last: at}
}

// BootMillis returns CLOCK_BOOTTIME in milliseconds: the clock of the probe
// cycle numbers.
func BootMillis() uint64 {
	return bootMillis()
}

// Observe takes the next control plane state. After FreshWitness or
// FreshCanceled the result does not change.
func (f *Freshness) Observe(st domain.CPState) Fresh {
	if f.result != FreshWait || st.ProbeSeq <= f.last {
		return f.result
	}

	f.last = st.ProbeSeq

	if st.ProbeOK {
		f.result = FreshCanceled

		return f.result
	}

	f.fails++
	if f.fails >= 2 {
		f.result = FreshWitness
	}

	return f.result
}
