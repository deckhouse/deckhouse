package hooks

import (
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: get_crds ::", func() {
	const (
		stateNGProper = `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: proper1
spec:
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper1
  bashible:
    options:
      kubernetesVersion: 1.15.4
    bundle: centos-7.1.1.1
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: proper2
spec:
  cloudInstances:
    classReference:
      kind: D8TestInstanceClass
      name: proper2
    zones: [a,b]
  bashible:
    options: {}
    bundle: slackware-14.1

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
spec:
  bashible:
    options: {}
    bundle: ubuntu-7.1.1.1
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
	)

	f := HookExecutionConfigInit(`{"cloudInstanceManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "D8TestInstanceClass", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "CloudInstanceGroup", false)

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
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectedJSON := `
				[
				  {
				    "bashible": {
				      "bundle": "centos-7.1.1.1",
				      "dynamicOptions": {},
				      "options": {
				        "kubernetesVersion": "1.15.4"
				      }
				    },
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      }
				    },
				    "manual-rollout-id": "",
				    "name": "proper1"
				  },
				  {
				    "bashible": {
				      "bundle": "slackware-14.1",
				      "dynamicOptions": {},
				      "options": {}
				    },
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
				    "manual-rollout-id": "",
				    "name": "proper2"
				  }
				]
`
			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))
		})
	})

	Context("Cluster with two pairs of NG+IC but without provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper))
			f.RunHook()
		})

		It("Hook must not fail, NG statuses must update", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.Session.Err).Should(gbytes.Say("ERROR: Can't find '.data.zones' in secret kube-system/d8-node-manager-cloud-provider."))
			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups").String()).To(Equal("[]"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").String()).To(Equal(`Can't find '.data.zones' in secret kube-system/d8-node-manager-cloud-provider.`))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").String()).To(Equal(`Can't find '.data.zones' in secret kube-system/d8-node-manager-cloud-provider.`))
		})
	})

	Context("Cluster with two pairs of NG+IC but without provider secret and previosly stored NGs data", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper))
			f.ValuesSetFromYaml("cloudInstanceManager.internal.nodeGroups", []byte(`
-
  name: proper2
  some: data2
-
  name: proper1
  some: data1
`))
			f.RunHook()
		})

		It("Hook must not fail and old data must be stored", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.Session.Err).Should(gbytes.Say("ERROR: Can't find '.data.zones' in secret kube-system/d8-node-manager-cloud-provider. Earlier stored version of NG is in use now!"))

			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups").String()).To(MatchJSON(`[{"name": "proper1","some": "data1"},{"name": "proper2","some": "data2"}]`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").String()).To(Equal(`Can't find '.data.zones' in secret kube-system/d8-node-manager-cloud-provider. Earlier stored version of NG is in use now!`))
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").String()).To(Equal(`Can't find '.data.zones' in secret kube-system/d8-node-manager-cloud-provider. Earlier stored version of NG is in use now!`))
		})

	})

	Context("With manual-rollout-id", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProperManualRolloutId + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Hook must not fail and Values should contain an id", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups.0.manual-rollout-id").String()).To(Equal("test"))
		})
	})

	Context("Proper cluster with two pairs of NG+IC and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("NGs must be stored to cloudInstanceManager.internal.nodeGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "bashible": {
				      "bundle": "centos-7.1.1.1",
				      "dynamicOptions": {},
				      "options": {
				        "kubernetesVersion": "1.15.4"
				      }
				    },
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
				    "manual-rollout-id": "",
				    "name": "proper1"
				  },
				  {
				    "bashible": {
				      "bundle": "slackware-14.1",
				      "dynamicOptions": {},
				      "options": {}
				    },
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
				    "manual-rollout-id": "",
				    "name": "proper2"
				  }
				]
			`
			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
		})
	})

	Context("Cluster with two proper pairs of NG+IC, one improper IC and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateICProper + stateICIMroper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("NGs must be stored to cloudInstanceManager.internal.nodeGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "bashible": {
				      "bundle": "centos-7.1.1.1",
				      "dynamicOptions": {},
				      "options": {
				        "kubernetesVersion": "1.15.4"
				      }
				    },
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
				    "manual-rollout-id": "",
				    "name": "proper1"
				  },
				  {
				    "bashible": {
				      "bundle": "slackware-14.1",
				      "dynamicOptions": {},
				      "options": {}
				    },
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
				    "manual-rollout-id": "",
				    "name": "proper2"
				  }
				]
	`
			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))
			// Expect(f.Session.Err).Should(gbytes.Say("Instance class improper1 is invalid: .spec.bashible.options.kubernetesVersion is mandatory for .spec.bashible.bundle ubuntu-7.1.1.1"))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
		})

	})

	Context("Two proper pairs of NG+IC and a NG with wrong ref kind", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongKind + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Proper NGs must be stored to cloudInstanceManager.internal.nodeGroups, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "bashible": {
				      "bundle": "centos-7.1.1.1",
				      "dynamicOptions": {},
				      "options": {
				        "kubernetesVersion": "1.15.4"
				      }
				    },
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "nova"
				      ]
				    },
				    "name": "proper1",
				    "manual-rollout-id": "",
				    "instanceClass": null
				  },
				  {
				    "bashible": {
				      "bundle": "slackware-14.1",
				      "dynamicOptions": {},
				      "options": {}
				    },
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
				    "name": "proper2",
				    "manual-rollout-id": "",
				    "instanceClass": null
				  }
				]
			`
			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say("ERROR: Bad NodeGroup improper: Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass."))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Wrong classReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass."))
		})
	})

	Context("Two proper pairs of NG+IC and a NG with wrong ref kind which was stored earlier", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongKind + stateICProper + stateCloudProviderSecret))
			f.ValuesSetFromYaml("cloudInstanceManager.internal.nodeGroups", []byte(`
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

		It("Proper NGs must be stored to cloudInstanceManager.internal.nodeGroups, old improper NG data must be saved, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "name": "improper",
				    "some": "imdata"
				  },
				  {
				    "bashible": {
				      "bundle": "centos-7.1.1.1",
				      "dynamicOptions": {},
				      "options": {
				        "kubernetesVersion": "1.15.4"
				      }
				    },
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "nova"
				      ]
				    },
				    "name": "proper1",
				    "manual-rollout-id": "",
				    "instanceClass": null
				  },
				  {
				    "bashible": {
				      "bundle": "slackware-14.1",
				      "dynamicOptions": {},
				      "options": {}
				    },
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
				    "name": "proper2",
				    "manual-rollout-id": "",
				    "instanceClass": null
				  }
				]
				`
			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

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

		It("Proper NGs must be stored to cloudInstanceManager.internal.nodeGroups, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "bashible": {
				      "bundle": "centos-7.1.1.1",
				      "dynamicOptions": {},
				      "options": {
				        "kubernetesVersion": "1.15.4"
				      }
				    },
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "nova"
				      ]
				    },
				    "name": "proper1",
				    "manual-rollout-id": "",
				    "instanceClass": null
				  },
				  {
				    "bashible": {
				      "bundle": "slackware-14.1",
				      "dynamicOptions": {},
				      "options": {}
				    },
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
				    "name": "proper2",
				    "manual-rollout-id": "",
				    "instanceClass": null
				  }
				]
			`
			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: Bad NodeGroup improper: Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass.`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass."))
		})
	})

	Context("Two proper pairs of NG+IC and a NG with wrong ref name but stored earlier", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGProper + stateNGWrongRefName + stateICProper + stateCloudProviderSecret))
			f.ValuesSetFromYaml("cloudInstanceManager.internal.nodeGroups", []byte(`
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

		It("Proper NGs must be stored to cloudInstanceManager.internal.nodeGroups, old improper NG data must be saved, hook must warn user about improper NG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				[
				  {
				    "name": "improper",
				    "some": "imdata"
				  },
				  {
				    "bashible": {
				      "bundle": "centos-7.1.1.1",
				      "dynamicOptions": {},
				      "options": {
				        "kubernetesVersion": "1.15.4"
				      }
				    },
				    "cloudInstances": {
				      "classReference": {
				        "kind": "D8TestInstanceClass",
				        "name": "proper1"
				      },
				      "zones": [
				        "nova"
				      ]
				    },
				    "name": "proper1",
				    "manual-rollout-id": "",
				    "instanceClass": null
				  },
				  {
				    "bashible": {
				      "bundle": "slackware-14.1",
				      "dynamicOptions": {},
				      "options": {}
				    },
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
				    "name": "proper2",
				    "manual-rollout-id": "",
				    "instanceClass": null
				  }
				]
			`
			Expect(f.ValuesGet("cloudInstanceManager.internal.nodeGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: Bad NodeGroup improper: Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass. Earlier stored version of NG is in use now!`))

			Expect(f.KubernetesGlobalResource("NodeGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "proper2").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("NodeGroup", "improper").Field("status.error").String()).To(Equal("Wrong classReference: There is no valid instance class improper of type D8TestInstanceClass. Earlier stored version of NG is in use now!"))
		})
	})
})
