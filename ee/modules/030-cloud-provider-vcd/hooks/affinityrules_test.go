/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
  // "fmt"

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
      Expect(b.ValuesGet("cloudProviderVcd.internal.affinityRules").Exists()).To(BeTrue())
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
})
