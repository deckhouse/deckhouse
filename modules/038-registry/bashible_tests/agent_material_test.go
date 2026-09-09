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

// These tests read the static pod the agent step writes as the node reads it: the manifest is
// pulled out of its heredoc and the step's own shell variables are expanded from the step's own
// assignments, so a path renamed in one place cannot leave the test asserting the old one.
//
// What they are for is the width of the mount. The agent parses what an external registry answers,
// which makes code execution in it a case worth defending against, and it used to be handed all of
// `/etc/kubernetes` — on a master that directory is `pki/ca.key` and `admin.conf`, so that one
// mount turned a bug in a proxy into the cluster's certificate authority.
package bashible_tests

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	registry_const "github.com/deckhouse/deckhouse/go_lib/registry/const"
)

const agentStep = "all/053_configure_registry_agent.sh.tpl"

// TestTheAgentIsHandedNothingOfTheNodeItDoesNotRead is the boundary itself.
func TestTheAgentIsHandedNothingOfTheNodeItDoesNotRead(t *testing.T) {
	body := render(t, agentStep, agentRegistry())
	pod := staticPod(t, body)

	require.True(t, pod.Spec.HostNetwork, "the runtime reaches the agent on the node's loopback")
	require.Len(t, pod.Spec.Containers, 1)

	roots := map[string]string{}
	for _, volume := range pod.Spec.Volumes {
		require.NotNil(t, volume.HostPath, "volume %q is not the node's, and everything here is",
			volume.Name)
		roots[volume.Name] = filepath.Clean(volume.HostPath.Path)
	}

	// The one mount that was the finding. Nothing may hand the agent the directory itself:
	// what it needs of the node is its own subdirectory of it, and the kubelet's certificate.
	for name, path := range roots {
		require.NotEqual(t, "/etc/kubernetes", path,
			"volume %q hands the agent the control plane's PKI along with what it came for", name)
		require.False(t, path == "/etc/kubernetes/pki" || strings.HasPrefix(path, "/etc/kubernetes/pki/"),
			"volume %q reaches into the cluster's certificate authority", name)
	}

	mounted := map[string]corev1.VolumeMount{}
	for _, mount := range pod.Spec.Containers[0].VolumeMounts {
		mounted[filepath.Clean(mount.MountPath)] = mount
	}

	agentDirectory := stepVariable(t, body, "agent_path")
	require.Equal(t, "/etc/kubernetes/registry-agent", agentDirectory,
		"the agent's own directory, which is the only thing under /etc the agent is given")

	material, ok := mounted[agentDirectory]
	require.True(t, ok, "the agent reads its PKI, its bootstrap layout and its kubeconfig from here")
	require.True(t, material.ReadOnly, "a bashible step writes this material; the agent only reads it")

	kubeletPKI, ok := mounted["/var/lib/kubelet/pki"]
	require.True(t, ok, "the identity the agent reads the API with is the kubelet's certificate")
	require.True(t, kubeletPKI.ReadOnly)

	// Every path the agent is told to read has to be inside something mounted, or the flag
	// names a file the container cannot see. This is what removing the two nested mounts had
	// to preserve.
	for _, argument := range pod.Spec.Containers[0].Args {
		flag, value, found := strings.Cut(argument, "=")
		if !found || !strings.HasPrefix(value, "/") {
			continue
		}

		var reachable bool
		for path := range mounted {
			if value == path || strings.HasPrefix(value, path+"/") {
				reachable = true
				break
			}
		}
		require.True(t, reachable, "%s points at %s, which is not mounted into the container",
			flag, value)
	}
}

