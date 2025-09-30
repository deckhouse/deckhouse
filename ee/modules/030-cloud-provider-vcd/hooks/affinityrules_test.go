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

var _ = Describe("Modules :: cloud-provider-vcd :: hooks :: affinity_rules_from_vcdinstanceclass ::", func() {
	initValues := `
cloudProviderVcd:
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
	instanceClassA := `
apiVersion: deckhouse.io/v1
kind: VCDInstanceClass
metadata:
	name: class-a
spec:
	affinityRule:
		polarity: Affinity
		required: true
status:
	nodeGroupConsumers:
	- monitoring
`	
	a := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(instanceClassA))
			a.RunHook()
		})
		It("Hook should not fail with errors", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.GoHookError).Should(BeNil())
			Expect(a.ValuesGet("cloudProviderVcd.internal.affinityRules").Exists()).To(BeTrue())
		})
		
	})
})
