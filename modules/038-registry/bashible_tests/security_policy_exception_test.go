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
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const agentException = "../templates/v2/agent-security-policy-exception.yaml"

// TestTheAgentExceptionMatchesTheStaticPod keeps the module's exception and the step's manifest in
// step. They live in two trees and are rendered by two renderers, and admission-policy-engine
// excepts a hostPath only when both the path and readOnly equal the mount's: a mount changed in
// the step alone leaves the agent refused once the policy enforces, on the pod every other pull
// on the node goes through.
func TestTheAgentExceptionMatchesTheStaticPod(t *testing.T) {
	pod := staticPod(t, render(t, agentStep, agentRegistry()))
	spe := readAgentException(t)

	require.Equal(t, "kube-system", pod.Namespace, "an exception is looked up in the pod's namespace")
	require.Equal(t, spe.Metadata.Name, pod.Labels["security.deckhouse.io/security-policy-exception"],
		"the static pod does not name the exception the module renders for it")
	require.Equal(t, "kube-system", spe.Metadata.Namespace)

	type hostPath struct {
		Path     string
		ReadOnly bool
	}
	mounted := map[hostPath]bool{}
	for _, volume := range pod.Spec.Volumes {
		require.NotNil(t, volume.HostPath, "volume %q", volume.Name)
		for _, mount := range pod.Spec.Containers[0].VolumeMounts {
			if mount.Name == volume.Name {
				mounted[hostPath{volume.HostPath.Path, mount.ReadOnly}] = true
			}
		}
	}
	excepted := map[hostPath]bool{}
	for _, entry := range spe.Spec.Volumes.HostPath.AllowedValues {
		excepted[hostPath{entry.Path, entry.ReadOnly}] = true
	}
	require.Equal(t, mounted, excepted, "the exception's host paths are not the ones the agent mounts")
	require.Equal(t, []string{"hostPath"}, spe.Spec.Volumes.Types.AllowedValues)

	require.True(t, pod.Spec.HostNetwork)
	require.True(t, spe.Spec.Network.HostNetwork.AllowedValue)
	for _, container := range pod.Spec.Containers {
		require.Empty(t, container.Ports, "a port on the host network needs a hostPorts entry in the exception")
	}
	require.Empty(t, spe.Spec.Network.HostPorts)

	sc := pod.Spec.Containers[0].SecurityContext
	require.NotNil(t, sc)
	require.Equal(t, int64(0), *sc.RunAsUser)
	require.Equal(t, []int64{0}, spe.Spec.SecurityContext.RunAsUser.AllowedValues)
	require.False(t, spe.Spec.SecurityContext.RunAsNonRoot.AllowedValue)
}

// agentExceptionObject is the part of the kind the agent's exception uses.
type agentExceptionObject struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		SecurityContext struct {
			RunAsNonRoot struct {
				AllowedValue bool `json:"allowedValue"`
			} `json:"runAsNonRoot"`
			RunAsUser struct {
				AllowedValues []int64 `json:"allowedValues"`
			} `json:"runAsUser"`
		} `json:"securityContext"`
		Network struct {
			HostNetwork struct {
				AllowedValue bool `json:"allowedValue"`
			} `json:"hostNetwork"`
			HostPorts []any `json:"hostPorts"`
		} `json:"network"`
		Volumes struct {
			Types struct {
				AllowedValues []string `json:"allowedValues"`
			} `json:"types"`
			HostPath struct {
				AllowedValues []struct {
					Path     string `json:"path"`
					ReadOnly bool   `json:"readOnly"`
				} `json:"allowedValues"`
			} `json:"hostPath"`
		} `json:"volumes"`
	} `json:"spec"`
}

var templateComment = regexp.MustCompile(`(?s)\{\{-?\s*/\*.*?\*/\s*-?\}\}`)

// readAgentException reads the exception from the module's template without Helm.
//
// It can, because the template is static YAML between its gates: every templated line is an
// action of its own — a condition, its end, or the module labels — and none of them is a value
// this test compares. A value templated into the object would be dropped here and fail the test
// rather than pass it.
func readAgentException(t *testing.T) agentExceptionObject {
	t.Helper()

	content, err := os.ReadFile(agentException)
	require.NoError(t, err)

	var lines []string
	for _, line := range strings.Split(templateComment.ReplaceAllString(string(content), ""), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{{") || trimmed == "---" {
			continue
		}
		require.NotContains(t, line, "{{", "a templated value in the exception: %q", line)
		lines = append(lines, line)
	}

	var spe agentExceptionObject
	require.NoError(t, yaml.Unmarshal([]byte(strings.Join(lines, "\n")), &spe))
	return spe
}
