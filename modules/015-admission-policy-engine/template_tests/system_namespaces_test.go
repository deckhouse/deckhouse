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
					// Selected by labels alone, with no name list: a namespace the platform owns
					// and a module opted in stays enforced even if it is not named `d8-*`, which
					// is how this constraint behaved before the names entered the picture.
					Expect(enforcing.Field("spec.match.namespaces").Exists()).To(BeFalse(), c.standard)
					Expect(enforcing.Field("spec.match.namespaceSelector.matchExpressions").String()).To(MatchJSON(
						`[{"key":"heritage","operator":"In","values":["deckhouse"]},
						  {"key":"security.deckhouse.io/enable-security-policy-check","operator":"In","values":["true"]}]`), c.standard)
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

	// How the platform treats its own namespaces is decided in the chart, by the
	// `system_namespaces_action` and `system_namespaces_excluded` constants, and not in the
	// ModuleConfig: raising the action blocks the converge of every module that does not yet
	// comply, which is a decision to ship with a release rather than to leave to a cluster
	// operator. The specs below pin the values those constants have now, so that changing one
	// shows up here.
	Context("With the constants the module ships", func() {
		BeforeEach(func() {
			renderWith("Baseline", "Deny", "deny")
		})

		It("Warns in the system namespaces no module opted in", func() {
			for _, c := range []struct{ kind, standard string }{
				{"D8HostNetwork", "baseline"},
				{"D8AllowedUsers", "restricted"},
			} {
				constraint := f.KubernetesGlobalResource(c.kind, fmt.Sprintf("d8-pod-security-%s-system", c.standard))
				Expect(constraint.Exists()).To(BeTrue(), c.standard)
				Expect(constraint.Field("spec.enforcementAction").String()).To(Equal("warn"), c.standard)
				Expect(constraint.Field("spec.match.namespaces").String()).To(MatchJSON(systemNamespaces), c.standard)
				Expect(constraint.Field("spec.match.excludedNamespaces").Exists()).To(BeFalse(), c.standard)
			}
		})

		It("Renders no excluded-namespace constraint while nothing is excluded", func() {
			// The constraint appears as soon as `system_namespaces_excluded` names a namespace,
			// and warns there whatever the other two constraints would do.
			Expect(f.KubernetesGlobalResource("D8AllowedUsers", "d8-pod-security-restricted-system-excluded").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8HostNetwork", "d8-pod-security-baseline-system-excluded").Exists()).To(BeFalse())
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
				`[{"key":"heritage","operator":"NotIn","values":["deckhouse"]},
				  {"key":"security.deckhouse.io/pod-policy","operator":"In","values":["baseline","restricted"]}]`))
		})
	})
})
