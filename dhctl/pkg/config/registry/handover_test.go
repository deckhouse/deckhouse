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

package registry

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
)

// TestThePrePullCoversEveryContainerOfTheStore guards the one thing that makes the handover
// recoverable.
//
// The images are read out of the store's own pod spec instead of being listed here, because the pull
// has to happen while the bootstrap registry still answers: after the port is freed there is nothing
// on that node to pull from, and a container whose image was missed does not fail loudly — it leaves
// the store, and with it every image in an air-gapped cluster, unreachable.
func TestThePrePullCoversEveryContainerOfTheStore(t *testing.T) {
	spec := corev1.PodSpec{
		InitContainers: []corev1.Container{
			{Name: "prepare", Image: "registry.d8-system.svc:5001/system/deckhouse@sha256:init"},
		},
		Containers: []corev1.Container{
			{Name: "distribution", Image: "registry.d8-system.svc:5001/system/deckhouse@sha256:one"},
			{Name: "auth", Image: "registry.d8-system.svc:5001/system/deckhouse@sha256:two"},
			// The store really does run four containers, and one of them — the syncer — is the one
			// that was not on the node during the first installation from a bundle: at that point the
			// syncer had only ever been installed as a package, never pulled as an image.
			{Name: "syncer", Image: "registry.d8-system.svc:5001/system/deckhouse@sha256:three"},
			{Name: "kube-rbac-proxy", Image: "registry.d8-system.svc:5001/system/deckhouse@sha256:four"},
		},
	}

	require.Equal(t, []string{
		"registry.d8-system.svc:5001/system/deckhouse@sha256:init",
		"registry.d8-system.svc:5001/system/deckhouse@sha256:one",
		"registry.d8-system.svc:5001/system/deckhouse@sha256:two",
		"registry.d8-system.svc:5001/system/deckhouse@sha256:three",
		"registry.d8-system.svc:5001/system/deckhouse@sha256:four",
	}, podImages(spec), "init containers count too: they run before the others and pull like them")

	t.Run("a repeated image is pulled once", func(t *testing.T) {
		same := corev1.PodSpec{Containers: []corev1.Container{
			{Name: "a", Image: "repo@sha256:x"},
			{Name: "b", Image: "repo@sha256:x"},
		}}
		require.Equal(t, []string{"repo@sha256:x"}, podImages(same))
	})
}

// TestTheStoreIsOnlyReadyWhenItHoldsTheWholeSet is the difference between "the installation finished"
// and "the installation finished and the cluster can actually pull".
//
// A store can report Ready while it is still filling, and whether that counts depends on whether
// there is anywhere else to pull from. Without an upstream — a bundle installation — there is not:
// the bundle was the only source and the installer is about to disconnect from it, so readiness has
// to mean the set is complete and the message has to say what is missing. With an upstream the
// cluster pulls through the store while it fills, and demanding a complete copy would add the whole
// first sync to every installation that merely turns the cache on.
func TestTheStoreIsOnlyReadyWhenItHoldsTheWholeSet(t *testing.T) {
	status := func(phase string, full bool, filled, total int64) map[string]any {
		return map[string]any{
			"status": map[string]any{
				"phase":           phase,
				"allReplicasFull": full,
				"fill": map[string]any{
					"filled": filled,
					"total":  total,
				},
			},
		}
	}

	t.Run("ready and full", func(t *testing.T) {
		require.NoError(t, storeStatusReady(status("Ready", true, 628, 556), true))
	})

	t.Run("ready but still filling", func(t *testing.T) {
		err := storeStatusReady(status("Ready", false, 120, 556), true)
		require.ErrorIs(t, err, ErrIsNotReady)
		require.Contains(t, err.Error(), "120 of 556",
			"the operator must be able to tell a store that is filling from one that is stuck")
	})

	t.Run("ready and still filling, with an upstream to fall back on", func(t *testing.T) {
		require.NoError(t, storeStatusReady(status("Ready", false, 120, 556), false),
			"a cache beside an upstream serves what it has and fetches the rest; waiting for the "+
				"whole set would add the first full sync to every installation that enables it")
	})

	t.Run("not ready", func(t *testing.T) {
		err := storeStatusReady(status("Processing", false, 0, 556), true)
		require.ErrorIs(t, err, ErrIsNotReady)
		require.Contains(t, err.Error(), "Processing")
	})

	t.Run("not ready, and fullness is not required either", func(t *testing.T) {
		// Not ready is not ready. Relaxing fullness must not relax the phase with it.
		err := storeStatusReady(status("Processing", false, 0, 556), false)
		require.ErrorIs(t, err, ErrIsNotReady)
		require.Contains(t, err.Error(), "Processing")
	})

	t.Run("nothing reported yet", func(t *testing.T) {
		// The object exists before the controller writes a status. Silence is not readiness.
		err := storeStatusReady(map[string]any{}, true)
		require.ErrorIs(t, err, ErrIsNotReady)
		require.Contains(t, err.Error(), "not reported yet")
	})
}

