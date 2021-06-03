package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controlPlaneManager :: hooks :: sync-etcd-certs ::", func() {
	f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal":{"etcdCerts": {}}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("secret exists", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(pkiState))
			f.RunHook()
		})

		It("Must fill internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.etcdCerts.ca").String()).To(BeEquivalentTo("YWFh"))
			Expect(f.ValuesGet("controlPlaneManager.internal.etcdCerts.crt").String()).To(BeEquivalentTo("YWFh"))
			Expect(f.ValuesGet("controlPlaneManager.internal.etcdCerts.key").String()).To(BeEquivalentTo("YmJi"))
		})
	})
})

var pkiState = `
apiVersion: v1
kind: Secret
metadata:
  name: d8-pki
  namespace: kube-system
data:
  etcd-ca.crt: YWFh # aaa
  etcd-ca.key: YmJi # bbb
`
