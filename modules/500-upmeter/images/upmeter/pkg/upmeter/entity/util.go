package entity

import "math"

func ClampToRange(value int64, from int64, to int64) int64 {
	if value < from {
		return from
	}
	if value > to {
		return to
	}
	return value
}

func Min(nums ...int64) int64 {
	if len(nums) == 0 {
		return 0
	}
	var res int64 = math.MaxInt64
	for _, n := range nums {
		if n < res {
			res = n
		}
	}
	return res
}

func Max(nums ...int64) int64 {
	if len(nums) == 0 {
		return 0
	}
	var res int64 = math.MinInt64
	for _, n := range nums {
		if n > res {
			res = n
		}
	}
	return res
}

// AdjustStep makes sure that the step is a multiple of 300.
func AdjustStep(step int64) int64 {
	if step <= 300 {
		return 300
	}
	if step%300 == 0 {
		return step
	}
	return (step / 300) * 300
}

func AdjustFrom(from int64, step int64) int64 {
	return (from / step) * step
	//return (from / 300) * 300
}

func AdjustTo(to int64, step int64) int64 {
	if to%step == 0 {
		return to
	}
	return ((to / step) + 1) * step
}

// Get5MinSlot returns 5 min slot for a timestamp.
// 5 min slot is a nearest future timestamp that is a multiple of 5 min.
// For example:
// - 5 min slot for 600 is 600
// - 5 min slot for 601 is 600
// - 5 min slot for 899 is 800
func Get5MinSlot(ts int64) int64 {
	if ts%300 == 0 {
		return ts
	}
	return ts / 300 * 300
}
