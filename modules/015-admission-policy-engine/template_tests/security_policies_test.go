/*
Copyright 2023 Flant JSC

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

var _ = Describe("Module :: admissionPolicyEngine :: helm template :: security policies", func() {
	f := SetupHelmConfig(`{admissionPolicyEngine: {podSecurityStandards: {}, internal: {"bootstrapped": true, "podSecurityStandards": {"enforcementActions": ["deny"]}, "securityPolicies": [
{
	"metadata":{"name":"genpolicy"},
	"spec":{
		"policies":{
				"allowHostIPC": true,
				"allowHostNetwork": false,
				"allowHostPID": false,
				"allowPrivilegeEscalation": false,
				"allowPrivileged": false,
				"allowedFlexVolumes": [{"driver": "vmware"}],
				"allowedHostPaths": [{"pathPrefix": "/dev","readOnly": true}],
				"allowedHostPorts": [{"max": 100,"min": 10}],
				"allowedUnsafeSysctls": ["*"],
				"forbiddenSysctls": ["user/example"],
				"allowedProcMount": "default",
				"allowedVolumes": {"volumes": ["csi"]},
				"requiredDropCapabilities": ["ALL"],
				"allowedAppArmor": ["unconfined"],
				"readOnlyRootFilesystem": "true",
				"automountServiceAccountToken": false,
				"allowedClusterRoles": ["*"],
				"runAsUser": {"ranges": [{"max": 500,"min": 300}],"rule": "MustRunAs"},
				"seLinux": [{"role": "role","user": "user"},{"level": "level","type": "type"}],
				"seccompProfiles": {"allowedLocalhostFiles": ["*"],"allowedProfiles": ["RuntimeDefault","Localhost"]},
				"supplementalGroups": {"ranges": [{"max": 1000,"min": 500}],"rule": "MustRunAs"}
		},
		"match":{"namespaceSelector":{"matchNames":["default"]}}}}],
		"trackedConstraintResources": [{"apiGroups":[""],"resources":["pods"]},{"apiGroups":["extensions","networking.k8s.io"],"resources":["ingresses"]}],
		"trackedMutateResources": [{"apiGroups":[""],"resources":["pods"]},{"apiGroups":["extensions","networking.k8s.io"],"resources":["ingresses"]}],
		"webhook": {ca: YjY0ZW5jX3N0cmluZwo=, crt: YjY0ZW5jX3N0cmluZwo=, key: YjY0ZW5jX3N0cmluZwo=}}}}`)

	Context("Cluster with security policies", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("D8AllowedCapabilities", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowedFlexVolumes", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowedHostPaths", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowedProcMount", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowedSeccompProfiles", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowedSysctls", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowedUsers", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowedVolumeTypes", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowPrivilegeEscalation", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8HostNetwork", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8HostProcesses", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8ReadOnlyRootFilesystem", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowedClusterRoles", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AutomountServiceAccountTokenPod", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8SeLinux", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AppArmor", testPolicyName).Exists()).To(BeTrue())
		})
	})
})
