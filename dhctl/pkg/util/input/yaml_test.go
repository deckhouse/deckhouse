// Copyright 2024 Flant JSC
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

package input_test

import (
	"testing"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/input"
	"github.com/stretchr/testify/assert"
)

func TestCombineYAMLs(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in  []string
		out string
	}{
		"combine": {
			in: []string{
				`
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
enum: [option1, option2]
---
`,
				`
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
description: |
  some text

  some new text
---
`,
				`


---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
spec:
  a: a
  b: b
---
`,
				``,
			},
			out: `apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
enum: [option1, option2]
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
description: |
  some text

  some new text
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
spec:
  a: a
  b: b
`,
		},
		"with ssh key": {
			in: []string{
				`---

---
---
key: |
  -----BEGIN RSA PRIVATE KEY-----
  MIIEpAIBAAKCAQEAvxymRHZIsjIXvxM7X/S8th4CH+3HgWa19HTPG8tOuAvEfBIt
  -----END RSA PRIVATE KEY-----
---
---
`,
				`---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
---
`,
			},
			out: `key: |
  -----BEGIN RSA PRIVATE KEY-----
  MIIEpAIBAAKCAQEAvxymRHZIsjIXvxM7X/S8th4CH+3HgWa19HTPG8tOuAvEfBIt
  -----END RSA PRIVATE KEY-----
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
`,
		},
		"empty with --- 1": {
			in: []string{`
---
---
---

---
---
---
---

---


---

`,
				``,
				``,
				`---
---
---`,
			},
			out: ``,
		},
		"empty with --- 2": {
			in: []string{`
---
---
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
---

---
---
---




---

`},
			out: `apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
`,
		},
		"adds \n": {
			in: []string{`apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig`},
			out: `apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
`},
	}

	for name, tt := range tests {
		tt := tt
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out := input.CombineYAMLs(tt.in...)
			assert.Equal(t, tt.out, out)
		})
	}
}
