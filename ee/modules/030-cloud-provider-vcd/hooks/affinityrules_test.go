/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vcd :: hooks :: affinity rules from vcdinstanceclass ::", func() {
	initValuesWithNoRules := `
cloudProviderVcd:
  internal:
    providerClusterConfiguration:
      masterNodeGroup:
        instanceClass:
          rootDiskSizeGb: 20
          etcdDiskSizeGb: 20
          sizingPolicy: 4cpu8ram
          storageProfile: nvme
          template: Templates/ubuntu-focal-20.04
      nodeGroups:
      - name: front
        instanceClass:
          rootDiskSizeGb: 20
          sizingPolicy: 16cpu32ram
          template: Templates/ubuntu-focal-20.04
      - name: worker
        instanceClass:
          rootDiskSizeGb: 20
          sizingPolicy: 16cpu32ram
          template: Templates/ubuntu-focal-20.04
`

	initValuesWithRules := `
cloudProviderVcd:
  internal:
    providerClusterConfiguration:
      masterNodeGroup:
        instanceClass:
          affinityRule:
            polarity: AntiAffinity
            required: true
          rootDiskSizeGb: 20
          etcdDiskSizeGb: 20
          sizingPolicy: 4cpu8ram
          storageProfile: nvme
          template: Templates/ubuntu-focal-20.04
      nodeGroups:
      - name: front
        instanceClass:
          rootDiskSizeGb: 20
          sizingPolicy: 16cpu32ram
          template: Templates/ubuntu-focal-20.04
          affinityRule:
            polarity: AntiAffinity
            required: false
      - name: worker
        instanceClass:
          rootDiskSizeGb: 20
          sizingPolicy: 16cpu32ram
          template: Templates/ubuntu-focal-20.04
          affinityRule:
            polarity: Affinity
`

	vcdInstanceClasses := `
---
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: one
spec:
  rootDiskSizeGb: 90
  sizingPolicy: c2m4
  storageProfile: vSAN-LAB-PLATFORM-MSK-1-R5
  template: DSS-LIBRARY/ubuntu-22.04-dkp
  affinityRule:
    polarity: Affinity
    required: true
status:
  nodeGroupConsumers:
  - ng-one
  - ng-two
  - ng-three
---
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: two
spec:
  rootDiskSizeGb: 90
  sizingPolicy: c2m4
  storageProfile: vSAN-LAB-PLATFORM-MSK-1-R5
  template: DSS-LIBRARY/ubuntu-22.04-dkp
status:
  nodeGroupConsumers:
  - ng-four
  - ng-five
---
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
  name: three
spec:
  rootDiskSizeGb: 90
  sizingPolicy: c2m4
  storageProfile: vSAN-LAB-PLATFORM-MSK-1-R5
  template: DSS-LIBRARY/ubuntu-22.04-dkp
  affinityRule:
    polarity: AntiAffinity
status:
  nodeGroupConsumers:
  - ng-six
`
	a := HookExecutionConfigInit(initValuesWithNoRules, "{}")
	Context("No affinity rules are defined", func() {
		BeforeEach(func() {
			a.RunHook()
		})

		It("Hook should not fail with errors", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.GoHookError).Should(BeNil())
			Expect(a.ValuesGet("cloudProviderVcd.internal.affinityRules").String()).To(MatchJSON("[]"))
		})

	})

	b := HookExecutionConfigInit(initValuesWithRules, "{}")
	Context("Affinity rules are defined", func() {
		BeforeEach(func() {
			b.RunHook()
		})

		It("Hook should not fail with errors", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.GoHookError).Should(BeNil())
			Expect(b.ValuesGet("cloudProviderVcd.internal.affinityRules").String()).To(MatchJSON(`
[
  {
    "polarity": "AntiAffinity",
    "required": true,
    "nodeGroupName": "master"
  },
  {
    "polarity": "AntiAffinity",
    "required": false,
    "nodeGroupName": "front"
  },
  {
    "polarity": "Affinity",
    "required": false,
    "nodeGroupName": "worker"
  }
]
`))
		})
	})

	c := HookExecutionConfigInit(initValuesWithNoRules, "{}")
	c.RegisterCRD("deckhouse.io", "v1", "VCDInstanceClass", false)
	Context("Affinity rules are defined in VCDInstanceClass", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(vcdInstanceClasses))
			c.RunHook()
		})

		It("Hook should not fail with errors and get rules from VCDInstanceClass", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(c.GoHookError).Should(BeNil())
			Expect(c.ValuesGet("cloudProviderVcd.internal.affinityRules").String()).To(MatchJSON(`
[
  {
    "polarity": "Affinity",
    "required": true,
    "nodeGroupName": "ng-one"
  },
  {
    "polarity": "Affinity",
    "required": true,
    "nodeGroupName": "ng-two"
  },
  {
    "polarity": "Affinity",
    "required": true,
    "nodeGroupName": "ng-three"
  },
  {
    "polarity": "AntiAffinity",
    "required": false,
    "nodeGroupName": "ng-six"
  }
]
`))
		})
	})

	d := HookExecutionConfigInit(initValuesWithRules, "{}")
	d.RegisterCRD("deckhouse.io", "v1", "VCDInstanceClass", false)
	Context("Affinity rules are defined in both values and VCDInstanceClass", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(vcdInstanceClasses))
			d.RunHook()
		})

		It("Hook should not fail with errors and merge rules from values and VCDInstanceClass", func() {
			Expect(d).To(ExecuteSuccessfully())
			Expect(d.GoHookError).Should(BeNil())
			Expect(d.ValuesGet("cloudProviderVcd.internal.affinityRules").String()).To(MatchJSON(`
[
  {
    "polarity": "AntiAffinity",
    "required": true,
    "nodeGroupName": "master"
  },
  {
    "polarity": "AntiAffinity",
    "required": false,
    "nodeGroupName": "front"
  },
  {
    "polarity": "Affinity",
    "required": false,
    "nodeGroupName": "worker"
  },
  {
    "polarity": "Affinity",
    "required": true,
    "nodeGroupName": "ng-one"
  },
  {
    "polarity": "Affinity",
    "required": true,
    "nodeGroupName": "ng-two"
  },
  {
    "polarity": "Affinity",
    "required": true,
    "nodeGroupName": "ng-three"
  },
  {
    "polarity": "AntiAffinity",
    "required": false,
    "nodeGroupName": "ng-six"
  }
]
`))
		})
	})
})
