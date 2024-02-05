/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
*/

package pricing

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Flant integration :: hooks :: envs_from_provider_cluster_configuration_secret ", func() {
	f := HookExecutionConfigInit(`{"flantIntegration":{"internal":{}}}`, `{}`)

	Context("Without d8-provider-cluster-configuration secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(``, 0))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantIntegration.internal.cloudLayout").Exists()).To(BeFalse())
		})
	})

	Context("With d8-provider-cluster-configuration secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
data:
  cloud-provider-cluster-configuration.yaml: YXBpVmVyc2lvbjogZGVja2hvdXNlLmlvL3YxYWxwaGExCmtpbmQ6IE9wZW5TdGFja0NsdXN0ZXJDb25maWd1cmF0aW9uCmxheW91dDogU3RhbmRhcmQK
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
type: Opaque
`, 0))
			f.RunHook()
		})

		It("Should fill flantIntegration internal", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantIntegration.internal.cloudLayout").String()).To(Equal(`Standard`))
		})
	})

	Context("With bad d8-provider-cluster-configuration secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
apiVersion: v1
data:
  foo: YmFy
kind: Secret
metadata:
  name: d8-provider-cluster-configuration
  namespace: kube-system
type: Opaque
`, 0))
			f.RunHook()
		})

		It("Should not fill flantIntegration internal", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("flantIntegration.internal.cloudLayout").Exists()).To(BeFalse())
		})
	})
})
