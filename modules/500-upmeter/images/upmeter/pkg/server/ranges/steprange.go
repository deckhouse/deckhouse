package ranges

import (
	"time"

	"d8.io/upmeter/pkg/util"
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

// NewStepRange adjust from, to and step and calculates
// intermediate step ranges.
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
		// go to next step
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
	return util.Max(minStep, alignedStep)
}

// alignStep makes sure the
func alignEdge(to, step int64) int64 {
	if to%step == 0 {
		return to
	}
	return to - to%step + step
}