// TestTheAgentReadsTheAPIWithItsOwnKubeconfig covers the file that replaced the mount.
//
// It has to carry the kubelet's rotating certificate by reference rather than a copy of any
// credential: a copy is a secret on disk that goes stale, and staleness here is a node that
// silently stops applying what the cluster configures.
func TestTheAgentReadsTheAPIWithItsOwnKubeconfig(t *testing.T) {
	body := render(t, agentStep, agentRegistry())

	kubeconfig := stepVariable(t, body, "agent_kubeconfig")
	require.Equal(t, stepVariable(t, body, "agent_path")+"/kubeconfig", kubeconfig,
		"it lives with the rest of the agent's material, which is what makes the narrow mount work")

	require.Contains(t, body, `bb-sync-file "${agent_kubeconfig}"`,
		"written on every pass, so the answer is never decided at the one moment it is 'not yet'")
	require.Contains(t, body, "client-certificate: /var/lib/kubelet/pki/kubelet-client-current.pem",
		"the kubelet's identity, by reference to the certificate the kubelet rotates")
	require.NotContains(t, body, "token:",
		"no credential is copied into this file")

	// Either copy of the cluster CA, because neither is present the whole time: step 098
	// clears the bashible one after bootstrap, and step 060 has not yet written the other
	// when this step first runs.
	require.Contains(t, body, "/var/lib/bashible/ca.crt")
	require.Contains(t, body, "/etc/kubernetes/pki/ca.crt")
	require.Contains(t, body, `certificate-authority-data: $(base64 -w0 < "${agent_ca}")`)

	// And written only when one of them is readable: an absent kubeconfig is the "no
	// credentials yet" path the agent handles, a truncated one is retried forever.
	require.Contains(t, body, `if [[ -z "${agent_ca}" ]]`)

	pod := staticPod(t, body)
	require.Contains(t, pod.Spec.Containers[0].Args, "--kubeconfig="+kubeconfig,
		"the manifest names the file, so the binary's default is not what decides")
}

// staticPod returns the manifest the step writes, as the kubelet would read it.
func staticPod(t *testing.T, body string) *corev1.Pod {
	t.Helper()

	const marker = "bb-sync-file /etc/kubernetes/manifests/registry-agent.yaml - << EOF\n"

	start := strings.Index(body, marker)
	require.GreaterOrEqual(t, start, 0, "the step does not write the static pod at all")

	manifest := body[start+len(marker):]
	end := strings.Index(manifest, "\nEOF\n")
	require.GreaterOrEqual(t, end, 0, "the manifest heredoc is not terminated")

	var pod corev1.Pod
	require.NoError(t, yaml.Unmarshal([]byte(expandStepVariables(t, body, manifest[:end])), &pod),
		"the step writes a manifest the kubelet cannot parse")

	return &pod
}

var stepAssignment = regexp.MustCompile(`(?m)^([a-z_]+)="([^"]*)"$`)

// stepVariables collects what the step assigns, expanded the way the shell would.
//
// Read from the step rather than restated here so that these tests follow a renamed path instead
// of asserting the one it used to have. Two of the assignments are not literals: the drop-in root
// comes from the node context through `dirname`, and the mount snippets are heredocs.
func stepVariables(t *testing.T, body string) map[string]string {
	t.Helper()

	values := map[string]string{
		"drop_in_root": filepath.Dir(filepath.Dir(registry_const.AgentDropInFile)),
		"agent_image":  "deckhouse.local/images:registry-agent",
		"kubeconfig_mount": heredoc(t, body,
			`kubeconfig_mount="$(cat << "MOUNT"`, "\nMOUNT\n"),
		"kubeconfig_volume": heredoc(t, body,
			`kubeconfig_volume="$(cat << "VOLUME"`, "\nVOLUME\n"),
	}

	for _, match := range stepAssignment.FindAllStringSubmatch(body, -1) {
		name, value := match[1], match[2]
		if _, taken := values[name]; taken {
			continue
		}
		if strings.Contains(value, "$(") {
			continue
		}
		values[name] = value
	}

	// Assignments refer to each other, so expand until nothing left refers to anything.
	for range len(values) {
		for name, value := range values {
			values[name] = replaceStepVariables(value, values)
		}
	}

	return values
}

func stepVariable(t *testing.T, body, name string) string {
	t.Helper()

	value, ok := stepVariables(t, body)[name]
	require.True(t, ok, "the step assigns no %s", name)
	return value
}

func expandStepVariables(t *testing.T, body, text string) string {
	t.Helper()

	values := stepVariables(t, body)
	expanded := replaceStepVariables(text, values)
	require.NotContains(t, expanded, "${",
		"a shell variable in the manifest is not one this test knows how to expand")

	return expanded
}

func replaceStepVariables(text string, values map[string]string) string {
	for name, value := range values {
		text = strings.ReplaceAll(text, "${"+name+"}", value)
	}
	return text
}

// heredoc returns the body of a quoted heredoc assignment, which the shell keeps verbatim.
func heredoc(t *testing.T, body, opening, closing string) string {
	t.Helper()

	start := strings.Index(body, opening)
	require.GreaterOrEqual(t, start, 0, "the step no longer assigns %s", opening)

	rest := body[start+len(opening):]
	end := strings.Index(rest, closing)
	require.GreaterOrEqual(t, end, 0, "the heredoc opened by %s is not terminated", opening)

	return strings.TrimPrefix(rest[:end], "\n")
}
