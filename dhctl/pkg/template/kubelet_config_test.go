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
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

// Bootstrap uses dhctl's renderer, not bashible-apiserver's. Exercise the real
// function map here too; the detailed matrix lives in bashible-apiserver.
func TestKubeletTopologyManagerBootstrapRendering(t *testing.T) {
	content, err := os.ReadFile("../../../candi/bashible/common-steps/all/064_configure_kubelet.sh.tpl")
	require.NoError(t, err)
	for _, policy := range []string{"", "None", "BestEffort", "Restricted", "SingleNumaNode"} {
		t.Run(policy, func(t *testing.T) {
			ng := map[string]interface{}{"name": "master"}
			if policy != "" {
				ng["kubelet"] = map[string]interface{}{
					"resourceReservation": map[string]interface{}{"mode": "Auto"},
					"topologyManager":     map[string]interface{}{"policy": policy, "scope": "Container"},
				}
			}
			rendered, err := RenderTemplate("064_configure_kubelet.sh.tpl", content, map[string]interface{}{
				"cri": "Containerd", "runType": "ClusterBootstrap", "kubernetesVersion": "1.34", "nodeGroup": ng,
				"clusterBootstrap": map[string]interface{}{"clusterDomain": "cluster.local", "clusterDNSAddress": "10.96.0.10"},
				"normal":           map[string]interface{}{"podSubnetNodeCIDRPrefix": "24"},
			})
			require.NoError(t, err)
			_, manifest, found := strings.Cut(rendered.Content.String(), "cat > \"${new_kubelet_config}\" << EOF\n")
			require.True(t, found)
			manifest, _, found = strings.Cut(manifest, "\nEOF\n")
			require.True(t, found)
			manifest = strings.ReplaceAll(manifest, "${numa_reserved_memory_yaml}", "reservedMemory:\n- numaNode: 0\n  limits:\n    memory: \"1\"")
			manifest = strings.ReplaceAll(manifest, "${numa_topology_options_yaml}", "")
			var config map[string]interface{}
			require.NoError(t, yaml.UnmarshalStrict([]byte(manifest), &config))
			switch policy {
			case "", "None":
				require.NotContains(t, config, "cpuManagerPolicy")
				require.NotContains(t, config, "memoryManagerPolicy")
				require.NotContains(t, config, "topologyManagerPolicy")
				require.NotContains(t, config, "topologyManagerScope")
			case "BestEffort":
				require.Equal(t, "static", config["cpuManagerPolicy"])
				require.Equal(t, "Static", config["memoryManagerPolicy"])
				require.Equal(t, "best-effort", config["topologyManagerPolicy"])
				require.Equal(t, "container", config["topologyManagerScope"])
				require.NotContains(t, config, "cpuManagerPolicyOptions")
			case "Restricted", "SingleNumaNode":
				require.Equal(t, "static", config["cpuManagerPolicy"])
				require.Equal(t, "Static", config["memoryManagerPolicy"])
				require.Equal(t, map[string]string{"Restricted": "restricted", "SingleNumaNode": "single-numa-node"}[policy], config["topologyManagerPolicy"])
				require.Equal(t, "container", config["topologyManagerScope"])
				require.Equal(t, map[string]interface{}{"full-pcpus-only": "true"}, config["cpuManagerPolicyOptions"])
			}
		})
	}
}
