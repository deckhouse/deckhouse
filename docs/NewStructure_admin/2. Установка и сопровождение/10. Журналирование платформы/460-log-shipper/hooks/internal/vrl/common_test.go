/*
Copyright 2022 Flant JSC

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

package vrl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCombine(t *testing.T) {
	tests := []struct {
		Name  string
		Rule1 Rule
		Rule2 Rule
		Res   string
	}{
		{
			Name:  "Normal",
			Rule1: Rule("abc"),
			Rule2: Rule("def"),
			Res:   "abc\n\ndef",
		},
		{
			Name:  "Multiline",
			Rule1: Rule("a\nbc"),
			Rule2: Rule("def\n"),
			Res:   "a\nbc\n\ndef",
		},
		{
			Name:  "Following tabs/spaces",
			Rule1: Rule("    \tabc\t"),
			Rule2: Rule(" def\t   \n \t"),
			Res:   "abc\n\ndef",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			require.Equal(t, Combine(tc.Rule1, tc.Rule2).String(), tc.Res)
		})
	}
}
