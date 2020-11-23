package hooks

import (
	"github.com/onsi/gomega/gbytes"
	"testing"

	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Modules :: common :: hooks :: copy_custom_certificate ::", func() {
	const (
		stateNamespaces = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
`
		stateSecrets = `
---
apiVersion: v1
data:
  tls.crt: CRTCRTCRT
  tls.key: KEYKEYKEY
kind: Secret
metadata:
  name: d8-tls-cert
  namespace: d8-system
type: kubernetes.io/tls
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Namespace and secret are in cluster, https mode set to Disabled", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaces + stateSecrets))
			f.ValuesSet("common.https.mode", "Disabled")
			f.RunHook()
		})

		It("Module value internal.customCertificateData must be unset", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.customCertificateData").Exists()).To(BeFalse())
		})

	})

	Context("Namespace and secret are in cluster, https mode set to customCertificate, but certificate name is wrong", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaces + stateSecrets))
			f.ValuesSet("common.https.mode", "CustomCertificate")
			f.ValuesSet("common.https.customCertificate.secretName", "blablabla")
			f.RunHook()
		})

		It("Hook must fail with error message", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.customCertificateData").Exists()).To(BeFalse())
			Expect(f.Session.Err).Should(gbytes.Say(`ERROR: custom certificate secret name is configured, but secret with this name doesn't exist.`))
		})

	})

	Context("Namespace and secret are in cluster, https mode set to customCertificate, certificate name is set correctly", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaces + stateSecrets))
			f.ValuesSet("common.https.mode", "CustomCertificate")
			f.ValuesSet("common.https.customCertificate.secretName", "d8-tls-cert")
			f.RunHook()
		})

		It("Hook must successfully save certificate data", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("common.internal.customCertificateData").Exists()).To(BeTrue())
			Expect(f.ValuesGet("common.internal.customCertificateData").String()).To(MatchYAML(`
tls.crt: CRTCRTCRT
tls.key: KEYKEYKEY
`))

		})

	})

})
