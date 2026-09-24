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

package dhregistry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dhregistry "github.com/deckhouse/deckhouse/pkg/deckhouse-registry"
)

func TestParseEdition(t *testing.T) {
	tests := []struct {
		in      string
		want    dhregistry.Edition
		wantErr bool
	}{
		{"fe", dhregistry.FEEdition, false},
		{"FE", dhregistry.FEEdition, false},
		{" ee ", dhregistry.EEEdition, false},
		{"se-plus", dhregistry.SEPlusEdition, false},
		{"", dhregistry.NoEdition, false},
		{"xx", dhregistry.NoEdition, true},
		{"enterprise", dhregistry.NoEdition, true},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := dhregistry.ParseEdition(tt.in)
			if tt.wantErr {
				require.ErrorIs(t, err, dhregistry.ErrUnknownEdition)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitEdition(t *testing.T) {
	tests := []struct {
		in          string
		wantRoot    string
		wantEdition dhregistry.Edition
	}{
		{"registry.deckhouse.io/deckhouse/ee/", "registry.deckhouse.io/deckhouse", dhregistry.EEEdition},
		{"registry.deckhouse.io/deckhouse/se-plus", "registry.deckhouse.io/deckhouse", dhregistry.SEPlusEdition},
		{"dev-registry.deckhouse.io/sys/deckhouse-oss", "dev-registry.deckhouse.io/sys/deckhouse-oss", dhregistry.NoEdition},
		{"myregistry.ru/deckhouse/", "myregistry.ru/deckhouse", dhregistry.NoEdition},
		{"registry.example.com", "registry.example.com", dhregistry.NoEdition},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			root, got := dhregistry.SplitEdition(tt.in)
			assert.Equal(t, tt.wantRoot, root)
			assert.Equal(t, tt.wantEdition, got)
		})
	}
}

func TestEditionIsValid(t *testing.T) {
	for _, e := range dhregistry.Editions() {
		assert.True(t, e.IsValid(), e)
	}

	assert.False(t, dhregistry.NoEdition.IsValid())
	assert.False(t, dhregistry.Edition("xx").IsValid())
	assert.Len(t, dhregistry.Editions(), 6)
}

// TestAllIsACopy checks that mutating the returned slice cannot corrupt the
// package-level edition list.
func TestEditionsIsACopy(t *testing.T) {
	all := dhregistry.Editions()
	all[0] = "mutated"

	assert.Equal(t, dhregistry.CEEdition, dhregistry.Editions()[0])
}
