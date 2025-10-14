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
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

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
