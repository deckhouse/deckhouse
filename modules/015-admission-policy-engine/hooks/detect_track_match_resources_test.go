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

var _ = Describe("Modules :: admission-policy-engine :: hooks :: detect_track_match_resources", func() {
	f := HookExecutionConfigInit(
		`{"admissionPolicyEngine": {"internal": {"bootstrapped": true} } }`,
		`{"admissionPolicyEngine":{}}`,
	)
	Context("CM exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cm))
			f.RunHook()
		})
		It("should have generated resources", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedConstraintResources").Array()).NotTo(BeEmpty())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedMutateResources").Array()).NotTo(BeEmpty())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedConstraintResources").String()).To(MatchJSON(`[{"apiGroups":[""],"resources":["pods"]}, {"apiGroups":["extensions","networking.k8s.io"],"resources":["ingresses"]}]`))
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedMutateResources").String()).To(MatchJSON(`[{"apiGroups":["apps"],"resources":["deployments"]}]`))
		})
	})

	Context("Empty CM", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(emptyCM))
			f.RunHook()
		})
		It("should have empty array", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedConstraintResources").Array()).To(BeEmpty())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedMutateResources").Array()).To(BeEmpty())
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedConstraintResources").String()).To(MatchJSON(`[]`))
			Expect(f.ValuesGet("admissionPolicyEngine.internal.trackedMutateResources").String()).To(MatchJSON(`[]`))
		})
	})

})

var cm = `
apiVersion: v1
data:
  validate-resources.yaml: |
    - apiGroups:
      - ""
      resources:
      - pods
    - apiGroups:
      - extensions
      - networking.k8s.io
      resources:
      - ingresses
  mutate-resources.yaml: |
    - apiGroups:
      - apps
      resources:
      - deployments
kind: ConfigMap
metadata:
  annotations:
    security.deckhouse.io/constraints-checksum: b73817c9948c3f7d823859980f0d8b1216f1f00222570bde38c9bd54b50cea87
  labels:
    owner: constraint-exporter
  name: constraint-exporter
  namespace: d8-admission-policy-engine
`

var emptyCM = `
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    owner: constraint-exporter
  name: constraint-exporter
  namespace: d8-admission-policy-engine
`
