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
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	libcon "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-connection/pkg"
	dhlog "fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/logger"
	"fox.flant.com/deckhouse/cluster-and-infrastructure/libs/lib-dhctl/pkg/retry"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

// TestAHandoverThatCannotReachTheNodeIsRefused is the failure this exists for. Both halves of the
// handover run commands on the first master, and both used to return nil when there was no way to
// reach it: a caller building its client as `&client.KubernetesClient{KubeClient: kubeCl}` leaves
// NodeInterface nil, so the handover reports success having done nothing at all — the store's pod
// stays Pending on the port the bootstrap registry still holds, and the installation waits on a store
// that cannot start.
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
// reads as every image pulled — and the store then has to start on an image whose only source, the
// bootstrap registry, was removed on the strength of that success.
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

// storeTolerations are the tolerations the module's template gives the store: any-node plus
// with-uninitialized, as helm_lib renders them. Spelled out rather than rendered, because what the
// handover is asked to judge is exactly what the template says, and the cloud taint is deliberately
// not among them.
var storeTolerations = []corev1.Toleration{
	{Key: "node-role.kubernetes.io/master"},
	{Key: "node-role.kubernetes.io/control-plane"},
	{Key: "dedicated.deckhouse.io", Operator: corev1.TolerationOpExists},
	{Key: "dedicated", Operator: corev1.TolerationOpExists},
	{Key: "node.deckhouse.io/bashible-uninitialized", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
	{Key: "node.deckhouse.io/uninitialized", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
}

func storePodSpec() corev1.PodSpec {
	return corev1.PodSpec{
		NodeSelector: map[string]string{controlPlaneLabel: ""},
		Tolerations:  storeTolerations,
		Containers: []corev1.Container{
			{Name: "distribution", Image: "registry.d8-system.svc:5001/system/deckhouse@sha256:one"},
			{Name: "syncer", Image: "registry.d8-system.svc:5001/system/deckhouse@sha256:two"},
		},
	}
}

func masterNode(name string, taints ...corev1.Taint) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{controlPlaneLabel: ""}},
		Spec: corev1.NodeSpec{Taints: append([]corev1.Taint{
			{Key: controlPlaneLabel, Effect: corev1.TaintEffectNoSchedule},
		}, taints...)},
	}
}

var cloudUninitialized = corev1.Taint{Key: cloudUninitializedTaint, Value: "true", Effect: corev1.TaintEffectNoSchedule}

