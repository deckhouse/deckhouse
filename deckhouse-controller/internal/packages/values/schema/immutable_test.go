// Copyright 2026 Flant JSC
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

package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/values/schema/defaults"
)

// settingsSchema loads a settings schema the same way a package's openapi/settings.yaml
// is loaded in production, so the transformers and extension parsing under test match.
func settingsSchema(t *testing.T, raw string) *Storage {
	t.Helper()

	storage, err := NewStorage([]byte(raw), nil)
	require.NoError(t, err)

	return storage
}

// checkWithDefaults mirrors the webhook: schema defaults land on both sides before the
// comparison, so a key left out of the manifest reads as the value already in effect.
func checkWithDefaults(t *testing.T, storage *Storage, oldValues, newValues map[string]any) []error {
	t.Helper()

	s := storage.GetSchema(TypeSettings)
	require.NotNil(t, s)

	defaults.Apply(oldValues, s)
	defaults.Apply(newValues, s)

	return CheckImmutable(s, oldValues, newValues)
}

func TestCheckImmutable(t *testing.T) {
	const flat = `
type: object
properties:
  storageClass:
    type: string
    x-deckhouse-immutable: true
  replicas:
    type: integer
`

	const nested = `
type: object
properties:
  postgres:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-immutable: true
      volumeSize:
        type: string
`

	const markedBlock = `
type: object
properties:
  postgres:
    type: object
    x-deckhouse-immutable: true
    properties:
      storageClass:
        type: string
      volumeSize:
        type: string
`

	const withDefault = `
type: object
properties:
  storageClass:
    type: string
    default: standard
    x-deckhouse-immutable: true
`

	const ignoredMarks = `
type: object
properties:
  disabledMark:
    type: string
    x-deckhouse-immutable: false
  stringMark:
    type: string
    x-deckhouse-immutable: "true"
`

	const mapEntries = `
type: object
properties:
  instances:
    type: object
    additionalProperties:
      type: object
      properties:
        storageClass:
          type: string
          x-deckhouse-immutable: true
`

	const listItems = `
type: object
properties:
  nodes:
    type: array
    items:
      type: object
      properties:
        storageClass:
          type: string
          x-deckhouse-immutable: true
`

	const composed = `
type: object
allOf:
  - properties:
      storageClass:
        type: string
        x-deckhouse-immutable: true
properties:
  replicas:
    type: integer
`

	const patterned = `
type: object
properties:
  instances:
    type: object
    patternProperties:
      "^[a-z]+$":
        type: object
        properties:
          storageClass:
            type: string
            x-deckhouse-immutable: true
`

	const tupleItems = `
type: object
properties:
  pair:
    type: array
    items:
      - type: object
        properties:
          storageClass:
            type: string
            x-deckhouse-immutable: true
      - type: object
        properties:
          storageClass:
            type: string
`

	const recursive = `
type: object
definitions:
  node:
    type: object
    properties:
      storageClass:
        type: string
        x-deckhouse-immutable: true
      child:
        $ref: '#/definitions/node'
properties:
  root:
    $ref: '#/definitions/node'
`

	const negated = `
type: object
not:
  properties:
    storageClass:
      type: string
      x-deckhouse-immutable: true
`

	tests := []struct {
		name       string
		schema     string
		oldValues  map[string]any
		newValues  map[string]any
		wantPaths  []string
		wantAllowd bool
	}{
		{
			name:       "unchanged marked field",
			schema:     flat,
			oldValues:  map[string]any{"storageClass": "fast", "replicas": float64(1)},
			newValues:  map[string]any{"storageClass": "fast", "replicas": float64(3)},
			wantAllowd: true,
		},
		{
			name:      "changed marked field",
			schema:    flat,
			oldValues: map[string]any{"storageClass": "fast"},
			newValues: map[string]any{"storageClass": "slow"},
			wantPaths: []string{"storageClass"},
		},
		{
			name:       "marked field absent on both sides",
			schema:     flat,
			oldValues:  map[string]any{"replicas": float64(1)},
			newValues:  map[string]any{"replicas": float64(2)},
			wantAllowd: true,
		},
		{
			name:       "marked field set for the first time",
			schema:     flat,
			oldValues:  map[string]any{},
			newValues:  map[string]any{"storageClass": "fast"},
			wantAllowd: true,
		},
		{
			name:      "marked field dropped without a default",
			schema:    flat,
			oldValues: map[string]any{"storageClass": "fast"},
			newValues: map[string]any{},
			wantPaths: []string{"storageClass"},
		},
		{
			name:       "marked field dropped back to its default value",
			schema:     withDefault,
			oldValues:  map[string]any{"storageClass": "standard"},
			newValues:  map[string]any{},
			wantAllowd: true,
		},
		{
			name:      "marked field dropped from a non-default value",
			schema:    withDefault,
			oldValues: map[string]any{"storageClass": "fast"},
			newValues: map[string]any{},
			wantPaths: []string{"storageClass"},
		},
		{
			name:      "nested marked field changed",
			schema:    nested,
			oldValues: map[string]any{"postgres": map[string]any{"storageClass": "fast", "volumeSize": "1Gi"}},
			newValues: map[string]any{"postgres": map[string]any{"storageClass": "slow", "volumeSize": "1Gi"}},
			wantPaths: []string{"postgres.storageClass"},
		},
		{
			name:       "sibling of a nested marked field changed",
			schema:     nested,
			oldValues:  map[string]any{"postgres": map[string]any{"storageClass": "fast", "volumeSize": "1Gi"}},
			newValues:  map[string]any{"postgres": map[string]any{"storageClass": "fast", "volumeSize": "2Gi"}},
			wantAllowd: true,
		},
		{
			name:      "mark on a block freezes every field below it",
			schema:    markedBlock,
			oldValues: map[string]any{"postgres": map[string]any{"storageClass": "fast", "volumeSize": "1Gi"}},
			newValues: map[string]any{"postgres": map[string]any{"storageClass": "fast", "volumeSize": "2Gi"}},
			wantPaths: []string{"postgres"},
		},
		{
			name:       "non-true marks are ignored",
			schema:     ignoredMarks,
			oldValues:  map[string]any{"disabledMark": "a", "stringMark": "a"},
			newValues:  map[string]any{"disabledMark": "b", "stringMark": "b"},
			wantAllowd: true,
		},
		{
			name:   "marked field inside an existing map entry changed",
			schema: mapEntries,
			oldValues: map[string]any{"instances": map[string]any{
				"a": map[string]any{"storageClass": "fast"},
			}},
			newValues: map[string]any{"instances": map[string]any{
				"a": map[string]any{"storageClass": "slow"},
			}},
			wantPaths: []string{"instances.a.storageClass"},
		},
		{
			name:   "map entry added by the update is free",
			schema: mapEntries,
			oldValues: map[string]any{"instances": map[string]any{
				"a": map[string]any{"storageClass": "fast"},
			}},
			newValues: map[string]any{"instances": map[string]any{
				"a": map[string]any{"storageClass": "fast"},
				"b": map[string]any{"storageClass": "slow"},
			}},
			wantAllowd: true,
		},
		{
			name:      "marked field inside an existing list element changed",
			schema:    listItems,
			oldValues: map[string]any{"nodes": []any{map[string]any{"storageClass": "fast"}}},
			newValues: map[string]any{"nodes": []any{map[string]any{"storageClass": "slow"}}},
			wantPaths: []string{"nodes[0].storageClass"},
		},
		{
			name:      "list element appended by the update is free",
			schema:    listItems,
			oldValues: map[string]any{"nodes": []any{map[string]any{"storageClass": "fast"}}},
			newValues: map[string]any{"nodes": []any{
				map[string]any{"storageClass": "fast"},
				map[string]any{"storageClass": "slow"},
			}},
			wantAllowd: true,
		},
		{
			name:      "mark declared in an allOf branch is honoured",
			schema:    composed,
			oldValues: map[string]any{"storageClass": "fast"},
			newValues: map[string]any{"storageClass": "slow"},
			wantPaths: []string{"storageClass"},
		},
		{
			name:   "map entry renamed by the update is compared",
			schema: mapEntries,
			oldValues: map[string]any{"instances": map[string]any{
				"a": map[string]any{"storageClass": "fast"},
			}},
			newValues: map[string]any{"instances": map[string]any{
				"b": map[string]any{"storageClass": "slow"},
			}},
			wantPaths: []string{"instances.a.storageClass"},
		},
		{
			name:   "map entry dropped by the update is compared",
			schema: mapEntries,
			oldValues: map[string]any{"instances": map[string]any{
				"a": map[string]any{"storageClass": "fast"},
			}},
			newValues: map[string]any{"instances": map[string]any{}},
			wantPaths: []string{"instances.a.storageClass"},
		},
		{
			name:      "list truncated by the update is compared",
			schema:    listItems,
			oldValues: map[string]any{"nodes": []any{map[string]any{"storageClass": "fast"}}},
			newValues: map[string]any{"nodes": []any{}},
			wantPaths: []string{"nodes[0].storageClass"},
		},
		{
			name:   "marked field under patternProperties changed",
			schema: patterned,
			oldValues: map[string]any{"instances": map[string]any{
				"a": map[string]any{"storageClass": "fast"},
			}},
			newValues: map[string]any{"instances": map[string]any{
				"a": map[string]any{"storageClass": "slow"},
			}},
			wantPaths: []string{"instances.a.storageClass"},
		},
		{
			name:   "marked field in a tuple position changed",
			schema: tupleItems,
			oldValues: map[string]any{"pair": []any{
				map[string]any{"storageClass": "fast"},
				map[string]any{"storageClass": "fast"},
			}},
			newValues: map[string]any{"pair": []any{
				map[string]any{"storageClass": "slow"},
				map[string]any{"storageClass": "slow"},
			}},
			wantPaths: []string{"pair[0].storageClass"},
		},
		{
			name:   "marked field behind a recursive $ref changed",
			schema: recursive,
			oldValues: map[string]any{"root": map[string]any{
				"storageClass": "fast",
				"child":        map[string]any{"storageClass": "fast"},
			}},
			newValues: map[string]any{"root": map[string]any{
				"storageClass": "fast",
				"child":        map[string]any{"storageClass": "slow"},
			}},
			wantPaths: []string{"root.child.storageClass"},
		},
		{
			name:      "mark declared inside not is honoured",
			schema:    negated,
			oldValues: map[string]any{"storageClass": "fast"},
			newValues: map[string]any{"storageClass": "slow"},
			wantPaths: []string{"storageClass"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := checkWithDefaults(t, settingsSchema(t, tt.schema), tt.oldValues, tt.newValues)

			if tt.wantAllowd {
				assert.Empty(t, errs, "expected the update to be allowed")
				return
			}

			require.Len(t, errs, len(tt.wantPaths))
			for i, want := range tt.wantPaths {
				assert.ErrorContains(t, errs[i], want)
			}
		})
	}
}

func TestCheckImmutableNilSchema(t *testing.T) {
	assert.Nil(t, CheckImmutable(nil, map[string]any{"a": 1}, map[string]any{"a": 2}))
}

func TestJoinPathAttachesIndexSegmentsWithoutDot(t *testing.T) {
	assert.Equal(t, "nodes[0].storageClass", joinPath([]string{"nodes", "[0]", "storageClass"}))
	assert.Equal(t, "postgres.storageClass", joinPath([]string{"postgres", "storageClass"}))
}
