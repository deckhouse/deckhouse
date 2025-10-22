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

package conversions

import (
	"testing"

	"github.com/deckhouse/deckhouse/go_lib/configtools/conversion"
)

type testCase struct {
	name            string
	settings        string
	expected        string
	currentVersion  int
	expectedVersion int
}

func v2testCase() testCase {
	return testCase{
		name: "should convert from 1 to 2 version",
		settings: `
auth:
  status:
    password: password
    allowedUserGroups:
      - group
      - group2
    whitelistSourceRange:
      - source1
  webui:
    password: password
    allowedUserGroups:
      - group
      - group2
`,
		expected: `
auth:
  status:
    allowedUserGroups:
      - group
      - group2
    whitelistSourceRange:
      - source1
  webui:
    allowedUserGroups:
      - group
      - group2
`,
		currentVersion:  1,
		expectedVersion: 2,
	}
}

func v3testCase() testCase {
	return testCase{
		name: "should convert from 2 to 3 version",
		settings: `
smokeMini:
  auth:
  ingressClass: nginx
`,
		expected: `
smokeMini:
  auth:
`,
		currentVersion:  2,
		expectedVersion: 3,
	}
}

func TestUpmeterConversions(t *testing.T) {
	conversions := "."
	cases := []testCase{
		v2testCase(),
		v3testCase(),
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := conversion.TestConvert(c.settings, c.expected, conversions, c.currentVersion, c.expectedVersion)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
