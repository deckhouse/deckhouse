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

package bashible_tests

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

// TestTheAgentRequestsWhatItNeedsAndIsNotLimited is the BestEffort agent.
//
// Without requests the pod lands in the smallest CPU weight on the node, and under load — a
// node bootstrapping, a busy worker — every pull on the node waited on an agent that got next to
// no CPU. A CPU limit would be the same starvation from the other side: every byte of every
// image crosses the agent's TLS, so throttling it throttles the node's pulls. The values are
// shared with the Engine manifest through the constants, and the Engine manifest with dhctl's
// copy through a byte-for-byte template test.
func TestTheAgentRequestsWhatItNeedsAndIsNotLimited(t *testing.T) {
	resources := staticPod(t, render(t, agentStep, agentRegistry())).Spec.Containers[0].Resources

	require.Equal(t, resource.MustParse(registry_const.AgentCPURequest), resources.Requests[corev1.ResourceCPU])
	require.Equal(t, resource.MustParse(registry_const.AgentMemoryRequest), resources.Requests[corev1.ResourceMemory])
	require.Empty(t, resources.Limits, "a limit on the agent throttles or kills every pull on the node")
}

// TestTheAgentMeasuresTheStoreWhereItIs is D8RegistryStaleCacheData never firing on a node
// bashible configures.
//
// The agent measures what the cache left behind once the cache is off, under --store-path. The
// step passed no such flag and mounted nothing there, so the agent measured its default path
// inside its own read-only container — a path that does not exist, which it reads as nothing
// left — and reported zero on every such node. The Engine manifest has had the mount from the
// start.
func TestTheAgentMeasuresTheStoreWhereItIs(t *testing.T) {
	body := render(t, agentStep, agentRegistry())
	pod := staticPod(t, body)

	// Only a node of an Immutable group keeps the store elsewhere, and bashible configures none.
	storePath := stepVariable(t, body, "store_path")
	require.Equal(t, registry_const.StorePath, storePath)
	require.Contains(t, pod.Spec.Containers[0].Args, "--store-path="+storePath,
		"the path is stated rather than left to the binary's default")

	mount := mountOf(t, pod, storePath)
	require.True(t, mount.ReadOnly, "the agent measures the store and never deletes anything there")
	require.NotNil(t, mount.MountPropagation)
	require.Equal(t, corev1.MountPropagationHostToContainer, *mount.MountPropagation,
		"a disk mounted there after the agent started has to be what it measures")

	volume := volumeOf(t, pod, mount.Name)
	require.Equal(t, storePath, volume.HostPath.Path)
	require.NotNil(t, volume.HostPath.Type)
	require.Equal(t, corev1.HostPathDirectoryOrCreate, *volume.HostPath.Type,
		"a node that never held a store has nothing there, and the agent has to start anyway")
}

// TestTheAgentWritesItsMetricsForTheNodeExporter is the agent's metrics reaching Prometheus.
//
// Nothing scrapes the agent: a kube-rbac-proxy beside it would authenticate against the API
// server it exists to outlive. It writes them into the node exporter's textfile directory
// instead, and that directory is a contract with monitoring-kubernetes, so the path is the
// shared constant.
func TestTheAgentWritesItsMetricsForTheNodeExporter(t *testing.T) {
	body := render(t, agentStep, agentRegistry())
	pod := staticPod(t, body)

	require.Contains(t, pod.Spec.Containers[0].Args, "--metrics-textfile="+registry_const.AgentMetricsTextfile)

	directory := filepath.Dir(registry_const.AgentMetricsTextfile)
	mount := mountOf(t, pod, directory)
	require.False(t, mount.ReadOnly, "the agent writes the file, and removes it when it stops")

	volume := volumeOf(t, pod, mount.Name)
	require.Equal(t, directory, volume.HostPath.Path, "the node exporter reads the node's directory, not whatever is mounted")
	require.NotNil(t, volume.HostPath.Type)
	require.Equal(t, corev1.HostPathDirectoryOrCreate, *volume.HostPath.Type,
		"the node exporter may not be there yet, or at all, and the agent has to start anyway")

	// Taken off the node, the agent leaves no file behind: the node exporter would go on
	// reporting its last counters and its last layout origin as current.
	require.Contains(t, effective(render(t, agentStep, legacyRegistry())),
		"rm -f "+registry_const.AgentMetricsTextfile)
}

// engineManifest is dhctl's copy of the Engine node's agent, which a template test holds equal
// to the module's NodeStaticPodRequest byte for byte.
const engineManifest = "../../../dhctl/pkg/immutable/registry-agent.yaml"

