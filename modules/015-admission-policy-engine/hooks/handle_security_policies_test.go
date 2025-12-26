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

package hooks

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	checkSum = "123123123123123"
	nowTime  = "2023-03-03T16:49:52Z"
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: handle security policies", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"internal": {"ratify": {}, "bootstrapped": true} } }`,
		`{"admissionPolicyEngine":{}}`,
	)
	f.RegisterCRD("templates.gatekeeper.sh", "v1", "ConstraintTemplate", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "SecurityPolicy", false)

	err := os.Setenv("TEST_CONDITIONS_CALC_NOW_TIME", nowTime)
	if err != nil {
		panic(err)
	}

	err = os.Setenv("TEST_CONDITIONS_CALC_CHKSUM", checkSum)
	if err != nil {
		panic(err)
	}

	Context("Security Policy is set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(testSecurityPolicy))
			f.RunHook()
		})
		It("should have generated resources", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.securityPolicies").Array()).To(HaveLen(2))
			Expect(f.ValuesGet("admissionPolicyEngine.internal.securityPolicies").Array()[0].Get("spec").String()).To(MatchJSON(`
			{
				"enforcementAction": "Deny",
				"match": {
					"namespaceSelector": {
						"labelSelector": {
							"matchLabels": {
								"security-policy.deckhouse.io/enabled": "true"
							}
						}
					},
					"labelSelector": {}
				},
				"policies": {
					"allowRbacWildcards": true,
					"automountServiceAccountToken": true,
 					"seccompProfiles": {},
					"verifyImageSignatures": [
						{
							"dockerCfg": "zxc=",
							"reference": "ghcr.io/*",
							"publicKeys": ["someKey1", "someKey2"]
						},
						{
							"reference": "*",
							"publicKeys": ["someKey3"],
							"ca": "someCA1"
						}
					]
				}
			}`))

			Expect(f.ValuesGet("admissionPolicyEngine.internal.securityPolicies").Array()[1].Get("spec").String()).To(MatchJSON(`
			{
				"enforcementAction": "Deny",
				"match": {
					"namespaceSelector": {
						"labelSelector": {
							"matchLabels": {
								"security-policy.deckhouse.io/enabled": "true"
							}
						}
					},
					"labelSelector": {}
				},
				"policies": {
					"allowedCapabilities": [],
					"allowedAppArmor": [
						"runtime/default"
					],
					"allowedFlexVolumes": [
						{
							"driver": "vmware"
						}
					],
					"allowedHostPaths": [
						{
							"pathPrefix": "/dev",
							"readOnly": true
						}
					],
					"allowedHostPorts": [
						{
							"max": 100,
							"min": 10
						}
					],
					"allowedUnsafeSysctls": [
						"user/huyser",
						"*"
					],
					"allowHostIPC": true,
					"allowHostNetwork": false,
					"allowHostPID": false,
					"allowPrivileged": false,
					"allowPrivilegeEscalation": false,
					"allowRbacWildcards": true,
					"automountServiceAccountToken": true,
					"forbiddenSysctls": [
						"user/huyser",
						"user/example"
					],
					"readOnlyRootFilesystem": true,
					"requiredDropCapabilities": [
						"ALL"
					],
					"runAsUser": {
						"ranges": [
							{
								"max": 500,
								"min": 300
							}
						],
						"rule": "MustRunAs"
					},
					"seccompProfiles": {
						"allowedLocalhostFiles": [
							"*"
						],
						"allowedProfiles": [
							"RuntimeDefault",
							"Localhost"
						]
					},
					"seLinux": [
						{
							"role": "role",
							"user": "user"
						},
						{
							"level": "level",
							"type": "type"
						}
					],
					"supplementalGroups": {
						"ranges": [
							{
								"max": 1000,
								"min": 500
							}
						],
						"rule": "MustRunAs"
					},
					"verifyImageSignatures": [
						{
							"dockerCfg": "zxc=",
							"reference": "ghcr.io/*",
							"publicKeys": ["someKey2"],
							"ca": "someCA2"
						}
					]
				}
			}`))
			Expect(f.KubernetesGlobalResource("SecurityPolicy", "foo").Field("status").String()).To(MatchJSON(`
			{
				"deckhouse": {
					"observed": {
						"checkSum": "123123123123123",
						"lastTimestamp": "2023-03-03T16:49:52Z"
					},
					"synced": "False"
				}
			}`))

			Expect(f.ValuesGet("admissionPolicyEngine.internal.ratify.imageReferences").String()).To(MatchJSON(`
			[
				{
					"publicKeys": [
						"someKey1",
						"someKey2"
					],
					"reference": "ghcr.io/*"
				},
				{
					"publicKeys": [
						"someKey3"
					],
					"reference": "*"
				}
			]
			`))
		})
	})

	Context("Preserve explicit empty arrays in Values for selected fields", func() {
		Context("Case A: allowedHostPaths is omitted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(testSecurityPolicyAllowedHostPathsOmitted))
				f.RunHook()
			})
			It("should not include allowedHostPaths key in Values", func() {
				Expect(f).To(ExecuteSuccessfully())
				p := f.ValuesGet("admissionPolicyEngine.internal.securityPolicies").Array()
				Expect(p).To(HaveLen(1))
				Expect(p[0].Get("spec.policies.allowedHostPaths").Exists()).To(BeFalse())
			})
		})

		Context("Case B: allowedHostPaths is explicitly set to []", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(testSecurityPolicyAllowedHostPathsEmpty))
				f.RunHook()
			})
			It("should include allowedHostPaths key with empty array in Values", func() {
				Expect(f).To(ExecuteSuccessfully())
				p := f.ValuesGet("admissionPolicyEngine.internal.securityPolicies").Array()
				Expect(p).To(HaveLen(1))
				Expect(p[0].Get("spec.policies.allowedHostPaths").Exists()).To(BeTrue())
				Expect(p[0].Get("spec.policies.allowedHostPaths").Array()).To(HaveLen(0))
			})
		})

		Context("Case C: allowedHostPaths is set with one item", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(testSecurityPolicyAllowedHostPathsNonEmpty))
				f.RunHook()
			})
			It("should include allowedHostPaths key with non-empty array in Values", func() {
				Expect(f).To(ExecuteSuccessfully())
				p := f.ValuesGet("admissionPolicyEngine.internal.securityPolicies").Array()
				Expect(p).To(HaveLen(1))
				Expect(p[0].Get("spec.policies.allowedHostPaths").Exists()).To(BeTrue())
				Expect(p[0].Get("spec.policies.allowedHostPaths").Array()).To(HaveLen(1))
			})
		})

		Context("Nested: seccompProfiles.allowedProfiles is explicitly set to []", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(testSecurityPolicySeccompAllowedProfilesEmpty))
				f.RunHook()
			})
			It("should include seccompProfiles.allowedProfiles key with empty array in Values", func() {
				Expect(f).To(ExecuteSuccessfully())
				p := f.ValuesGet("admissionPolicyEngine.internal.securityPolicies").Array()
				Expect(p).To(HaveLen(1))
				Expect(p[0].Get("spec.policies.seccompProfiles").Exists()).To(BeTrue())
				Expect(p[0].Get("spec.policies.seccompProfiles.allowedProfiles").Exists()).To(BeTrue())
				Expect(p[0].Get("spec.policies.seccompProfiles.allowedProfiles").Array()).To(HaveLen(0))
			})
		})
	})

	Context("Pointer slice semantics: omit vs [] vs non-empty (full coverage for policy slice fields used by constraints)", func() {
		type sliceCase struct {
			name         string
			path         string
			omitSnippet  string
			emptySnippet string
			nonEmpty     string
		}

		securityPolicyYAML := func(policiesSnippet string) string {
			return fmt.Sprintf(`
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy.deckhouse.io/enabled: "true"
  policies:
    allowPrivileged: true
%s
`, policiesSnippet)
		}

		runAndGet := func(yaml string) gjson.Result {
			f.BindingContexts.Set(f.KubeStateSet(yaml))
			f.RunHook()
			Expect(f).To(ExecuteSuccessfully())
			arr := f.ValuesGet("admissionPolicyEngine.internal.securityPolicies").Array()
			Expect(arr).To(HaveLen(1))
			return arr[0]
		}

		cases := []sliceCase{
			{
				name:         "allowedHostPaths",
				path:         "spec.policies.allowedHostPaths",
				omitSnippet:  "",
				emptySnippet: "    allowedHostPaths: []",
				nonEmpty: "    allowedHostPaths:\n" +
					"    - pathPrefix: /dev\n" +
					"      readOnly: true",
			},
			{
				name:         "allowedHostPorts",
				path:         "spec.policies.allowedHostPorts",
				omitSnippet:  "",
				emptySnippet: "    allowedHostPorts: []",
				nonEmpty: "    allowedHostPorts:\n" +
					"    - min: 10\n" +
					"      max: 11",
			},
			{
				name:         "allowedVolumes",
				path:         "spec.policies.allowedVolumes",
				omitSnippet:  "",
				emptySnippet: "    allowedVolumes: []",
				nonEmpty: "    allowedVolumes:\n" +
					"    - configMap",
			},
			{
				name:         "allowedFlexVolumes",
				path:         "spec.policies.allowedFlexVolumes",
				omitSnippet:  "",
				emptySnippet: "    allowedFlexVolumes: []",
				nonEmpty: "    allowedFlexVolumes:\n" +
					"    - driver: vmware",
			},
			{
				name:         "allowedClusterRoles",
				path:         "spec.policies.allowedClusterRoles",
				omitSnippet:  "",
				emptySnippet: "    allowedClusterRoles: []",
				nonEmpty: "    allowedClusterRoles:\n" +
					"    - view",
			},
			{
				name:         "allowedCapabilities",
				path:         "spec.policies.allowedCapabilities",
				omitSnippet:  "",
				emptySnippet: "    allowedCapabilities: []",
				nonEmpty: "    allowedCapabilities:\n" +
					"    - NET_ADMIN",
			},
			{
				name:         "requiredDropCapabilities",
				path:         "spec.policies.requiredDropCapabilities",
				omitSnippet:  "",
				emptySnippet: "    requiredDropCapabilities: []",
				nonEmpty: "    requiredDropCapabilities:\n" +
					"    - ALL",
			},
			{
				name:         "allowedAppArmor",
				path:         "spec.policies.allowedAppArmor",
				omitSnippet:  "",
				emptySnippet: "    allowedAppArmor: []",
				nonEmpty: "    allowedAppArmor:\n" +
					"    - runtime/default",
			},
			{
				name:         "allowedUnsafeSysctls",
				path:         "spec.policies.allowedUnsafeSysctls",
				omitSnippet:  "",
				emptySnippet: "    allowedUnsafeSysctls: []",
				nonEmpty: "    allowedUnsafeSysctls:\n" +
					"    - kernel.shm_rmid_forced",
			},
			{
				name:         "forbiddenSysctls",
				path:         "spec.policies.forbiddenSysctls",
				omitSnippet:  "",
				emptySnippet: "    forbiddenSysctls: []",
				nonEmpty: "    forbiddenSysctls:\n" +
					"    - kernel.shm_rmid_forced",
			},
			{
				name:         "seLinux",
				path:         "spec.policies.seLinux",
				omitSnippet:  "",
				emptySnippet: "    seLinux: []",
				nonEmpty: "    seLinux:\n" +
					"    - type: container_t",
			},
			{
				name:         "verifyImageSignatures",
				path:         "spec.policies.verifyImageSignatures",
				omitSnippet:  "",
				emptySnippet: "    verifyImageSignatures: []",
				nonEmpty: "    verifyImageSignatures:\n" +
					"    - reference: ghcr.io/*\n" +
					"      publicKeys: [\"k1\"]",
			},
			{
				name:         "allowedServiceTypes",
				path:         "spec.policies.allowedServiceTypes",
				omitSnippet:  "",
				emptySnippet: "    allowedServiceTypes: []",
				nonEmpty: "    allowedServiceTypes:\n" +
					"    - ClusterIP",
			},
			{
				name:         "seccompProfiles.allowedProfiles",
				path:         "spec.policies.seccompProfiles.allowedProfiles",
				omitSnippet:  "",
				emptySnippet: "    seccompProfiles:\n      allowedProfiles: []",
				nonEmpty: "    seccompProfiles:\n" +
					"      allowedProfiles: [\"RuntimeDefault\"]",
			},
			{
				name:         "seccompProfiles.allowedLocalhostFiles",
				path:         "spec.policies.seccompProfiles.allowedLocalhostFiles",
				omitSnippet:  "",
				emptySnippet: "    seccompProfiles:\n      allowedLocalhostFiles: []",
				nonEmpty: "    seccompProfiles:\n" +
					"      allowedProfiles: [\"Localhost\"]\n" +
					"      allowedLocalhostFiles: [\"*\"]",
			},
		}

		It("should preserve slice tri-state semantics in Values", func() {
			for _, tc := range cases {
				By("omit: " + tc.name)
				o := runAndGet(securityPolicyYAML(tc.omitSnippet))
				Expect(o.Get(tc.path).Exists()).To(BeFalse())

				By("empty: " + tc.name)
				e := runAndGet(securityPolicyYAML(tc.emptySnippet))
				Expect(e.Get(tc.path).Exists()).To(BeTrue())
				Expect(e.Get(tc.path).Array()).To(HaveLen(0))

				By("non-empty: " + tc.name)
				n := runAndGet(securityPolicyYAML(tc.nonEmpty))
				Expect(n.Get(tc.path).Exists()).To(BeTrue())
				Expect(n.Get(tc.path).Array()).ToNot(BeEmpty())
			}
		})

		It("should keep preprocess optimizations (nil out specific fields)", func() {
			// allowedVolumes: ['*'] => should be dropped by preprocess
			o1 := runAndGet(securityPolicyYAML("    allowedVolumes: ['*']"))
			Expect(o1.Get("spec.policies.allowedVolumes").Exists()).To(BeFalse())

			// allowedClusterRoles: ['*'] => should be dropped by preprocess
			o2 := runAndGet(securityPolicyYAML("    allowedClusterRoles: ['*']"))
			Expect(o2.Get("spec.policies.allowedClusterRoles").Exists()).To(BeFalse())

			// allowedCapabilities: ['ALL'] with requiredDropCapabilities omitted => drop allowedCapabilities
			o3 := runAndGet(securityPolicyYAML("    allowedCapabilities: ['ALL']"))
			Expect(o3.Get("spec.policies.allowedCapabilities").Exists()).To(BeFalse())

			// allowedUnsafeSysctls: ['*'] with forbiddenSysctls omitted => drop allowedUnsafeSysctls
			o4 := runAndGet(securityPolicyYAML("    allowedUnsafeSysctls: ['*']"))
			Expect(o4.Get("spec.policies.allowedUnsafeSysctls").Exists()).To(BeFalse())

			// seccompProfiles allowedProfiles='*' and allowedLocalhostFiles='*' => drop both (and likely the whole seccompProfiles)
			o5 := runAndGet(securityPolicyYAML("    seccompProfiles:\n      allowedProfiles: ['*']\n      allowedLocalhostFiles: ['*']"))
			Expect(o5.Get("spec.policies.seccompProfiles.allowedProfiles").Exists()).To(BeFalse())
			Expect(o5.Get("spec.policies.seccompProfiles.allowedLocalhostFiles").Exists()).To(BeFalse())
		})
	})
})

