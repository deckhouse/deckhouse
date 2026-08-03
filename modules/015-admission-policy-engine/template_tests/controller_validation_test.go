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

var _ = Describe("Module :: admissionPolicyEngine :: helm template :: controller validation toggle", func() {
	var ctlValidationConfigs = []struct {
		name                 string
		controllerValidation interface{}
		expectControllers   bool
	}{
		{
			name:                 "controllerValidation true (default) — matches Pods and controllers",
			controllerValidation: true,
			expectControllers:   true,
		},
		{
			name:                 "controllerValidation false — matches only Pods",
			controllerValidation: false,
			expectControllers:   false,
		},
		{
			name:                 "controllerValidation not set — defaults to true (matches Pods and controllers)",
			controllerValidation: nil,
			expectControllers:   true,
		},
	}

	for _, tc := range ctlValidationConfigs {
		tc := tc

		Context(tc.name, func() {
			f := SetupHelmConfig(`
	admissionPolicyEngine:
			internal:
			  ratify:
			    webhook:
			      ca: test-ca-placeholder
			      crt: test-crt-placeholder
			      key: test-key-placeholder
			  podSecurityStandards:
			    enforcementActions:
			      - deny
			  bootstrapped: true
			  webhook:
			    ca: test-ca-placeholder
			    crt: test-crt-placeholder
			    key: test-key-placeholder
			  trackedConstraintResources: []
			  trackedMutateResources: []
			podSecurityStandards:
			  defaultPolicy: Baseline
			  enforcementAction: Deny
			  policies:
			    hostPorts:
			      knownRanges:
			        - max: 35000
			          min: 30000
	`)

			BeforeEach(func() {
				if tc.controllerValidation != nil {
					f.ValuesSet("admissionPolicyEngine.podSecurityStandards.controllerValidation", tc.controllerValidation)
				}
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.HelmRender()
			})

			It("should render without errors", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("D8HostNetwork constraint should have correct match.kinds", func() {
				constraint := f.KubernetesGlobalResource("D8HostNetwork", "d8-pod-security-baseline-deny-default")
				Expect(constraint.Exists()).To(BeTrue(), "D8HostNetwork constraint should exist")

				spec := getConstraintSpecMap(constraint)
				match, ok := spec["match"].(map[string]interface{})
				Expect(ok).To(BeTrue(), "spec.match should exist")

				kinds, ok := match["kinds"].([]interface{})
				Expect(ok).To(BeTrue(), "spec.match.kinds should be a list")

				validateKinds(kinds, tc.expectControllers, "D8HostNetwork")
			})

			It("D8HostProcesses constraint should have correct match.kinds", func() {
				constraint := f.KubernetesGlobalResource("D8HostProcesses", "d8-pod-security-baseline-deny-default")
				Expect(constraint.Exists()).To(BeTrue(), "D8HostProcesses constraint should exist")

				spec := getConstraintSpecMap(constraint)
				match, ok := spec["match"].(map[string]interface{})
				Expect(ok).To(BeTrue(), "spec.match should exist")

				kinds, ok := match["kinds"].([]interface{})
				Expect(ok).To(BeTrue(), "spec.match.kinds should be a list")

				validateKinds(kinds, tc.expectControllers, "D8HostProcesses")
			})

			It("D8PrivilegedContainer constraint should have correct match.kinds", func() {
				constraint := f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-pod-security-baseline-deny-default")
				Expect(constraint.Exists()).To(BeTrue(), "D8PrivilegedContainer constraint should exist")

				spec := getConstraintSpecMap(constraint)
				match, ok := spec["match"].(map[string]interface{})
				Expect(ok).To(BeTrue(), "spec.match should exist")

				kinds, ok := match["kinds"].([]interface{})
				Expect(ok).To(BeTrue(), "spec.match.kinds should be a list")

				validateKinds(kinds, tc.expectControllers, "D8PrivilegedContainer")
			})
		})
	}
})

// validateKinds checks that match.kinds contains Pod and optionally controllers.
func validateKinds(kinds []interface{}, expectControllers bool, constraintName string) {
	Expect(kinds).NotTo(BeEmpty(), "%s should have at least one kind entry", constraintName)

	hasPod := false
	hasDeployment := false
	hasJob := false

	for _, k := range kinds {
		kindEntry, ok := k.(map[string]interface{})
		Expect(ok).To(BeTrue(), "%s kind entry should be a map", constraintName)

		kindsList, ok := kindEntry["kinds"].([]interface{})
		Expect(ok).To(BeTrue(), "%s kinds should be a list", constraintName)

		for _, kindName := range kindsList {
			name, ok := kindName.(string)
			Expect(ok).To(BeTrue())
			switch name {
			case "Pod":
				hasPod = true
			case "Deployment":
				hasDeployment = true
			case "Job":
				hasJob = true
			}
		}
	}

	Expect(hasPod).To(BeTrue(), "%s should always match Pod kind", constraintName)

	if expectControllers {
		Expect(hasDeployment).To(BeTrue(), "%s should match Deployment when controllerValidation is enabled", constraintName)
		Expect(hasJob).To(BeTrue(), "%s should match Job when controllerValidation is enabled", constraintName)
	} else {
		Expect(hasDeployment).To(BeFalse(), "%s should NOT match Deployment when controllerValidation is disabled", constraintName)
		Expect(hasJob).To(BeFalse(), "%s should NOT match Job when controllerValidation is disabled", constraintName)
	}
}

