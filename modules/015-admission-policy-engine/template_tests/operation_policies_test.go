/*
Copyright 2022 Flant JSC

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
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	testPolicyName = "genpolicy"
)

var _ = Describe("Module :: admissionPolicyEngine :: helm template :: operation policies", func() {
	f := SetupHelmConfig(`{admissionPolicyEngine: {podSecurityStandards: {}, internal: {"bootstrapped": true, "operationPolicies": [
{
	"metadata":{"name":"genpolicy"},
	"spec":{
		"policies":{
			"allowedRepos":["foo"],
			"requiredResources":{"limits":["memory"],"requests":["cpu","memory"]},
			"disallowedImageTags":["latest"],
			"requiredLabels": {
				"labels": [
					{ "key": "foo" },
					{ "key": "bar", "allowRegex": "^[a-zA-Z]+.agilebank.demo$" }
				],
				"watchKinds": ["/Pod", "networking.k8s.io/Ingress"]
			},
			"requiredProbes":["livenessProbe","readinessProbe"],
			"maxRevisionHistoryLimit":3,
			"imagePullPolicy":"Always",
			"priorityClassNames":["foo","bar"],
			"checkHostNetworkDNSPolicy":true
		},
		"match":{"namespaceSelector":{"matchNames":["default"]}}}}],
		"trackedConstraintResources": [{"apiGroups":[""],"resources":["pods"]},{"apiGroups":["extensions","networking.k8s.io"],"resources":["ingresses"]}],
		"trackedMutateResources": [{"apiGroups":[""],"resources":["pods"]},{"apiGroups":["extensions","networking.k8s.io"],"resources":["ingresses"]}],
		"webhook": {ca: YjY0ZW5jX3N0cmluZwo=, crt: YjY0ZW5jX3N0cmluZwo=, key: YjY0ZW5jX3N0cmluZwo=}}}}`)

	Context("Cluster with operation policies", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("D8AllowedRepos", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RequiredResources", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8DisallowedTags", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RequiredProbes", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RevisionHistoryLimit", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8ImagePullPolicy", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8PriorityClass", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8DNSPolicy", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RequiredLabels", testPolicyName).Exists()).To(BeTrue())
		})
	})

	Context("Test policies", func() {
		BeforeEach(func() {
			if !gatorAvailable() {
				Skip("gator binary is not available")
			}
		})

		It("Should pass tests", func() {
			gatorCLI := exec.Command("/deckhouse/bin/gator", "verify", "-v", "/deckhouse/modules/015-admission-policy-engine/charts/constraint-templates/templates/operation-policy/test_samples/...")
			res, err := gatorCLI.CombinedOutput()
			if err != nil {
				output := strings.ReplaceAll(string(res), "deckhouse/modules/015-admission-policy-engine/charts/constraint-templates/templates/operation-policy/test_samples", "")
				fmt.Println(output)
				Fail("Gatekeeper policy tests failed:" + err.Error())
			}
		})
	})
})
