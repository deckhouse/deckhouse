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

package capi

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// generationOf recovers the counter for the two objects whose snapshot cannot supply it: a v1
// checksum-named object being adopted, and a generation object being recreated after deletion.
// A wrong answer re-uses a live generation's name or skips numbers for no reason.
func TestGenerationOf(t *testing.T) {
	tests := []struct {
		name   string
		object string
		expGen int
	}{
		{name: "generation name", object: "worker-a1b2c3d4-gen4", expGen: 4},
		{name: "first generation", object: "worker-a1b2c3d4-gen1", expGen: 1},
		{name: "v1 checksum name has no generation", object: "worker-8ad9c341", expGen: 0},
		{name: "node group named like a generation", object: "gen5-a1b2c3d4-gen2", expGen: 2},
		{name: "trailing garbage is not a generation", object: "worker-a1b2c3d4-genX", expGen: 0},
		{name: "empty suffix", object: "worker-a1b2c3d4-gen", expGen: 0},
		{name: "negative number", object: "worker-a1b2c3d4-gen-2", expGen: 0},
		{name: "empty name", object: "", expGen: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expGen, generationOf(tc.object))
		})
	}
}

// The pruner keeps a few superseded generations so their snapshots outlive the NodeGroup event.
// Ranking is per zone: one zone's history must not push another zone's out.
func TestRecentGenerations(t *testing.T) {
	tests := []struct {
		name string
		// objects are the templates the NodeGroup has; liveZones defaults to every zone present.
		objects   []string
		liveZones []string
		expKept   []string
	}{
		{
			name:    "fewer than the limit: all kept",
			objects: []string{"worker-a1b2c3d4-gen1", "worker-a1b2c3d4-gen2"},
			expKept: []string{"worker-a1b2c3d4-gen1", "worker-a1b2c3d4-gen2"},
		},
		{
			name: "more than the limit: the newest survive",
			objects: []string{
				"worker-a1b2c3d4-gen1", "worker-a1b2c3d4-gen2",
				"worker-a1b2c3d4-gen3", "worker-a1b2c3d4-gen4",
			},
			expKept: []string{"worker-a1b2c3d4-gen2", "worker-a1b2c3d4-gen3", "worker-a1b2c3d4-gen4"},
		},
		{
			name: "each zone keeps its own",
			objects: []string{
				"worker-a1b2c3d4-gen1", "worker-a1b2c3d4-gen2", "worker-a1b2c3d4-gen3", "worker-a1b2c3d4-gen4",
				"worker-e5f6a7b8-gen1", "worker-e5f6a7b8-gen2",
			},
			expKept: []string{
				"worker-a1b2c3d4-gen2", "worker-a1b2c3d4-gen3", "worker-a1b2c3d4-gen4",
				"worker-e5f6a7b8-gen1", "worker-e5f6a7b8-gen2",
			},
		},
		{
			name:    "double-digit generations sort as numbers, not strings",
			objects: []string{"worker-a1b2c3d4-gen9", "worker-a1b2c3d4-gen10", "worker-a1b2c3d4-gen11", "worker-a1b2c3d4-gen12"},
			expKept: []string{"worker-a1b2c3d4-gen10", "worker-a1b2c3d4-gen11", "worker-a1b2c3d4-gen12"},
		},
		{
			// A v1 checksum-named object has no generation to rank, so it is pruned the moment
			// nothing references it — exactly as before v2.
			name:    "v1 names are not kept",
			objects: []string{"worker-8ad9c341", "worker-a1b2c3d4-gen1"},
			expKept: []string{"worker-a1b2c3d4-gen1"},
		},
		{
			// A zone removed from the NodeGroup has no history worth keeping, and keeping it
			// would leak one object per removed zone for the life of the NodeGroup.
			name:      "a zone that is gone keeps nothing",
			objects:   []string{"worker-a1b2c3d4-gen1", "worker-deadbeef-gen1", "worker-deadbeef-gen2"},
			liveZones: []string{"worker-a1b2c3d4"},
			expKept:   []string{"worker-a1b2c3d4-gen1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			live := map[string]struct{}{}
			for _, zone := range tc.liveZones {
				live[zone] = struct{}{}
			}
			if len(live) == 0 {
				for _, name := range tc.objects {
					if idx := strings.LastIndex(name, generationSuffix); idx > 0 {
						live[name[:idx]] = struct{}{}
					}
				}
			}
			kept := recentGenerations(tc.objects, live)
			names := make([]string, 0, len(kept))
			for name := range kept {
				names = append(names, name)
			}
			assert.ElementsMatch(t, tc.expKept, names)
		})
	}
}
