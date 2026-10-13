/*
Copyright 2026 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: export pss default policy metric", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"podSecurityStandards": {"enforcementAction": "Deny"},"internal": {"bootstrapped": true, "podSecurityStandards": {"enforcementActions": []}}}}`,
		`{"admissionPolicyEngine": {"podSecurityStandards": {}}}`,
	)

	expectMetric := func(code float64) {
		m := f.MetricsCollector.CollectedMetrics()
		Expect(m).To(HaveLen(2))
		Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
			Group:  "d8_admission_policy_engine_pss_default_policy",
			Action: operation.ActionExpireMetrics,
		}))
		Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
			Name:   "d8_admission_policy_engine_pss_default_policy",
			Group:  "d8_admission_policy_engine_pss_default_policy",
			Action: operation.ActionGaugeSet,
			Value:  ptr.To(code),
			Labels: map[string]string{},
		}))
	}

	Context("The default policy is not set in the configuration", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Reports the Baseline policy, the default of the parameter", func() {
			Expect(f).To(ExecuteSuccessfully())
			expectMetric(2)
		})
	})

	for policy, code := range map[string]float64{"Privileged": 1, "Baseline": 2, "Restricted": 3} {
		policy, code := policy, code

		Context("The default policy is set to "+policy, func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.ValuesSet("admissionPolicyEngine.podSecurityStandards.defaultPolicy", policy)
				f.RunHook()
			})

			It("Reports the configured policy and leaves it unchanged", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("admissionPolicyEngine.podSecurityStandards.defaultPolicy").String()).To(Equal(policy))
				expectMetric(code)
			})
		})
	}
})
