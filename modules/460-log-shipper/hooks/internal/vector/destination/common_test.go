/*
Copyright 2023 Flant JSC

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

package destination

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_CommonSettings_Validate(t *testing.T) {

	tests := []struct {
		name       string
		testCase   *CommonSettings
		shouldFail bool
		errString  string
	}{
		{
			name:       "Normal settings disk",
			shouldFail: false,
			errString:  "",
			testCase: &CommonSettings{
				Name: "test1",
				Buffer: &Buffer{
					Type:     "disk",
					MaxBytes: 268435489,
					WhenFull: "block",
				},
			},
		},
		{
			name:       "Normal settings memory",
			shouldFail: false,
			errString:  "",
			testCase: &CommonSettings{
				Name: "test2",
				Buffer: &Buffer{
					Type:      "memory",
					MaxEvents: 15,
					WhenFull:  "block",
				},
			},
		},
		{
			name:       "Type disk and max_events",
			shouldFail: true,
			errString:  "can't set max_events when buffer type is 'disk'",
			testCase: &CommonSettings{
				Name: "test3",
				Buffer: &Buffer{
					Type:      "disk",
					MaxEvents: 10,
					WhenFull:  "block",
				},
			},
		},
		{
			name:       "Type memory and max_bytes",
			shouldFail: true,
			errString:  "can't set max_bytes when buffer type is 'memory'",
			testCase: &CommonSettings{
				Name: "test4",
				Buffer: &Buffer{
					Type:      "memory",
					MaxBytes:  10,
					MaxEvents: 10,
					WhenFull:  "block",
				},
			},
		},
		{
			name:       "no when_full",
			shouldFail: true,
			errString:  "'when_full' field can't be with value ''",
			testCase: &CommonSettings{
				Name: "test5",
				Buffer: &Buffer{
					Type:      "memory",
					MaxEvents: 10,
				},
			},
		},
		{
			name:       "no type",
			shouldFail: true,
			errString:  "'type' field can't be with value ''",
			testCase: &CommonSettings{
				Name: "test6",
				Buffer: &Buffer{
					MaxEvents: 10,
					MaxBytes:  123123,
					WhenFull:  "block",
				},
			},
		},
		{
			name:       "bad type",
			shouldFail: true,
			errString:  "'type' field can't be with value 'test-bad'",
			testCase: &CommonSettings{
				Name: "test7",
				Buffer: &Buffer{
					Type:      "test-bad",
					MaxEvents: 10,
					MaxBytes:  123123,
					WhenFull:  "test-bad",
				},
			},
		},
		{
			name:       "bad when full",
			shouldFail: true,
			errString:  "'when_full' field can't be with value 'test-bad'",
			testCase: &CommonSettings{
				Name: "test7",
				Buffer: &Buffer{
					Type:      "disk",
					MaxEvents: 10,
					MaxBytes:  123123,
					WhenFull:  "test-bad",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.testCase.Validate()
			if !tc.shouldFail {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.errString)
		})
	}
}
