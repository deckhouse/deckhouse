// Copyright 2025 Flant JSC
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

package source

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
)

func TestResolveEmbeddedTargetSource(t *testing.T) {
	const embedded = v1alpha1.ModuleSourceEmbedded

	tests := []struct {
		name             string
		chosenSource     string
		availableSources []string
		wantTarget       string
		wantConflict     bool
	}{
		{
			name:             "explicitly chosen source that is offered wins",
			chosenSource:     "deckhouse-upstream-ee",
			availableSources: []string{"deckhouse", "deckhouse-upstream-ee"},
			wantTarget:       "deckhouse-upstream-ee",
			wantConflict:     false,
		},
		{
			name:             "chosen source that is no longer offered is a conflict",
			chosenSource:     "gone",
			availableSources: []string{"deckhouse", "deckhouse-upstream-ee"},
			wantTarget:       "",
			wantConflict:     true,
		},
		{
			name:             "single real source is used",
			availableSources: []string{"deckhouse-upstream-ee"},
			wantTarget:       "deckhouse-upstream-ee",
			wantConflict:     false,
		},
		{
			// the case that produced the false-positive ModuleAtConflict alert
			name:             "deckhouse plus a mirror resolves to deckhouse, not a conflict",
			availableSources: []string{"deckhouse", "deckhouse-upstream-ee"},
			wantTarget:       "deckhouse",
			wantConflict:     false,
		},
		{
			name:             "source order does not matter, deckhouse still wins",
			availableSources: []string{"deckhouse-upstream-ee", "deckhouse"},
			wantTarget:       "deckhouse",
			wantConflict:     false,
		},
		{
			name:             "Embedded sentinel plus one real source is not a conflict",
			availableSources: []string{embedded, "deckhouse-upstream-ee"},
			wantTarget:       "deckhouse-upstream-ee",
			wantConflict:     false,
		},
		{
			name:             "Embedded plus deckhouse plus a mirror resolves to deckhouse",
			availableSources: []string{embedded, "deckhouse", "deckhouse-upstream-ee"},
			wantTarget:       "deckhouse",
			wantConflict:     false,
		},
		{
			name:             "only the Embedded sentinel is available - nothing to pre-stage, not a conflict",
			availableSources: []string{embedded},
			wantTarget:       "",
			wantConflict:     false,
		},
		{
			name:             "several non-default real sources with no selection is a genuine conflict",
			availableSources: []string{"vendor-a", "vendor-b"},
			wantTarget:       "",
			wantConflict:     true,
		},
		{
			name:             "no sources at all is not a conflict",
			availableSources: nil,
			wantTarget:       "",
			wantConflict:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, conflict := resolveEmbeddedTargetSource(tt.chosenSource, tt.availableSources)
			assert.Equal(t, tt.wantTarget, target, "target source")
			assert.Equal(t, tt.wantConflict, conflict, "conflict")
		})
	}
}
