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
	"gopkg.in/yaml.v3"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// pssStandards are the two standards the module renders a SecurityPolicy for, together with the
// constraints each standard is made of. A constraint added to a standard without being reflected in
// the rules of the SecurityPolicy object fails the drift check below.
var pssStandards = map[string][]string{
	"baseline": {
		"D8HostNetwork",
		"D8HostProcesses",
		"D8PrivilegedContainer",
		"D8AppArmor",
		"D8AllowedCapabilities",
		"D8AllowedHostPaths",
		"D8AllowedProcMount",
		"D8SeLinux",
		"D8AllowedSysctls",
		"D8AllowedSeccompProfiles",
	},
	"restricted": {
		"D8AllowedCapabilities",
		"D8AllowPrivilegeEscalation",
		"D8AllowedVolumeTypes",
		"D8AllowedUsers",
		"D8AllowedSeccompProfiles",
	},
}

const pssBaseValues = `
admissionPolicyEngine:
  podSecurityStandards:
    defaultPolicy: Restricted
    enforcementAction: Deny
  internal:
    bootstrapped: true
    ratify:
      webhook:
        key: test-webhook-key
        crt: test-webhook-crt
        ca: test-webhook-ca
    podSecurityStandards:
      enforcementActions:
        - deny
    securityPolicies: []
    trackedConstraintResources: []
    trackedMutateResources: []
    webhook:
      ca: test-webhook-ca
      crt: test-webhook-crt
      key: test-webhook-key
`

// mirrorPolicyName is the name of the SecurityPolicy that carries the rules of a standard through
// the generic SecurityPolicy rendering.
func mirrorPolicyName(standard string) string {
	return "pss-drift-mirror-" + standard
}

// pssPolicyName is the name of the SecurityPolicy the module renders for a standard.
func pssPolicyName(standard string) string {
	return "d8-pod-security-" + standard
}

// pssConstraintName is the name the templates give a constraint of a standard when the action of
// that constraint is the default action of the module.
func pssConstraintName(standard string) string {
	return fmt.Sprintf("d8-pod-security-%s-deny-default", standard)
}

var _ = Describe("Module :: admissionPolicyEngine :: helm template :: pod security standards as SecurityPolicy", func() {
	// f renders the module as it runs in a cluster. mirror renders the rules of the two standards
	// through the generic SecurityPolicy path, so that the constraints those rules amount to can be
	// compared with the constraints the standards render for themselves. Comparing the two renders is
	// what keeps the objects and the constraints from describing different things, without this test
	// having to repeat the mapping between a rule and a constraint parameter.
	f := SetupHelmConfig(pssBaseValues)
	mirror := SetupHelmConfig(pssBaseValues)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		mirrored := make([]interface{}, 0, len(pssStandards))
		for standard := range pssStandards {
			policy := f.KubernetesGlobalResource("SecurityPolicy", pssPolicyName(standard))
			Expect(policy.Exists()).To(BeTrue())

			var policies map[string]interface{}
			Expect(yaml.Unmarshal([]byte(policy.Field("spec.policies").Raw), &policies)).To(Succeed())

			mirrored = append(mirrored, map[string]interface{}{
				"metadata": map[string]interface{}{"name": mirrorPolicyName(standard)},
				"spec": map[string]interface{}{
					"enforcementAction": "Deny",
					"policies":          policies,
					"match":             map[string]interface{}{},
				},
			})
		}

		encoded, err := yaml.Marshal(mirrored)
		Expect(err).ShouldNot(HaveOccurred())

		mirror.ValuesSetFromYaml("global", globalValues)
		mirror.ValuesSet("global.modulesImages", GetModulesImages())
		mirror.ValuesSetFromYaml("admissionPolicyEngine.internal.securityPolicies", string(encoded))
		mirror.HelmRender()
		Expect(mirror.RenderError).ShouldNot(HaveOccurred())
	})

	It("renders a SecurityPolicy for every standard", func() {
		for standard := range pssStandards {
			policy := f.KubernetesGlobalResource("SecurityPolicy", pssPolicyName(standard))
			Expect(policy.Exists()).To(BeTrue(), "SecurityPolicy for the %s standard should exist", standard)
			Expect(policy.Field(`metadata.labels.security\.deckhouse\.io/pod-standard`).String()).
				To(Equal(standard), "the label ties the object to its constraints and keeps it out of the SecurityPolicy rendering")
			Expect(policy.Field("metadata.labels.heritage").String()).To(Equal("deckhouse"))
			Expect(policy.Field("spec.enforcementAction").String()).To(Equal("Deny"))
		}
	})

	It("keeps the object out of the way of the constraints of a standard", func() {
		// A constraint of a standard carries the name of the standard, not the name of the object, so
		// an object that started rendering constraints of its own would show up here.
		for standard, kinds := range pssStandards {
			for _, kind := range kinds {
				Expect(f.KubernetesGlobalResource(kind, pssPolicyName(standard)).Exists()).To(BeFalse(),
					"%s named after the SecurityPolicy object means the object is rendered twice", kind)
			}
		}
	})

	It("describes the same rules as the constraints of the baseline standard", func() {
		expectNoDrift(f, mirror, "baseline")
	})

	It("describes the same rules as the constraints of the restricted standard", func() {
		expectNoDrift(f, mirror, "restricted")
	})
})

// inertParameters lists the parameters a standard sets that the rego of the constraint never reads,
// which is why the generic SecurityPolicy path leaves them out. `allowPrivilegeEscalation` is
// declared in the schema of D8AllowPrivilegeEscalation, but the rule passes the expected value in
// itself and consults the constraint only through a SecurityPolicyException, so the value the
// standard sets changes nothing. The drift check drops such a parameter instead of reporting a
// difference that does not affect what is enforced.
var inertParameters = map[string][]string{
	"D8AllowPrivilegeEscalation": {"allowPrivilegeEscalation"},
}

// expectNoDrift compares the constraints a standard renders with the constraints its rules amount to
// when they go through the generic SecurityPolicy path.
func expectNoDrift(f, mirror *Config, standard string) {
	for _, kind := range pssStandards[standard] {
		standardConstraint := f.KubernetesGlobalResource(kind, pssConstraintName(standard))
		Expect(standardConstraint.Exists()).To(BeTrue(),
			"the %s standard should render %s", standard, kind)

		mirrored := mirror.KubernetesGlobalResource(kind, mirrorPolicyName(standard))
		Expect(mirrored.Exists()).To(BeTrue(),
			"the rules of the SecurityPolicy object should amount to %s, which the %s standard enforces",
			kind, standard)

		fromStandard := constraintParameters(standardConstraint.Field("spec.parameters").String())
		for _, key := range inertParameters[kind] {
			delete(fromStandard, key)
		}

		Expect(constraintParameters(mirrored.Field("spec.parameters").String())).To(Equal(fromStandard),
			"the rules of the SecurityPolicy object and the parameters of %s have drifted apart", kind)
	}
}

// constraintParameters decodes a spec.parameters block. A constraint that carries no parameters
// decodes to an empty map, so that two such constraints compare equal.
func constraintParameters(raw string) map[string]interface{} {
	parameters := map[string]interface{}{}
	if raw == "" {
		return parameters
	}
	Expect(yaml.Unmarshal([]byte(raw), &parameters)).To(Succeed())
	if parameters == nil {
		parameters = map[string]interface{}{}
	}
	return parameters
}
