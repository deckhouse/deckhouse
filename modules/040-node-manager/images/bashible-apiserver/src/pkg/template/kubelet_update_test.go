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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestKubeletTopologyOptions(t *testing.T) {
	for _, mode := range []string{"Disabled", "None", "BestEffort", "Restricted", "SingleNumaNode"} {
		for _, tc := range []struct {
			name     string
			ids      []int
			distance string
			closest  bool
			max      string
		}{
			{"single", []int{0}, "valid", true, ""},
			{"two", []int{0, 1}, "valid", true, ""},
			{"sparse compact matrix", []int{2, 10}, "valid", false, ""},
			{"missing distance", []int{0, 1}, "", false, ""},
			{"short row", []int{0, 1}, "10\n", false, ""},
			{"eight nodes", []int{0, 1, 2, 3, 4, 5, 6, 7}, "valid", true, ""},
			{"nine including memoryless", []int{0, 1, 2, 3, 4, 5, 6, 7, 8}, "valid", true, "9"},
			{"nine missing distance", []int{0, 1, 2, 3, 4, 5, 6, 7, 8}, "", false, "9"},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				result := executeKubeletStep(t, reservedKubelet(mode, "1Gi"), "4096", func(root string) {
					for i, id := range tc.ids {
						memory := int64(0)
						if i == 0 {
							memory = 4
						}
						writeNUMAFixture(t, root, numaFixture{id: id, gib: memory})
						if tc.distance == "valid" {
							var row []string
							for j := range tc.ids {
								if i == j {
									row = append(row, "10")
								} else {
									row = append(row, "20")
								}
							}
							writeFixtureFile(t, root, fmt.Sprintf("node%d/distance", id), strings.Join(row, " ")+"\n")
						} else if tc.distance != "" {
							writeFixtureFile(t, root, fmt.Sprintf("node%d/distance", id), tc.distance)
						}
					}
				})
				config := result.success(t)
				pinning := mode != "Disabled" && mode != "None"
				options := map[string]interface{}{}
				closestPolicy := mode == "Restricted"
				if closestPolicy && tc.closest {
					options["prefer-closest-numa-nodes"] = "true"
				}
				if pinning && tc.max != "" {
					options["max-allowable-numa-nodes"] = tc.max
				}
				if len(options) == 0 {
					require.NotContains(t, config, "topologyManagerPolicyOptions")
				} else {
					require.Equal(t, options, config["topologyManagerPolicyOptions"])
				}
				if closestPolicy && !tc.closest {
					require.Contains(t, result.output, "prefer-closest-numa-nodes is not set")
				} else {
					require.NotContains(t, result.output, "prefer-closest-numa-nodes is not set")
				}
			})
		}
	}
}

