package hooks

import (
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: generate_certificates ::", func() {
	f := HookExecutionConfigInit(`{"global":{"discovery":{"clusterDomain":"cluster.flomaster"}},"istio":{"internal":{"ca":{},"apiProxyListenCertificate":{}}}}`, "")

	Context("Empty cluster; empty values", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should generate ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.root").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.chain").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.apiProxyListenCertificate.key").Exists()).To(BeFalse())
			Expect(f.ValuesGet("istio.internal.apiProxyListenCertificate.cert").Exists()).To(BeFalse())

			caCert := f.ValuesGet("istio.internal.ca.cert").String()
			caRoot := f.ValuesGet("istio.internal.ca.root").String()
			caChain := f.ValuesGet("istio.internal.ca.chain").String()

			Expect(caCert).To(Equal(caRoot))
			Expect(caCert).To(Equal(caChain))

			block, _ := pem.Decode([]byte(caCert))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeTrue())
			Expect(cert.Subject.Organization[0]).To(Equal("d8-istio"))
		})
	})

	Context("Empty cluster; multicluster is on", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.multicluster.enabled", true)
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should generate ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.root").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.ca.chain").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.apiProxyListenCertificate.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("istio.internal.apiProxyListenCertificate.cert").Exists()).To(BeTrue())

			listenCert := f.ValuesGet("istio.internal.apiProxyListenCertificate.cert").String()
			block, _ := pem.Decode([]byte(listenCert))
			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).To(BeNil())
			Expect(cert.IsCA).To(BeFalse())

			Expect(cert.Subject.CommonName).To(Equal("api-proxy"))
			Expect(cert.DNSNames).To(Equal([]string{"api-proxy", "api-proxy.d8-istio", "api-proxy.d8-istio.svc"}))
		})
	})

	Context("Secret cacerts is in cluster; values aren't set", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa
  ca-key.pem: YmJi # bbb
  cert-chain.pem: Y2Nj # ccc
  root-cert.pem: ZWVl # eee
`))
			f.RunHook()
		})
		It("Should add existing ca certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal("aaa"))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal("bbb"))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal("ccc"))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal("eee"))
		})
	})

	Context("Secret api-proxy-listen-cert is in cluster; multicluster is on", func() {
		BeforeEach(func() {
			f.ValuesSet("istio.multicluster.enabled", true)
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: api-proxy-listen-cert
  namespace: d8-istio
data:
  tls.crt: YWFh # aaa
  tls.key: YmJi # bbb
`))
			f.RunHook()
		})
		It("Should add existing api-proxy-listen-cert certificate to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.apiProxyListenCertificate.cert").String()).To(Equal("aaa"))
			Expect(f.ValuesGet("istio.internal.apiProxyListenCertificate.key").String()).To(Equal("bbb"))
		})
	})

	Context("Secret cacerts is in cluster; values are set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.ca", []byte(`
cert: xxx
key: yyy
chain: zzz
root: kkk
`))
			// this secret should be ignored
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa
  ca-key.pem: YmJi # bbb
  cert-chain.pem: Y2Nj # ccc
  root-cert.pem: ZWVl # eee
`))
			f.RunHook()
		})
		It("Should copy cert data from values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal("xxx"))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal("yyy"))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal("zzz"))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal("kkk"))
		})
	})

	Context("Secret cacerts is in cluster; values are not fully set", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("istio.ca", []byte(`
cert: xxx
key: yyy
`))
			// this secret should be ignored
			f.BindingContexts.Set(f.KubeStateSet(`
apiVersion: v1
kind: Secret
metadata:
  name: cacerts
  namespace: d8-istio
data:
  ca-cert.pem: YWFh # aaa
  ca-key.pem: YmJi # bbb
  cert-chain.pem: Y2Nj # ccc
  root-cert.pem: ZWVl # eee
`))
			f.RunHook()
		})
		It("Should copy cert data from values, root and chain should be set to cert", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("istio.internal.ca.cert").String()).To(Equal("xxx"))
			Expect(f.ValuesGet("istio.internal.ca.key").String()).To(Equal("yyy"))
			Expect(f.ValuesGet("istio.internal.ca.chain").String()).To(Equal("xxx"))
			Expect(f.ValuesGet("istio.internal.ca.root").String()).To(Equal("xxx"))
		})
	})
})
