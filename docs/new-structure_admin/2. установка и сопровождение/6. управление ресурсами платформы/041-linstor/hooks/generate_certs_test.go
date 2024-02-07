/*
Copyright 2022 Flant JSC

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
	"crypto/x509"
	"encoding/pem"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"linstor":{"internal":{"httpsControllerCert":{}, "httpsClientCert":{}, "sslControllerCert":{}, "sslNodeCert":{}}},"global":{"discovery":{"clusterDomain":"mycluster.local"}}}`
	initConfigValuesString = `{}`
)

var _ = Describe("Modules :: linstor :: hooks :: generate_certs ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("HTTPS Certs :: Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(``),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.ca").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.ca").Exists()).To(BeTrue())

			// client certificate
			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.httpsClientCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("linstor.internal.httpsClientCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// controller certificate
			certPool = x509.NewCertPool()
			ok = certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.httpsControllerCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ = pem.Decode([]byte(f.ValuesGet("linstor.internal.httpsControllerCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// Additional checks for controller certificate
			opts := x509.VerifyOptions{
				DNSName: "linstor.d8-linstor.svc",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())

			opts = x509.VerifyOptions{
				DNSName: "127.0.0.1",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())

		})
	})

	Context("HTTPS Certs :: One secret is missing", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-controller-https-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.ca").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.ca").Exists()).To(BeTrue())

			// client certificate
			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.httpsClientCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("linstor.internal.httpsClientCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// controller certificate
			certPool = x509.NewCertPool()
			ok = certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.httpsControllerCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ = pem.Decode([]byte(f.ValuesGet("linstor.internal.httpsControllerCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err = x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(f.ValuesGet("linstor.internal.httpsClientCert.ca").String()).To(Equal(f.ValuesGet("linstor.internal.httpsControllerCert.ca").String()))
		})
	})

	Context("HTTPS Certs :: Secrets are having different CA", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-controller-https-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-client-https-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  ZAo= # d
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.ca").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.ca").Exists()).To(BeTrue())

			// client certificate
			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.httpsClientCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("linstor.internal.httpsClientCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// controller certificate
			certPool = x509.NewCertPool()
			ok = certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.httpsControllerCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ = pem.Decode([]byte(f.ValuesGet("linstor.internal.httpsControllerCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err = x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("HTTPS Certs :: Secret Created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-controller-https-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-client-https-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.ca").String()).To(Equal("c\n"))
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.ca").String()).To(Equal("c\n"))
		})
	})

	Context("HTTPS Certs :: Before Helm", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-controller-https-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-client-https-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("linstor.internal.httpsControllerCert.ca").String()).To(Equal("c\n"))
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("linstor.internal.httpsClientCert.ca").String()).To(Equal("c\n"))
		})
	})

	Context("SSL Certs :: Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(``),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.ca").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.ca").Exists()).To(BeTrue())

			// client certificate
			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.sslNodeCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("linstor.internal.sslNodeCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// controller certificate
			certPool = x509.NewCertPool()
			ok = certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.sslControllerCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ = pem.Decode([]byte(f.ValuesGet("linstor.internal.sslControllerCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// Additional checks for controller certificate
			opts := x509.VerifyOptions{
				DNSName: "linstor.d8-linstor.svc",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())

			opts = x509.VerifyOptions{
				DNSName: "127.0.0.1",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())

		})
	})

	Context("SSL Certs :: One secret is missing", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-controller-ssl-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.ca").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.ca").Exists()).To(BeTrue())

			// client certificate
			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.sslNodeCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("linstor.internal.sslNodeCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// controller certificate
			certPool = x509.NewCertPool()
			ok = certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.sslControllerCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ = pem.Decode([]byte(f.ValuesGet("linstor.internal.sslControllerCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err = x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			Expect(f.ValuesGet("linstor.internal.sslNodeCert.ca").String()).To(Equal(f.ValuesGet("linstor.internal.sslControllerCert.ca").String()))
		})
	})

	Context("HTTPS Certs :: Secrets are having different CA", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-controller-ssl-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-node-ssl-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  ZAo= # d
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.ca").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.cert").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.ca").Exists()).To(BeTrue())

			// client certificate
			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.sslNodeCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("linstor.internal.sslNodeCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			// controller certificate
			certPool = x509.NewCertPool()
			ok = certPool.AppendCertsFromPEM([]byte(f.ValuesGet("linstor.internal.sslControllerCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ = pem.Decode([]byte(f.ValuesGet("linstor.internal.sslControllerCert.cert").String()))
			Expect(block).ShouldNot(BeNil())

			_, err = x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("HTTPS Certs :: Secret Created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-controller-ssl-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-node-ssl-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.ca").String()).To(Equal("c\n"))
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.ca").String()).To(Equal("c\n"))
		})
	})

	Context("HTTPS Certs :: Before Helm", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-controller-ssl-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
---
apiVersion: v1
kind: Secret
metadata:
  name: linstor-node-ssl-cert
  namespace: d8-linstor
data:
  tls.crt: YQo= # a
  tls.key: Ygo= # b
  ca.crt:  Ywo= # c
			`),
				f.GenerateBeforeHelmContext(),
			)
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("linstor.internal.sslControllerCert.ca").String()).To(Equal("c\n"))
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.cert").String()).To(Equal("a\n"))
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.key").String()).To(Equal("b\n"))
			Expect(f.ValuesGet("linstor.internal.sslNodeCert.ca").String()).To(Equal("c\n"))
		})
	})

})
