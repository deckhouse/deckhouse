/*
Copyright 2021 Flant CJSC

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
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: order_certificates", func() {
	f := HookExecutionConfigInit("", "")
	var log = logrus.New()
	log.Level = logrus.InfoLevel
	log.Out = os.Stdout
	var logEntry = log.WithContext(context.TODO())

	selfSignedCA, _ := certificate.GenerateCA(logEntry, "kubernetes")
	cert, _ := certificate.GenerateSelfSignedCert(logEntry, "test", []string{"test.kube-system.svc"}, selfSignedCA)

	Context(":: empty_cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It(":: Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context(":: ready_cluster", func() {
		BeforeEach(func() {
			f.RunHook()
		})
		It(":: should_run_successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context(":: ready_cluster_with_one_ingress_controller_and_not_expired_certificate", func() {
		BeforeEach(func() {
			values := `
internal:
 ingressControllers:
 - name: first
`
			f.ValuesSetFromYaml("ingressNginx", []byte(values))

			tlsAuthSecret := fmt.Sprintf(`
---
apiVersion: v1
data:
  client.crt: %s
  client.key: %s
kind: Secret
metadata:
  name: ingress-nginx-first-auth-tls
  namespace: d8-ingress-nginx
type: Opaque
`, base64.StdEncoding.EncodeToString([]byte(cert.Cert)), base64.StdEncoding.EncodeToString([]byte(cert.Key)))

			f.BindingContexts.Set(f.KubeStateSet(tlsAuthSecret))

			var secret *v1.Secret
			err := yaml.Unmarshal([]byte(tlsAuthSecret), &secret)
			if err != nil {
				fmt.Printf("yaml unmarshal error: %v", err)
			}

			_, _ = f.KubeClient().CoreV1().Secrets("d8-ingress-nginx").Create(context.TODO(), secret, metav1.CreateOptions{})

			f.RunHook()
		})
		It(":: should_run_successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It(":: certificate_must_be_valid_and_not_updated", func() {
			certFirst := f.ValuesGet("ingressNginx.internal.nginxAuthTLS.0")
			Expect(certFirst.Exists()).To(BeTrue())
			Expect(certFirst.Get("controllerName").String()).To(Equal("first"))
			Expect(certFirst.Get("data.key").Exists()).To(BeTrue())
			Expect(certFirst.Get("data.certificate_updated").Exists()).To(BeFalse())

			certFromValues := certFirst.Get("data.certificate").String()
			parsedCert, err := helpers.ParseCertificatePEM([]byte(certFromValues))
			if err != nil {
				fmt.Printf("certificate parsing error: %v", err)
			}

			Expect(time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC).Equal(parsedCert.NotBefore)).To(BeFalse())
			Expect(time.Now().Before(parsedCert.NotAfter.AddDate(0, 0, -10))).To(BeTrue())
		})
	})

})
