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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: admissionPolicyEngine :: helm template :: security policies", func() {
	f := SetupHelmConfig(`{admissionPolicyEngine: {podSecurityStandards: {}, internal: {"bootstrapped": true, "ratify": {"webhook": {"key": "YjY0ZW5jX3N0cmluZwo=", "crt": "YjY0ZW5jX3N0cmluZwo=" , "ca": "YjY0ZW5jX3N0cmluZwo="}}, "podSecurityStandards": {"enforcementActions": ["deny"]}, "securityPolicies": [
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
				"allowRbacWildcards": false,
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
				"supplementalGroups": {"ranges": [{"max": 1000,"min": 500}],"rule": "MustRunAs"},
				"verifyImageSignatures": [{"dockerCfg": "zxc=", "reference": "*", "publicKeys": ["someKey1", "someKey2"]}]
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
			Expect(f.KubernetesGlobalResource("D8VerifyImageSignatures", testPolicyName).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowRbacWildcards", testPolicyName).Exists()).To(BeTrue())
		})

		It("Minimal security policy must not render unrelated constraints", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// allowPrivileged and allowPrivilegeEscalation have documented default "false",
			// so their constraints must be created even when fields are omitted.
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "minpolicy").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8AllowPrivilegeEscalation", "minpolicy").Exists()).To(BeTrue())

			// All other constraints must NOT be created when their fields are not specified.
			Expect(f.KubernetesGlobalResource("D8HostNetwork", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8HostProcesses", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AutomountServiceAccountTokenPod", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8ReadOnlyRootFilesystem", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowedCapabilities", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowedFlexVolumes", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowedHostPaths", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowedVolumeTypes", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowedSysctls", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowedUsers", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8SeLinux", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowedProcMount", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AppArmor", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowedSeccompProfiles", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowedClusterRoles", "minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8AllowRbacWildcards", "minpolicy").Exists()).To(BeFalse())
		})

		It("Policy with only allowedHostPorts must create D8HostNetwork with allowHostNetwork=true", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			hostNet := f.KubernetesGlobalResource("D8HostNetwork", "hostportspolicy")
			Expect(hostNet.Exists()).To(BeTrue())
			Expect(hostNet.Field("spec.parameters.allowHostNetwork").Bool()).To(BeTrue())
		})

		It("Policy with only allowHostPID must create D8HostProcesses with allowHostIPC=true", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			hostProc := f.KubernetesGlobalResource("D8HostProcesses", "pidpolicy")
			Expect(hostProc.Exists()).To(BeTrue())
			Expect(hostProc.Field("spec.parameters.allowHostPID").Bool()).To(BeFalse())
			Expect(hostProc.Field("spec.parameters.allowHostIPC").Bool()).To(BeTrue())
		})

		It("All security policy constraints must have valid YAML", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			securityConstraints := []string{
				"D8AllowedCapabilities",
				"D8AllowedFlexVolumes",
				"D8AllowedHostPaths",
				"D8AllowedProcMount",
				"D8AllowedSeccompProfiles",
				"D8AllowedSysctls",
				"D8AllowedUsers",
				"D8AllowedVolumeTypes",
				"D8AllowPrivilegeEscalation",
				"D8HostNetwork",
				"D8HostProcesses",
				"D8PrivilegedContainer",
				"D8ReadOnlyRootFilesystem",
				"D8AllowedClusterRoles",
				"D8AutomountServiceAccountTokenPod",
				"D8SeLinux",
				"D8AppArmor",
				"D8VerifyImageSignatures",
				"D8AllowRbacWildcards",
			}

			for _, constraintKind := range securityConstraints {
				constraint := f.KubernetesGlobalResource(constraintKind, testPolicyName)
				if constraint.Exists() {
					// Get the resource as a map to validate YAML structure
					var resourceMap map[string]interface{}
					err := yaml.Unmarshal([]byte(constraint.ToYaml()), &resourceMap)
					if err != nil {
						Fail(fmt.Sprintf("Invalid YAML for resource %s: %v\nYAML content:\n%s", constraintKind, err, constraint.ToYaml()))
					}
					validateYAML(resourceMap, constraintKind)
				}
			}
		})
	})
})