var testSecurityPolicy = `
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: bar
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy.deckhouse.io/enabled: "true"
  policies:
    verifyImageSignatures:
    - dockerCfg: zxc=
      reference: ghcr.io/*
      publicKeys:
      - someKey1
      - someKey2
    - reference: "*"
      publicKeys:
      - someKey3
      ca: someCA1
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy.deckhouse.io/enabled: "true"
  policies:
    allowHostNetwork: false
    allowPrivilegeEscalation: false
    allowPrivileged: false
    allowRbacWildcards: true
    allowedCapabilities: []
    allowedAppArmor:
    - runtime/default
    allowedFlexVolumes:
    - driver: vmware
    allowedProcMount: Unmasked
    allowedUnsafeSysctls:
    - user/huyser
    - "*"
    allowedVolumes:
    - '*'
    forbiddenSysctls:
    - user/huyser
    - user/example
    fsGroup:
      rule: RunAsAny
    readOnlyRootFilesystem: true
    allowedClusterRoles: ["*"]
    runAsGroup:
      ranges:
      - max: 500
        min: 300
      rule: RunAsAny
    runAsUser:
      ranges:
      - max: 500
        min: 300
      rule: MustRunAs
    supplementalGroups:
      ranges:
      - max: 1000
        min: 500
      rule: MustRunAs
    seLinux:
    - role: role
      user: user
    - level: level
      type: type
    allowHostIPC: true
    allowHostPID: false
    allowedHostPaths:
    - pathPrefix: /dev
      readOnly: true
    allowedHostPorts:
    - min: 10
      max: 100
    requiredDropCapabilities:
    - ALL
    seccompProfiles:
      allowedProfiles:
      - RuntimeDefault
      - Localhost
      allowedLocalhostFiles:
      - '*'
    verifyImageSignatures:
    - dockerCfg: zxc=
      reference: ghcr.io/*
      publicKeys:
      - someKey2
      ca: someCA2
`

var testSecurityPolicyAllowedHostPathsOmitted = `
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy.deckhouse.io/enabled: "true"
  policies:
    allowPrivileged: false
`

var testSecurityPolicyAllowedHostPathsEmpty = `
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy.deckhouse.io/enabled: "true"
  policies:
    allowPrivileged: false
    allowedHostPaths: []
`

var testSecurityPolicyAllowedHostPathsNonEmpty = `
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy.deckhouse.io/enabled: "true"
  policies:
    allowPrivileged: false
    allowedHostPaths:
    - pathPrefix: /dev
      readOnly: true
`

var testSecurityPolicySeccompAllowedProfilesEmpty = `
---
apiVersion: deckhouse.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: foo
spec:
  enforcementAction: Deny
  match:
    namespaceSelector:
      labelSelector:
        matchLabels:
          security-policy.deckhouse.io/enabled: "true"
  policies:
    allowPrivileged: false
    seccompProfiles:
      allowedProfiles: []
`
