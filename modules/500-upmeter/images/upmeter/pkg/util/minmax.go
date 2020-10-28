package util

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
