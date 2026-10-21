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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

const kubeletStepName = "064_configure_kubelet.sh.tpl"
const kubeletStepPath = "../../../../../../../candi/bashible/common-steps/all/" + kubeletStepName

func TestKubeletTopologyManagerPolicies(t *testing.T) {
	type expectation struct {
		name           string
		kubelet        map[string]interface{}
		cpuPolicy      string
		memoryPolicy   string
		topologyPolicy string
		topologyScope  string
		cpuOptions     map[string]interface{}
	}
	tm := func(fields map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{"topologyManager": fields}
	}
	strict := map[string]interface{}{"full-pcpus-only": "true"}
	cases := []expectation{
		{name: "absent"},
		{name: "empty topologyManager", kubelet: tm(map[string]interface{}{})},
		{name: "None", kubelet: tm(map[string]interface{}{"policy": "None", "scope": "Container"})},
		{name: "deprecated enabled true alone", kubelet: tm(map[string]interface{}{"enabled": true})},
		{name: "deprecated enabled false is ignored", kubelet: tm(map[string]interface{}{"enabled": false, "policy": "SingleNumaNode"}),
			cpuPolicy: "static", memoryPolicy: "Static", topologyPolicy: "single-numa-node", topologyScope: "pod", cpuOptions: strict},
	}
	for _, policy := range []struct {
		api, kubelet, memory string
		options              map[string]interface{}
	}{
		{"BestEffort", "best-effort", "Static", nil}, {"Restricted", "restricted", "Static", strict}, {"SingleNumaNode", "single-numa-node", "Static", strict},
	} {
		for _, scope := range []struct{ api, kubelet string }{{"", "pod"}, {"Container", "container"}, {"Pod", "pod"}} {
			fields := map[string]interface{}{"policy": policy.api}
			if scope.api != "" {
				fields["scope"] = scope.api
			}
			cases = append(cases, expectation{
				name: policy.api + "/" + scope.api, kubelet: tm(fields),
				cpuPolicy: "static", memoryPolicy: policy.memory, topologyPolicy: policy.kubelet, topologyScope: scope.kubelet, cpuOptions: policy.options,
			})
		}
	}
	optional := func(t *testing.T, config map[string]interface{}, key, value string) {
		t.Helper()
		if value == "" {
			require.NotContains(t, config, key)
		} else {
			require.Equal(t, value, config[key])
		}
	}
	for _, version := range []string{"1.33", "1.34", "1.35", "1.36", "1.37"} {
		for _, tc := range cases {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				kubelet := map[string]interface{}{"resourceReservation": map[string]interface{}{"mode": "Auto"}}
				for key, value := range tc.kubelet {
					kubelet[key] = value
				}
				body := renderKubeletStep(t, kubeletTemplateContext(version, "Normal", kubelet))
				config := kubeletConfigFromStep(t, body)
				optional(t, config, "cpuManagerPolicy", tc.cpuPolicy)
				optional(t, config, "memoryManagerPolicy", tc.memoryPolicy)
				optional(t, config, "topologyManagerPolicy", tc.topologyPolicy)
				optional(t, config, "topologyManagerScope", tc.topologyScope)
				if tc.cpuOptions == nil {
					require.NotContains(t, config, "cpuManagerPolicyOptions")
				} else {
					require.Equal(t, tc.cpuOptions, config["cpuManagerPolicyOptions"])
				}
				if tc.memoryPolicy == "Static" {
					require.NotEmpty(t, config["reservedMemory"])
				} else {
					require.NotContains(t, config, "reservedMemory")
				}
				require.Equal(t, "70m", config["systemReserved"].(map[string]interface{})["cpu"], "Auto CPU reserve must stay unchanged")
				require.NotContains(t, config, "reservedSystemCPUs")
				require.Equal(t, "50Mi", config["containerLogMaxSize"])
				require.EqualValues(t, 4, config["containerLogMaxFiles"])
			})
		}
	}
}