func TestKubeletApprovalBeforeSync(t *testing.T) {
	arrange := func(root string) { writeNUMAFixture(t, root, numaFixture{id: 0, gib: 4}) }
	baseline := executeKubeletUpdate(t, reservedKubelet("SingleNumaNode", "1Gi"), "4096", arrange, kubeletUpdateFixture{bootstrap: true})
	baseline.success(t)
	require.Equal(t, "sync\nkubelet-need-restart\n", baseline.events, "bootstrap must not query pods or ask for approval")
	changed := kubeletConfigMutation(t, baseline.config, func(config map[string]interface{}) { config["topologyManagerScope"] = "container" })
	guaranteed := podList("Running", "Guaranteed", "worker-1")
	for _, tc := range []struct {
		name   string
		update kubeletUpdateFixture
		events string
	}{
		{"running Guaranteed", kubeletUpdateFixture{pods: guaranteed}, "pods\napproval\nsync\nkubelet-need-restart\n"},
		{"running Burstable", kubeletUpdateFixture{pods: podList("Running", "Burstable", "worker-1")}, "pods\nsync\nkubelet-need-restart\n"},
		{"running BestEffort", kubeletUpdateFixture{pods: podList("Running", "BestEffort", "worker-1")}, "pods\nsync\nkubelet-need-restart\n"},
		{"pending Guaranteed", kubeletUpdateFixture{pods: podList("Pending", "Guaranteed", "worker-1")}, "pods\nsync\nkubelet-need-restart\n"},
		{"completed Guaranteed", kubeletUpdateFixture{pods: podList("Succeeded", "Guaranteed", "worker-1")}, "pods\nsync\nkubelet-need-restart\n"},
		{"empty pods", kubeletUpdateFixture{}, "pods\nsync\nkubelet-need-restart\n"},
		{"missing metadata", kubeletUpdateFixture{pods: `{"kind":"PodList","items":[]}`}, "pods\nsync\nkubelet-need-restart\n"},
		{"pagination", kubeletUpdateFixture{pods: `{"kind":"PodList","metadata":{"continue":"next"},"items":[]}`}, "pods\nsync\nkubelet-need-restart\n"},
		{"invalid continue", kubeletUpdateFixture{pods: `{"kind":"PodList","metadata":{"continue":false},"items":[]}`}, "pods\nsync\nkubelet-need-restart\n"},
		{"wrong node", kubeletUpdateFixture{pods: podList("Running", "Guaranteed", "other-node")}, "pods\napproval\nsync\nkubelet-need-restart\n"},
		{"unknown QoS", kubeletUpdateFixture{pods: podList("Running", "", "worker-1")}, "pods\nsync\nkubelet-need-restart\n"},
		{"missing status", kubeletUpdateFixture{pods: `{"kind":"PodList","metadata":{},"items":[{"spec":{"nodeName":"worker-1"}}]}`}, "pods\nsync\nkubelet-need-restart\n"},
		{"multiple documents", kubeletUpdateFixture{pods: guaranteed + "\n" + guaranteed}, "pods\napproval\nsync\nkubelet-need-restart\n"},
		{"DaemonSet pod", kubeletUpdateFixture{pods: `{"kind":"PodList","items":[{"metadata":{"ownerReferences":[{"kind":"DaemonSet"}]},"status":{"phase":"Running","qosClass":"Guaranteed"}}]}`}, "pods\nsync\nkubelet-need-restart\n"},
		{"mirror pod", kubeletUpdateFixture{pods: `{"kind":"PodList","items":[{"metadata":{"annotations":{"kubernetes.io/config.mirror":"x"}},"status":{"phase":"Running","qosClass":"Guaranteed"}}]}`}, "pods\nsync\nkubelet-need-restart\n"},
		{"ReplicaSet pod", kubeletUpdateFixture{pods: `{"kind":"PodList","items":[{"metadata":{"ownerReferences":[{"kind":"ReplicaSet"}]},"status":{"phase":"Running","qosClass":"Guaranteed"}}]}`}, "pods\napproval\nsync\nkubelet-need-restart\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.update.oldConfig = changed
			result := executeKubeletUpdate(t, reservedKubelet("SingleNumaNode", "1Gi"), "4096", arrange, tc.update)
			result.success(t)
			require.Equal(t, tc.events, result.events)
		})
	}
	for _, tc := range []struct {
		name   string
		update kubeletUpdateFixture
		events string
	}{
		{"API error", kubeletUpdateFixture{apiFailure: true}, "pods\n"},
		{"invalid JSON", kubeletUpdateFixture{pods: "invalid"}, "pods\n"},
		{"Status response", kubeletUpdateFixture{pods: `{"kind":"Status","message":"forbidden"}`}, "pods\n"},
		{"missing items", kubeletUpdateFixture{pods: `{"kind":"PodList","metadata":{}}`}, "pods\n"},
		{"null items", kubeletUpdateFixture{pods: `{"kind":"PodList","metadata":{},"items":null}`}, "pods\n"},
		{"unknown node identity", kubeletUpdateFixture{nodeFailure: true}, ""},
	} {
		// Unknown pod state fails the step before sync: bashible retries it instead of a blind drain.
		t.Run("unknown pods/"+tc.name, func(t *testing.T) {
			tc.update.oldConfig = changed
			result := executeKubeletUpdate(t, reservedKubelet("SingleNumaNode", "1Gi"), "4096", arrange, tc.update)
			require.Error(t, result.err)
			require.False(t, result.synced)
			require.Equal(t, changed, string(result.config))
			require.Equal(t, tc.events, result.events)
		})
	}
	t.Run("first run skips pod check", func(t *testing.T) {
		result := executeKubeletUpdate(t, reservedKubelet("SingleNumaNode", "1Gi"), "4096", arrange, kubeletUpdateFixture{oldConfig: changed, apiFailure: true, firstRun: true})
		result.success(t)
		require.Equal(t, "sync\nkubelet-need-restart\n", result.events)
	})
	t.Run("approval denied", func(t *testing.T) {
		result := executeKubeletUpdate(t, reservedKubelet("SingleNumaNode", "1Gi"), "4096", arrange, kubeletUpdateFixture{oldConfig: changed, pods: guaranteed, approvalFailure: true})
		require.Error(t, result.err)
		require.False(t, result.synced)
		require.Equal(t, changed, string(result.config))
		require.Equal(t, "pods\napproval\n", result.events)
	})
	t.Run("identical repeat", func(t *testing.T) {
		result := executeKubeletUpdate(t, reservedKubelet("SingleNumaNode", "1Gi"), "4096", arrange, kubeletUpdateFixture{oldConfig: string(baseline.config), pods: guaranteed})
		result.success(t)
		require.Equal(t, baseline.config, result.config)
		require.Equal(t, "sync\n", result.events)
	})
	for _, old := range []string{"invalid: [", "# comment only\n"} {
		t.Run("unreadable old/"+old, func(t *testing.T) {
			result := executeKubeletUpdate(t, reservedKubelet("SingleNumaNode", "1Gi"), "4096", arrange, kubeletUpdateFixture{oldConfig: old, pods: guaranteed})
			result.success(t)
			require.Equal(t, "pods\napproval\nsync\nkubelet-need-restart\n", result.events)
		})
	}
	for _, mode := range []string{"Disabled", "None"} {
		t.Run(mode+" ignores old Static and pods", func(t *testing.T) {
			result := executeKubeletUpdate(t, reservedKubelet(mode, "1Gi"), "4096", nil, kubeletUpdateFixture{oldConfig: string(baseline.config), pods: guaranteed, apiFailure: true})
			result.success(t)
			require.Equal(t, "sync\nkubelet-need-restart\n", result.events)
		})
	}
}

