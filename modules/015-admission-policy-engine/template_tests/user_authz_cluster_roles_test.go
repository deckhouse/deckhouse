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

var _ = Describe("Module :: admissionPolicyEngine :: user-authz cluster roles", func() {
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

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
	})

	// A SecurityPolicyException lifts the pod-security checks a SecurityPolicy enforces, so it must not
	// be writable below ClusterAdmin: the Admin access level is bound per namespace by AuthorizationRule,
	// and a namespace-scoped Admin able to create an exception can grant its own pods hostPath and
	// privileged, escaping to the node.
	It("must not grant any access level below ClusterAdmin write access to securitypolicyexceptions", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		Expect(f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:admission-policy-engine:admin").Exists()).To(BeFalse())

		user := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:admission-policy-engine:user")
		Expect(user.Exists()).To(BeTrue())
		Expect(user.Field("rules.0.verbs").String()).To(Equal(`["get","list","watch"]`))
	})

	It("must keep securitypolicyexceptions writable by ClusterAdmin", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		clusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:admission-policy-engine:cluster-admin")
		Expect(clusterAdmin.Exists()).To(BeTrue())
		Expect(clusterAdmin.Field("rules.0.resources").String()).
			To(Equal(`["operationpolicies","securitypolicies","securitypolicyexceptions"]`))
	})
})
