// Copyright 2021 Flant CJSC
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

package config

import (
	"testing"
)

func TestSchemaStore(t *testing.T) {
	newStore := newSchemaStore("/tmp")

	err := newStore.upload([]byte(`
kind: TestKind
apiVersions:
- apiVersion: test
  openAPISpec:
    type: object
    additionalProperties: false
    required: [kind, apiVersion, one, two]
    properties:
      kind:
        type: string
      apiVersion:
        type: string
      one:
        type: string
      two:
        type: string
`))
	if err != nil {
		t.Errorf("uploading error : %v", err)
	}

	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			"Valid config",
			`
apiVersion: test
kind: TestKind
one: test
two: test
`,
			false,
		},
		{
			"Without version",
			`
kind: TestKind
one: "1"
two: "2"
`,
			true,
		},
		{
			"Without kind",
			`
apiVersion: test
one: "1"
two: "2"
`,
			true,
		},
		{
			"Wrong spec",
			`
apiVersion: test
kind: TestKind
one: "1"
`,
			true,
		},
	}

	for _, tc := range tests {
		content := []byte(tc.content)

		_, err := newStore.Validate(&content)
		if err != nil && !tc.wantErr {
			t.Errorf("%s: %v", tc.name, err)
		}

		if err == nil && tc.wantErr {
			t.Errorf("%s: expected error, didn't get one", tc.name)
		}
	}
}
