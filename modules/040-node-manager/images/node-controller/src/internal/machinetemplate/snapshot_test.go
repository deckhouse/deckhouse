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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSnapshotRoundTrip(t *testing.T) {
	spec := map[string]any{
		"vmClassName": "generic",
		"rootDisk":    map[string]any{"size": "50Gi"},
	}

	annotations, err := EncodeSnapshot(Snapshot{InstanceClass: spec, RolloutID: "2026-07-31", Generation: 4})
	require.NoError(t, err)

	stored, ok := DecodeSnapshot(annotations)
	require.True(t, ok)
	assert.Equal(t, "2026-07-31", stored.RolloutID)
	assert.Equal(t, 4, stored.Generation)

	changes, err := Changes(stored.InstanceClass, spec, []string{"vmClassName", "rootDisk.size"})
	require.NoError(t, err)
	assert.Empty(t, changes, "a snapshot read back must compare equal to what was stored")
}

func TestDecodeSnapshot(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expFound    bool
		expGen      int
	}{
		{
			name:        "no snapshot at all — a v1 object waiting to be adopted",
			annotations: map[string]string{"checksum/instance-class": "8ad9c341"},
			expFound:    false,
		},
		{
			// A corrupted annotation must not read as "everything changed" — that would recreate
			// the template and roll the machines.
			name:        "unparsable snapshot is treated as absent",
			annotations: map[string]string{AppliedInstanceClassAnnotation: "{not json"},
			expFound:    false,
		},
		{
			name: "snapshot written before the generation annotation existed",
			annotations: map[string]string{
				AppliedInstanceClassAnnotation: `{"flavorName":"m1.large"}`,
			},
			expFound: true,
			expGen:   0,
		},
		{
			name: "malformed generation counts as zero",
			annotations: map[string]string{
				AppliedInstanceClassAnnotation: `{"flavorName":"m1.large"}`,
				AppliedGenerationAnnotation:    "-3",
			},
			expFound: true,
			expGen:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snapshot, found := DecodeSnapshot(tc.annotations)
			assert.Equal(t, tc.expFound, found)
			assert.Equal(t, tc.expGen, snapshot.Generation)
		})
	}
}
