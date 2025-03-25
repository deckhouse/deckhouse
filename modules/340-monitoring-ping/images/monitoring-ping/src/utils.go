// Package ping Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"math"
	"slices"
)

func Summarize(rtts []float64) (min, max, mean, std, sum float64) {
	n := float64(len(rtts))
	if n == 0 {
		return 0, 0, 0, 0, 0
	}

	min = slices.Min(rtts)
	max = slices.Max(rtts)

	for _, v := range rtts {
		sum += v
	}
	mean = sum / n

	var variance float64
	for _, v := range rtts {
		d := v - mean
		variance += d * d
	}
	std = math.Sqrt(variance / n)

	// Sanity check
	if math.IsNaN(std) {
		std = 0
	}
	return
}

func GetTargetName(name, address string) string {
	if name == "" {
		return address
	}
	return name
}
