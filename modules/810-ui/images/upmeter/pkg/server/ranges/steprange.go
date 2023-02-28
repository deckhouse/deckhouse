/*
Copyright 2023 Flant JSC

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

package ranges

import (
	"fmt"
	"time"
)

type StepRange struct {
	From      int64
	To        int64
	Step      int64
	Subranges []Range
}

func (r StepRange) String() string {
	return fmt.Sprintf("StepRange{From: %d, To: %d, Step: %d, Subranges: (N=%d)}", r.From, r.To, r.Step, len(r.Subranges))
}

type Range struct {
	From int64
	To   int64
}

func (r Range) Dur() time.Duration {
	return time.Duration(r.To-r.From) * time.Second
}

// New5MinStepRange returns SteRange aligned to 5 minute step.
func New5MinStepRange(from, to, step int64) StepRange {
	step = alignStep(step, 300)
	return NewStepRange(from, to, step)
}

// New30SecStepRange returns SteRange aligned to 30 seconds step.
func New30SecStepRange(from, to, step int64) StepRange {
	step = alignStep(step, 30)
	return NewStepRange(from, to, step)
}

// New5MinStepRange aligns range borders and calculates subranges.
func NewStepRange(from, to, step int64) StepRange {
	to = alignEdgeForward(to, step)
	count := (to - from) / step
	from = to - step*count

	res := StepRange{
		From:      from,
		To:        to,
		Step:      step,
		Subranges: make([]Range, 0),
	}

	// return one point
	if res.From == res.To || res.To == 0 {
		res.Subranges = append(res.Subranges, Range{From: res.From, To: res.From + res.Step})
		return res
	}

	// from is already adjusted to nearest multiple of step
	stepStart := res.From
	for {
		stepEnd := stepStart + res.Step
		if stepEnd >= res.To {
			res.Subranges = append(res.Subranges, Range{From: stepStart, To: res.To})
			break
		}
		res.Subranges = append(res.Subranges, Range{From: stepStart, To: stepEnd})
		stepStart = stepEnd
	}
	return res
}

// alignStep makes sure that the step is a multiple of min.
func alignStep(step, min int64) int64 {
	return maxInt64(min, step-step%min)
}

// alignEdgeForward makes sure the edge is a multiple of step, rounded to bigger side.
func alignEdgeForward(to, step int64) int64 {
	if to%step == 0 {
		return to
	}
	return to - to%step + step
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