// TestTheStoreIsHandedOnlyToANodeThatAcceptsIt pins the judgement the handover makes before it frees
// the port.
//
// In a cloud the first master carries node.cloudprovider.kubernetes.io/uninitialized until the
// cloud-controller-manager initializes it, and nothing earlier in the bootstrap waits for that: the
// readiness wait before the handover is on the Deckhouse pod, which tolerates every taint. Freeing the
// port then removes the only registry on the node in favour of a store the scheduler will not place —
// and the store must not be made to tolerate the taint instead, because without the cloud's address
// its syncer does not start. So the handover waits, and says what it is waiting for.
func TestTheStoreIsHandedOnlyToANodeThatAcceptsIt(t *testing.T) {
	newClient := func(t *testing.T, nodes ...*corev1.Node) *client.KubernetesClient {
		t.Helper()

		kubeCl := client.NewFakeKubernetesClient()
		for _, node := range nodes {
			_, err := kubeCl.CoreV1().Nodes().Create(t.Context(), node, metav1.CreateOptions{})
			require.NoError(t, err)
		}
		return kubeCl
	}

	t.Run("a static master is accepted on the first look", func(t *testing.T) {
		kubeCl := newClient(t, masterNode("master-0"))

		require.NoError(t, storeSchedulable(t.Context(), kubeCl, storePodSpec()),
			"the control-plane taint is tolerated, and a static cluster carries nothing else")
	})

	t.Run("a master the cloud has not initialized is refused, by name", func(t *testing.T) {
		kubeCl := newClient(t, masterNode("master-0", cloudUninitialized))

		err := storeSchedulable(t.Context(), kubeCl, storePodSpec())

		require.ErrorContains(t, err, "the cloud controller manager has not initialized node master-0")
		require.ErrorContains(t, err, "the bootstrap registry is kept")
		require.ErrorContains(t, err, cloudUninitializedTaint)
	})

	t.Run("the wait ends once the cloud has initialized the node", func(t *testing.T) {
		kubeCl := newClient(t, masterNode("master-0", cloudUninitialized))
		require.Error(t, storeSchedulable(t.Context(), kubeCl, storePodSpec()))

		_, err := kubeCl.CoreV1().Nodes().Update(t.Context(), masterNode("master-0"), metav1.UpdateOptions{})
		require.NoError(t, err)

		require.NoError(t, storeSchedulable(t.Context(), kubeCl, storePodSpec()))
	})

	t.Run("any taint the store does not tolerate keeps the bootstrap registry", func(t *testing.T) {
		kubeCl := newClient(t, masterNode("master-0",
			corev1.Taint{Key: "example.com/maintenance", Effect: corev1.TaintEffectNoExecute}))

		err := storeSchedulable(t.Context(), kubeCl, storePodSpec())

		require.ErrorContains(t, err, "node master-0 has taint example.com/maintenance:NoExecute")
		require.ErrorContains(t, err, "the bootstrap registry is kept")
	})

	t.Run("a preference keeps nothing off the node", func(t *testing.T) {
		kubeCl := newClient(t, masterNode("master-0",
			corev1.Taint{Key: "example.com/busy", Effect: corev1.TaintEffectPreferNoSchedule}))

		require.NoError(t, storeSchedulable(t.Context(), kubeCl, storePodSpec()))
	})

	t.Run("only the nodes the store runs on are asked", func(t *testing.T) {
		worker := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
			Spec:       corev1.NodeSpec{Taints: []corev1.Taint{cloudUninitialized}},
		}
		kubeCl := newClient(t, masterNode("master-0"), worker)

		require.NoError(t, storeSchedulable(t.Context(), kubeCl, storePodSpec()))
	})

	t.Run("an empty selector means the control plane, not every node", func(t *testing.T) {
		worker := &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "worker-0"},
			Spec:       corev1.NodeSpec{Taints: []corev1.Taint{cloudUninitialized}},
		}
		kubeCl := newClient(t, masterNode("master-0"), worker)

		spec := storePodSpec()
		spec.NodeSelector = nil

		require.NoError(t, storeSchedulable(t.Context(), kubeCl, spec))
	})

	t.Run("no master yet is not an acceptance", func(t *testing.T) {
		err := storeSchedulable(t.Context(), newClient(t), storePodSpec())

		require.ErrorContains(t, err, "no node matches")
	})
}

// TestTheWaitForTheNodeSaysWhatItIsWaitingFor keeps the ten-minute wait for the
// cloud-controller-manager from being silent in the terminal.
//
// The retry loop reports each attempt only to the debug log, so the operator saw the loop's name and
// nothing else until the timeout, which is when the reason (and the advice to check the
// cloud-controller-manager) finally appeared. The reason has to reach the compact view when the wait
// starts and then once a minute, and nothing has to be said for a node that accepts the store at once.
func TestTheWaitForTheNodeSaysWhatItIsWaitingFor(t *testing.T) {
	// The real ten minutes in milliseconds: the same 60 looks, without the wait between them.
	wait := storeSchedulableWait
	storeSchedulableWait = time.Millisecond
	t.Cleanup(func() { storeSchedulableWait = wait })

	newClient := func(t *testing.T, node *corev1.Node) *client.KubernetesClient {
		t.Helper()

		kubeCl := client.NewFakeKubernetesClient()
		_, err := kubeCl.CoreV1().Nodes().Create(t.Context(), node, metav1.CreateOptions{})
		require.NoError(t, err)
		return kubeCl
	}

	t.Run("the reason is shown when the wait starts and once a minute", func(t *testing.T) {
		var terminal bytes.Buffer
		ctx := dhlog.ToContext(t.Context(), dhlog.NewBufferLogger(&terminal))

		err := waitForStoreSchedulable(ctx, newClient(t, masterNode("master-0", cloudUninitialized)), storePodSpec())
		require.ErrorContains(t, err, "the cloud controller manager has not initialized node master-0")

		// Only a line tagged for the compact view reaches the terminal: the loop's own per-attempt
		// lines are Debug, which is the silence this test is about.
		var first, repeated int
		for _, line := range strings.Split(terminal.String(), "\n") {
			if !strings.Contains(line, "compact=true") {
				continue
			}
			require.Contains(t, line, "the cloud controller manager has not initialized node master-0",
				"the line says what the wait is for, not only that it is waiting")

			switch {
			case strings.Contains(line, "Waiting up to 60ms for the first master to accept the cluster store"):
				first++
			case strings.Contains(line, "Still waiting for the first master to accept the cluster store"):
				repeated++
			}
		}

		require.Equal(t, 1, first, "the wait announces itself and its limit on the first refused look")
		require.Equal(t, 9, repeated, "60 looks, one line every 6 after the first: one a minute at the real wait")
	})

	t.Run("a node that accepts the store at once is not reported", func(t *testing.T) {
		var terminal bytes.Buffer
		ctx := dhlog.ToContext(t.Context(), dhlog.NewBufferLogger(&terminal))

		require.NoError(t, waitForStoreSchedulable(ctx, newClient(t, masterNode("master-0")), storePodSpec()))
		require.NotContains(t, terminal.String(), "compact=true")
	})
}

