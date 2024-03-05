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

package hooks

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: admission-policy-engine :: hooks :: handle security policies", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"internal": {"bootstrapped": true} } }`,
		`{"admissionPolicyEngine":{}}`,
	)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "SecurityPolicy", false)

	Context("AutomountServiceAccountToken and NS annotation are not set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(testNamespaceWithoutAnnotation + testShortSecurityPolicy))
			f.RunHook()
		})

		It("should have AutomountServiceAccountToken: true and NS annotation", func() {
			Expect(f).To(ExecuteSuccessfully())

			securityPolicy := f.KubernetesResource("SecurityPolicy", "", "foo")
			Expect(securityPolicy.Field(`spec.policies.automountServiceAccountToken`).Bool()).To(Equal(true))

			ns := f.KubernetesResource("Namespace", "", "d8-admission-policy-engine")
			Expect(ns.Field(fmt.Sprintf(`metadata.annotations.%s`, annotationName)).Exists()).To(BeTrue())
		})
	})

	Context("AutomountServiceAccountToken and NS annotation are set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(testNamespaceWithAnnotation + testShortSecurityPolicyWithServiceAccountToken))
			f.RunHook()
		})

		It("AutomountServiceAccountToken should not be owerwrited", func() {
			Expect(f).To(ExecuteSuccessfully())

			securityPolicy := f.KubernetesResource("SecurityPolicy", "", "foo")
			Expect(securityPolicy.Field(`spec.policies.automountServiceAccountToken`).Bool()).To(Equal(true))

			ns := f.KubernetesResource("Namespace", "", "d8-admission-policy-engine")
			Expect(ns.Field(fmt.Sprintf(`metadata.annotations.%s`, annotationName)).Exists()).To(BeTrue())
		})
	})
})

var testNamespaceWithoutAnnotation = `
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    meta.helm.sh/release-name: admission-policy-engine
    meta.helm.sh/release-namespace: d8-system
  name: d8-admission-policy-engine
`

var testNamespaceWithAnnotation = `
---
apiVersion: v1
kind: Namespace
metadata:
  annotations:
    migrationAutomountServiceAccountToken: applied
    meta.helm.sh/release-name: admission-policy-engine
    meta.helm.sh/release-namespace: d8-system
  name: d8-admission-policy-engine
`

const testShortSecurityPolicyWithServiceAccountToken = `
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
    automountServiceAccountToken: false
status:
  deckhouse:
    observed:
      checkSum: "123123123123123"
      lastTimestamp: "2023-03-03T16:49:52Z"
    synced: "False"
`
