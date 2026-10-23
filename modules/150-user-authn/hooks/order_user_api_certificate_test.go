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

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("User Authn hooks :: order user-api certificate ::", func() {
	f := HookExecutionConfigInit(`{"global":{"discovery":{"clusterDomain":"cluster.example"}},"userAuthn":{"internal":{"userAPITLS":{}}}}`, "")

	verifiedCertificate := func() *x509.Certificate {
		roots := x509.NewCertPool()
		Expect(roots.AppendCertsFromPEM([]byte(f.ValuesGet("userAuthn.internal.userAPITLS.ca").String()))).To(BeTrue())

		block, _ := pem.Decode([]byte(f.ValuesGet("userAuthn.internal.userAPITLS.crt").String()))
		Expect(block).ToNot(BeNil())
		cert, err := x509.ParseCertificate(block.Bytes)
		Expect(err).ToNot(HaveOccurred())

		// Every name the console may use for the Service verifies against the CA in values.
		for _, name := range []string{
			"user-api.d8-user-authn",
			"user-api.d8-user-authn.svc",
			"user-api.d8-user-authn.svc.cluster.example",
		} {
			_, err := cert.Verify(x509.VerifyOptions{
				DNSName:   name,
				Roots:     roots,
				KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			})
			Expect(err).ToNot(HaveOccurred(), "the certificate should be valid for %s", name)
		}
		return cert
	}

	Context("Without the Secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should generate a certificate for the user-api Service", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("userAuthn.internal.userAPITLS.key").String()).ToNot(BeEmpty())
			Expect(verifiedCertificate().Subject.CommonName).To(Equal("user-api.d8-user-authn"))
		})

		Context("Then with the Secret Helm rendered from these values", func() {
			var crt string

			BeforeEach(func() {
				crt = f.ValuesGet("userAuthn.internal.userAPITLS.crt").String()
				f.BindingContexts.Set(f.KubeStateSet(userAPITLSSecret(
					f.ValuesGet("userAuthn.internal.userAPITLS.ca").String(),
					crt,
					f.ValuesGet("userAuthn.internal.userAPITLS.key").String(),
				)))
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())
				f.RunHook()
			})

			It("Should keep the certificate", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("userAuthn.internal.userAPITLS.crt").String()).To(Equal(crt))
				verifiedCertificate()
			})
		})
	})
})

func userAPITLSSecret(ca, crt, key string) string {
	return fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: user-api-tls
  namespace: d8-user-authn
type: kubernetes.io/tls
data:
  ca.crt: %s
  tls.crt: %s
  tls.key: %s
`,
		base64.StdEncoding.EncodeToString([]byte(ca)),
		base64.StdEncoding.EncodeToString([]byte(crt)),
		base64.StdEncoding.EncodeToString([]byte(key)),
	)
}
