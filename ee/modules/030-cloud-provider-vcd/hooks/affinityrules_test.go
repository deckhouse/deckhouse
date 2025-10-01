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
	initValues := `cloudProviderVcd:
  internal:
    providerClusterConfiguration:
      masterNodeGroup:
        affinityRule:
          polarity: AntiAffinity
          required: true
      nodeGroups:
      - name: front
        affinityRule:
          polarity: AntiAffinity
          required: false
      - name: worker
        affinityRule:
          polarity: Affinity
`
	abc := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			abc.BindingContexts.Set(abc.KubeStateSet(""))
			abc.RunHook()
		})

		It("Hook should not fail with errors", func() {
			Expect(abc).To(ExecuteSuccessfully())
			Expect(abc.GoHookError).Should(BeNil())
			Expect(abc.ValuesGet("cloudProviderVcd.internal.affinityRules").Exists()).To(BeTrue())
		})

	})
})
