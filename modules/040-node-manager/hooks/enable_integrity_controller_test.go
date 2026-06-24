/*
Copyright 2026 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: enable_integrity_controller ::", func() {
	const (
		containerdIntegrityPolicy = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ContainerdIntegrityPolicy
metadata:
  name: test-policy
spec:
  ca: |
    -----BEGIN CERTIFICATE-----
    MIIB
    -----END CERTIFICATE-----
  protectedNamespaces:
    matchLabels:
      integrity: enabled
`
		secondContainerdIntegrityPolicy = `
---
apiVersion: deckhouse.io/v1alpha1
kind: ContainerdIntegrityPolicy
metadata:
  name: another-policy
spec:
  ca: |
    -----BEGIN CERTIFICATE-----
    MIIC
    -----END CERTIFICATE-----
  protectedNamespaces:
    matchLabels:
      integrity: enabled
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "ContainerdIntegrityPolicy", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail; flag must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.containerdIntegrityControllerEnabled").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with ContainerdIntegrityPolicy", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(containerdIntegrityPolicy, 1))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.containerdIntegrityControllerEnabled").String()).To(Equal("true"))
		})
	})

	Context("Cluster with multiple ContainerdIntegrityPolicies", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(containerdIntegrityPolicy+secondContainerdIntegrityPolicy, 2))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.containerdIntegrityControllerEnabled").String()).To(Equal("true"))
		})
	})

	Context("Cluster with ContainerdIntegrityPolicy removed", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.internal.containerdIntegrityControllerEnabled", true)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail; flag must be removed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.containerdIntegrityControllerEnabled").Exists()).To(BeFalse())
		})
	})
})