func TestKubeletBootstrapWithoutKubelet(t *testing.T) {
	for _, runType := range []string{"Normal", "ClusterBootstrap"} {
		t.Run(runType, func(t *testing.T) {
			body := renderKubeletStep(t, kubeletTemplateContext("1.34", runType, nil))
			config := kubeletConfigFromStep(t, body)
			for _, key := range []string{"cpuManagerPolicy", "memoryManagerPolicy", "reservedMemory", "topologyManagerPolicy", "topologyManagerScope", "cpuManagerPolicyOptions"} {
				require.NotContains(t, config, key)
			}
			require.Equal(t, "50Mi", config["containerLogMaxSize"])
			require.EqualValues(t, 4, config["containerLogMaxFiles"])
			require.Equal(t, true, config["failSwapOn"])
		})
	}
}

func TestKubeletStepsStorageRendering(t *testing.T) {
	content, err := os.ReadFile(kubeletStepPath)
	require.NoError(t, err)
	root := t.TempDir()
	directory := filepath.Join(root, "bashible", "common-steps", "all")
	require.NoError(t, os.MkdirAll(directory, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(directory, kubeletStepName), content, 0600))
	storage := NewStepsStorage(t.Context(), root, nil)
	steps, err := storage.Render("all", "", kubeletTemplateContext("1.34", "Normal", map[string]interface{}{
		"topologyManager":     map[string]interface{}{"policy": "SingleNumaNode"},
		"resourceReservation": map[string]interface{}{"mode": "Auto"},
	}))
	require.NoError(t, err)
	config := kubeletConfigFromStep(t, steps["064_configure_kubelet.sh"])
	require.Equal(t, "single-numa-node", config["topologyManagerPolicy"])
	require.Equal(t, map[string]interface{}{"full-pcpus-only": "true"}, config["cpuManagerPolicyOptions"])
}

func kubeletTemplateContext(version, runType string, kubelet map[string]interface{}) map[string]interface{} {
	ng := map[string]interface{}{"name": "worker"}
	if kubelet != nil {
		ng["kubelet"] = kubelet
	}
	return map[string]interface{}{
		"cri": "Containerd", "runType": runType, "kubernetesVersion": version, "nodeGroup": ng,
		"normal":           map[string]interface{}{"clusterDomain": "cluster.local", "clusterDNSAddress": "10.96.0.10", "podSubnetNodeCIDRPrefix": "24"},
		"clusterBootstrap": map[string]interface{}{"clusterDomain": "cluster.local", "clusterDNSAddress": "10.96.0.10"},
	}
}

func renderKubeletStep(t *testing.T, ctx map[string]interface{}) string {
	t.Helper()
	content, err := os.ReadFile(kubeletStepPath)
	require.NoError(t, err)
	rendered, err := RenderTemplate(kubeletStepName, content, ctx)
	require.NoError(t, err)
	body := rendered.Content.String()
	// Syntax checking parses the real script without executing writes to /var/lib.
	command := exec.Command("bash", "-n")
	command.Stdin = strings.NewReader(body)
	output, err := command.CombinedOutput()
	require.NoError(t, err, "%s", output)
	return body
}

func kubeletConfigFromStep(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	const marker = "cat > \"${new_kubelet_config}\" << EOF\n"
	_, manifest, found := strings.Cut(body, marker)
	require.True(t, found, "step must contain the kubelet config heredoc")
	manifest, _, found = strings.Cut(manifest, "\nEOF\n")
	require.True(t, found, "kubelet config heredoc must be terminated")
	// This check covers template structure. Execution tests verify the actual
	// multiline reservation fragment produced from sysfs.
	fragment := "reservedMemory:\n- numaNode: 0\n  limits:\n    memory: \"1\""
	manifest = strings.ReplaceAll(manifest, "${numa_reserved_memory_yaml}", fragment)
	manifest = strings.ReplaceAll(manifest, "${numa_topology_options_yaml}", "")
	var config map[string]interface{}
	require.NoError(t, yaml.UnmarshalStrict([]byte(manifest), &config), "rendered YAML must parse without duplicate keys")
	return config
}
