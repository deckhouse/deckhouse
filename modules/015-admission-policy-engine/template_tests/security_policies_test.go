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
	f := SetupHelmConfig(`
admissionPolicyEngine:
  podSecurityStandards: {}
  internal:
    bootstrapped: true
    ratify:
      webhook:
        key: YjY0ZW5jX3N0cmluZwo=
        crt: YjY0ZW5jX3N0cmluZwo=
        ca: YjY0ZW5jX3N0cmluZwo=
    podSecurityStandards:
      enforcementActions:
        - deny
    securityPolicies:
      - metadata:
          name: genpolicy
        spec:
          enforcementAction: Warn
          policies:
            allowHostIPC: true
            allowHostNetwork: false
            allowHostPID: false
            allowPrivilegeEscalation: false
            allowPrivileged: false
            allowedFlexVolumes:
              - driver: vmware
            allowedHostPaths:
              - pathPrefix: /dev
                readOnly: true
            allowedHostPorts:
              - max: 100
                min: 10
            allowedUnsafeSysctls:
              - "*"
            allowRbacWildcards: false
            forbiddenSysctls:
              - user/example
            allowedProcMount: default
            allowedVolumes:
              - csi
            requiredDropCapabilities:
              - ALL
            allowedAppArmor:
              - unconfined
            readOnlyRootFilesystem: true
            automountServiceAccountToken: false
            allowedClusterRoles:
              - "*"
            runAsUser:
              ranges:
                - max: 500
                  min: 300
              rule: MustRunAs
            seLinux:
              - role: role
                user: user
              - level: level
                type: type
            seccompProfiles:
              allowedLocalhostFiles:
                - "*"
              allowedProfiles:
                - RuntimeDefault
                - Localhost
            supplementalGroups:
              ranges:
                - max: 1000
                  min: 500
              rule: MustRunAs
            verifyImageSignatures:
              - dockerCfg: zxc=
                reference: "*"
                publicKeys:
                  - someKey1
                  - someKey2
          match:
            namespaceSelector:
              matchNames:
                - default
              excludeNames:
                - kube-system
              labelSelector:
                matchLabels:
                  security-policy.deckhouse.io/enabled: "true"
            labelSelector:
              matchLabels:
                security-policy.deckhouse.io/enabled: "true"
      - metadata:
          name: minpolicy
        spec:
          policies:
            allowPrivileged: false
          match:
            namespaceSelector:
              matchNames:
                - default
      - metadata:
          name: hostportspolicy
        spec:
          policies:
            allowedHostPorts:
              - min: 8080
                max: 8080
          match:
            namespaceSelector:
              matchNames:
                - default
      - metadata:
          name: pidpolicy
        spec:
          policies:
            allowHostPID: false
          match:
            namespaceSelector:
              matchNames:
                - default
      - metadata:
          name: syspolicy
        spec:
          policies:
            allowPrivileged: false
          match:
            namespaceSelector:
              matchNames:
                - d8-monitoring
                - kube-*
      - metadata:
          name: excludedpolicy
        spec:
          policies:
            allowPrivileged: false
          match:
            namespaceSelector:
              excludeNames:
                - d8-*
                - kube-*
      - metadata:
          name: globpolicy
        spec:
          policies:
            allowPrivileged: false
          match:
            namespaceSelector:
              matchNames:
                - "*-system"
      - metadata:
          name: exprpolicy
        spec:
          policies:
            allowPrivileged: false
            allowedHostPorts:
              - min: 8080
                max: 8080
          match:
            namespaceSelector:
              labelSelector:
                matchExpressions:
                  - key: env
                    operator: In
                    values: ["prod"]
    trackedConstraintResources:
      - apiGroups:
          - ""
        resources:
          - pods
      - apiGroups:
          - extensions
          - networking.k8s.io
        resources:
          - ingresses
    trackedMutateResources:
      - apiGroups:
          - ""
        resources:
          - pods
      - apiGroups:
          - extensions
          - networking.k8s.io
        resources:
          - ingresses
    webhook:
      ca: YjY0ZW5jX3N0cmluZwo=
      crt: YjY0ZW5jX3N0cmluZwo=
      key: YjY0ZW5jX3N0cmluZwo=
`)

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

		It("Security policy constraints must use values for enforcementAction, match and parameters", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			expectedSelector := constraintSelectorExpectation{
				namespaces: mustParseYaml("- default"),
				excludedNamespaces: mustParseYaml("- kube-system"),
				namespaceSelector: mustParseYaml("matchLabels:\n  security-policy.deckhouse.io/enabled: \"true\""),
				labelSelector: mustParseYaml("matchLabels:\n  security-policy.deckhouse.io/enabled: \"true\""),
			}
			expectedAction := "warn"

			expectedParameters := map[string]interface{}{
				"D8AllowedCapabilities": mustParseYaml("requiredDropCapabilities:\n  - ALL"),
				"D8AllowedFlexVolumes": mustParseYaml("allowedFlexVolumes:\n  - driver: vmware"),
				"D8AllowedHostPaths": mustParseYaml("allowedHostPaths:\n  - pathPrefix: /dev\n    readOnly: true"),
				"D8AllowedProcMount": mustParseYaml("allowedProcMount: default"),
				"D8AllowedSeccompProfiles": mustParseYaml("allowedProfiles:\n  - RuntimeDefault\n  - Localhost\nallowedLocalhostFiles:\n  - \"*\""),
				"D8AllowedSysctls": mustParseYaml("allowedSysctls:\n  - \"*\"\nforbiddenSysctls:\n  - user/example"),
				"D8AllowedUsers": mustParseYaml("runAsUser:\n  ranges:\n    - min: 300\n      max: 500\n  rule: MustRunAs\nsupplementalGroups:\n  ranges:\n    - min: 500\n      max: 1000\n  rule: MustRunAs"),
				"D8AllowedVolumeTypes": mustParseYaml("volumes:\n  - csi"),
				"D8HostNetwork": mustParseYaml("allowHostNetwork: false\nranges:\n  - min: 10\n    max: 100"),
				"D8HostProcesses": mustParseYaml("allowHostPID: false\nallowHostIPC: true"),
				"D8AllowedClusterRoles": mustParseYaml("allowedClusterRoles:\n  - \"*\""),
				"D8SeLinux": mustParseYaml("allowedSELinuxOptions:\n  - role: role\n    user: user\n  - level: level\n    type: type"),
				"D8AppArmor": mustParseYaml("allowedProfiles:\n  - unconfined"),
				"D8VerifyImageSignatures": mustParseYaml("references:\n  - \"^.*$\""),
			}

			constraintsWithoutParameters := []string{
				"D8AllowPrivilegeEscalation",
				"D8PrivilegedContainer",
				"D8ReadOnlyRootFilesystem",
				"D8AutomountServiceAccountTokenPod",
				"D8AllowRbacWildcards",
			}

			for constraintKind, expected := range expectedParameters {
				constraint := f.KubernetesGlobalResource(constraintKind, testPolicyName)
				Expect(constraint.Exists()).To(BeTrue())
				spec := getConstraintSpecMap(constraint)
				expectConstraintAction(spec, expectedAction)
				expectConstraintSelector(spec, expectedSelector)
				expectConstraintParameters(spec, expected)
			}

			for _, constraintKind := range constraintsWithoutParameters {
				constraint := f.KubernetesGlobalResource(constraintKind, testPolicyName)
				Expect(constraint.Exists()).To(BeTrue())
				spec := getConstraintSpecMap(constraint)
				expectConstraintAction(spec, expectedAction)
				expectConstraintSelector(spec, expectedSelector)
				expectConstraintParameters(spec, nil)
			}
		})

		It("A policy that only warns is not split by namespace scope", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// genpolicy sets enforcementAction: Warn, which is already harmless in system
			// namespaces, so it keeps a single constraint with an unscoped selector.
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-warn-"+testPolicyName).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-enforce-"+testPolicyName).Exists()).To(BeFalse())

			spec := getConstraintSpecMap(f.KubernetesGlobalResource("D8PrivilegedContainer", testPolicyName))
			match := spec["match"].(map[string]interface{})
			Expect(match["namespaceSelector"]).To(Equal(mustParseYaml("matchLabels:\n  security-policy.deckhouse.io/enabled: \"true\"")))
		})

		It("A denying policy that spans both scopes is split into three constraints", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// exprpolicy omits enforcementAction, so it denies, and it names no namespace,
			// which means it reaches system namespaces and has to be split.
			userScoped := f.KubernetesGlobalResource("D8PrivilegedContainer", "exprpolicy")
			systemWarn := f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-warn-exprpolicy")
			systemEnforce := f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-enforce-exprpolicy")

			Expect(userScoped.Exists()).To(BeTrue())
			Expect(systemWarn.Exists()).To(BeTrue())
			Expect(systemEnforce.Exists()).To(BeTrue())

			Expect(userScoped.Field("spec.enforcementAction").String()).To(Equal("deny"))
			Expect(systemWarn.Field("spec.enforcementAction").String()).To(Equal("warn"))
			Expect(systemEnforce.Field("spec.enforcementAction").String()).To(Equal("deny"))

			// The user-scoped constraint keeps every namespace but the system ones.
			Expect(userScoped.Field("spec.match.namespaces").Exists()).To(BeFalse())
			Expect(userScoped.Field("spec.match.excludedNamespaces").String()).To(MatchJSON(`["d8-*","kube-*"]`))

			Expect(systemWarn.Field("spec.match.namespaces").String()).To(MatchJSON(`["d8-*","kube-*"]`))
			Expect(systemEnforce.Field("spec.match.namespaces").String()).To(MatchJSON(`["d8-*","kube-*"]`))

			// Only the opt-in label separates the two system constraints.
			Expect(systemWarn.Field("spec.match.namespaceSelector.matchExpressions").String()).To(MatchJSON(
				`[{"key":"env","operator":"In","values":["prod"]},
				  {"key":"security.deckhouse.io/enable-security-policy-check","operator":"NotIn","values":["true"]}]`))
			Expect(systemEnforce.Field("spec.match.namespaceSelector.matchExpressions").String()).To(MatchJSON(
				`[{"key":"env","operator":"In","values":["prod"]},
				  {"key":"security.deckhouse.io/enable-security-policy-check","operator":"In","values":["true"]}]`))

			// The internal scope keys must never reach the rendered manifest.
			Expect(userScoped.Field("spec.match.d8NamespaceScope").Exists()).To(BeFalse())
			Expect(systemWarn.Field("spec.match.d8SystemNamespaces").Exists()).To(BeFalse())
		})

		It("A denying policy that names no system namespace stays a single constraint", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// minpolicy denies but is limited to the `default` namespace, so the system variants
			// would match nothing and are not rendered at all.
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-warn-minpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-enforce-minpolicy").Exists()).To(BeFalse())

			userScoped := f.KubernetesGlobalResource("D8PrivilegedContainer", "minpolicy")
			Expect(userScoped.Exists()).To(BeTrue())
			Expect(userScoped.Field("spec.match.namespaces").String()).To(MatchJSON(`["default"]`))
			Expect(userScoped.Field("spec.match.excludedNamespaces").Exists()).To(BeFalse())
		})

		It("A denying policy limited to system namespaces keeps its name on the enforcing constraint", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// syspolicy names only system namespaces, so no user-scoped constraint is needed and
			// the enforcing one keeps the name the policy author wrote.
			systemEnforce := f.KubernetesGlobalResource("D8PrivilegedContainer", "syspolicy")
			systemWarn := f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-warn-syspolicy")

			Expect(systemEnforce.Exists()).To(BeTrue())
			Expect(systemWarn.Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-enforce-syspolicy").Exists()).To(BeFalse())

			Expect(systemEnforce.Field("spec.enforcementAction").String()).To(Equal("deny"))
			Expect(systemWarn.Field("spec.enforcementAction").String()).To(Equal("warn"))

			// The policy named d8-monitoring and the glob kube-*, and both survive the intersection.
			Expect(systemEnforce.Field("spec.match.namespaces").String()).To(MatchJSON(`["d8-monitoring","kube-*"]`))
			Expect(systemWarn.Field("spec.match.namespaces").String()).To(MatchJSON(`["d8-monitoring","kube-*"]`))
		})

		It("A denying policy that excludes every system namespace stays a single constraint", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// excludedpolicy already excludes d8-* and kube-*, so the system variants would be
			// shadowed by their own excludedNamespaces and cost an audit pass for nothing.
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-warn-excludedpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-enforce-excludedpolicy").Exists()).To(BeFalse())

			constraint := f.KubernetesGlobalResource("D8PrivilegedContainer", "excludedpolicy")
			Expect(constraint.Exists()).To(BeTrue())
			Expect(constraint.Field("spec.match.namespaces").Exists()).To(BeFalse())
			Expect(constraint.Field("spec.match.excludedNamespaces").String()).To(MatchJSON(`["d8-*","kube-*"]`))
		})

		It("A denying policy whose namespace list cannot be intersected is left alone", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// A suffix glob intersects `d8-*` in a set no Gatekeeper glob describes, so the policy
			// keeps the single constraint it had rather than matching wider or narrower than asked.
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-warn-globpolicy").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("D8PrivilegedContainer", "d8-system-enforce-globpolicy").Exists()).To(BeFalse())

			constraint := f.KubernetesGlobalResource("D8PrivilegedContainer", "globpolicy")
			Expect(constraint.Exists()).To(BeTrue())
			Expect(constraint.Field("spec.match.namespaces").String()).To(MatchJSON(`["*-system"]`))
			Expect(constraint.Field("spec.match.excludedNamespaces").Exists()).To(BeFalse())
		})

		It("Every variant keeps the parameters of the policy it came from", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, name := range []string{"exprpolicy", "d8-system-warn-exprpolicy", "d8-system-enforce-exprpolicy"} {
				constraint := f.KubernetesGlobalResource("D8HostNetwork", name)
				Expect(constraint.Exists()).To(BeTrue(), name)
				Expect(constraint.Field("spec.parameters.allowHostNetwork").Bool()).To(BeTrue(), name)
				Expect(constraint.Field("spec.parameters.ranges").String()).To(MatchJSON(`[{"min":8080,"max":8080}]`), name)
			}
		})
	})
})
