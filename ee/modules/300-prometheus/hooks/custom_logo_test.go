/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const prometheusValues = `
https:
  mode: CustomCertificate
internal:
  customCertificateData:
    tls.crt: |
      -----BEGIN CERTIFICATE-----
      TEST
      -----END CERTIFICATE-----
    tls.key: |
      -----BEGIN PRIVATE KEY-----
      TEST
      -----END PRIVATE KEY-----
`

var _ = Describe("Global hooks :: set custom logo for grafana", func() {
	f := HookExecutionConfigInit(`{"global": {"clusterIsBootstrapped": true}, "prometheus": {"internal": {"grafana": {"customLogo": {}}}}}`, `{}`)
	Context("ConfigMap with logo in d8-system does not exist", func() {
		f.ValuesSetFromYaml("prometheus", []byte(prometheusValues))
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should run and set customLogo value to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.customLogo.enabled").Bool()).To(BeFalse())
		})
	})

	Context("ConfigMap with logo in d8-system exists", func() {
		f.ValuesSetFromYaml("prometheus", []byte(prometheusValues))
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: whitelabel-custom-logo
  namespace: d8-system
data:
  grafanaLogo: svg
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should run and set customLogo value to true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.customLogo.enabled").Bool()).To(BeTrue())
			cm := f.KubernetesResource("ConfigMap", ns, cmName)
			Expect(cm.Exists()).To(BeTrue())
			Expect(cm.Field("data.grafanaLogo").String()).ToNot(BeEmpty())
		})

		Context("ConfigMap was deleted", func() {
			f.ValuesSetFromYaml("prometheus", []byte(prometheusValues))
			BeforeEach(func() {
				f.KubeStateSet(``)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("Hook should run and set customLogo value to false. CM must be deleted", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("prometheus.internal.grafana.customLogo.enabled").Bool()).To(BeFalse())
				Expect(f.KubernetesResource("ConfigMap", ns, cmName).Exists()).To(BeFalse())
			})
		})
	})

	Context("ConfigMap with logo in d8-system exists but does not have grafanaLogo", func() {
		f.ValuesSetFromYaml("prometheus", []byte(prometheusValues))
		BeforeEach(func() {
			f.KubeStateSet(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: whitelabel-custom-logo
  namespace: d8-system
data:
  dexLogo: svg
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should run and set customLogo enabled value to false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("prometheus.internal.grafana.customLogo.enabled").Bool()).To(BeFalse())
		})
	})
})
