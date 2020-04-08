package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controler-plane-manager :: hooks :: get_pki_checksum ::", func() {
	const (
		stateSecret = `
---
apiVersion: v1
data:
  ca.crt: test
  ca.key: test
  front-proxy-ca.crt: test
  front-proxy-ca.key: test
  sa.pub: test
  sa.key: test
  etcd-ca.crt: test
  etcd-ca.key: test
kind: Secret
metadata:
  name: d8-pki
  namespace: kube-system
`
		stateSecretModified = `
---
apiVersion: v1
data:
  ca.crt: testtest
  ca.key: testtest
  front-proxy-ca.crt: testtest
  front-proxy-ca.key: testtest
  sa.pub: testtest
  sa.key: testtest
  etcd-ca.crt: testtest
  etcd-ca.key: testtest
kind: Secret
metadata:
  name: d8-pki
  namespace: kube-system
`
	)

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must fail", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: There is no Secret named "d8-pki" in NS "kube-system"`))
		})

		Context("Someone added d8-cloud-instance-manager-cloud-provider", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecret))
				f.RunHook()
			})

			It("controlPlaneManager.internal.pkiChecksum must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.pkiChecksum").String()).To(Equal("f9b971fe0fe86b72105a2d3bb17d25323f7a1cf97baee8dcf1f58bddd4927412"))
			})
		})
	})

	Context("Secret d8-cloud-instance-manager-cloud-provider is in cluster", func() {
		f := HookExecutionConfigInit(`{"controlPlaneManager":{"internal": {}}}`, `{}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateSecret))
			f.RunHook()
		})

		It("controlPlaneManager.internal.pkiChecksum must be filled with data from secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.pkiChecksum").String()).To(Equal("f9b971fe0fe86b72105a2d3bb17d25323f7a1cf97baee8dcf1f58bddd4927412"))
		})

		Context("Secret d8-pki was modified", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateSecretModified))
				f.RunHook()
			})

			It("controlPlaneManager.internal.pkiChecksum must be filled with data from secret", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("controlPlaneManager.internal.pkiChecksum").String()).To(Equal("f4e2e610e599a236af6d76a673249469987e353309c36bf0469abae132ee51e1"))
			})
		})

		Context("Secret d8-pki was deleted", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(""))
				f.RunHook()
			})

			It("controlPlaneManager.internal.pkiChecksum must be filled with data from secret", func() {
				Expect(f).ToNot(ExecuteSuccessfully())
				Expect(f.Session.Err).Should(gbytes.Say(`ERROR: There is no Secret named "d8-pki" in NS "kube-system"`))
			})
		})
	})
})
