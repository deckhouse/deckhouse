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

func TestStorageClassConversions(t *testing.T) {
	conversions := "."
	cases := []struct {
		name            string
		settings        string
		expected        string
		currentVersion  int
		expectedVersion int
	}{
		{
			name: "should move `global.storageClass` to `global.modules.storageClass`",
			settings: `
  storageClass: some-storage-class
  modules:
    ingressClass: nginx
`,
			expected: `
  modules:
    ingressClass: nginx
    storageClass: some-storage-class
`,
			currentVersion:  1,
			expectedVersion: 2,
		},

		{
			name: "do not override `global.modules.storageClass` if it already exists",
			settings: `
  storageClass: some-storage-class
  modules:
    ingressClass: nginx
    storageClass: do-not-override-storage-class
`,
			expected: `
  modules:
    ingressClass: nginx
    storageClass: do-not-override-storage-class
`,
			currentVersion:  1,
			expectedVersion: 2,
		},
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
