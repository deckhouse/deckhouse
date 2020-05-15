package hooks

import (
	"os/exec"
	"strings"

	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func calculateEpoch(ngName string, clusterUUID string) string {
	epochCmd := exec.Command(`/bin/bash`, `-c`, `awk -v seed="`+clusterUUID+ngName+`" -v timestamp="1234567890" 'BEGIN{srand(seed); printf("%d\n", ((rand() * 14400) + timestamp) / 14400)}'`)
	epochOut, err := epochCmd.Output()
	if err != nil {
		panic(err)
	}
	return strings.TrimSpace(string(epochOut))
}

var _ = Describe("Modules :: node-manager :: hooks :: get_crds ::", func() {
	const (
		stateNGProper = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: proper1
spec:
  nodeType: Cloud
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
  kubernetesVersion: "1.42"
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: proper2
spec:
  nodeType: Cloud
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper2
    zones: [a,b]
`
		stateNGStaticAndHybrid = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: static1
spec:
  nodeType: Static
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: hybrid1
spec:
  nodeType: Hybrid
`
		stateNGProperManualRolloutId = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: proper1
  annotations:
    manual-rollout-id: test
spec:
  nodeType: Cloud
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: proper2
  annotations:
    manual-rollout-id: test
spec:
  nodeType: Cloud
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper2
    zones: [a,b]

`
		stateNGWrongKind = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: improper
spec:
  nodeType: Cloud
  cloudInstances:
    classReference:
      kind: ImproperInstanceClass
      name: improper
`
		stateNGWrongRefName = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: improper
spec:
  nodeType: Cloud
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: improper
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
		stateICIMroper = `
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
  zones: WyJub3ZhIl0= # ["nova"]
`
		machineDeployments = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: aaa
  labels:
    heritage: deckhouse
  name: proper1-aaa
  namespace: d8-cloud-instance-manager
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: bbb
  labels:
    heritage: deckhouse
  name: proper2-bbb
  namespace: d8-cloud-instance-manager
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.15.5", "kubernetesVersions":["1.15.5"]},"clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"},"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "D8TestInstanceClass", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)

	Context("Cluster with NGs, MDs and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + machineDeployments + stateCloudProviderSecret + stateICProper))
			f.RunHook()
		})

		It("Hook must not fail; zones must be correct", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.cloudInstances.zones").String()).To(MatchJSON(`["aaa","bbb","nova"]`))
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
				    "nodeType": "Cloud",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": []
				    },
                    "instanceClass": null,
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
				    "name": "proper1",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "nodeType": "Cloud",
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
                    "kubernetesVersion": "1.15",
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))
		})
	})

	Context("Cluster with two pairs of NG+IC but without provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper))
			f.RunHook()
		})

		It("Hook must not fail, NG statuses must update", func() {
			expectedJSON := `
				[
				  {
				    "nodeType": "Cloud",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": []
				    },
				    "instanceClass": null,
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
				    "name": "proper1",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "nodeType": "Cloud",
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
                    "kubernetesVersion": "1.15",
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
`
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))
		})
	})

	Context("With manual-rollout-id", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProperManualRolloutId + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Hook must not fail and Values should contain an id", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.manualRolloutID").String()).To(Equal("test"))
		})
	})

	Context("Proper cluster with two pairs of NG+IC, provider secret and two extra NodeGroups — static and hybrid", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper + stateCloudProviderSecret + stateNGStaticAndHybrid))
			f.RunHook()
		})

		It("NGs must be stored to nodeManager.internal.nodeGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
                  {
                    "kubernetesVersion": "1.15",
                    "manualRolloutID": "",
                    "name": "hybrid1",
                    "nodeType": "Hybrid",
                    "updateEpoch": "` + calculateEpoch("hybrid1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
                  },
				  {
				    "nodeType": "Cloud",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "nova"
				      ]
				    },
				    "instanceClass": null,
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
				    "name": "proper1",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "nodeType": "Cloud",
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
                    "kubernetesVersion": "1.15",
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
                  {
                    "kubernetesVersion": "1.15",
                    "manualRolloutID": "",
                    "name": "static1",
                    "nodeType": "Static",
                    "updateEpoch": "` + calculateEpoch("static1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
                  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
		})
	})

	Context("Cluster with two proper pairs of NG+IC, one improper IC and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper + stateICIMroper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("NGs must be stored to nodeManager.internal.nodeGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "nodeType": "Cloud",
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "nova"
				      ]
				    },
				    "instanceClass": null,
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
				    "name": "proper1",
                    "updateEpoch": "` + calculateEpoch("proper1", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  },
				  {
				    "nodeType": "Cloud",
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
                    "kubernetesVersion": "1.15",
				    "name": "proper2",
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
	`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
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
				        "nova"
				      ]
				    },
                    "nodeType": "Cloud",
				    "name": "proper1",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
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
                    "nodeType": "Cloud",
				    "name": "proper2",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
				    "instanceClass": null,
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say("ERROR: Bad NodeGroup improper: Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass."))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
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
  some: imdata
`))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, old improper NG data must be saved, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "name": "improper",
				    "some": "imdata"
				  },
				  {
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "nova"
				      ]
				    },
                    "nodeType": "Cloud",
				    "name": "proper1",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
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
                    "nodeType": "Cloud",
				    "name": "proper2",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
				    "instanceClass": null,
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
				`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say("ERROR: Bad NodeGroup improper: Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass. Earlier stored version of NG is in use now!"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
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
				        "nova"
				      ]
				    },
                    "nodeType": "Cloud",
				    "name": "proper1",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
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
                    "nodeType": "Cloud",
				    "name": "proper2",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
				    "instanceClass": null,
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: Bad NodeGroup improper: Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass.`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass."))
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
 some: imdata
