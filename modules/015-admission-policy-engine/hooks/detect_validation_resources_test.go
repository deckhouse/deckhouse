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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Modules :: admission-policy-engine :: hooks :: detect_validation_resources", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"internal": {"bootstrapped": true} } }`,
		`{"admissionPolicyEngine":{}}`,
	)
	Context("CM exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cm))
			f.RunHook()
		})
		It("should generate resources", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedResources").Array()).NotTo(BeEmpty())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedResources").String()).To(MatchJSON(`[{"apiGroups":[""],"resources":["pods"]}, {"apiGroups":["extensions","networking.k8s.io"],"resources":["ingresses"]}]`))
		})
	})

})

var cm = `
apiVersion: v1
data:
  kinds.yaml: |
    - apiGroups:
      - ""
      kinds:
      - Pod
    - apiGroups:
      - extensions
      - networking.k8s.io
      kinds:
      - Ingress
kind: ConfigMap
metadata:
  annotations:
    security.deckhouse.io/constraints-checksum: b73817c9948c3f7d823859980f0d8b1216f1f00222570bde38c9bd54b50cea87
  creationTimestamp: "2022-11-11T19:00:16Z"
  labels:
    owner: constraint-exporter
  name: constraint-exporter
  namespace: d8-admission-policy-engine
  resourceVersion: "116772759"
  uid: db6044f5-9d18-449d-979d-b862598654ba
`
