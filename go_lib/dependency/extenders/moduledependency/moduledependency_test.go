/*
Copyright 2024 Flant JSC

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

package moduledependency

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type testCase struct {
	name        string
	constraints map[string]string
	formsLoop   bool
}

func TestExtender(t *testing.T) {
	var testCases = []testCase{
		{
			name:        "d",
			constraints: map[string]string{"b": "", "c": ""},
			formsLoop:   false,
		},
		{
			name:        "e",
			constraints: map[string]string{"b": "", "a:": ""},
			formsLoop:   true,
		},
	}

	e := Instance()
	err := e.AddConstraint("a", map[string]string{})
	require.NoError(t, err)
	err = e.AddConstraint("b", map[string]string{"a": "> v0.0.0", "c": "> v0.0.0"})
	require.NoError(t, err)
	err = e.AddConstraint("c", map[string]string{"a": "> v0.0.0", "e": "> v0.0.0"})
	require.NoError(t, err)

	for _, tc := range testCases {
		test(t, e, tc)
	}
}

func test(t *testing.T, extender *Extender, tc testCase) {
	loop, _ := extender.constraintFormsLoop(tc.name, tc.constraints)
	require.Equal(t, tc.formsLoop, loop)
}
