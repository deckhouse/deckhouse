/*
Copyright 2021 Flant JSC

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

package hooks

import (
	"bytes"
	"fmt"
	"strconv"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/apis/v1alpha1"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// Test calculateUpdateEpoch method with different input cases:
// - different clusterID or nodeGroup should return different epoch numbers
// - also test epoch calculation with different timestamps and for edge cases
func Test_calculateUpdateEpoch(t *testing.T) {
	clusterID := "test-cluster-1"
	nodeGroup := "test-node-group-1"

	ts0 := 20001
	epochStr0 := calculateUpdateEpoch(int64(ts0), clusterID, nodeGroup)
	epoch0, _ := strconv.Atoi(epochStr0)

	if ts0 > epoch0 {
		t.Fatalf("epoch for %d should not be smaller. Got: %d", ts0, epoch0)
	}

	// Test different clusterID and nodeGroupName.
	// 1. epoch for different clusters should be different for the same timestamp and node group name
	epochStr1 := calculateUpdateEpoch(int64(ts0), "test-cluster-2", nodeGroup)
	epoch1, _ := strconv.Atoi(epochStr1)
	if epoch0 == epoch1 {
		t.Fatalf("epoch for same ts == %d but different cluster should not be equal to %d. Got: %d", ts0, epoch0, epoch1)
	}

	// 2. epoch for different node groups should be different for the same timestamp and cluster.
	epochStr1 = calculateUpdateEpoch(int64(ts0), clusterID, "test-node-group-2")
	epoch1, _ = strconv.Atoi(epochStr1)
	if epoch0 == epoch1 {
		t.Fatalf("epoch for same ts == %d but different node group should not be equal to %d. Got: %d", ts0, epoch0, epoch1)
	}

	// Timestamp cases.
	// epoch for ts==epoch is epoch
	ts1 := int64(epoch0)
	epochStr1 = calculateUpdateEpoch(ts1, clusterID, nodeGroup)
	epoch1, _ = strconv.Atoi(epochStr1)

	if epoch1 != epoch0 {
		t.Fatalf("epoch for ts == epoch (%d) should be equal to epoch. Got: %d", ts1, epoch1)
	}

	// Previous timestamps.
	// epoch for ts==epoch-1 is epoch
	ts1 = int64(epoch0 - 1)
	epochStr1 = calculateUpdateEpoch(ts1, clusterID, nodeGroup)
	epoch1, _ = strconv.Atoi(epochStr1)

	if epoch1 != epoch0 {
		t.Fatalf("epoch for ts == epoch-1 (%d) should be equal to %d. Got: %d", ts1, epoch0, epoch1)
	}

	// epoch for window start ts==(epoch - window size + 1) is epoch
	ts1 = int64(epoch0 - int(EpochWindowSize) + 1)
	epochStr1 = calculateUpdateEpoch(ts1, clusterID, nodeGroup)
	epoch1, _ = strconv.Atoi(epochStr1)

	if epoch1 != epoch0 {
		t.Fatalf("epoch for ts == epoch-14400+1 (%d) should be equal to %d. Got: %d", ts1, epoch0, epoch1)
	}

	// epoch for ts==(epoch - window size) should be a previous epoch
	ts1 = int64(epoch0 - int(EpochWindowSize))
	epochStr1 = calculateUpdateEpoch(ts1, clusterID, nodeGroup)
	epoch1, _ = strconv.Atoi(epochStr1)

	if epoch1 != epoch0-int(EpochWindowSize) {
		t.Fatalf("epoch for ts == epoch-14400 (%d) should not be equal to %d. Got: %d", ts1, epoch0, epoch1)
	}

	// Future timestamp.
	// epoch for ts==epoch+1 is the next epoch
	ts1 = int64(epoch0 + 1)
	epochStr1 = calculateUpdateEpoch(ts1, clusterID, nodeGroup)
	epoch1, _ = strconv.Atoi(epochStr1)

	if epoch1 != epoch0+int(EpochWindowSize) {
		t.Fatalf("epoch for ts == epoch+1 (%d) should be the next epoch (%d). Got: %d", ts1, epoch0+14400, epoch1)
	}
}

const TestTimestampForUpdateEpoch int64 = 1234567890

// calculateEpoch is a helper to minimize test changes during implementation of Go hooks.
func calculateEpoch(ngName string, clusterUUID string) string {
	return calculateUpdateEpoch(TestTimestampForUpdateEpoch, clusterUUID, ngName)
}

// calculateEpoch is a helper to minimize test changes during implementation of Go hooks.
func setK8sVersionAsClusterConfig(f *HookExecutionConfig, version string) {
	cnf := fmt.Sprintf(`
apiVersion: deckhouse.io/v1
cloud:
  prefix: sandbox
  provider: vSphere
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "%s"
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`, version)

	f.ValuesSetFromYaml("global.clusterConfiguration", []byte(cnf))
}

var _ = Describe("Modules :: node-manager :: hooks :: get_crds ::", func() {
	const (
		stateNGProper = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: proper1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: proper2
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper2
    zones: [a,b]
`
		stateNGStaticAndCloudPermanent = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: static1
spec:
  nodeType: Static
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: cp1
spec:
  nodeType: CloudPermanent
`
		stateNGProperManualRolloutID = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: proper1
  annotations:
    manual-rollout-id: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: proper2
  annotations:
    manual-rollout-id: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper2
    zones: [a,b]

`
		stateNGWrongKind = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: improper
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: ImproperInstanceClass
      name: improper
`
		stateNGWrongRefName = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: improper
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: improper
`
		stateNGWrongZones = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: improper
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones:
    - xxx
`
		stateNGSimple = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: proper1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
`
		stateICProper = `
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: proper1
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: proper2
`
		stateICImproper = `
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: improper1
spec: {}
`
		stateCloudProviderSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-node-manager-cloud-provider
  namespace: kube-system
data:
  zones: WyJhIiwiYiIsImMiXQ== # ["a","b","c"]
`
		machineDeployments = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: a
  labels:
    heritage: deckhouse
  name: proper1-aaa
  namespace: d8-cloud-instance-manager
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: b
  labels:
    heritage: deckhouse
  name: proper2-bbb
  namespace: d8-cloud-instance-manager
`
	)

	// Setup hook for test environment.
	// Freeze timestampt for updateEpoch field.
	epochTimestampAccessor = func() int64 {
		return TestTimestampForUpdateEpoch
	}
	// Set Kind for "ics" binding.
	getCRDsHookConfig.Kubernetes[0].Kind = "D8TestInstanceClass"
	getCRDsHookConfig.Kubernetes[0].ApiVersion = "deckhouse.io/v1alpha1"
	detectInstanceClassKind = func(_ *go_hook.HookInput, _ *go_hook.HookConfig) (string, string) {
		return "D8TestInstanceClass", "D8TestInstanceClass"
	}

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.30.5", "kubernetesVersions":["1.30.5"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},},"nodeManager":{"internal": {"static": {"internalNetworkCIDRs":["172.18.200.0/24"]}}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "D8TestInstanceClass", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "InstanceTypesCatalog", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)

	Context("Cluster with NGs, MDs and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + machineDeployments + stateCloudProviderSecret + stateICProper))
			f.RunHook()
		})

		It("Hook must not fail; zones must be correct", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cloudInstances.zones").String()).To(MatchJSON(`["a","b","c"]`))
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.1.cloudInstances.zones").String()).To(MatchJSON(`["a","b"]`))
		})
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with NG", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectedJSON := `
				[
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": []
				    },
                    "instanceClass": null,
				    "kubelet": {
					"containerLogMaxSize": "50Mi",
					"containerLogMaxFiles": 4,
					"resourceReservation": {
						"mode": "Auto"
					},
					"topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "name": "proper1",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper2"
				      },
				      "zones": [
				        "a",
				        "b"
				      ]
				    },
                    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
					  "resourceReservation": {
						"mode": "Auto"
					  },
					  "topologyManager": {}
				    },
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
                    "serializedLabels": "",
                    "serializedTaints": "",
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			// node_group_info metric should be set
			metrics := f.MetricsCollector.CollectedMetrics()
			Expect(metrics).To(HaveLen(3))
			Expect(metrics[1].Labels).To(BeEquivalentTo(map[string]string{"name": "proper1", "cri_type": "Containerd"}))
			Expect(metrics[2].Labels).To(BeEquivalentTo(map[string]string{"name": "proper2", "cri_type": "Containerd"}))
		})
	})

	Context("With manual-rollout-id", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProperManualRolloutID + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Hook must not fail and Values should contain an id", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.manualRolloutID").String()).To(Equal("test"))
		})
	})

	Context("Proper cluster with two pairs of NG+IC, provider secret and two extra NodeGroups — static and CloudPermanent", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper + stateCloudProviderSecret + stateNGStaticAndCloudPermanent))
			f.RunHook()
		})

		It("NGs must be stored to nodeManager.internal.nodeGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
                  {
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
					"manualRolloutID": "",
                    "kubelet": {
                      "containerLogMaxSize": "50Mi",
			          "containerLogMaxFiles": 4,
			          "resourceReservation": {
				        "mode": "Auto"
                      },
			          "topologyManager": {}
                    },
                    "serializedLabels": "",
                    "serializedTaints": "",
                    "name": "cp1",
                    "nodeType": "CloudPermanent",
                    "updateEpoch": "` + calculateEpoch("cp1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
                  },
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "a",
						"b",
						"c"
				      ]
				    },
				    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
					  "resourceReservation": {
						"mode": "Auto"
					  },
					  "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "name": "proper1",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper2"
				      },
				      "zones": [
				        "a",
				        "b"
				      ]
				    },
				    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
					  "resourceReservation": {
						"mode": "Auto"
					  },
					  "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
                  {
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
                    "manualRolloutID": "",
		            "kubelet": {
			          "containerLogMaxSize": "50Mi",
			          "containerLogMaxFiles": 4,
			          "resourceReservation": {
				        "mode": "Auto"
			          },
			          "topologyManager": {}
		            },
                    "serializedLabels": "",
                    "serializedTaints": "",
                    "name": "static1",
                    "nodeType": "Static",
                    "updateEpoch": "` + calculateEpoch("static1", f.ValuesGet("global.discovery.clusterUUID").String()) + `",
                    "static": {
                      "internalNetworkCIDRs": ["172.18.200.0/24"]
                    }
                  }
				]
			`
			valuesJSON := f.ValuesGet("nodeManager.internal.nodeGroups").String()
			Expect(valuesJSON).To(MatchJSON(expectedJSON))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))
		})
	})

	Context("Schedule: Proper cluster with two pairs of NG+IC, provider secret and two extra NodeGroups — static and CloudPermanent", func() {
		BeforeEach(func() {
			f.KubeStateSet(stateNGProper + stateICProper + stateCloudProviderSecret + stateNGStaticAndCloudPermanent)
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunHook()
		})

		It("NGs must be stored to nodeManager.internal.nodeGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
                  {
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
		            "kubelet": {
			          "containerLogMaxSize": "50Mi",
			          "containerLogMaxFiles": 4,
			          "resourceReservation": {
				        "mode": "Auto"
			         },
			         "topologyManager": {}
		            },
                    "serializedLabels": "",
                    "serializedTaints": "",
                    "manualRolloutID": "",
                    "name": "cp1",
                    "nodeType": "CloudPermanent",
                    "updateEpoch": "` + calculateEpoch("cp1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
                  },
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "a",
						"b",
						"c"
				      ]
				    },
				    "instanceClass": null,
				    "kubelet": {
					   "containerLogMaxSize": "50Mi",
					   "containerLogMaxFiles": 4,
					   "resourceReservation": {
                         "mode": "Auto"
					   },
					   "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "name": "proper1",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper2"
				      },
				      "zones": [
				        "a",
				        "b"
				      ]
				    },
				    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
                      "resourceReservation": {
						"mode": "Auto"
					   },
					   "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
                  {
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
		           "kubelet": {
                     "containerLogMaxSize": "50Mi",
			         "containerLogMaxFiles": 4,
			         "resourceReservation": {
				       "mode": "Auto"
                     },
			         "topologyManager": {}
		            },
                    "serializedLabels": "",
                    "serializedTaints": "",
                    "manualRolloutID": "",
                    "name": "static1",
                    "nodeType": "Static",
                    "updateEpoch": "` + calculateEpoch("static1", f.ValuesGet("global.discovery.clusterUUID").String()) + `",
                    "static": {
                      "internalNetworkCIDRs": ["172.18.200.0/24"]
                    }
                  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))
		})
	})

	Context("Cluster with two proper pairs of NG+IC, one improper IC and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper + stateICImproper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("NGs must be stored to nodeManager.internal.nodeGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "a",
						"b",
						"c"
				      ]
				    },
				    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
					  "resourceReservation": {
						"mode": "Auto"
					  },
					  "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "name": "proper1",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "nodeType": "CloudEphemeral",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper2"
				      },
				      "zones": [
				        "a",
				        "b"
				      ]
				    },
				    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
					  "resourceReservation": {
						"mode": "Auto"
					  },
					  "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
	`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))
		})
	})

	Context("Two proper pairs of NG+IC and a NG with wrong ref kind", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongKind + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, hook must warn user about improper NG", func() {
			Expect(f).NotTo(ExecuteSuccessfully())

			Expect(bytes.Contains(f.LoggerOutput.Contents(), []byte("Invalid classReference.kind 'ImproperInstanceClass'. Expected 'D8TestInstanceClass'. Please update the NodeGroup to use the correct instance class kind.")))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`NodeGroup validation failed: 1 NodeGroups have configuration errors.`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`• NodeGroup 'improper': Invalid classReference.kind 'ImproperInstanceClass'. Expected 'D8TestInstanceClass'. Please update the NodeGroup to use the correct instance class kind.`))
		})
	})

	Context("Two proper pairs of NG+IC and a NG with wrong ref kind which was stored earlier", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongKind + stateICProper + stateCloudProviderSecret))
			f.ValuesSetFromYaml("nodeManager.internal.nodeGroups", []byte(`
-
  name: proper1
  some: data1
-
  name: proper2
  some: data2
-
  name: improper
  nodeType: CloudEphemeral
`))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, old improper NG data must be saved, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "name": "improper",
				    "nodeType": "CloudEphemeral"
				  },
				  {
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "a",
						"b",
						"c"
				      ]
				    },
                    "nodeType": "CloudEphemeral",
				    "name": "proper1",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
					  "resourceReservation": {
						"mode": "Auto"
					  },
					  "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper2"
				      },
				      "zones": [
				        "a",
				        "b"
				      ]
				    },
                    "nodeType": "CloudEphemeral",
				    "name": "proper2",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
					  "resourceReservation": {
						"mode": "Auto"
					  },
					  "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
				`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Invalid classReference.kind 'ImproperInstanceClass'. Expected 'D8TestInstanceClass'. Please update the NodeGroup to use the correct instance class kind. Using previously stored NodeGroup configuration to prevent cluster disruption."))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Invalid classReference.kind 'ImproperInstanceClass'. Expected 'D8TestInstanceClass'. Please update the NodeGroup to use the correct instance class kind. Using previously stored NodeGroup configuration to prevent cluster disruption."))
		})
	})

	Context("Two proper pairs of NG+IC and a NG with wrong ref name", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongRefName + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, hook must warn user about improper NG", func() {
			Expect(f).NotTo(ExecuteSuccessfully())

			Expect(bytes.Contains(f.LoggerOutput.Contents(), []byte("Instance class 'improper' of type 'D8TestInstanceClass' not found. Please create the required instance class or update the NodeGroup to reference an existing one.")))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`NodeGroup validation failed: 1 NodeGroups have configuration errors.`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`• NodeGroup 'improper': Instance class 'improper' of type 'D8TestInstanceClass' not found. Please create the required instance class or update the NodeGroup to reference an existing one.`))
		})
	})

	Context("Two proper pairs of NG+IC and a NG with wrong zones", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongZones + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, hook must warn user about improper NG", func() {
			Expect(f).NotTo(ExecuteSuccessfully())

			Expect(bytes.Contains(f.LoggerOutput.Contents(), []byte("Invalid zones specified: [xxx]. Available zones: [a b c]. Please update the NodeGroup to use valid zones.")))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`NodeGroup validation failed: 1 NodeGroups have configuration errors.`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`• NodeGroup 'improper': Invalid zones specified: [xxx]. Available zones: [a b c]. Please update the NodeGroup to use valid zones.`))
		})
	})

	Context("Two proper pairs of NG+IC and a NG with wrong ref name but stored earlier", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongRefName + stateICProper + stateCloudProviderSecret))
			f.ValuesSetFromYaml("nodeManager.internal.nodeGroups", []byte(`
-
 name: proper1
 some: data1
-
 name: proper2
 some: data2
-
 name: improper
 nodeType: CloudEphemeral
`))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, old improper NG data must be saved, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "name": "improper",
				    "nodeType": "CloudEphemeral"
				  },
				  {
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "a",
						"b",
						"c"
				      ]
				    },
                    "nodeType": "CloudEphemeral",
				    "name": "proper1",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
					  "resourceReservation": {
						"mode": "Auto"
					  },
					  "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper2"
				      },
				      "zones": [
				        "a",
				        "b"
				      ]
				    },
                    "nodeType": "CloudEphemeral",
				    "name": "proper2",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.30",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
				    "kubelet": {
					  "containerLogMaxSize": "50Mi",
					  "containerLogMaxFiles": 4,
					  "resourceReservation": {
						"mode": "Auto"
					  },
					  "topologyManager": {}
				    },
                    "serializedLabels": "",
                    "serializedTaints": "",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))
			Expect(string(f.LoggerOutput.Contents())).To(ContainSubstring("Instance class 'improper' of type 'D8TestInstanceClass' not found. Please create the required instance class or update the NodeGroup to reference an existing one. Using previously stored NodeGroup configuration to prevent cluster disruption."))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.30"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Instance class 'improper' of type 'D8TestInstanceClass' not found. Please create the required instance class or update the NodeGroup to reference an existing one. Using previously stored NodeGroup configuration to prevent cluster disruption."))
		})
	})

	// config    1.31
	// apiserver 1.31.X  |  effective 1.31
	Context("Cluster with NG", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: Static
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
			f.BindingContexts.Set(f.KubeStateSet(ng + stateICProper))
			setK8sVersionAsClusterConfig(f, "1.31")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.31.0")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.31.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.31", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.31"))
		})
	})

	// config    1.30
	// apiserver 1.31.X  |  effective 1.30
	Context("Cluster with NG", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
			f.BindingContexts.Set(f.KubeStateSet(ng + stateICProper))
			setK8sVersionAsClusterConfig(f, "1.30")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.31.0")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.31.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.30", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.30"))
		})
	})

	// config    null
	// apiserver 1.31  |  target 1.31
	Context("Cluster with NG", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
			f.BindingContexts.Set(f.KubeStateSet(ng + stateICProper))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.31.0")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.31.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.31", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.31"))
		})
	})

	Context("Cluster with NG node-role.deckhouse.io/system", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
			f.BindingContexts.Set(f.KubeStateSet(ng + stateICProper))
			f.RunHook()
		})

		It("Hook must not fail; label must be added", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeTemplate.labels").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
		})
	})

	Context("Cluster with NG node-role.deckhouse.io/stateful", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  nodeTemplate:
    labels:
      node-role.deckhouse.io/stateful: ""
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
			f.BindingContexts.Set(f.KubeStateSet(ng + stateICProper))
			f.RunHook()
		})

		It("Hook must not fail; label must not be added", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeTemplate.labels").String()).To(MatchJSON(`{"node-role.deckhouse.io/stateful": ""}`))
		})
	})

	Context("Cluster with proper NG, global cri is set to containerd", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGSimple + stateICProper))
			setK8sVersionAsClusterConfig(f, "1.30")
			f.ValuesSet("global.clusterConfiguration.defaultCRI", "Containerd")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.30.5")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.30.5")
			f.RunHook()
		})

		It("Hook must not fail; cri must be correct", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cri.type").String()).To(Equal("Containerd"))
		})
	})

	Context("Cluster with proper NG, global cri is set to not managed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGSimple + stateICProper))
			setK8sVersionAsClusterConfig(f, "1.30")
			f.ValuesSet("global.clusterConfiguration.defaultCRI", "NotManaged")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.30.5")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.30.5")
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cri.type").String()).To(Equal("NotManaged"))
		})
	})

	assertNodeCapacity := func(f *HookExecutionConfig, expectType v1alpha1.InstanceType) {
		Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeCapacity").Exists()).To(BeTrue())

		Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeCapacity.name").String()).To(Equal(expectType.Name))
		Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeCapacity.cpu").String()).To(Equal(expectType.CPU.String()))
		Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeCapacity.memory").String()).To(Equal(expectType.Memory.String()))
		Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.nodeCapacity.rootDisk").String()).To(Equal(expectType.RootDisk.String()))
	}

	Context("ScaleFromZero: can't find a capacity", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 0
    maxPerZone: 3
    classReference:
      kind: D8TestInstanceClass
      name: caperror
    zones: [a,b]
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: caperror
spec: {}
`))
			f.RunHook()
		})

		It("NodeGroup values must be valid", func() {
			Expect(f).NotTo(ExecuteSuccessfully())

			Expect(bytes.Contains(f.LoggerOutput.Contents(), []byte("Calculate capacity failed")))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`NodeGroup validation failed: 1 NodeGroups have configuration errors.`))
			Expect(f.GoHookError.Error()).Should(ContainSubstring(`• NodeGroup 'test': Capacity calculation failed for instance class 'D8TestInstanceClass'. The instance type is not found in built-in types and no capacity is set. ScaleFromZero will not work. Please set capacity in the D8TestInstanceClass 'caperror' or use a supported instance type.`))
		})
	})

	Context("ScaleFromZero: with a capacity", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 0
    maxPerZone: 3
    classReference:
      kind: D8TestInstanceClass
      name: cap
    zones: [a,b]
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: cap
spec:
  type: test
  capacity:
    cpu: 4
    memory: 8Gi
`))
			f.RunHook()
		})

		It("NodeGroup values must be valid", func() {
			Expect(f).To(ExecuteSuccessfully())
			// cloudInstances field exists
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cloudInstances").Exists()).To(BeTrue())
			// nodeCapacity field does not exist
			assertNodeCapacity(f, v1alpha1.InstanceType{
				CPU:    resource.MustParse("4"),
				Memory: resource.MustParse("8Gi"),
			})
		})
	})

	Context("ScaleFromZero: using catalog", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceTypesCatalog
metadata:
  name: for-cluster-autoscaler
instanceTypes:
- name: test
  cpu: "8"
  memory: "16Gi"
  rootDisk: "20Gi"
- name: not-used
  cpu: "1"
  memory: "1Gi"
  rootDisk: "10Gi"
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    minPerZone: 0
    maxPerZone: 3
    classReference:
      kind: D8TestInstanceClass
      name: cap
    zones: [a,b]
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: cap
spec:
  type: test
  capacity:
    cpu: 4
    memory: 8Gi
`))
			f.RunHook()
		})

		It("NodeGroup values must be valid and get from catalog", func() {
			Expect(f).To(ExecuteSuccessfully())
			// cloudInstances field exists
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cloudInstances").Exists()).To(BeTrue())
			// nodeCapacity field does not exist
			assertNodeCapacity(f, v1alpha1.InstanceType{
				CPU:      resource.MustParse("8"),
				Memory:   resource.MustParse("16Gi"),
				RootDisk: resource.MustParse("20Gi"),
				Name:     "test",
			})
		})
	})

	const (
		staticNodeGroupWithStaticInstances = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  staticInstances:
    labelSelector:
      matchLabels:
        node-group: worker
`
	)

	Context("Static instances", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(staticNodeGroupWithStaticInstances))
			f.RunHook()
		})

		It("StaticMachineTemplate and MachineDeployment should be generated", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.staticInstances").Exists()).To(BeTrue())
		})
	})

	const (
		staticNodeGroupWithFencing = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  staticInstances:
    labelSelector:
      matchLabels:
        node-group: worker
  fencing:
    mode: Watchdog
`
	)

	Context("Static instances with fencing", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(staticNodeGroupWithFencing))
			f.RunHook()
		})

		It("Internal fencing values should be generated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.fencing.mode").Value()).To(Equal("Watchdog"))
		})
	})

	Context("Serialized labels and taints", func() {
		Context("NG without labels and taints", func() {
			BeforeEach(func() {
				ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
				f.BindingContexts.Set(f.KubeStateSet(ng + stateICProper))
				setK8sVersionAsClusterConfig(f, "1.30")
				f.ValuesSet("global.discovery.kubernetesVersion", "1.31.0")
				f.ValuesSet("global.discovery.kubernetesVersions.0", "1.31.0")
				f.RunHook()
			})

			It("Should set empty serializedLabels and serializedTaints", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedLabels").Exists()).To(BeTrue())
				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedLabels").String()).To(Equal(""))

				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedTaints").Exists()).To(BeTrue())
				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedTaints").String()).To(Equal(""))
			})
		})

		Context("NG with labels only", func() {
			BeforeEach(func() {
				ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  nodeTemplate:
    labels:
      environment: production
      app: warp-drive-ai
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
				f.BindingContexts.Set(f.KubeStateSet(ng + stateICProper))
				setK8sVersionAsClusterConfig(f, "1.30")
				f.ValuesSet("global.discovery.kubernetesVersion", "1.31.0")
				f.ValuesSet("global.discovery.kubernetesVersions.0", "1.31.0")
				f.RunHook()
			})

			It("Should set serializedLabels in desired format and empty serializedTaints", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedLabels").Exists()).To(BeTrue())
				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedLabels").String()).To(Equal("app=warp-drive-ai,environment=production"))

				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedTaints").Exists()).To(BeTrue())
				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedTaints").String()).To(Equal(""))
			})
		})

		Context("NG with taints only", func() {
			BeforeEach(func() {
				ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  nodeTemplate:
    taints:
    - effect: NoExecute
      key: b
      value: v
    - key: a
    - effect: NoExecute
      key: d
    - key: c
      value: v1
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
				f.BindingContexts.Set(f.KubeStateSet(ng + stateICProper))
				setK8sVersionAsClusterConfig(f, "1.30")
				f.ValuesSet("global.discovery.kubernetesVersion", "1.31.0")
				f.ValuesSet("global.discovery.kubernetesVersions.0", "1.31.0")
				f.RunHook()
			})

			It("Should set serializedTaints in desired format and empty serializedLabels", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedLabels").Exists()).To(BeTrue())
				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedLabels").String()).To(Equal(""))

				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedTaints").Exists()).To(BeTrue())
				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedTaints").String()).To(Equal("b=v:NoExecute,a,d:NoExecute,c=v1:"))
			})
		})

		Context("Fill default mainNetwork for vSphere", func() {
			const icWithEmptySpec = `
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: proper1
spec: {}
`
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateNGSimple + icWithEmptySpec))
				f.ValuesSetFromYaml("nodeManager.internal.cloudProvider", []byte(`
---
type: vsphere
vsphere:
  instances:
    mainNetwork: mynet
`))
				f.RunHook()
			})

			It("should inject mainNetwork into instanceClass when missing in CRD", func() {
				Expect(f).To(ExecuteSuccessfully())
				val := f.ValuesGet("nodeManager.internal.nodeGroups.0.instanceClass.mainNetwork")
				Expect(val.Exists()).To(BeTrue())
				Expect(val.String()).To(Equal("mynet"))
			})
		})

		Context("NG with taints and labels", func() {
			BeforeEach(func() {
				ng := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: test
spec:
  nodeType: CloudEphemeral
  nodeTemplate:
    labels:
      environment: production
      app: warp-drive-ai
    taints:
    - effect: NoExecute
      key: b
      value: v
    - key: a
    - effect: NoExecute
      key: d
    - key: c
      value: v1
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
    zones: [a,b]
`
				f.BindingContexts.Set(f.KubeStateSet(ng + stateICProper))
				setK8sVersionAsClusterConfig(f, "1.30")
				f.ValuesSet("global.discovery.kubernetesVersion", "1.31.0")
				f.ValuesSet("global.discovery.kubernetesVersions.0", "1.31.0")
				f.RunHook()
			})

			It("Should set serializedLabels and serializedTaints in desired format", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedLabels").Exists()).To(BeTrue())
				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedLabels").String()).To(Equal("app=warp-drive-ai,environment=production"))

				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedTaints").Exists()).To(BeTrue())
				Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.serializedTaints").String()).To(Equal("b=v:NoExecute,a,d:NoExecute,c=v1:"))
			})
		})
	})
})
