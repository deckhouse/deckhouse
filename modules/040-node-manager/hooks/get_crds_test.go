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
	"fmt"
	"strconv"
	"testing"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
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

		stateNGDockerUnmanaged = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: proper1
spec:
  cri:
    type: Docker
    docker:
      manage: false
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
	detectInstanceClassKind = func(_ *go_hook.HookInput, _ *go_hook.HookConfig) (inUse string, fromSecret string) {
		return "D8TestInstanceClass", "D8TestInstanceClass"
	}

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.27.5", "kubernetesVersions":["1.27.5"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},},"nodeManager":{"internal": {"static": {"internalNetworkCIDRs":["172.18.200.0/24"]}}}}`, `{}`)
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
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.27",
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
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
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
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.27",
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
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
                  {
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
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
			valuesJSON := f.ValuesGet("nodeManager.internal.nodeGroups").String()
			Expect(valuesJSON).To(MatchJSON(expectedJSON))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
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
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.27",
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
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
                  {
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
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
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))
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
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.27",
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
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.27",
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
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))
		})

	})

	Context("Two proper pairs of NG+IC and a NG with wrong ref kind", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongKind + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.LogrusOutput).Should(gbytes.Say("Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass."))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass."))
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
				`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.LogrusOutput).Should(gbytes.Say("Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass. Earlier stored version of NG is in use now!"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass. Earlier stored version of NG is in use now!"))
		})
	})

	Context("Two proper pairs of NG+IC and a NG with wrong ref name", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongRefName + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.LogrusOutput).Should(gbytes.Say(`Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass.`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass."))
		})
	})

	Context("Two proper pairs of NG+IC and a NG with wrong zones", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongZones + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.LogrusOutput).Should(gbytes.Say(`unknown cloudInstances\.zones: \[xxx\]`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(ContainSubstring("unknown cloudInstances.zones"))
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
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
                    "kubernetesVersion": "1.27",
					"cri": {
                      "type": "Containerd"
                    },
				    "instanceClass": null,
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.LogrusOutput).Should(gbytes.Say(`Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass. Earlier stored version of NG is in use now!`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(Equal(""))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.kubernetesVersion").Value()).To(Equal("1.27"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass. Earlier stored version of NG is in use now!"))
		})
	})

	// config    1.29
	// apiserver 1.29.X  |  effective 1.29
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
			setK8sVersionAsClusterConfig(f, "1.29")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.29.0")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.29.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.29", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.29"))
		})
	})

	// config    1.25
	// apiserver 1.27.X  |  effective 1.25
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
			setK8sVersionAsClusterConfig(f, "1.25")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.27.0")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.27.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.25", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.25"))
		})
	})

	// config    null
	// apiserver 1.29  |  target 1.29
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
			f.ValuesSet("global.discovery.kubernetesVersion", "1.29.0")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.29.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.29", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.29"))
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

	Context("Cluster with proper NG, global cri is set to docker", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGSimple + stateICProper))
			setK8sVersionAsClusterConfig(f, "1.27")
			f.ValuesSet("global.clusterConfiguration.defaultCRI", "Docker")
			f.RunHook()
		})

		It("Hook must not fail; cri must be correct", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cri.type").String()).To(Equal("Docker"))
		})
	})

	Context("Cluster with proper NG, global cri is set to containerd", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGSimple + stateICProper))
			setK8sVersionAsClusterConfig(f, "1.27")
			f.ValuesSet("global.clusterConfiguration.defaultCRI", "Containerd")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.27.5")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.27.5")
			f.RunHook()
		})

		It("Hook must not fail; cri must be correct", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cri.type").String()).To(Equal("Containerd"))
		})
	})

	Context("Cluster with proper NG, docker is not managed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGDockerUnmanaged + stateICProper))
			f.RunHook()
		})

		It("Hook must not fail; zones must be correct", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cri.type").String()).To(Equal("NotManaged"))
		})
	})

	Context("Cluster with proper NG, global cri is set to not managed", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGSimple + stateICProper))
			setK8sVersionAsClusterConfig(f, "1.27")
			f.ValuesSet("global.clusterConfiguration.defaultCRI", "NotManaged")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.27.5")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.27.5")
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
			Expect(f).To(ExecuteSuccessfully())
			// nodeGroup should not be rendered at all
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").Array()).To(HaveLen(0))
			// but we have an error in logs
			Expect(string(f.LogrusOutput.Contents())).To(ContainSubstring("Calculate capacity failed for: D8TestInstanceClass"))
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
})