// TestTheHandoverKeepsTheBootstrapRegistryOnAnUninitializedNode is the order the handover exists for,
// taken end to end: on a node the store cannot be placed on, nothing is run on it at all — no pull,
// and above all no removal of the bootstrap registry. Once the node accepts the store, the images are
// pulled and only then is the port freed.
func TestTheHandoverKeepsTheBootstrapRegistryOnAnUninitializedNode(t *testing.T) {
	// One attempt, no waiting: the loop's patience is not what is being tested.
	retry.InTestEnvironment = true
	t.Cleanup(func() { retry.InTestEnvironment = false })

	newClient := func(t *testing.T, node *corev1.Node) (*client.KubernetesClient, *recordingNode) {
		t.Helper()

		recorder := &recordingNode{}
		kubeCl := client.NewFakeKubernetesClient().WithNodeInterface(recorder)

		_, err := kubeCl.CoreV1().Nodes().Create(t.Context(), node, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = kubeCl.AppsV1().StatefulSets(secretsNamespace).Create(t.Context(), &appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{Name: storageStatefulSetName, Namespace: secretsNamespace},
			Spec: appsv1.StatefulSetSpec{
				Template: corev1.PodTemplateSpec{Spec: storePodSpec()},
			},
		}, metav1.CreateOptions{})
		require.NoError(t, err)

		return kubeCl, recorder
	}

	t.Run("refused while the cloud has not initialized the node", func(t *testing.T) {
		kubeCl, recorder := newClient(t, masterNode("master-0", cloudUninitialized))

		err := HandOverBundleStore(t.Context(), kubeCl, Config{BundleBootstrap: true})

		require.ErrorContains(t, err, "the cloud controller manager has not initialized node master-0")
		require.Empty(t, recorder.commands,
			"the bootstrap registry is the only source of images on the node and must survive the refusal")
	})

	t.Run("handed over once the node accepts the store", func(t *testing.T) {
		kubeCl, recorder := newClient(t, masterNode("master-0"))

		require.NoError(t, HandOverBundleStore(t.Context(), kubeCl, Config{BundleBootstrap: true}))

		require.Equal(t, [][]string{
			{crictlPath, "pull", "registry.d8-system.svc:5001/system/deckhouse@sha256:one"},
			{crictlPath, "pull", "registry.d8-system.svc:5001/system/deckhouse@sha256:two"},
			{"rm", "-f", bootstrapStaticPodPath},
		}, recorder.commands, "the images are pulled while the bootstrap registry still serves them, and the port is freed last")
	})
}

// recordingNode stands in for the first master: it records every command it is asked to run and runs
// none. Anything else the handover asked of the node would panic on the nil interface it embeds.
type recordingNode struct {
	libcon.Interface

	commands [][]string
}

func (n *recordingNode) Command(name string, args ...string) libcon.Command {
	n.commands = append(n.commands, append([]string{name}, args...))
	return &recordingCommand{}
}

type recordingCommand struct {
	libcon.Command
}

func (*recordingCommand) Sudo(context.Context)                           {}
func (*recordingCommand) WithTimeout(time.Duration)                      {}
func (*recordingCommand) Run(context.Context) error                      { return nil }
func (*recordingCommand) CombinedOutput(context.Context) ([]byte, error) { return nil, nil }
