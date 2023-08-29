package hooks

import (
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Modules :: CSE :: hooks :: get_dechouse_webhook_cert ::", func() {
	state := `
apiVersion: v1
data:
  ca.crt: Zm9vCg==
  tls.crt: Zm9vCg==
  tls.key: Zm9vCg==
kind: Secret
metadata:
  annotations:
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2023-05-29T16:28:23Z"
  labels:
    app: webhook-handler
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: deckhouse
  name: webhook-handler-certs
  namespace: d8-system
type: kubernetes.io/tls
`
	f := HookExecutionConfigInit(`{"deckhouse":{"internal":{"admissionWebhookCert":{"ca":""}}}}`, `{}`)
	Context("Cluster initialization", func() {
		BeforeEach(func() {
			f.KubeStateSet(state)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("webhook-handler-certs ca.crt exist in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			webhookHandlerCerts := f.KubernetesResource("Secret", "d8-system", "webhook-handler-certs")
			Expect(webhookHandlerCerts.Exists()).To(BeTrue())
			data, err := base64.StdEncoding.DecodeString(webhookHandlerCerts.Field("data.ca\\.crt").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(data)).Should(ContainSubstring("foo"))
			Expect(f.ValuesGet("deckhouse.internal.admissionWebhookCert.ca").String()).Should(ContainSubstring("foo"))
		})
	})
})
