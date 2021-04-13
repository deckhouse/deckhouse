package time

import (
	"math"
	"time"
)

// Longest returns the longest duration among arguments
func Longest(ds ...time.Duration) time.Duration {
	var longest time.Duration

	for _, d := range ds {
		if d > longest {
			longest = d
		}
	}

	return longest
}

// Shortest returns the Shortest duration among arguments
func Shortest(ds ...time.Duration) time.Duration {
	var shortest = time.Duration(math.MaxInt64)

	for _, d := range ds {
		if d < shortest {
			shortest = d
		}
	}

	return shortest
}

func ClampToRange(value, from, to time.Duration) time.Duration {
	if value < from {
		return from
	}
	if value > to {
		return to
	}
	return value
}