func TestKubeletApprovalProjection(t *testing.T) {
	arrange := func(root string) {
		writeNUMAFixture(t, root, numaFixture{id: 0, gib: 4})
		writeNUMAFixture(t, root, numaFixture{id: 1, gib: 4})
	}
	baseline := executeKubeletUpdate(t, reservedKubelet("SingleNumaNode", "1Gi"), "8192", arrange, kubeletUpdateFixture{bootstrap: true})
	baseline.success(t)
	for _, tc := range []struct {
		name     string
		mutate   func(map[string]interface{})
		approval bool
	}{
		{"CPU policy", func(c map[string]interface{}) { c["cpuManagerPolicy"] = "none" }, true},
		{"memory policy", func(c map[string]interface{}) { c["memoryManagerPolicy"] = "None" }, true},
		{"topology policy", func(c map[string]interface{}) { c["topologyManagerPolicy"] = "restricted" }, true},
		{"topology scope", func(c map[string]interface{}) { c["topologyManagerScope"] = "container" }, true},
		{"CPU options", func(c map[string]interface{}) { delete(c, "cpuManagerPolicyOptions") }, true},
		{"NUMA reservation", func(c map[string]interface{}) {
			c["reservedMemory"].([]interface{})[0].(map[string]interface{})["limits"].(map[string]interface{})["memory"] = "1"
		}, true},
		{"system reservation", func(c map[string]interface{}) { c["systemReserved"].(map[string]interface{})["memory"] = "2Gi" }, true},
		{"fractional CPU differs", func(c map[string]interface{}) { c["systemReserved"].(map[string]interface{})["cpu"] = "500m" }, true},
		{"foreign system reservation", func(c map[string]interface{}) { c["systemReserved"].(map[string]interface{})["pid"] = "100" }, false},
		{"ephemeral-storage reservation", func(c map[string]interface{}) {
			c["systemReserved"].(map[string]interface{})["ephemeral-storage"] = "5Gi"
		}, false},
		{"only topology options", func(c map[string]interface{}) {
			c["topologyManagerPolicyOptions"] = map[string]interface{}{"prefer-closest-numa-nodes": "true"}
		}, false},
		{"other kubelet setting", func(c map[string]interface{}) { c["maxPods"] = 42 }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			old := kubeletConfigMutation(t, baseline.config, tc.mutate)
			result := executeKubeletUpdate(t, reservedKubelet("SingleNumaNode", "1Gi"), "8192", arrange, kubeletUpdateFixture{oldConfig: old, pods: podList("Running", "Guaranteed", "worker-1")})
			result.success(t)
			if tc.approval {
				require.Equal(t, "pods\napproval\nsync\nkubelet-need-restart\n", result.events)
			} else {
				require.Equal(t, "sync\nkubelet-need-restart\n", result.events)
			}
		})
	}
}

func kubeletConfigMutation(t *testing.T, config []byte, mutate func(map[string]interface{})) string {
	t.Helper()
	var fields map[string]interface{}
	require.NoError(t, yaml.UnmarshalStrict(config, &fields))
	mutate(fields)
	result, err := yaml.Marshal(fields)
	require.NoError(t, err)
	return "# reformatted old config\n" + string(result)
}

func podList(phase, qos, node string) string {
	return fmt.Sprintf(`{"kind":"PodList","metadata":{},"items":[{"spec":{"nodeName":%q},"status":{"phase":%q,"qosClass":%q}}]}`, node, phase, qos)
}