// TestTheAgentIsRestartedWhenItStopsAnswering is the agent that froze and was left that way.
//
// A frozen agent completes no handshake, so containerd waited out its handshake timeout on every
// pull and then pulled without the agent, where platform images cannot be pulled; with no probe
// the kubelet never restarted it, and the mirror pod stayed Ready, so nothing said so either.
func TestTheAgentIsRestartedWhenItStopsAnswering(t *testing.T) {
	pod := staticPod(t, render(t, agentStep, agentRegistry()))
	container := pod.Spec.Containers[0]

	host, port := listenAddress(t, container)
	for name, probe := range map[string]*corev1.Probe{
		"startup":   container.StartupProbe,
		"liveness":  container.LivenessProbe,
		"readiness": container.ReadinessProbe,
	} {
		require.NotNil(t, probe, "no %s probe", name)
		require.NotNil(t, probe.HTTPGet, "the %s probe is not an HTTP request", name)

		// The listener the runtime uses, over TLS: the probe passes only if a pull's handshake
		// would. The metrics listener answers whatever the proxy is doing.
		require.Equal(t, corev1.URISchemeHTTPS, probe.HTTPGet.Scheme, "%s probe", name)
		require.Equal(t, host, probe.HTTPGet.Host, "%s probe", name)
		require.Equal(t, port, probe.HTTPGet.Port.IntValue(), "%s probe", name)
		require.Equal(t, "/healthz", probe.HTTPGet.Path, "%s probe", name)
	}

	// A restart drops every pull in flight on the node, so it takes a minute of silence.
	liveness := container.LivenessProbe
	require.GreaterOrEqual(t, liveness.PeriodSeconds*liveness.FailureThreshold, int32(60),
		"liveness gives up on the agent in under a minute")
	require.Less(t, liveness.TimeoutSeconds, liveness.PeriodSeconds,
		"a probe that times out after the next one is due")

	// The agent liveness gives up on is frozen and cannot act on SIGTERM: the kubelet waits out
	// the whole grace period before it kills it, and the default 30 s is half a minute more of
	// pulls that fail.
	grace := pod.Spec.TerminationGracePeriodSeconds
	require.NotNil(t, grace, "the default grace period of 30 s")
	require.LessOrEqual(t, *grace, int64(15), "a frozen agent is killed only when the grace period ends")
}

// TestTheProbesFollowTheEndpoint is liveness restarting a healthy agent forever: probes on a
// port the agent no longer listens on fail every time.
func TestTheProbesFollowTheEndpoint(t *testing.T) {
	registry := agentRegistry()
	registry["agent"].(map[string]any)["endpoint"] = "127.0.0.1:4281"

	container := staticPod(t, render(t, agentStep, registry)).Spec.Containers[0]

	require.Contains(t, container.Args, "--listen-address=127.0.0.1:4281")
	require.Equal(t, 4281, container.LivenessProbe.HTTPGet.Port.IntValue())
	require.Equal(t, 4281, container.ReadinessProbe.HTTPGet.Port.IntValue())
	require.Equal(t, 4281, container.StartupProbe.HTTPGet.Port.IntValue())
}

// TestTheAgentIsTheSamePodOnEveryKindOfNode holds the bashible node's agent to the Engine node's
// in what the alerts read: the name, namespace and container D8RegistryAgentNotReady and
// D8RegistryAgentRestartingTooOften select, and the probes whose answers they report. One manifest
// drifting from the other would leave the alerts watching half the cluster. The grace period too:
// it decides how soon a frozen agent is replaced.
func TestTheAgentIsTheSamePodOnEveryKindOfNode(t *testing.T) {
	bashible := staticPod(t, render(t, agentStep, agentRegistry()))

	content, err := os.ReadFile(engineManifest)
	require.NoError(t, err)
	var engine corev1.Pod
	require.NoError(t, yaml.Unmarshal(content, &engine))
	require.Len(t, engine.Spec.Containers, 1)

	require.Equal(t, engine.Name, bashible.Name)
	require.Equal(t, engine.Namespace, bashible.Namespace)
	require.Equal(t, engine.Spec.Containers[0].Name, bashible.Spec.Containers[0].Name)

	require.Equal(t, engine.Spec.Containers[0].StartupProbe, bashible.Spec.Containers[0].StartupProbe)
	require.Equal(t, engine.Spec.Containers[0].LivenessProbe, bashible.Spec.Containers[0].LivenessProbe)
	require.Equal(t, engine.Spec.Containers[0].ReadinessProbe, bashible.Spec.Containers[0].ReadinessProbe)
	require.Equal(t, engine.Spec.TerminationGracePeriodSeconds, bashible.Spec.TerminationGracePeriodSeconds)
}

// listenAddress is where the container serves the runtime, from its own arguments.
func listenAddress(t *testing.T, container corev1.Container) (string, int) {
	t.Helper()

	for _, arg := range container.Args {
		address, ok := strings.CutPrefix(arg, "--listen-address=")
		if !ok {
			continue
		}
		host, port, err := net.SplitHostPort(address)
		require.NoError(t, err)
		number, err := strconv.Atoi(port)
		require.NoError(t, err)
		return host, number
	}
	require.Fail(t, "the agent is given no --listen-address")
	return "", 0
}

func mountOf(t *testing.T, pod *corev1.Pod, path string) corev1.VolumeMount {
	t.Helper()

	for _, mount := range pod.Spec.Containers[0].VolumeMounts {
		if filepath.Clean(mount.MountPath) == path {
			return mount
		}
	}
	require.Failf(t, "not mounted", "nothing is mounted at %s", path)
	return corev1.VolumeMount{}
}

func volumeOf(t *testing.T, pod *corev1.Pod, name string) corev1.Volume {
	t.Helper()

	for _, volume := range pod.Spec.Volumes {
		if volume.Name == name {
			require.NotNil(t, volume.HostPath, "volume %q", name)
			return volume
		}
	}
	require.Failf(t, "no volume", "the mount names volume %q, which the pod does not have", name)
	return corev1.Volume{}
}
