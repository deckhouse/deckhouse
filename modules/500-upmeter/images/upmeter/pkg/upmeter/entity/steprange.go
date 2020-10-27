package entity

type StepRanges struct {
	From   int64
	To     int64
	Step   int64
	Ranges [][]int64 // array of 2 items arrays: 0 is a step range start, 1 is a step range end.
}

// CalculateAdjustedStepRanges adjust from, to and step and calculates
// intermediate step ranges.
func CalculateAdjustedStepRanges(from, to, step int64) StepRanges {
	count := (to - from) / step
	step = AdjustStep(step)
	to = AdjustTo(to, step)
	from = to - step*count
	res := StepRanges{
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
