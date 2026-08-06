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
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	testClusterDomain = "cluster.local"

	testServiceSAN        = "registry-packages-proxy.d8-cloud-instance-manager"
	testServiceSVCSAN     = "registry-packages-proxy.d8-cloud-instance-manager.svc"
	testServiceDomainSAN  = "registry-packages-proxy.d8-cloud-instance-manager.cluster.local"
	testServiceSVCDomSAN  = "registry-packages-proxy.d8-cloud-instance-manager.svc.cluster.local"
	testFirstMasterAddr   = "192.168.0.1"
	testSecondMasterAddr  = "192.168.0.2"
	testReplacedMasterIP  = "192.168.0.3"
	testLocalhostCertAddr = "127.0.0.1"
)

func certValues(addresses string) string {
	return fmt.Sprintf(
		`{"global":{"discovery":{"clusterDomain":%q}},"registryPackagesProxy":{"internal":{"proxyAddresses":%s}}}`,
		testClusterDomain, addresses,
	)
}

// proxyCertFixture issues a certificate the same way the hook does, to seed the
// secret the hook reads.
func proxyCertFixture(sans ...string) certificate.Certificate {
	ca, err := certificate.GenerateCA(log.NewNop(), proxyCertCN,
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry("87600h"))
	Expect(err).To(BeNil())

	cert, err := certificate.GenerateSelfSignedCert(log.NewNop(), proxyCertCN, ca,
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithSigningDefaultExpiry((24*time.Hour)*365*10),
		certificate.WithSigningDefaultUsage([]string{"signing", "key encipherment", "server auth"}),
		certificate.WithSANs(sans...))
	Expect(err).To(BeNil())

	return cert
}

func proxyCertSecret(cert certificate.Certificate) string {
	return fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: registry-packages-proxy-tls
  namespace: d8-cloud-instance-manager
data:
  ca.crt: %s
  tls.crt: %s
  tls.key: %s
`,
		base64.StdEncoding.EncodeToString([]byte(cert.CA)),
		base64.StdEncoding.EncodeToString([]byte(cert.Cert)),
		base64.StdEncoding.EncodeToString([]byte(cert.Key)),
	)
}

func storedCert(f *HookExecutionConfig) *x509.Certificate {
	pemData := f.ValuesGet(proxyCertValuesPath + ".crt").String()
	Expect(pemData).ToNot(BeEmpty())

	cert, err := certificate.ParseCertificate(pemData)
	Expect(err).To(BeNil())

	return cert
}

func certIPs(cert *x509.Certificate) []string {
	ips := make([]string, 0, len(cert.IPAddresses))
	for _, ip := range cert.IPAddresses {
		ips = append(ips, ip.String())
	}

	return ips
}

var _ = Describe("Module :: registry-packages-proxy :: hooks :: generate proxy certs ::", func() {
	Context("Cluster without the secret and without discovered addresses", func() {
		f := HookExecutionConfigInit(certValues(`[]`), initConfigValuesString)

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("issues a certificate signed by a CA of its own", func() {
			Expect(f).To(ExecuteSuccessfully())

			ca, err := certificate.ParseCertificate(f.ValuesGet(proxyCertValuesPath + ".ca").String())
			Expect(err).To(BeNil())
			Expect(ca.IsCA).To(BeTrue())
			Expect(ca.Subject.CommonName).To(Equal(proxyCertCN))

			cert := storedCert(f)
			Expect(cert.IsCA).To(BeFalse())
			Expect(cert.Subject.CommonName).To(Equal(proxyCertCN))
			Expect(f.ValuesGet(proxyCertValuesPath + ".key").Exists()).To(BeTrue())
		})

		It("allows the certificate to be used for serving TLS", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(storedCert(f).ExtKeyUsage).To(ContainElement(x509.ExtKeyUsageServerAuth))
		})

		It("covers the service names and localhost", func() {
			Expect(f).To(ExecuteSuccessfully())

			cert := storedCert(f)
			Expect(cert.DNSNames).To(ConsistOf(
				testServiceSAN, testServiceSVCSAN, testServiceDomainSAN, testServiceSVCDomSAN,
			))
			Expect(certIPs(cert)).To(ConsistOf(testLocalhostCertAddr))
		})
	})

	Context("Cluster with discovered master addresses", func() {
		f := HookExecutionConfigInit(
			certValues(fmt.Sprintf(`[%q,%q]`, testFirstMasterAddr, testSecondMasterAddr)),
			initConfigValuesString,
		)

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("puts every master address into the certificate", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(certIPs(storedCert(f))).To(ConsistOf(
				testLocalhostCertAddr, testFirstMasterAddr, testSecondMasterAddr,
			))
		})
	})

	Context("Secret holds a certificate that already covers the addresses", func() {
		f := HookExecutionConfigInit(certValues(fmt.Sprintf(`[%q]`, testFirstMasterAddr)), initConfigValuesString)

		existing := proxyCertFixture(
			testLocalhostCertAddr, testServiceSAN, testServiceSVCSAN,
			testServiceDomainSAN, testServiceSVCDomSAN, testFirstMasterAddr,
		)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(proxyCertSecret(existing)))
			f.RunHook()
		})

		It("keeps it instead of issuing a new one", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(proxyCertValuesPath + ".ca").String()).To(Equal(existing.CA))
			Expect(f.ValuesGet(proxyCertValuesPath + ".crt").String()).To(Equal(existing.Cert))
			Expect(f.ValuesGet(proxyCertValuesPath + ".key").String()).To(Equal(existing.Key))
		})
	})

	Context("Master address changed after the certificate was issued", func() {
		f := HookExecutionConfigInit(certValues(fmt.Sprintf(`[%q]`, testReplacedMasterIP)), initConfigValuesString)

		existing := proxyCertFixture(
			testLocalhostCertAddr, testServiceSAN, testServiceSVCSAN,
			testServiceDomainSAN, testServiceSVCDomSAN, testFirstMasterAddr,
		)

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(proxyCertSecret(existing)))
			f.RunHook()
		})

		It("reissues the certificate for the new address", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(proxyCertValuesPath + ".crt").String()).ToNot(Equal(existing.Cert))
			Expect(certIPs(storedCert(f))).To(ConsistOf(testLocalhostCertAddr, testReplacedMasterIP))
		})
	})
})
