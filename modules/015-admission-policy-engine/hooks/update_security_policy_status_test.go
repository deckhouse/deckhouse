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
	securityPoliciesValues = `[
		{
			"metadata": {
				"name": "foo"
			},
			"spec": {
				"enforcementAction": Deny,
				"match": {
					"namespaceSelector": {
						"labelSelector": {
							"matchLabels": {
								"operation-policy.deckhouse.io/enabled": "true"
							},
						},
					},
				},
				"policies": {
					"allowHostNetwork": false,
					"allowPrivilegeEscalation": false,
					"allowPrivileged": false
				}
			}
		}
	]`
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: update security policies statuses", func() {
	f := HookExecutionConfigInit(`{"admissionPolicyEngine": {"internal": {"bootstrapped": true}}}`,
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

	Context("Security Policy status is updated", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("admissionPolicyEngine.internal.securityPolicies", []byte(securityPoliciesValues))
			f.KubeStateSet(testShortSecurityPolicy)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})
		It("should have generated resources", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.securityPolicies").Array()).To(HaveLen(1))
			const expectedStatus = `{
				"deckhouse": {
        				"observed": {
						"checkSum": "123123123123123",
						"lastTimestamp": "2023-03-03T16:49:52Z"
					},
        				"processed": {
						"checkSum": "123123123123123",
						"lastTimestamp": "2023-03-03T16:49:52Z"
					},
					"synced": "True"
				}
			}`
			Expect(f.KubernetesGlobalResource("SecurityPolicy", "foo").Field("status").String()).To(MatchJSON(expectedStatus))
		})
	})
})

var testShortSecurityPolicy = `
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
          operation-policy.deckhouse.io/enabled: "true"
  policies:
    allowHostNetwork: false
    allowPrivilegeEscalation: false
    allowPrivileged: false
status:
  deckhouse:
    observed:
      checkSum: "123123123123123"
      lastTimestamp: "2023-03-03T16:49:52Z"
    synced: "False"
`
