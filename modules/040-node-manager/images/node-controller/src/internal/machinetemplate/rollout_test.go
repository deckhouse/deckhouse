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

package machinetemplate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChanges(t *testing.T) {
	tests := []struct {
		name      string
		old       map[string]any
		new       map[string]any
		fields    []string
		expPaths  []string
		expReason string
	}{
		{
			name:     "same values, no change",
			old:      map[string]any{"flavorName": "m1.large"},
			new:      map[string]any{"flavorName": "m1.large"},
			fields:   []string{"flavorName"},
			expPaths: []string{},
		},
		{
			// The whole reason v2 exists: under v1 the name was a hash over the bytes of a YAML
			// serialization, so int64(50) and float64(50) produced different names and rolled
			// every machine in every cluster.
			name:     "same number through different Go types, no change",
			old:      map[string]any{"rootDiskSize": int64(50)},
			new:      map[string]any{"rootDiskSize": float64(50)},
			fields:   []string{"rootDiskSize"},
			expPaths: []string{},
		},
		{
			name:     "changed value",
			old:      map[string]any{"flavorName": "m1.large"},
			new:      map[string]any{"flavorName": "m1.xlarge"},
			fields:   []string{"flavorName"},
			expPaths: []string{"flavorName"},
		},
		{
			name:     "field outside rolloutFields is ignored",
			old:      map[string]any{"capacity": map[string]any{"cores": float64(4)}},
			new:      map[string]any{"capacity": map[string]any{"cores": float64(8)}},
			fields:   []string{"flavorName"},
			expPaths: []string{},
		},
		{
			name:     "nested path",
			old:      map[string]any{"rootDisk": map[string]any{"size": "50Gi"}},
			new:      map[string]any{"rootDisk": map[string]any{"size": "100Gi"}},
			fields:   []string{"rootDisk.size"},
			expPaths: []string{"rootDisk.size"},
		},
		{
			name:     "absent and explicit null are the same",
			old:      map[string]any{},
			new:      map[string]any{"imageName": nil},
			fields:   []string{"imageName"},
			expPaths: []string{},
		},
		{
			name:     "field appears",
			old:      map[string]any{},
			new:      map[string]any{"imageName": "ubuntu"},
			fields:   []string{"imageName"},
			expPaths: []string{"imageName"},
		},
		{
			name:     "field disappears",
			old:      map[string]any{"imageName": "ubuntu"},
			new:      map[string]any{},
			fields:   []string{"imageName"},
			expPaths: []string{"imageName"},
		},
		{
			name:     "list order matters",
			old:      map[string]any{"additionalNetworks": []any{"a", "b"}},
			new:      map[string]any{"additionalNetworks": []any{"b", "a"}},
			fields:   []string{"additionalNetworks"},
			expPaths: []string{"additionalNetworks"},
		},
		{
			name:     "map key order does not matter",
			old:      map[string]any{"additionalTags": map[string]any{"a": "1", "b": "2"}},
			new:      map[string]any{"additionalTags": map[string]any{"b": "2", "a": "1"}},
			fields:   []string{"additionalTags"},
			expPaths: []string{},
		},
		{
			name:     "several changes come back sorted",
			old:      map[string]any{"flavorName": "a", "imageName": "x"},
			new:      map[string]any{"flavorName": "b", "imageName": "y"},
			fields:   []string{"imageName", "flavorName"},
			expPaths: []string{"flavorName", "imageName"},
		},
		{
			// A path that runs into a scalar must not panic or report a phantom change.
			name:     "path through a scalar is treated as absent",
			old:      map[string]any{"rootDisk": "50Gi"},
			new:      map[string]any{"rootDisk": "50Gi"},
			fields:   []string{"rootDisk.size"},
			expPaths: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			changes, err := Changes(tc.old, tc.new, tc.fields)
			require.NoError(t, err)

			paths := make([]string, 0, len(changes))
			for _, change := range changes {
				paths = append(paths, change.Path)
			}
			assert.Equal(t, tc.expPaths, paths)
		})
	}
}

func TestFormatChanges(t *testing.T) {
	changes, err := Changes(
		map[string]any{"flavorName": "m1.large"},
		map[string]any{"flavorName": "m1.xlarge"},
		[]string{"flavorName", "imageName"},
	)
	require.NoError(t, err)
	assert.Equal(t, `flavorName "m1.large" → "m1.xlarge"`, FormatChanges(changes))
}

func TestFormatChangesMarksAbsentValues(t *testing.T) {
	changes, err := Changes(map[string]any{}, map[string]any{"imageName": "ubuntu"}, []string{"imageName"})
	require.NoError(t, err)
	assert.Equal(t, `imageName <none> → "ubuntu"`, FormatChanges(changes))
}

// An event has to stay readable: one long field must not push the rest out of the message.
func TestFormatChangesTruncatesLongValues(t *testing.T) {
	long := strings.Repeat("x", 200)
	changes, err := Changes(
		map[string]any{"additionalTags": map[string]any{"note": "short"}},
		map[string]any{"additionalTags": map[string]any{"note": long}},
		[]string{"additionalTags"},
	)
	require.NoError(t, err)

	formatted := FormatChanges(changes)
	assert.Contains(t, formatted, "…")
	assert.Less(t, len(formatted), 200)
}
