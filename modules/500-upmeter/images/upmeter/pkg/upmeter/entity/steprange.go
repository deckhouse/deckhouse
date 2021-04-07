package entity

import (
	"upmeter/pkg/check"
	"upmeter/pkg/util"
)

// CalculateAdjustedStepRanges adjust from, to and step and calculates
// intermediate step ranges.
func CalculateAdjustedStepRanges(from, to, step int64) check.StepRanges {
	count := (to - from) / step
	step = alignStep(step)
	to = alignEdge(to, step)
	from = to - step*count

	res := check.StepRanges{
		From:   from,
		To:     to,
		Step:   step,
		Ranges: make([]check.Range, 0),
	}

	// return one point
	if res.From == res.To || res.To == 0 {
		res.Ranges = append(res.Ranges, check.Range{From: res.From, To: res.From + res.Step})
		return res
	}

	// from is already adjusted to nearest multiple of step
	stepStart := res.From
	for {
		stepEnd := stepStart + res.Step
		if stepEnd >= res.To {
			res.Ranges = append(res.Ranges, check.Range{From: stepStart, To: res.To})
			break
		}
		res.Ranges = append(res.Ranges, check.Range{From: stepStart, To: stepEnd})
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
