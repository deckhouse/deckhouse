/*
Copyright 2021 Flant JSC

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

/*

User-stories:
1. Webhook mechanism requires a pair of certificates. This hook generates them and stores in values.

*/

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const (
	initValuesString       = `{"deckhouse":{"internal":{"webhookHandlerCert":{}}},"global":{"discovery":{"clusterDomain":"mycluster.local"}}}`
	initConfigValuesString = `{}`
)

var (
	secretWithActualCert, actualCA, actualCert, actualKey     = generateSecret(false)
	secretWithExpiredCert, expiredCA, expiredCert, expiredKey = generateSecret(true)
)

var _ = Describe("Deckhouse hooks :: generate_selfsigned_ca ::", func() {
	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("New cert data must be generated and stored to values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.crt").Exists()).To(BeTrue())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.key").Exists()).To(BeTrue())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.ca").Exists()).To(BeTrue())

			certPool := x509.NewCertPool()
			ok := certPool.AppendCertsFromPEM([]byte(f.ValuesGet("deckhouse.internal.webhookHandlerCert.ca").String()))
			Expect(ok).To(BeTrue())

			block, _ := pem.Decode([]byte(f.ValuesGet("deckhouse.internal.webhookHandlerCert.crt").String()))
			Expect(block).ShouldNot(BeNil())

			cert, err := x509.ParseCertificate(block.Bytes)
			Expect(err).ShouldNot(HaveOccurred())

			opts := x509.VerifyOptions{
				DNSName: "webhook-handler.d8-system.svc.mycluster.local",
				Roots:   certPool,
			}
			_, err = cert.Verify(opts)
			Expect(err).ShouldNot(HaveOccurred())
		})
	})

	Context("Secret Created", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(secretWithActualCert))
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.crt").String()).To(Equal(actualCert))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.key").String()).To(Equal(actualKey))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.ca").String()).To(Equal(actualCA))
		})
	})

	Context("Before Helm", func() {
		BeforeEach(func() {
			f.KubeStateSet(secretWithActualCert)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Cert data must be stored in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.crt").String()).To(Equal(actualCert))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.key").String()).To(Equal(actualKey))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.ca").String()).To(Equal(actualCA))
		})
	})

	Context("Expired certificate", func() {
		BeforeEach(func() {
			f.KubeStateSet(secretWithExpiredCert)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Cert data must be changed", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.crt").String()).ToNot(Equal(expiredCert))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.key").String()).ToNot(Equal(expiredKey))
			Expect(f.ValuesGet("deckhouse.internal.webhookHandlerCert.ca").String()).ToNot(Equal(expiredCA))
		})
	})
})

func generateTestCert(expired bool) certificate.Certificate {
	expireStr := "87600h"
	expire := 87600 * time.Hour

	if expired {
		expireStr = "-1m"
		expire = -1 * time.Minute
	}

	l := logrus.NewEntry(logrus.New())

	ca, _ := certificate.GenerateCA(l,
		"webhook-handler.d8-system.svc",
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithCAExpiry(expireStr))

	webhookServiceFQDN := fmt.Sprintf(
		"%s.%s",
		webhookServiceHost,
		"mycluster.local",
	)

	sans := []string{
		webhookServiceHost,
		webhookServiceFQDN,
		"validating-" + webhookServiceHost,
		"conversion-" + webhookServiceHost,
		"validating-" + webhookServiceFQDN,
		"conversion-" + webhookServiceFQDN,
	}

	cert, _ := certificate.GenerateSelfSignedCert(l,
		"webhook-handler.d8-system.svc",
		ca,
		certificate.WithSANs(sans...),
		certificate.WithKeyAlgo("ecdsa"),
		certificate.WithKeySize(256),
		certificate.WithSigningDefaultExpiry(expire),
		certificate.WithSigningDefaultUsage([]string{
			"signing",
			"key encipherment",
			"requestheader-client",
		}),
	)

	return cert
}

func generateSecret(expired bool) (string, string, string, string) {
	cert := generateTestCert(expired)
	ca, crt, key := cert.CA, cert.Cert, cert.Key

	sec := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: webhook-handler-certs
  namespace: d8-system
data:
  tls.crt: %s
  tls.key: %s
  ca.crt: %s
`, base64.StdEncoding.EncodeToString([]byte(crt)), base64.StdEncoding.EncodeToString([]byte(key)), base64.StdEncoding.EncodeToString([]byte(ca)))

	return sec, ca, crt, key
}
