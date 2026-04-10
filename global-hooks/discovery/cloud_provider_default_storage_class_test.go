/*
Copyright 2025 Flant JSC

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

var _ = Describe("Global hooks :: discovery :: cloud_provider_default_storage_class ::", func() {
	f := HookExecutionConfigInit(`{"global":{"discovery":{}},"cloudProviderDvp":{"internal":{}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("global.discovery.cloudProviderDefaultStorageClass must not be set", func() {
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").Exists()).To(BeFalse())
		})
	})

	Context("Secret has default storage class", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  discovery-data.json: eyJzdG9yYWdlQ2xhc3NlcyI6W3sibmFtZSI6InJlcGxpY2F0ZWQiLCJpc0RlZmF1bHQiOnRydWV9XX0=
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("global.discovery.cloudProviderDefaultStorageClass must be set to 'replicated'", func() {
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").String()).To(Equal("replicated"))
		})
	})

	Context("Secret has no default storage class", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.cloudProviderDefaultStorageClass", "old-value")
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  discovery-data.json: eyJzdG9yYWdlQ2xhc3NlcyI6W3sibmFtZSI6ImxvY2FsIiwiaXNEZWZhdWx0IjpmYWxzZX1dfQ==
`))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("global.discovery.cloudProviderDefaultStorageClass must be removed", func() {
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").Exists()).To(BeFalse())
		})
	})
})
