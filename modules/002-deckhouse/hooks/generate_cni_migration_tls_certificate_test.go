/*
Copyright 2025 Flant JSC

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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	cniInitValuesString       = `{"deckhouse":{"internal":{"cniMigrationCert":{},"webhookHandlerCert":{},"admissionWebhookCert":{}}},"global":{"discovery":{"clusterDomain":"mycluster.local"},"modules":{"publicDomainTemplate":"%s.example.com"}}}`
	cniInitConfigValuesString = `{}`
	cniWebhookServiceHost     = "cni-migration-webhook.d8-system.svc"
)

var (
	cniSecretWithActualCert, cniActualCA, cniActualCert, cniActualKey     = generateCNISecret(false)
	cniSecretWithExpiredCert, cniExpiredCA, cniExpiredCert, cniExpiredKey = generateCNISecret(true)
)

var _ = Describe("002-deckhouse :: hooks :: generate_cni_migration_tls_certificate ::", func() {
	f := HookExecutionConfigInit(cniInitValuesString, cniInitConfigValuesString)
	f.RegisterCRD("network.deckhouse.io", "v1alpha1", "CNIMigration", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.cniMigrationCert.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("deckhouse.internal.cniMigrationCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("deckhouse.internal.cniMigrationCert.ca").Exists()).To(BeTrue())

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("deckhouse.internal.cniMigrationCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("deckhouse.internal.cniMigrationCert.crt").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			opts := x509.VerifyOptions{
				DNSName: "cni-migration-webhook.d8-system.svc.mycluster.local",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("Secret Created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cniSecretWithActualCert))
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.cniMigrationCert.crt").String()).To(Equal(cniActualCert))
			Expect(f.ValuesGet("deckhouse.internal.cniMigrationCert.key").String()).To(Equal(cniActualKey))
			Expect(f.ValuesGet("deckhouse.internal.cniMigrationCert.ca").String()).To(Equal(cniActualCA))
		})
	})

	Context("Expired certificate", func() {
		BeforeEach(func() {
			f.KubeStateSet(cniSecretWithExpiredCert)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Cert data must be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.cniMigrationCert.crt").String()).ToNot(Equal(cniExpiredCert))
			Expect(f.ValuesGet("deckhouse.internal.cniMigrationCert.key").String()).ToNot(Equal(cniExpiredKey))
			Expect(f.ValuesGet("deckhouse.internal.cniMigrationCert.ca").String()).ToNot(Equal(cniExpiredCA))
		})
	})
})

func generateCNITestCert(expired bool) certificate.Certificate {
	expireStr := "87600h"
	expire := 87600 * time.Hour

	if expired {
		expireStr = "-1m"
		expire = -1 * time.Minute
	}

	ca, _ := certificate.GenerateCA(log.NewNop(),
		cniWebhookServiceHost,
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry(expireStr))

	webhookServiceFQDN := fmt.Sprintf(
		"cni-migration-webhook.d8-system.svc.%s",
		"mycluster.local",
	)

	sans := []string{
		cniWebhookServiceHost,
		webhookServiceFQDN,
	}

	cert, _ := certificate.GenerateSelfSignedCert(log.NewNop(),
		cniWebhookServiceHost,
		ca,
		certificate.WithSANs(sans...),
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithSigningDefaultExpiry(expire),
		certificate.WithSigningDefaultUsage([]string{
			"signing",
			"key encipherment",
		}),
	)

	return cert
}

func generateCNISecret(expired bool) (string, string, string, string) {
	cert := generateCNITestCert(expired)
	ca, crt, key := cert.CA, cert.Cert, cert.Key

	sec := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: cni-migration-certs
  namespace: d8-system
data:
  tls.crt: %s
  tls.key: %s
  ca.crt: %s
`, base64.StdEncoding.EncodeToString([]byte(crt)), base64.StdEncoding.EncodeToString([]byte(key)), base64.StdEncoding.EncodeToString([]byte(ca)))

	return sec, ca, crt, key
}
