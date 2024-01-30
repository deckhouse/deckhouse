// Copyright 2021 Flant JSC
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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionBackwardCompatibility(t *testing.T) {
	newStore := newSchemaStore([]string{"/tmp"}, LoadOptions{})

	schema := []byte(`
kind: ClusterConfiguration
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
    additionalProperties: false
    required: [apiVersion, kind, clusterType]
    properties:
      apiVersion:
        type: string
        enum: [deckhouse.io/v1, deckhouse.io/v1alpha1]
      kind:
        type: string
        enum: [ClusterConfiguration]
      clusterType:
        type: string
        enum: [Cloud, Static]
`)

	err := newStore.upload(schema)
	require.NoError(t, err)

	oldDoc := []byte(`
apiVersion: deckhouse.io/v1alpha1
kind: ClusterConfiguration
clusterType: Cloud
`)
	newDoc := []byte(`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Cloud
`)
	_, err = newStore.Validate(&oldDoc)
	assert.NoError(t, err)
	_, err = newStore.Validate(&newDoc)
	assert.NoError(t, err)
}

func TestSchemaPattern(t *testing.T) {
	newStore := newSchemaStore([]string{"/tmp"}, LoadOptions{})

	schema := []byte(`
kind: ClusterConfiguration
apiVersions:
- apiVersion: deckhouse.io/v1
  openAPISpec:
    type: object
    additionalProperties: false
    required: [kind, apiVersion, jsonObject]
    properties:
      kind:
        type: string
      apiVersion:
        type: string
      jsonObject:
        type: string
        pattern: '^[ \t]*\{.*\}[ \t]*$'
`)

	err := newStore.upload(schema)
	require.NoError(t, err)

	errorDoc := []byte(`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
jsonObject: "error"
`)
	_, err = newStore.Validate(&errorDoc)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), `Document validation failed:
---

apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
jsonObject: "error"


1 error occurred:
	* jsonObject should match '^[ \t]*\{.*\}[ \t]*$'

`)

	okDoc := []byte(`
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
jsonObject: " {}"
`)

	_, err = newStore.Validate(&okDoc)
	assert.NoError(t, err)
}

func TestSchemaStore(t *testing.T) {
	newStore := newSchemaStore([]string{"/tmp"}, LoadOptions{})

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
