/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