// TestTheHandoverOnlyHappensForABundleInstall pins the gate.
//
// The file removed by the handover is the same file the previous implementation manages on a legacy
// Local cluster, where it is not a leftover but the working registry. Removing it there would take the
// registry away from a cluster that has no other one.
func TestTheHandoverOnlyHappensForABundleInstall(t *testing.T) {
	// No kube client and no node: reaching either would panic, which is the assertion.
	require.NoError(t, HandOverBundleStore(t.Context(), nil, Config{}))
	require.NoError(t, HandOverBundleStore(t.Context(), nil, Config{LegacyMode: true}))
}

// TestAHandoverThatCannotReachTheNodeIsRefused is the failure this exists for, and it cost an
// installation: both halves of the handover run commands on the first master, and both used to return
// nil when there was no way to reach it. The caller built its client as
// `&client.KubernetesClient{KubeClient: kubeCl}`, leaving NodeInterface nil, so the handover reported
// success in 18 milliseconds having done nothing at all — the store's pod stayed Pending on the port
// the bootstrap registry still held, and the installation then waited thirty-three minutes for a store
// that could not start.
//
// Refusing here is what makes that impossible to repeat quietly: the wait downstream can only report
// that the store is Idle, which is true and says nothing about why.
func TestAHandoverThatCannotReachTheNodeIsRefused(t *testing.T) {
	// Bounded, because the assertion is that the refusal comes BEFORE anything is waited on: without
	// it this call spends ten minutes retrying a StatefulSet read against a cluster that has none.
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()

	err := HandOverBundleStore(ctx, client.NewFakeKubernetesClient(), Config{BundleBootstrap: true})

	require.ErrorIs(t, err, ErrHandoverNeedsTheNode,
		"a handover with no way to reach the node must fail, not succeed silently")
}

// TestThePullIsNotHandedToAShell is the defect this pins, and the shape of it is worth stating: a
// command that looks right, exits zero, and does nothing.
//
// The node interface wraps what it is given in a shell, so a command built as a shell line arrives
// nested — and `bash -c "<prog> pull <ref>"` passed through another `bash -c` turns pull and the
// reference into $0 and $1. crictl with no arguments prints its usage and exits 0, which the caller
// reads as four images pulled. Measured on a node; the store then crash-looped on an image that the
// registry it would have come from had just been removed to make room for it.
func TestThePullIsNotHandedToAShell(t *testing.T) {
	const image = "registry.d8-system.svc:5001/system/deckhouse@sha256:abc"

	args := pullArgs(image)

	require.Equal(t, []string{crictlPath, "pull", image}, args,
		"the pull must be a program and its arguments, not a line for a shell to re-split")

	for _, arg := range args {
		require.NotContains(t, arg, "bash",
			"a shell here means quoting, and the quoting is what silently swallowed the arguments")
	}
}
