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

package deckhouseversion

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	currentVersion string
	moduleName     string
	constraint     string
	passedExists   bool
	passed         bool
	addingError    bool
	error          error
}

func TestExtender(t *testing.T) {
	var testCases = []testCase{
		{
			currentVersion: "v0.0.0",
			moduleName:     "test1",
			constraint:     "< v1.60.4",
			passedExists:   true,
			passed:         true,
			addingError:    false,
			error:          nil,
		},
		{
			currentVersion: "v1.60.5",
			moduleName:     "test2",
			constraint:     "< v1.60.4",
			passedExists:   true,
			passed:         false,
			addingError:    false,
			error:          nil,
		},
		{
			currentVersion: "v1.60.5",
			moduleName:     "test2",
			constraint:     "= v1.60.5",
			passedExists:   true,
			passed:         true,
			addingError:    false,
			error:          nil,
		},
	}
	for _, tc := range testCases {
		test(t, tc)
	}
}

func test(t *testing.T, tc testCase) {
	currentVersion, err := semver.NewVersion(tc.currentVersion)
	require.NoError(t, err)
	extender := Extender{currentVersion: currentVersion, modulesConstraints: make(map[string]*semver.Constraints)}
	err = extender.AddConstraint(tc.moduleName, tc.constraint)
	if tc.addingError {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
	}
	passed, err := extender.Filter(tc.moduleName, nil)
	if tc.error != nil {
		require.ErrorAs(t, err, tc.error)
	}
	if tc.passedExists {
		require.NotNil(t, passed)
		if passed != nil {
			if tc.passed {
				require.True(t, *passed)
			} else {
				require.False(t, *passed)
			}
		}
	} else {
		require.Nil(t, passed)
	}
}
