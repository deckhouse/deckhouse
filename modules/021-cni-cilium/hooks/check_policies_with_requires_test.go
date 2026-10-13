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

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cni-cilium :: hooks :: check_policies_with_requires ::", func() {
	const policies = `
---
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: from-requires
  namespace: shop
spec:
  endpointSelector: {}
  ingress:
  - fromRequires:
    - matchLabels: {env: prod}
---
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: empty-selector
  namespace: shop
spec:
  endpointSelector: {}
  egressDeny:
  - toRequires:
    - {}
---
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: empty-list
  namespace: shop
spec:
  endpointSelector: {}
  ingress:
  - fromRequires: []
    fromEndpoints:
    - {}
---
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: specs-to-requires
specs:
- endpointSelector: {}
- endpointSelector: {}
  egress:
  - toRequires:
    - matchLabels: {env: prod}
---
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: clean
spec:
  endpointSelector: {}
  ingress:
  - fromEndpoints:
    - {}
`

	// Affected policies migrated off fromRequires/toRequires, the rest are deleted.
	const fixedPolicies = `
---
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: from-requires
  namespace: shop
spec:
  endpointSelector: {}
  ingress:
  - fromEndpoints:
    - matchLabels: {env: prod}
---
apiVersion: cilium.io/v2
kind: CiliumClusterwideNetworkPolicy
metadata:
  name: specs-to-requires
specs:
- endpointSelector: {}
- endpointSelector: {}
  egress:
  - toEndpoints:
    - matchLabels: {env: prod}
`

	f := HookExecutionConfigInit(`{"cniCilium":{"internal":{}}}`, "")
	f.RegisterCRD("cilium.io", "v2", "CiliumNetworkPolicy", true)
	f.RegisterCRD("cilium.io", "v2", "CiliumClusterwideNetworkPolicy", false)

	Context("Cluster without policies", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Only expires metrics and saves zero count", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Action).To(Equal(operation.ActionExpireMetrics))

			count, found := requirements.GetValue(PoliciesWithRequiresRequirementKey)
			Expect(found).To(BeTrue())
			Expect(count).To(Equal(0))
		})
	})

	Context("Cluster with policies using fromRequires/toRequires", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(policies))
			f.RunHook()
		})

		It("Sets a metric per affected policy and saves their count", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(4))
			Expect(m[0].Action).To(Equal(operation.ActionExpireMetrics))

			var names []string
			for _, metric := range m[1:] {
				Expect(metric.Name).To(Equal(policiesWithRequiresMetricName))
				Expect(metric.Value).To(Equal(ptr.To(1.0)))
				names = append(names, metric.Labels["kind"]+"/"+metric.Labels["namespace"]+"/"+metric.Labels["name"])
			}
			Expect(names).To(ConsistOf(
				"CiliumNetworkPolicy/shop/from-requires",
				"CiliumNetworkPolicy/shop/empty-selector",
				"CiliumClusterwideNetworkPolicy//specs-to-requires",
			))

			count, found := requirements.GetValue(PoliciesWithRequiresRequirementKey)
			Expect(found).To(BeTrue())
			Expect(count).To(Equal(3))
		})

		Context("All policies are fixed", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(fixedPolicies))
				f.RunHook()
			})

			It("Expires metrics and saves zero count", func() {
				Expect(f).To(ExecuteSuccessfully())
				m := f.MetricsCollector.CollectedMetrics()
				Expect(m).To(HaveLen(1))
				Expect(m[0].Action).To(Equal(operation.ActionExpireMetrics))

				count, found := requirements.GetValue(PoliciesWithRequiresRequirementKey)
				Expect(found).To(BeTrue())
				Expect(count).To(Equal(0))
			})
		})
	})
})
