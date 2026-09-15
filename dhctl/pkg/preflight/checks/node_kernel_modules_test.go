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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
)

// TestNodeKernelModulesRun: step 005 runs `modprobe br_netfilter` with no guard at all, so a
// minimal image whose kernel package has no extra modules takes the whole bundle down and repeats
// until it is killed — never naming the package that supplies the module.
func TestNodeKernelModulesRun(t *testing.T) {
	loadable := func(modules ...string) *fakeNode {
		node := newFakeNode()
		for _, module := range modules {
			node = node.on("sudo modprobe -n -q " + module).succeeds()
		}
		return node
	}

	// A module compiled into the kernel is not a file modprobe can load, and older kmod exits
	// non-zero for it. /sys/module/<name> is there either way.
	builtin := func(modules ...string) *fakeNode {
		node := newFakeNode()
		for _, module := range modules {
			node = node.on("test -d /sys/module/" + module).succeeds()
		}
		return node
	}

	t.Run("every module loads", func(t *testing.T) {
		node := loadable("br_netfilter", "overlay")

		detail, err := NodeKernelModulesCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "br_netfilter, overlay")
		assert.NotContains(t, node.ran(), "sudo modprobe -n -q erofs",
			"erofs belongs to ContainerdV2 and this cluster did not ask for it")
	})

	t.Run("a module is missing", func(t *testing.T) {
		node := loadable("br_netfilter")

		_, err := NodeKernelModulesCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "overlay")
		assert.NotContains(t, failure.Observed, "br_netfilter")
		// The package is the whole point: "overlay cannot be loaded" is not actionable on its
		// own.
		assert.Contains(t, failure.Fix, "linux-modules-extra")
	})

	t.Run("ContainerdV2 adds erofs", func(t *testing.T) {
		// The one module step 005 does check and exit on, and containerd v2 cannot start
		// without it.
		node := loadable("br_netfilter", "overlay")

		check := NodeKernelModulesCheck{
			MetaConfig:    containerdV2MetaConfig(t),
			NodeInterface: FixedNodeInterface(node),
		}
		_, err := check.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "erofs")
	})

	t.Run("a module built into the kernel counts as present", func(t *testing.T) {
		// The false refusal this guards against: a kernel that already has what Deckhouse
		// needs, and a modprobe that says it cannot load it.
		node := builtin("br_netfilter", "overlay")

		detail, err := NodeKernelModulesCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "br_netfilter, overlay")
		// modprobe is not even asked: /sys/module answered.
		assert.NotContains(t, node.ran(), "sudo modprobe -n -q br_netfilter")
	})

	t.Run("an absent module still falls back to modprobe", func(t *testing.T) {
		node := loadable("br_netfilter", "overlay")

		_, err := NodeKernelModulesCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, node.ran(), "sudo modprobe -n -q br_netfilter")
	})

	t.Run("the probe changes nothing on the node", func(t *testing.T) {
		// -n is a dry run: a preflight check may not load modules into the operator's kernel.
		node := loadable("br_netfilter", "overlay")

		_, err := NodeKernelModulesCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.NoError(t, err)
		assert.NotContains(t, node.ran(), "sudo modprobe br_netfilter")
		assert.Contains(t, node.ran(), "sudo modprobe -n -q br_netfilter")
	})
}

// TestNodeSELinuxToolsRun: step 005 compiles the Deckhouse SELinux policy with tools a minimal
// RedOS or RHEL does not ship, and runs them without checking (issue #15167).
func TestNodeSELinuxToolsRun(t *testing.T) {
	withTools := func(tools ...string) *fakeNode {
		node := newFakeNode().on("getenforce").prints("Enforcing")
		for _, tool := range tools {
			node = node.on("command -v " + tool).succeeds()
		}
		return node
	}

	t.Run("enforcing with every tool present", func(t *testing.T) {
		node := withTools("checkmodule", "semodule_package", "semodule")

		detail, err := NodeSELinuxToolsCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.NoError(t, err)
		assert.Contains(t, detail, "enforcing")
	})

	t.Run("enforcing without the tools", func(t *testing.T) {
		node := withTools("semodule")

		_, err := NodeSELinuxToolsCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.Error(t, err)
		var failure *preflight.Failure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Observed, "checkmodule")
		assert.Contains(t, failure.Observed, "semodule_package")
		assert.Contains(t, failure.Fix, "checkpolicy")
	})

	t.Run("SELinux is permissive", func(t *testing.T) {
		// Step 005 exits early here, so the tools are never run and their absence means
		// nothing.
		node := newFakeNode().on("getenforce").prints("Permissive")

		_, err := NodeSELinuxToolsCheck{NodeInterface: FixedNodeInterface(node)}.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "Permissive")
		assert.NotContains(t, node.ran(), "command -v checkmodule")
	})

	t.Run("the node has no SELinux", func(t *testing.T) {
		_, err := NodeSELinuxToolsCheck{NodeInterface: FixedNodeInterface(newFakeNode())}.Run(t.Context())

		require.ErrorIs(t, err, preflight.ErrNotApplicable)
		assert.Contains(t, err.Error(), "does not have SELinux")
	})
}
