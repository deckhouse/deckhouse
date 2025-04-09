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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency/requirements"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: check_cloud_conditions ::", func() {
	const (
		emptyConditions = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cloud-provider-conditions
  namespace: kube-system
data: 
  conditions: |
    []
`
		unmetCloudConditions = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-cloud-provider-conditions
  namespace: kube-system
data:
  conditions: |
    [{"name":"test","message":"test", "ok": false}]
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.16.15", "kubernetesVersions":["1.16.15"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},"nodeManager":{"internal": {}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(unmetCloudConditionsKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})

	Context("emptyConditions", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(emptyConditions))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(unmetCloudConditionsKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeFalse())
		})
	})

	Context("unmetCloudConditions", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(unmetCloudConditions))
			f.RunHook()
		})

		It("unmetCloudConditions requirement value should be true", func() {
			Expect(f).To(ExecuteSuccessfully())
			value, exists := requirements.GetValue(unmetCloudConditionsKey)
			Expect(exists).To(BeTrue())
			Expect(value).To(BeTrue())
		})
	})
})
