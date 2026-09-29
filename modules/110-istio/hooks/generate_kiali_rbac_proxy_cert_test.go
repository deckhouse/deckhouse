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
	"encoding/pem"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Istio hooks :: generate_kiali_rbac_proxy_cert ::", func() {
	f := HookExecutionConfigInit(`{"global":{"internal":{"modules":{"kubeRBACProxyCA":{}}}},"istio":{"internal":{"kialiRBACProxyTLS":{}}}}`, "")

	ca, caErr := certificate.GenerateCA(log.NewNop(), "kubernetes-api-proxy")
	otherCA, otherCAErr := certificate.GenerateCA(log.NewNop(), "some-other-ca")

	BeforeEach(func() {
		Expect(caErr).ToNot(HaveOccurred())
		Expect(otherCAErr).ToNot(HaveOccurred())

		f.ValuesSet("global.internal.modules.kubeRBACProxyCA.cert", ca.Cert)
		f.ValuesSet("global.internal.modules.kubeRBACProxyCA.key", ca.Key)
	})

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Issues the certificate from the cluster-wide CA", func() {
			Expect(f).To(ExecuteSuccessfully())

			crt := f.ValuesGet("istio.internal.kialiRBACProxyTLS.crt").String()
			Expect(crt).ToNot(BeEmpty())
			Expect(f.ValuesGet("istio.internal.kialiRBACProxyTLS.key").String()).ToNot(BeEmpty())
			Expect(f.ValuesGet("istio.internal.kialiRBACProxyTLS.ca").String()).To(Equal(ca.Cert))

			Expect(verifyAgainstCA(crt, ca.Cert)).To(Succeed())
		})

		It("Issues the certificate for the in-cluster address of the service", func() {
			Expect(f).To(ExecuteSuccessfully())

			parsed, err := certificate.ParseCertificate(f.ValuesGet("istio.internal.kialiRBACProxyTLS.crt").String())
			Expect(err).ToNot(HaveOccurred())
			Expect(parsed.DNSNames).To(ContainElement("kiali.d8-istio.svc"))
		})
	})

	Context("Certificate of the current CA is in the cluster", func() {
		var existing certificate.Certificate

		BeforeEach(func() {
			var err error
			existing, err = certificate.GenerateSelfSignedCert(log.NewNop(), "kiali.d8-istio.svc", ca,
				certificate.WithSANs("kiali.d8-istio.svc"))
			Expect(err).ToNot(HaveOccurred())

			f.KubeStateSet(kialiRBACProxySecretManifest(existing.Cert, existing.Key, ca.Cert))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Keeps the certificate instead of reissuing it", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("istio.internal.kialiRBACProxyTLS.crt").String()).To(Equal(existing.Cert))
			Expect(f.ValuesGet("istio.internal.kialiRBACProxyTLS.key").String()).To(Equal(existing.Key))
		})
	})

	Context("Certificate of a rotated CA is in the cluster", func() {
		var stale certificate.Certificate

		BeforeEach(func() {
			var err error
			stale, err = certificate.GenerateSelfSignedCert(log.NewNop(), "kiali.d8-istio.svc", otherCA,
				certificate.WithSANs("kiali.d8-istio.svc"))
			Expect(err).ToNot(HaveOccurred())

			f.KubeStateSet(kialiRBACProxySecretManifest(stale.Cert, stale.Key, otherCA.Cert))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Reissues the certificate from the current CA", func() {
			Expect(f).To(ExecuteSuccessfully())

			crt := f.ValuesGet("istio.internal.kialiRBACProxyTLS.crt").String()
			Expect(crt).ToNot(Equal(stale.Cert))
			Expect(f.ValuesGet("istio.internal.kialiRBACProxyTLS.ca").String()).To(Equal(ca.Cert))
			Expect(verifyAgainstCA(crt, ca.Cert)).To(Succeed())
		})
	})

	Context("Cluster-wide CA is not generated yet", func() {
		BeforeEach(func() {
			f.ValuesSet("global.internal.modules.kubeRBACProxyCA.cert", "")
			f.ValuesSet("global.internal.modules.kubeRBACProxyCA.key", "")
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Fails loudly instead of issuing an unverifiable certificate", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
		})
	})
})

func kialiRBACProxySecretManifest(crt, key, caCert string) string {
	return fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: kiali-kube-rbac-proxy-tls
  namespace: d8-istio
type: kubernetes.io/tls
data:
  ca.crt: %s
  tls.crt: %s
  tls.key: %s
`,
		base64.StdEncoding.EncodeToString([]byte(caCert)),
		base64.StdEncoding.EncodeToString([]byte(crt)),
		base64.StdEncoding.EncodeToString([]byte(key)),
	)
}

func verifyAgainstCA(crt, caCert string) error {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caCert)) {
		return fmt.Errorf("cannot parse CA certificate")
	}

	block, _ := pem.Decode([]byte(crt))
	if block == nil {
		return fmt.Errorf("cannot decode certificate PEM")
	}

	parsed, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("cannot parse certificate: %w", err)
	}

	_, err = parsed.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}})

	return err
}
