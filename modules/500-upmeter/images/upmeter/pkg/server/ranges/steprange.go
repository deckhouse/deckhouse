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
	"time"
)

type StepRange struct {
	From      int64
	To        int64
	Step      int64
	Subranges []Range
}

type Range struct {
	From int64
	To   int64
}

func (r Range) Diff() time.Duration {
	return time.Duration(r.To-r.From) * time.Second
}

// NewStepRange aligns range borders and calculates subranges.
func NewStepRange(from, to, step int64) StepRange {
	count := (to - from) / step
	step = alignStep(step)
	to = alignEdge(to, step)
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

// alignStep makes sure that the step is a multiple of 300.
func alignStep(step int64) int64 {
	var (
		minStep     = int64(300)
		alignedStep = step - step%minStep
	)
	return maxInt64(minStep, alignedStep)
}

// alignStep makes sure the
func alignEdge(to, step int64) int64 {
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
