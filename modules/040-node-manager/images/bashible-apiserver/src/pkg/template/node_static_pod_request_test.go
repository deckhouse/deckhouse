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

package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodeStaticPodRequestSpecIsEqual(t *testing.T) {
	const manifest = "apiVersion: v1\nkind: Pod\n"

	spec := NodeStaticPodRequestSpec{
		NodeGroupSelector: NodeGroupSelector{MatchNames: []string{"worker"}},
		Manifest:          manifest,
	}

	tests := []struct {
		name  string
		other NodeStaticPodRequestSpec
		equal bool
	}{
		{
			name: "the same spec",
			other: NodeStaticPodRequestSpec{
				NodeGroupSelector: NodeGroupSelector{MatchNames: []string{"worker"}},
				Manifest:          manifest,
			},
			equal: true,
		},
		{
			name: "another manifest",
			other: NodeStaticPodRequestSpec{
				NodeGroupSelector: NodeGroupSelector{MatchNames: []string{"worker"}},
				Manifest:          manifest + "metadata: {}\n",
			},
			equal: false,
		},
		{
			name: "another node group",
			other: NodeStaticPodRequestSpec{
				NodeGroupSelector: NodeGroupSelector{MatchNames: []string{"master"}},
				Manifest:          manifest,
			},
			equal: false,
		},
		{
			name:  "no node group selector",
			other: NodeStaticPodRequestSpec{Manifest: manifest},
			equal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.equal, spec.IsEqual(tt.other))
		})
	}

	// An absent selector and an empty one are the same thing: a resync that
	// spells it differently must not re-render every node group.
	empty := NodeStaticPodRequestSpec{
		NodeGroupSelector: NodeGroupSelector{MatchNames: []string{}},
		Manifest:          manifest,
	}
	require.True(t, empty.IsEqual(NodeStaticPodRequestSpec{Manifest: manifest}))
}
