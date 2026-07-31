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
