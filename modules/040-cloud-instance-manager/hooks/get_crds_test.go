package hooks

import (
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: get_crds ::", func() {
	const (
		stateCIGProper = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: proper1
spec:
  instanceClassReference:
    kind: D8TestInstanceClass
    name: proper1
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: proper2
spec:
  instanceClassReference:
    kind: D8TestInstanceClass
    name: proper2
  zones: [a,b]

`
		stateCIGProperManualRolloutId = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: proper1
  annotations:
    manual-rollout-id: test
spec:
  instanceClassReference:
    kind: D8TestInstanceClass
    name: proper1
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: proper2
  annotations:
    manual-rollout-id: test
spec:
  instanceClassReference:
    kind: D8TestInstanceClass
    name: proper2
  zones: [a,b]

`
		stateCIGWrongKind = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: improper
spec:
  instanceClassReference:
    kind: ImproperInstanceClass
    name: improper
`
		stateCIGWrongRefName = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: improper
spec:
  instanceClassReference:
    kind: D8TestInstanceClass
    name: improper
`
		stateICProper = `
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: proper1
spec:
  bashible:
    options:
      kubernetesVersion: 1.15.4
    bundle: centos-7.1.1.1
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: proper2
spec:
  bashible:
    options: {}
    bundle: slackware-14.1
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
  name: d8-cloud-instance-manager-cloud-provider
  namespace: kube-system
data:
  zones: WyJub3ZhIl0= # ["nova"]
`
	)

	f := HookExecutionConfigInit(`{"cloudInstanceManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "CloudInstanceGroup", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "D8TestInstanceClass", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with two pairs of CIG+IC but without provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateICProper))
			f.RunHook()
		})

		It("Hook must not fail, CIG statuses must update", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.Session.Err).Should(gbytes.Say("ERROR: Can't find '.data.zones' in secret kube-system/d8-cloud-instance-manager-cloud-provider."))
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(Equal("[]"))

			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper1").Field("status.error").String()).To(Equal(`Can't find '.data.zones' in secret kube-system/d8-cloud-instance-manager-cloud-provider.`))
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper2").Field("status.error").String()).To(Equal(`Can't find '.data.zones' in secret kube-system/d8-cloud-instance-manager-cloud-provider.`))
		})
	})

	Context("Cluster with two pairs of CIG+IC but without provider secret and previosly stored CIGs data", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateICProper))
			f.ValuesSetFromYaml("cloudInstanceManager.internal.instanceGroups", []byte(`
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
			Expect(f.Session.Err).Should(gbytes.Say("ERROR: Can't find '.data.zones' in secret kube-system/d8-cloud-instance-manager-cloud-provider. Earlier stored version of CIG is in use now!"))

			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(MatchJSON(`[{"name": "proper1","some": "data1"},{"name": "proper2","some": "data2"}]`))

			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper1").Field("status.error").String()).To(Equal(`Can't find '.data.zones' in secret kube-system/d8-cloud-instance-manager-cloud-provider. Earlier stored version of CIG is in use now!`))
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper2").Field("status.error").String()).To(Equal(`Can't find '.data.zones' in secret kube-system/d8-cloud-instance-manager-cloud-provider. Earlier stored version of CIG is in use now!`))
		})

	})

	Context("With manual-rollout-id", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProperManualRolloutId + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Hook must not fail and Values should contain an id", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups.0.manual-rollout-id").String()).To(Equal("test"))
		})
	})

	Context("Proper cluster with two pairs of CIG+IC and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("CIGs must be stored to cloudInstanceManager.internal.instanceGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
	       [
	         {
	           "instanceClassReference": {
	             "kind": "D8TestInstanceClass",
	             "name": "proper1"
	           },
	           "name": "proper1",
               "manual-rollout-id": "",
	           "instanceClass": {
	             "bashible": {
	               "bundle": "centos-7.1.1.1",
	               "dynamicOptions": {},
	               "options": {
	                 "kubernetesVersion": "1.15.4"
	               }
	             }
	           },
	           "zones": [
	             "nova"
	           ]
	         },
	         {
	           "instanceClassReference": {
	             "kind": "D8TestInstanceClass",
	             "name": "proper2"
	           },
	           "zones": [
	             "a",
	             "b"
	           ],
	           "name": "proper2",
               "manual-rollout-id": "",
	           "instanceClass": {
	             "bashible": {
	               "bundle": "slackware-14.1",
	               "dynamicOptions": {},
	               "options": {}
	             }
	           }
	         }
	       ]
	`
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper2").Field("status.error").Value()).To(BeNil())
		})
	})

	Context("Cluster with two proper pairs of CIG+IC, one improper IC and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateICProper + stateICIMroper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("CIGs must be stored to cloudInstanceManager.internal.instanceGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
	       [
	         {
	           "instanceClassReference": {
	             "kind": "D8TestInstanceClass",
	             "name": "proper1"
	           },
	           "name": "proper1",
               "manual-rollout-id": "",
	           "instanceClass": {
	             "bashible": {
	               "bundle": "centos-7.1.1.1",
	               "dynamicOptions": {},
	               "options": {
	                 "kubernetesVersion": "1.15.4"
	               }
	             }
	           },
	           "zones": [
	             "nova"
	           ]
	         },
	         {
	           "instanceClassReference": {
	             "kind": "D8TestInstanceClass",
	             "name": "proper2"
	           },
	           "zones": [
	             "a",
	             "b"
	           ],
	           "name": "proper2",
               "manual-rollout-id": "",
	           "instanceClass": {
	             "bashible": {
	               "bundle": "slackware-14.1",
	               "dynamicOptions": {},
	               "options": {}
	             }
	           }
	         }
	       ]
	`
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(MatchJSON(expectedJSON))
			Expect(f.Session.Err).Should(gbytes.Say("Instance class improper1 is invalid: .spec.bashible.options.kubernetesVersion is mandatory for .spec.bashible.bundle ubuntu-7.1.1.1"))

			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper2").Field("status.error").Value()).To(BeNil())
		})

	})

	Context("Two proper pairs of CIG+IC and a CIG with wrong ref kind", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateCIGWrongKind + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Proper CIGs must be stored to cloudInstanceManager.internal.instanceGroups, hook must warn user about improper CIG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
			       [
			         {
			           "instanceClassReference": {
			             "kind": "D8TestInstanceClass",
			             "name": "proper1"
			           },
			           "name": "proper1",
                       "manual-rollout-id": "",
			           "instanceClass": {
			             "bashible": {
			               "bundle": "centos-7.1.1.1",
			               "dynamicOptions": {},
			               "options": {
			                 "kubernetesVersion": "1.15.4"
			               }
			             }
			           },
			           "zones": [
			             "nova"
			           ]
			         },
			         {
			           "instanceClassReference": {
			             "kind": "D8TestInstanceClass",
			             "name": "proper2"
			           },
			           "zones": [
			             "a",
			             "b"
			           ],
			           "name": "proper2",
                       "manual-rollout-id": "",
			           "instanceClass": {
			             "bashible": {
			               "bundle": "slackware-14.1",
			               "dynamicOptions": {},
			               "options": {}
			             }
			           }
			         }
			       ]
			`
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say("ERROR: Bad CloudInstanceGroup improper: Wrong instanceClassReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass."))

			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper2").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "improper").Field("status.error").String()).To(Equal("Wrong instanceClassReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass."))
		})
	})

	Context("Two proper pairs of CIG+IC and a CIG with wrong ref kind which was stored earlier", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateCIGWrongKind + stateICProper + stateCloudProviderSecret))
			f.ValuesSetFromYaml("cloudInstanceManager.internal.instanceGroups", []byte(`
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

		It("Proper CIGs must be stored to cloudInstanceManager.internal.instanceGroups, old improper CIG data must be saved, hook must warn user about improper CIG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
				       [
				         {
				           "name": "improper",
	                      "some": "imdata"
				         },
				         {
				           "instanceClassReference": {
				             "kind": "D8TestInstanceClass",
				             "name": "proper1"
				           },
				           "name": "proper1",
                           "manual-rollout-id": "",
				           "instanceClass": {
				             "bashible": {
				               "bundle": "centos-7.1.1.1",
				               "dynamicOptions": {},
				               "options": {
				                 "kubernetesVersion": "1.15.4"
				               }
				             }
				           },
				           "zones": [
				             "nova"
				           ]
				         },
				         {
				           "instanceClassReference": {
				             "kind": "D8TestInstanceClass",
				             "name": "proper2"
				           },
				           "zones": [
				             "a",
				             "b"
				           ],
				           "name": "proper2",
                           "manual-rollout-id": "",
				           "instanceClass": {
				             "bashible": {
				               "bundle": "slackware-14.1",
				               "dynamicOptions": {},
				               "options": {}
				             }
				           }
				         }
	                  ]
				`
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say("ERROR: Bad CloudInstanceGroup improper: Wrong instanceClassReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass. Earlier stored version of CIG is in use now!"))

			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper2").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "improper").Field("status.error").String()).To(Equal("Wrong instanceClassReference: Kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass. Earlier stored version of CIG is in use now!"))
		})
	})

	Context("Two proper pairs of CIG+IC and a CIG with wrong ref name", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateCIGWrongRefName + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Proper CIGs must be stored to cloudInstanceManager.internal.instanceGroups, hook must warn user about improper CIG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
			       [
			         {
			           "instanceClassReference": {
			             "kind": "D8TestInstanceClass",
			             "name": "proper1"
			           },
			           "name": "proper1",
                       "manual-rollout-id": "",
			           "instanceClass": {
			             "bashible": {
			               "bundle": "centos-7.1.1.1",
			               "dynamicOptions": {},
			               "options": {
			                 "kubernetesVersion": "1.15.4"
			               }
			             }
			           },
			           "zones": [
			             "nova"
			           ]
			         },
			         {
			           "instanceClassReference": {
			             "kind": "D8TestInstanceClass",
			             "name": "proper2"
			           },
			           "zones": [
			             "a",
			             "b"
			           ],
			           "name": "proper2",
                       "manual-rollout-id": "",
			           "instanceClass": {
			             "bashible": {
			               "bundle": "slackware-14.1",
			               "dynamicOptions": {},
			               "options": {}
			             }
			           }
			         }
			       ]
			`
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: Bad CloudInstanceGroup improper: Wrong instanceClassReference: There is no valid instance class improper of type D8TestInstanceClass.`))

			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper2").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "improper").Field("status.error").String()).To(Equal("Wrong instanceClassReference: There is no valid instance class improper of type D8TestInstanceClass."))
		})
	})

	Context("Two proper pairs of CIG+IC and a CIG with wrong ref name but stored earlier", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateCIGWrongRefName + stateICProper + stateCloudProviderSecret))
			f.ValuesSetFromYaml("cloudInstanceManager.internal.instanceGroups", []byte(`
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

		It("Proper CIGs must be stored to cloudInstanceManager.internal.instanceGroups, old improper CIG data must be saved, hook must warn user about improper CIG", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
			       [
				     {
				       "name": "improper",
	                   "some": "imdata"
				     },
			         {
			           "instanceClassReference": {
			             "kind": "D8TestInstanceClass",
			             "name": "proper1"
			           },
			           "name": "proper1",
                       "manual-rollout-id": "",
			           "instanceClass": {
			             "bashible": {
			               "bundle": "centos-7.1.1.1",
			               "dynamicOptions": {},
			               "options": {
			                 "kubernetesVersion": "1.15.4"
			               }
			             }
			           },
			           "zones": [
			             "nova"
			           ]
			         },
			         {
			           "instanceClassReference": {
			             "kind": "D8TestInstanceClass",
			             "name": "proper2"
			           },
			           "zones": [
			             "a",
			             "b"
			           ],
			           "name": "proper2",
                       "manual-rollout-id": "",
			           "instanceClass": {
			             "bashible": {
			               "bundle": "slackware-14.1",
			               "dynamicOptions": {},
			               "options": {}
			             }
			           }
			         }
			       ]
			`
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(MatchJSON(expectedJSON))

			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: Bad CloudInstanceGroup improper: Wrong instanceClassReference: There is no valid instance class improper of type D8TestInstanceClass. Earlier stored version of CIG is in use now!`))

			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper1").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "proper2").Field("status.error").Value()).To(BeNil())
			Expect(f.KubernetesGlobalResource("CloudInstanceGroup", "improper").Field("status.error").String()).To(Equal("Wrong instanceClassReference: There is no valid instance class improper of type D8TestInstanceClass. Earlier stored version of CIG is in use now!"))
		})
	})
})
