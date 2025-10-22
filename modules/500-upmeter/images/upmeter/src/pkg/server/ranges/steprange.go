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
	return fmt.Sprintf("StepRange{From: %d, To: %d, Step: %d, Subranges: (%v)}", r.From, r.To, r.Step, r.Subranges)
}

type Range struct {
	To   int64 // in seconds
	From int64 // in seconds
}

func (r Range) Dur() time.Duration {
	return time.Duration(r.To-r.From) * time.Second
}

func New(start, end time.Time, step time.Duration, includeCurrent bool) StepRange {
	startAligned := start.Truncate(step)
	endAligned := end.Truncate(step)
	if includeCurrent && endAligned != end {
		endAligned = endAligned.Add(step)
	}

	return NewStepRange(
		int64(startAligned.Unix()),
		int64(endAligned.Unix()),
		int64(step.Seconds()),
	)
}

func AlignStep(step, align time.Duration) time.Duration {
	if step < align {
		// minimal step
		return align
	}
	// reduce the step to make it aligned
	return step - step%align
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

// NewStepRange aligns range borders and calculates subranges.
func NewStepRange(fromSeconds, toSeconds, stepSecods int64) StepRange {
	toSeconds = alignEdgeForward(toSeconds, stepSecods)
	count := (toSeconds - fromSeconds) / stepSecods
	fromSeconds = toSeconds - stepSecods*count // align 'from'

	res := StepRange{
		From:      fromSeconds,
		To:        toSeconds,
		Step:      stepSecods,
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

// alignEdgeForward makes sure the edge is a multiple of the step, rounded to bigger side.
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
