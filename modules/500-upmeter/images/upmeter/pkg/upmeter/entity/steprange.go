package entity

import "upmeter/pkg/upmeter/db/dao"

// CalculateAdjustedStepRanges adjust from, to and step and calculates
// intermediate step ranges.
func CalculateAdjustedStepRanges(from, to, step int64) dao.StepRanges {
	count := (to - from) / step
	step = AdjustStep(step)
	to = AdjustTo(to, step)
	from = to - step*count
	res := dao.StepRanges{
		From:   from,
		To:     to,
		Step:   step,
		Ranges: make([][]int64, 0),
	}

	// return one point
	if res.From == res.To || res.To == 0 {
		res.Ranges = append(res.Ranges, []int64{res.From, res.From + res.Step})
		return res
	}

	// from is already adjusted to nearest multiple of step
	stepStart := res.From
	for {
		stepEnd := stepStart + res.Step
		if stepEnd >= res.To {
			res.Ranges = append(res.Ranges, []int64{stepStart, res.To})
			break
		}
		res.Ranges = append(res.Ranges, []int64{stepStart, stepEnd})
		// go to next step
		stepStart = stepEnd
	}
	return res
}
