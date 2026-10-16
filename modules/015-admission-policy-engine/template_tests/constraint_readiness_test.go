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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// A release that adds a ConstraintTemplate must not remove the webhooks and the constraints that
// already work: only the constraints whose CRD is missing wait for it.
var _ = Describe("Module :: admissionPolicyEngine :: constraints wait for their CRDs", func() {
	f := SetupHelmConfig(`
admissionPolicyEngine:
  podSecurityStandards: {}
  internal:
    bootstrapped: true
    ratify:
      webhook:
        ca: test-ca-placeholder
        crt: test-crt-placeholder
        key: test-key-placeholder
    podSecurityStandards:
      enforcementActions:
        - deny
    trackedConstraintResources: []
    trackedMutateResources: []
    webhook:
      ca: test-ca-placeholder
      crt: test-crt-placeholder
      key: test-key-placeholder
`)

	const baselineDefault = "d8-pod-security-baseline-deny-default"

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
	})

	Context("Without the list of ready kinds, as in a render without the bootstrap hook", func() {
		BeforeEach(func() { f.HelmRender() })

		It("renders every constraint", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("D8HostNetwork", baselineDefault).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", baselineDefault).Exists()).To(BeTrue())
		})
	})

	Context("When only some constraint CRDs exist", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.readyConstraintKinds", `["D8HostNetwork"]`)
			f.HelmRender()
		})

		It("renders the constraints of the ready kinds and keeps the webhook configuration", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("D8HostNetwork", baselineDefault).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", baselineDefault).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-admission-policy-engine-config").Exists()).To(BeTrue())
		})

		It("keeps the documents of the ready kinds intact", func() {
			network := f.KubernetesGlobalResource("D8HostNetwork", baselineDefault)
			Expect(network.Field("apiVersion").String()).To(Equal("constraints.gatekeeper.sh/v1beta1"))
			Expect(network.Field("spec.enforcementAction").String()).To(Equal("deny"))
		})
	})

	Context("With a SecurityPolicy whose kinds are partly ready", func() {
		const securityPolicy = `[{"metadata": {"name": "sp"}, "spec": {"enforcementAction": "Deny", "match": {"namespaceSelector": {"labelSelector": {"matchLabels": {"policy": "on"}}}, "labelSelector": {}}, "policies": {"allowPrivileged": false, "allowHostNetwork": false}}}]`

		It("renders both constraints of the policy when every kind is ready", func() {
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.securityPolicies", securityPolicy)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "sp").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8HostNetwork", "sp").Exists()).To(BeTrue())
		})

		It("drops only the constraint whose kind is not ready", func() {
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.securityPolicies", securityPolicy)
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.readyConstraintKinds", `["D8HostNetwork", "D8PrivilegedContainer"]`)
			f.HelmRender()
			full := f.KubernetesGlobalResource("D8HostNetwork", "sp").ToYaml()

			f.ValuesSetFromYaml("admissionPolicyEngine.internal.readyConstraintKinds", `["D8HostNetwork"]`)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "sp").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8HostNetwork", "sp").ToYaml()).To(Equal(full))
		})
	})
})
