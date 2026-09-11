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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// Pod Security Standards in system namespaces. Every namespace named `d8-*` or `kube-*`
// is checked in warn mode, whether or not it carries
// `security.deckhouse.io/enable-security-policy-check`.
var _ = Describe("Module :: admissionPolicyEngine :: helm template :: system namespaces", func() {
	f := SetupHelmConfig(``)

	const (
		systemNamespaces      = `["d8-*","kube-*"]`
		enforcementNotEnabled = `[{"key":"security.deckhouse.io/enable-security-policy-check","operator":"NotIn","values":["true"]}]`
	)

	renderWith := func(defaultPolicy, enforcementAction string, enforcementActions ...string) {
		actions := ""
		for _, action := range enforcementActions {
			actions += fmt.Sprintf("\n        - %s", action)
		}
		f.ValuesSetFromYaml("admissionPolicyEngine", fmt.Sprintf(`
podSecurityStandards:
  defaultPolicy: %s
  enforcementAction: %s
internal:
  bootstrapped: true
  podSecurityStandards:
    enforcementActions:%s
  ratify:
    webhook:
      ca: test-ca
      crt: test-crt
      key: test-key
  webhook:
    ca: test-ca
    crt: test-crt
    key: test-key
  trackedConstraintResources: []
  trackedMutateResources: []
`, defaultPolicy, enforcementAction, actions))
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	Context("With the default policy set to Baseline and enforcement set to Deny", func() {
		BeforeEach(func() {
			renderWith("Baseline", "Deny", "deny")
		})

		It("Warns on the restricted standard in every system namespace", func() {
			// D8AllowedUsers carries the restricted requirement to run as a non-root user.
			constraint := f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-warn-system")
			Expect(constraint.Exists()).To(BeTrue())
			Expect(constraint.Field("spec.enforcementAction").String()).To(Equal("warn"))
			Expect(constraint.Field("spec.parameters.runAsUser.rule").String()).To(Equal("MustRunAsNonRoot"))
			Expect(constraint.Field("spec.match.namespaces").String()).To(MatchJSON(systemNamespaces))
			Expect(constraint.Field("spec.match.namespaceSelector.matchExpressions").String()).To(MatchJSON(enforcementNotEnabled))
		})

		It("Warns on the baseline standard in every system namespace", func() {
			// The restricted standard builds on baseline, so hostNetwork and the other
			// baseline checks have to reach system namespaces as well.
			constraint := f.KubernetesGlobalResource("D8HostNetwork", "d8-pod-security-baseline-warn-system")
			Expect(constraint.Exists()).To(BeTrue())
			Expect(constraint.Field("spec.enforcementAction").String()).To(Equal("warn"))
			Expect(constraint.Field("spec.match.namespaces").String()).To(MatchJSON(systemNamespaces))
			Expect(constraint.Field("spec.match.namespaceSelector.matchExpressions").String()).To(MatchJSON(enforcementNotEnabled))
		})

		It("Leaves the pod exemption labels working", func() {
			constraint := f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-warn-system")
			Expect(constraint.Field("spec.match.labelSelector.matchExpressions").String()).To(MatchJSON(
				`[{"key":"security.deckhouse.io/skip-pss-check","operator":"NotIn","values":["true"]},
				  {"key":"gatekeeper.sh/operation","operator":"NotIn","values":["webhook"]}]`))
		})
	})

	Context("With more than one enforcement action in use", func() {
		BeforeEach(func() {
			renderWith("Baseline", "Deny", "deny", "warn")
		})

		It("Renders the system constraint once, independently of the action", func() {
			constraint := f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-warn-system")
			Expect(constraint.Exists()).To(BeTrue())
			Expect(constraint.Field("spec.enforcementAction").String()).To(Equal("warn"))
			// The per-action naming of the non-system constraints must not leak into it.
			Expect(f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-warn-system-default").Exists()).To(BeFalse())
		})
	})

	Context("With the default policy set to Restricted", func() {
		BeforeEach(func() {
			renderWith("Restricted", "Deny", "deny")
		})

		It("Warns in opted-in namespaces too, because no restricted constraint enforces there", func() {
			// d8-pod-security-restricted-deny-d8-default is not rendered for this default policy,
			// so excluding opted-in namespaces would leave them without any restricted check.
			Expect(f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-deny-d8-default").Exists()).To(BeFalse())

			constraint := f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-warn-system")
			Expect(constraint.Exists()).To(BeTrue())
			Expect(constraint.Field("spec.match.namespaces").String()).To(MatchJSON(systemNamespaces))
			Expect(constraint.Field("spec.match.namespaceSelector").Exists()).To(BeFalse())
		})
	})

	Context("With the default policy set to Privileged", func() {
		BeforeEach(func() {
			renderWith("Privileged", "Deny", "deny")
		})

		It("Keeps the denying baseline constraint away from system namespaces", func() {
			// Most system namespaces are labeled `security.deckhouse.io/pod-policy: restricted`,
			// so without the exclusion this constraint would deny in them as soon as the
			// system-namespaces webhook starts routing their workloads to Gatekeeper.
			constraint := f.KubernetesGlobalResource("D8HostNetwork", "d8-pod-security-baseline-deny-default")
			Expect(constraint.Exists()).To(BeTrue())
			Expect(constraint.Field("spec.match.excludedNamespaces").String()).To(MatchJSON(systemNamespaces))
			Expect(constraint.Field("spec.match.namespaceSelector.matchExpressions").String()).To(MatchJSON(
				`[{"key":"security.deckhouse.io/pod-policy","operator":"In","values":["baseline","restricted"]}]`))
		})
	})
})
