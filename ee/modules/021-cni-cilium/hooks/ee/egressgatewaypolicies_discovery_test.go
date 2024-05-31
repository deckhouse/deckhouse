/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package ee

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("cni-cilium :: hooks :: egress_policies_discovery ::", func() {
	f := HookExecutionConfigInit(`{"cniCilium":{"internal": {"egressGatewayPolicies": {}}}}`, "")
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "EgressGatewayPolicy", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})
		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())
		})

		Context("Adding EgressGatewayPolicies", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGatewayPolicy
metadata:
  name: egp-dev
spec:
  selectors:
  - podSelector:
      matchLabels:
        role: worker
  destinationCIDRs:
  - "192.168.0.0/16"
  excludedCIDRs:
  - "10.0.2.0/24"
  egressGatewayName: egfg-dev
---
apiVersion: network.deckhouse.io/v1alpha1
kind: EgressGatewayPolicy
metadata:
  name: egp-prod
spec:
  selectors:
  - podSelector:
      matchLabels:
        role: master
  destinationCIDRs:
  - "172.16.0.0/16"
  excludedCIDRs:
  - "10.0.3.0/24"
  egressGatewayName: egfg-prod
`))
				f.RunHook()
			})

			It("EgressGatewayPolicies must be present in internal values", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

				Expect(f.ValuesGet("cniCilium.internal.egressGatewayPolicies").String()).To(MatchJSON(`
[
	{
		"name": "egp-dev",
		"egressGatewayName": "egfg-dev",
		"selectors": [
                {
                  "podSelector": {
                    "matchLabels": {
                      "role": "worker"
                    }
                  }
                }
		],
		"destinationCIDRs": [
			"192.168.0.0/16"
		],
		"excludedCIDRs": [
			"10.0.2.0/24"
		]
	},
	{
		"name": "egp-prod",
		"egressGatewayName": "egfg-prod",
		"selectors": [
                {
                  "podSelector":{
                    "matchLabels": {
                      "role": "master"
                    }
                  }
                }
		],
		"destinationCIDRs": [
			"172.16.0.0/16"
		],
		"excludedCIDRs": [
			"10.0.3.0/24"
		]
	}
]
`))
			})
		})
	})
})
