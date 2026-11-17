/*
Copyright 2026 Flant JSC

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

func TestDeckhouseConversions(t *testing.T) {
	cases := []struct {
		name     string
		settings string
		expected string
	}{
		{
			name: "drops enableModuleV2, now global.moduleV2Enabled",
			settings: `
logLevel: Info
enableModuleV2: true
`,
			expected: `
logLevel: Info
`,
		},
		{
			name: "leaves settings without enableModuleV2 as they are",
			settings: `
logLevel: Info
`,
			expected: `
logLevel: Info
`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := conversion.TestConvert(c.settings, c.expected, ".", 1, 2); err != nil {
				t.Error(err)
			}
		})
	}
}
