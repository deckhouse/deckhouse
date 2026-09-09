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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	mtRuleWithLimits = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: limited
spec:
  accessLevel: User
  subjects:
  - kind: Group
    name: everyone
  limitNamespaces:
  - dev
`
	// Options written out with the values they would have had anyway. Nothing here needs the
	// webhook, so nothing here should be reported.
	mtRuleWithInertOptions = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: inert
spec:
  accessLevel: User
  subjects:
  - kind: Group
    name: everyone
  allowAccessToSystemNamespaces: false
  limitNamespaces: []
`
	mtRuleWithoutOptions = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: plain
spec:
  accessLevel: User
  subjects:
  - kind: Group
    name: everyone
`
	mtRuleWithSelector = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: by-selector
spec:
  accessLevel: User
  subjects:
  - kind: Group
    name: everyone
  namespaceSelector:
    labelSelector:
      matchLabels:
        team: a
`
	mtRuleWithSystemAccess = `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: system-access
spec:
  accessLevel: User
  subjects:
  - kind: Group
    name: everyone
  allowAccessToSystemNamespaces: true
`
)

// named returns the Set operations for the per-rule metric, keyed by rule name.
func namedRules(ops []operation.MetricOperation) map[string]string {
	out := make(map[string]string)
	for _, op := range ops {
		if op.Name == "d8_user_authz_rule_needs_multitenancy" && op.Action != operation.ActionExpireMetrics {
			out[op.Labels["name"]] = op.Labels["options"]
		}
	}
	return out
}

func total(ops []operation.MetricOperation) (float64, bool) {
	for _, op := range ops {
		if op.Name == "d8_user_authz_rules_needing_multitenancy" && op.Value != nil {
			return *op.Value, true
		}
	}
	return 0, false
}

var _ = Describe("User-authz hooks :: alert_multitenancy_disabled ::", func() {
	// The alert exists because this used to be a Helm `fail` that stopped the whole module from
	// rendering. One rule, written by anyone allowed to create them, froze every other change to
	// the module - and the message reached only whoever read the release logs.
	f := HookExecutionConfigInit(`{"userAuthz":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "ClusterAuthorizationRule", false)

	Context("Multi-tenancy is off and a rule asks for namespace limits", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthz.enableMultiTenancy", false)
			f.BindingContexts.Set(f.KubeStateSet(mtRuleWithLimits + mtRuleWithSelector + mtRuleWithSystemAccess))
			f.RunHook()
		})

		It("names each rule and the option that will not take effect", func() {
			Expect(f).To(ExecuteSuccessfully())
			named := namedRules(f.MetricsCollector.CollectedMetrics())
			Expect(named).To(HaveLen(3))
			Expect(named).To(HaveKeyWithValue("limited", "limitNamespaces"))
			Expect(named).To(HaveKeyWithValue("by-selector", "namespaceSelector"))
			Expect(named).To(HaveKeyWithValue("system-access", "allowAccessToSystemNamespaces"))
		})

		It("reports the total, which is what the alert uses when there are more than it names", func() {
			sum, ok := total(f.MetricsCollector.CollectedMetrics())
			Expect(ok).To(BeTrue())
			Expect(sum).To(BeEquivalentTo(3))
		})

		It("expires the previous values first, so a fixed rule stops firing", func() {
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).ToNot(BeEmpty())
			Expect(m[0].Action).To(Equal(operation.ActionExpireMetrics))
			Expect(m[0].Group).To(Equal("d8_user_authz_rule_needs_multitenancy"))
		})
	})

	Context("Multi-tenancy is off and the options are written out with their default values", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthz.enableMultiTenancy", false)
			f.BindingContexts.Set(f.KubeStateSet(mtRuleWithInertOptions + mtRuleWithoutOptions))
			f.RunHook()
		})

		// `allowAccessToSystemNamespaces: false` and an emptied `limitNamespaces` ask for nothing
		// the webhook would enforce. Reporting them would send an operator looking for a problem
		// that is not there, and on a cluster where somebody templates their rules it would report
		// most of them.
		It("reports nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(namedRules(f.MetricsCollector.CollectedMetrics())).To(BeEmpty())
			sum, ok := total(f.MetricsCollector.CollectedMetrics())
			Expect(ok).To(BeTrue())
			Expect(sum).To(BeEquivalentTo(0))
		})
	})

	Context("Multi-tenancy is on", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthz.enableMultiTenancy", true)
			f.BindingContexts.Set(f.KubeStateSet(mtRuleWithLimits + mtRuleWithSelector))
			f.RunHook()
		})

		// The options take effect, so there is nothing to report - and the total has to be set to
		// zero rather than left alone, or the alert would keep firing on the last value from before
		// multi-tenancy was turned on.
		It("reports nothing and zeroes the total", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(namedRules(f.MetricsCollector.CollectedMetrics())).To(BeEmpty())
			sum, ok := total(f.MetricsCollector.CollectedMetrics())
			Expect(ok).To(BeTrue())
			Expect(sum).To(BeEquivalentTo(0))
		})
	})

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("userAuthz.enableMultiTenancy", false)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("reports a total of zero", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(namedRules(f.MetricsCollector.CollectedMetrics())).To(BeEmpty())
			sum, ok := total(f.MetricsCollector.CollectedMetrics())
			Expect(ok).To(BeTrue())
			Expect(sum).To(BeEquivalentTo(0))
		})
	})

	Context("More affected rules than the hook will name", func() {
		BeforeEach(func() {
			var b strings.Builder
			for i := 0; i < multitenancyMaxNamed+7; i++ {
				fmt.Fprintf(&b, `
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: bulk-%d
spec:
  accessLevel: User
  subjects:
  - kind: Group
    name: everyone
  limitNamespaces:
  - dev
`, i)
			}
			f.ValuesSet("userAuthz.enableMultiTenancy", false)
			f.BindingContexts.Set(f.KubeStateSet(b.String()))
			f.RunHook()
		})

		// The cap is what keeps a cluster with thousands of rules from turning into thousands of
		// series. The total is how the rest are accounted for, which is why it has to be right.
		It("names at most the cap and counts them all", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(namedRules(f.MetricsCollector.CollectedMetrics())).To(HaveLen(multitenancyMaxNamed))
			sum, ok := total(f.MetricsCollector.CollectedMetrics())
			Expect(ok).To(BeTrue())
			Expect(sum).To(BeEquivalentTo(multitenancyMaxNamed + 7))
		})
	})
})
