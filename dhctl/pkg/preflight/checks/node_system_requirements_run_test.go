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

package checks

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// nodeWithSpecs answers the two files this check reads.
func nodeWithSpecs(ramKB int, cores int) *fakeNode {
	cpuinfo := ""
	for i := range cores {
		cpuinfo += fmt.Sprintf("processor\t: %d\nvendor_id\t: GenuineIntel\n\n", i)
	}
	return newFakeNode().
		on("cat /proc/meminfo").prints(fmt.Sprintf("MemTotal:       %d kB\nMemFree:        1024 kB\n", ramKB)).
		on("cat /proc/cpuinfo").prints(cpuinfo)
}

// TestNodeSystemRequirementsRun: a machine too small for the bundle fails deep inside bashible,
// and the hardware will not have changed by the next attempt.
func TestNodeSystemRequirementsRun(t *testing.T) {
	// 4 CPU / 8 GB is the default bundle's floor; these sit either side of it.
	const (
		enoughRAM = 8 * 1024 * 1024
		smallRAM  = 2 * 1024 * 1024
	)

	t.Run("the machine is big enough", func(t *testing.T) {
		check := NodeSystemRequirementsCheck{NodeInterface: FixedNodeInterface(nodeWithSpecs(enoughRAM, 4))}

		detail, err := check.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "4 CPU")
		assert.Contains(t, detail, "8192 MiB of RAM")
	})

	t.Run("too little of both", func(t *testing.T) {
		check := NodeSystemRequirementsCheck{NodeInterface: FixedNodeInterface(nodeWithSpecs(smallRAM, 1))}

		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		// Both violations, not the first one: the operator resizing the machine needs to know
		// about the CPU as well as the RAM.
		assert.Contains(t, failure.Observed, "1 CPU")
		assert.Contains(t, failure.Observed, "MiB of RAM")
		assert.Contains(t, failure.Fix, "bundle: Minimal")
	})

	t.Run("/proc/meminfo says something unexpected", func(t *testing.T) {
		node := newFakeNode().
			on("cat /proc/meminfo").prints("MemFree: 1024 kB\n").
			on("cat /proc/cpuinfo").prints("processor\t: 0\n")

		_, err := NodeSystemRequirementsCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "MemTotal")
	})

	t.Run("/proc/meminfo cannot be read", func(t *testing.T) {
		node := newFakeNode().on("cat /proc/meminfo").exits(1)

		_, err := NodeSystemRequirementsCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 1")
	})

	t.Run("/proc/cpuinfo cannot be read", func(t *testing.T) {
		node := newFakeNode().
			on("cat /proc/meminfo").prints("MemTotal: 8388608 kB\n").
			on("cat /proc/cpuinfo").exits(1)

		_, err := NodeSystemRequirementsCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 1")
	})
}
