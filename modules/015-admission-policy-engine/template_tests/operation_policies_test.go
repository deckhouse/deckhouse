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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	testPolicyName = "genpolicy"
)

var _ = Describe("Module :: admissionPolicyEngine :: helm template :: operation policies", func() {
	f := SetupHelmConfig(`{global: {discovery: {kubernetesVersion: "1.30"}},admissionPolicyEngine: {denyVulnerableImages: {}, podSecurityStandards: {}, internal: {"bootstrapped": true, "ratify": {"webhook": {"key": "YjY0ZW5jX3N0cmluZwo=", "crt": "YjY0ZW5jX3N0cmluZwo=" , "ca": "YjY0ZW5jX3N0cmluZwo="}}, "podSecurityStandards": {"enforcementActions": ["deny"]}, "operationPolicies": [
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
					{ "key": "bar", "allowedRegex": "^[a-zA-Z]+.agilebank.demo$" }
				],
				"watchKinds": ["/Pod", "networking.k8s.io/Ingress"]
			},
            "requiredAnnotations": {
				"annotations": [
					{ "key": "foo" },
					{ "key": "bar", "allowedRegex": "^[a-zA-Z]+.myapp.demo$" }
				],
				"watchKinds": ["/Namespace"]
			},
			"requiredProbes":["livenessProbe","readinessProbe"],
			"maxRevisionHistoryLimit":3,
			"imagePullPolicy":"Always",
			"priorityClassNames":["foo","bar"],
			"ingressClassNames": ["ing1", "ing2"],
			"storageClassNames": ["st1", "st2"],
			"checkHostNetworkDNSPolicy":true,
			"checkContainerDuplicates":true,
			"replicaLimits":{
					"minReplicas":1,
					"maxReplicas":10
			},
			"disallowedTolerations": [
				{"key": "node-role.kubernetes.io/master", "operator": "Exists"},
				{"key": "node-role.kubernetes.io/control-plane", "operator": "Exists"}
			]
		},
		"match":{"namespaceSelector":{"matchNames":["default"]}}}}],
		"trackedConstraintResources": [{"apiGroups":[""],"resources":["pods","nodes","namespaces"]},{"apiGroups":["extensions","networking.k8s.io"],"resources":["ingresses"]}],
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
			Expect(f.KubernetesGlobalResource("D8IngressClass", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8StorageClass", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8DNSPolicy", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RequiredLabels", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8RequiredAnnotations", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8ContainerDuplicates", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8ReplicaLimits", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8DisallowedTolerations", testPolicyName).Exists()).To(BeTrue())
		})
	})
})
