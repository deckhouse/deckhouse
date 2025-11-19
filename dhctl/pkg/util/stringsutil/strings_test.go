// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stringsutil

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// This test will fail at the original implementation of RandomStrElement with the message such as:
// "strings_test.go:56: RandomStrElement produces 100 consecutive repetitions out of 100 elements"
func TestRandomStrElement(t *testing.T) {
	const selectionSize = 100
	const stringsSize = 10000
	const maximumConsecutiveRepetitionsThreshold = 4 // allow for no more than 4+1 consecutive elements (can very rarely happen due to the nature of random numbers)
	strings := make([]string, 0, stringsSize)
	for i := 0; i < stringsSize; i++ {
		strings = append(strings, strconv.Itoa(i)) // this will yield a different value for each string in the array
	}
	// as the string size is very big, consecutive repetitions should be extremely rare
	previousValue := ""
	maximumConsecutiveRepetitions := 0 // repetition is how many elements repeat the previous: one repetition means there are two consecutive elements
	consecutiveRepetitions := 0
	reconsiderMaximum := func() {
		if consecutiveRepetitions > maximumConsecutiveRepetitions {
			maximumConsecutiveRepetitions = consecutiveRepetitions
		}
	}
	for i := 0; i < selectionSize; i++ {
		v, index := RandomStrElement(strings)
		require.Equal(t, strings[index], v, "index does not agree with the value after RandomStrElement")
		if v == previousValue {
			consecutiveRepetitions++
		} else {
			reconsiderMaximum()
			previousValue = v
			consecutiveRepetitions = 0
		}
	}
	reconsiderMaximum()
	if maximumConsecutiveRepetitions > maximumConsecutiveRepetitionsThreshold {
		t.Errorf("RandomStrElement produces %d consecutive repetitions out of %d elements", maximumConsecutiveRepetitions, selectionSize)
	}
}

func TestTrimLeftChars(t *testing.T) {
	type tst struct {
		name      string
		input     string
		trimCount int
		expected  string
	}

	tests := []tst{
		{name: "empty string", input: "", expected: "", trimCount: 2},
		{name: "zero trim", input: "not empty", expected: "not empty", trimCount: 0},
		{name: "one symbol ASCII", input: "E", expected: "", trimCount: 1},
		{name: "one symbol UTF", input: "Ъ", expected: "", trimCount: 1},
		{name: "multiple symbols UTF", input: "Ъъъъ", expected: "ъъ", trimCount: 2},
		{name: "multiple symbols ASCII", input: "E mpty", expected: "mpty", trimCount: 2},
		{name: "multiple symbols ASCII one trim", input: "Empty", expected: "mpty", trimCount: 1},
		{name: "multiple symbols UTF one trim", input: "Ъъъъ", expected: "ъъъ", trimCount: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := TrimLeftChars(test.input, test.trimCount)
			require.Equal(t, test.expected, e)
		})
	}
}
