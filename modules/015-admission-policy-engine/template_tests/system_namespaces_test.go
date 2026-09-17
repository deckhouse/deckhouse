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

	// systemNamespaces settings, rendered into the podSecurityStandards section by renderWith.
	systemSettings := ""

	renderWith := func(defaultPolicy, enforcementAction string, enforcementActions ...string) {
		actions := ""
		for _, action := range enforcementActions {
			actions += fmt.Sprintf("\n        - %s", action)
		}
		f.ValuesSetFromYaml("admissionPolicyEngine", fmt.Sprintf(`
podSecurityStandards:
  defaultPolicy: %s
  enforcementAction: %s%s
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
`, defaultPolicy, enforcementAction, systemSettings, actions))
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	BeforeEach(func() {
		systemSettings = ""
	})

	Context("With the default policy set to Baseline and enforcement set to Deny", func() {
		BeforeEach(func() {
			renderWith("Baseline", "Deny", "deny")
		})

		It("Carries the parameters of the standard it warns about", func() {
			// The selectors are checked for every defaultPolicy further down; what matters here is
			// that the warning constraint carries the same checks as the enforcing one, so that a
			// system namespace is measured against the full restricted set.
			restricted := f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-system")
			Expect(restricted.Field("spec.parameters.runAsUser.rule").String()).To(Equal("MustRunAsNonRoot"))

			// The restricted standard builds on baseline, so hostNetwork and the other baseline
			// checks have to reach system namespaces as well.
			baseline := f.KubernetesGlobalResource("D8HostNetwork", "d8-pod-security-baseline-system")
			Expect(baseline.Exists()).To(BeTrue())
			Expect(baseline.Field("spec.parameters.allowHostNetwork").Bool()).To(BeFalse())
		})

		It("Leaves the pod exemption labels working", func() {
			constraint := f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-system")
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
			constraint := f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-system")
			Expect(constraint.Exists()).To(BeTrue())
			Expect(constraint.Field("spec.enforcementAction").String()).To(Equal("warn"))
			// The per-action naming of the non-system constraints must not leak into it.
			Expect(f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-system-default").Exists()).To(BeFalse())
		})
	})

	// defaultPolicy governs non-system namespaces only, so the constraints that serve system ones
	// must come out the same for every value of it. Gating them on defaultPolicy used to disable a
	// standard exactly where the namespace had asked for it: with Restricted the restricted
	// constraint went missing, with Privileged the baseline one, and the namespace was left
	// enforced against the weaker standard while the stronger only warned.
	for _, defaultPolicy := range []string{"Privileged", "Baseline", "Restricted"} {
		defaultPolicy := defaultPolicy

		Context("With the default policy set to "+defaultPolicy, func() {
			BeforeEach(func() {
				renderWith(defaultPolicy, "Deny", "deny")
			})

			It("Enforces both standards in opted-in namespaces", func() {
				for _, c := range []struct{ kind, standard string }{
					{"D8HostNetwork", "baseline"},
					{"D8AllowedUsers", "restricted"},
				} {
					enforcing := f.KubernetesGlobalResource(c.kind, fmt.Sprintf("d8-pod-security-%s-deny-d8-default", c.standard))
					Expect(enforcing.Exists()).To(BeTrue(), c.standard)
					Expect(enforcing.Field("spec.enforcementAction").String()).To(Equal("deny"), c.standard)
					Expect(enforcing.Field("spec.match.namespaces").String()).To(MatchJSON(systemNamespaces), c.standard)
					Expect(enforcing.Field("spec.match.namespaceSelector.matchExpressions").String()).To(MatchJSON(
						`[{"key":"security.deckhouse.io/enable-security-policy-check","operator":"In","values":["true"]}]`), c.standard)
				}
			})

			It("Warns on both standards in namespaces that did not opt in", func() {
				for _, c := range []struct{ kind, standard string }{
					{"D8HostNetwork", "baseline"},
					{"D8AllowedUsers", "restricted"},
				} {
					warning := f.KubernetesGlobalResource(c.kind, fmt.Sprintf("d8-pod-security-%s-system", c.standard))
					Expect(warning.Exists()).To(BeTrue(), c.standard)
					Expect(warning.Field("spec.enforcementAction").String()).To(Equal("warn"), c.standard)
					Expect(warning.Field("spec.match.namespaces").String()).To(MatchJSON(systemNamespaces), c.standard)
					// Opted-in namespaces are excluded, so none is both enforced and warned about.
					Expect(warning.Field("spec.match.namespaceSelector.matchExpressions").String()).To(
						MatchJSON(enforcementNotEnabled), c.standard)
				}
			})
		})
	}

	// The only lever a cluster operator has over system namespaces: the labels that tune the
	// constraints are written by the module that owns the namespace and cannot be edited from
	// outside it, so everything an operator can decide lives in the ModuleConfig.
	Context("With enforcement in system namespaces turned on", func() {
		BeforeEach(func() {
			systemSettings = `
  systemNamespaces:
    enforcementAction: Deny`
			renderWith("Baseline", "Deny", "deny")
		})

		It("Denies in the system namespaces no module opted in", func() {
			for _, c := range []struct{ kind, standard string }{
				{"D8HostNetwork", "baseline"},
				{"D8AllowedUsers", "restricted"},
			} {
				constraint := f.KubernetesGlobalResource(c.kind, fmt.Sprintf("d8-pod-security-%s-system", c.standard))
				Expect(constraint.Exists()).To(BeTrue(), c.standard)
				Expect(constraint.Field("spec.enforcementAction").String()).To(Equal("deny"), c.standard)
				Expect(constraint.Field("spec.match.namespaces").String()).To(MatchJSON(systemNamespaces), c.standard)
			}
		})

		It("Renders no excluded-namespace constraint when nothing is excluded", func() {
			Expect(f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-system-excluded").Exists()).To(BeFalse())
		})
	})

	Context("With system namespaces excluded from enforcement", func() {
		BeforeEach(func() {
			systemSettings = `
  systemNamespaces:
    enforcementAction: Deny
    excludeNamespaces:
      - d8-monitoring
      - d8-team-*`
			renderWith("Baseline", "Deny", "deny")
		})

		const excluded = `["d8-monitoring","d8-team-*"]`

		It("Warns in the excluded namespaces instead of denying", func() {
			for _, c := range []struct{ kind, standard string }{
				{"D8HostNetwork", "baseline"},
				{"D8AllowedUsers", "restricted"},
			} {
				constraint := f.KubernetesGlobalResource(c.kind, fmt.Sprintf("d8-pod-security-%s-system-excluded", c.standard))
				Expect(constraint.Exists()).To(BeTrue(), c.standard)
				Expect(constraint.Field("spec.enforcementAction").String()).To(Equal("warn"), c.standard)
				Expect(constraint.Field("spec.match.namespaces").String()).To(MatchJSON(excluded), c.standard)
				// No namespaceSelector: the exclusion holds whatever labels the namespace carries.
				Expect(constraint.Field("spec.match.namespaceSelector").Exists()).To(BeFalse(), c.standard)
			}
		})

		It("Keeps both enforcing constraints away from the excluded namespaces", func() {
			// The operator's exclusion outranks the module's opt-in label, which the operator
			// cannot edit, so the label-keyed constraint has to honour it too.
			for _, name := range []string{
				"d8-pod-security-restricted-system",
				"d8-pod-security-restricted-deny-d8-default",
			} {
				constraint := f.KubernetesGlobalResource("D8AllowedUsers", name)
				Expect(constraint.Exists()).To(BeTrue(), name)
				Expect(constraint.Field("spec.match.excludedNamespaces").String()).To(MatchJSON(excluded), name)
			}
		})
	})

	Context("With a default policy that reaches non-system namespaces", func() {
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