`))
			f.RunHook()
		})

		It("Proper NGs must be stored to nodeManager.internal.nodeGroups, old improper NG data must be saved, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "name": "improper",
				    "some": "imdata"
				  },
				  {
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "nova"
				      ]
				    },
                    "nodeType": "Cloud",
				    "name": "proper1",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
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
                    "nodeType": "Cloud",
				    "name": "proper2",
				    "manualRolloutID": "",
                    "kubernetesVersion": "1.15",
				    "instanceClass": null,
                    "updateEpoch": "` + calculateEpoch("proper2", f.ValuesGet("global.discovery.clusterUUID").String()) + `"
				  }
				]
			`
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: Bad NodeGroup improper: Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass. Earlier stored version of NG is in use now!`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass. Earlier stored version of NG is in use now!"))
		})
	})

	// nodegroup 1.16
	// config    1.16
	// apiserver 1.16.X  |  effective 1.16
	Context("Cluster with NG", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test
spec:
  kubernetesVersion: "1.16"
`
			f.BindingContexts.Set(f.KubeStateSet(ng))
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion", "1.16")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.16.0")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.16.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.16", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.16"))
		})
	})

	// nodegroup 1.15
	// config    null
	// apiserver 1.16.X  |  effective 1.15
	Context("Cluster with NG", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test
spec:
  kubernetesVersion: "1.15"
`
			f.BindingContexts.Set(f.KubeStateSet(ng))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.16.0")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.16.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.15", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.15"))
		})
	})

	// nodegroup 1.17
	// config    null
	// apiserver 1.16.X  |  effective 1.16
	Context("Cluster with NG", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test
spec:
  kubernetesVersion: "1.17"
`
			f.BindingContexts.Set(f.KubeStateSet(ng))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.16.0")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.16.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.16", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.16"))
		})
	})

	// nodegroup null
	// config    1.15
	// apiserver 1.16.X  |  effective 1.15
	Context("Cluster with NG", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test
`
			f.BindingContexts.Set(f.KubeStateSet(ng))
			f.ValuesSet("global.clusterConfiguration.kubernetesVersion", "1.15")
			f.ValuesSet("global.discovery.kubernetesVersion", "1.16.0")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.16.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.15", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.15"))
		})
	})

	// nodegroup null
	// config    null
	// apiserver 1.16  |  target 1.16
	Context("Cluster with NG", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test
`
			f.BindingContexts.Set(f.KubeStateSet(ng))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.16.0")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.16.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.16", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.16"))
		})
	})

	// nodegroup 1.13
	// config    null
	// apiserver 1.16  |  target 1.14
	Context("Cluster with NG", func() {
		BeforeEach(func() {
			ng := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: test
spec:
  kubernetesVersion: "1.13"
`
			f.BindingContexts.Set(f.KubeStateSet(ng))
			f.ValuesSet("global.discovery.kubernetesVersion", "1.16.0")
			f.ValuesSet("global.discovery.kubernetesVersions.0", "1.16.0")
			f.RunHook()
		})

		It("must be executed successfully; kubernetesVersion must be 1.14", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.nodeGroups.0.kubernetesVersion").String()).To(Equal("1.14"))
		})
	})

})
