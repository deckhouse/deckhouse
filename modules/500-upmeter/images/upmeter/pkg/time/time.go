/*
Copyright 2021 Flant CJSC

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
	shortest := time.Duration(math.MaxInt64)

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
