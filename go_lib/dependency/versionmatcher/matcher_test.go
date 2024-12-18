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

package versionmatcher

import (
	"errors"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	baseVersion string
	name        string
	constraint  string
	error       error
}

func TestExtender(t *testing.T) {
	var testCases = []testCase{
		{
			baseVersion: "v0.0.0",
			name:        "test1",
			constraint:  "< v1.60.4",
			error:       nil,
		},
		{
			baseVersion: "v1.60.5",
			name:        "test2",
			constraint:  "< v1.60.4",
			error:       errors.New("1.60.5 is greater than or equal to v1.60.4"),
		},
		{
			baseVersion: "v1.60.5",
			name:        "test2",
			constraint:  "= v1.60.5",
			error:       nil,
		},
	}
	for _, tc := range testCases {
		test(t, tc)
	}
}

func test(t *testing.T, tc testCase) {
	currentVersion, err := semver.NewVersion(tc.baseVersion)
	require.NoError(t, err)
	matcher := New(false)
	matcher.ChangeBaseVersion(currentVersion)
	err = matcher.AddConstraint(tc.name, tc.constraint)
	require.NoError(t, err)
	if err = matcher.Validate(tc.name); err != nil {
		if tc.error == nil {
			require.NoError(t, err)
		} else {
			require.Equal(t, tc.error.Error(), err.Error())
		}
	}
}
