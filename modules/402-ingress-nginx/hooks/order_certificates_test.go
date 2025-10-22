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

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	"github.com/deckhouse/deckhouse/pkg/log"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("ingress-nginx :: hooks :: order_certificates", func() {
	f := HookExecutionConfigInit("", "")

	logger := log.NewNop()

	selfSignedCA, _ := certificate.GenerateCA(logger, "kube-rbac-proxy-ca-key-pair")
	cert, _ := certificate.GenerateSelfSignedCert(logger, "test", selfSignedCA, certificate.WithSigningDefaultExpiry(10*365*24*time.Hour))

	selfSignedCAKey := addIndentsToMultilineString(selfSignedCA.Key, 4)
	selfSignedCACert := addIndentsToMultilineString(selfSignedCA.Cert, 4)

	Context(":: empty_cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.BindingContexts.Set(f.GenerateScheduleContext("42 4 * * *"))
			f.RunHook()
		})
		It(":: Should run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context(":: ready_cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateScheduleContext("42 4 * * *"))
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

			f.KubeStateSet(tlsAuthSecret)

			var secret *v1.Secret
			err := yaml.Unmarshal([]byte(tlsAuthSecret), &secret)
			if err != nil {
				fmt.Printf("yaml unmarshal error: %v", err)
			}

			_, _ = f.KubeClient().CoreV1().Secrets("d8-ingress-nginx").Create(context.TODO(), secret, metav1.CreateOptions{})

			f.BindingContexts.Set(f.GenerateScheduleContext("42 4 * * *"))

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

			certFromValues := certFirst.Get("data.cert").String()
			parsedCert, err := helpers.ParseCertificatePEM([]byte(certFromValues))
			if err != nil {
				fmt.Printf("certificate parsing error: %v", err)
			}

			Expect(time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC).Equal(parsedCert.NotBefore)).To(BeFalse())
			Expect(time.Now().Before(parsedCert.NotAfter.AddDate(0, 0, -10))).To(BeTrue())
		})
	})

	Context(":: ready_cluster_with_one_ingress_controller_and_no_certificate", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)

			kubeRBACProxyCA := fmt.Sprintf(`
kubeRBACProxyCA:
  cert: |
%s
  key: |
%s
`, selfSignedCACert, selfSignedCAKey)

			f.ValuesSetFromYaml("global.internal.modules", []byte(kubeRBACProxyCA))

			values := `
internal:
 ingressControllers:
 - name: new-controller
`
			f.ValuesSetFromYaml("ingressNginx", []byte(values))

			f.BindingContexts.Set(f.GenerateScheduleContext("42 4 * * *"))

			f.RunHook()
		})
		It(":: should_run_successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It(":: certificate_must_be_valid", func() {
			certNewController := f.ValuesGet("ingressNginx.internal.nginxAuthTLS.0")
			Expect(certNewController.Exists()).To(BeTrue())
			Expect(certNewController.Get("controllerName").String()).To(Equal("new-controller"))
			Expect(certNewController.Get("data.key").Exists()).To(BeTrue())

			certFromValues := certNewController.Get("data.cert").String()
			parsedCert, err := helpers.ParseCertificatePEM([]byte(certFromValues))
			if err != nil {
				fmt.Printf("certificate parsing error: %v", err)
			}

			Expect(time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC).Equal(parsedCert.NotBefore)).To(BeFalse())
			Expect(time.Now().Before(parsedCert.NotAfter.AddDate(0, 0, -10))).To(BeTrue())
		})
	})

	// this test could be deleted after release 1.42, with migration branch
	Context(":: Cluster with one ingress controller and old certificate", func() {
		BeforeEach(func() {
			kubeRBACProxyCA := fmt.Sprintf(`
kubeRBACProxyCA:
  cert: |
%s
  key: |
%s
`, selfSignedCACert, selfSignedCAKey)

			f.ValuesSetFromYaml("global.internal.modules", []byte(kubeRBACProxyCA))

			values := `
internal:
 ingressControllers:
 - name: main
`
			f.ValuesSetFromYaml("ingressNginx", []byte(values))

			tlsAuthSecret := `
---
apiVersion: v1
data:
  client.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNaVENDQVUyZ0F3SUJBZ0lSQU50K1ZqUkN0V0JZR2Y4ZElQR0FGZzB3RFFZSktvWklodmNOQVFFTEJRQXcKRlRFVE1CRUdBMVVFQXhNS2EzVmlaWEp1WlhSbGN6QWVGdzB5TWpFeU1qRXhNVEU0TXpsYUZ3MHlNekV5TWpFeApNVEU0TXpsYU1Eb3hHekFaQmdOVkJBb1RFbWx1WjNKbGMzTXRibWRwYm5nNllYVjBhREViTUJrR0ExVUVBeE1TCmJtZHBibmd0YVc1bmNtVnpjenB0WVdsdU1Ga3dFd1lIS29aSXpqMENBUVlJS29aSXpqMERBUWNEUWdBRWttdGYKcTlyamlnZEpmSEN6Zk15d2xJQk1mNHJUMGZyaXgzNE1zU20ydHROMkFXR092L2tod0pQZ0xpbi9nd3N2ZXNJcQpadUhZWVBBRkh0MHBwMHBLaWFOV01GUXdEZ1lEVlIwUEFRSC9CQVFEQWdXZ01CTUdBMVVkSlFRTU1Bb0dDQ3NHCkFRVUZCd01DTUF3R0ExVWRFd0VCL3dRQ01BQXdId1lEVlIwakJCZ3dGb0FVcEkyT0w3YkFjcTczTm9XZWcrczcKSlBnRU1sa3dEUVlKS29aSWh2Y05BUUVMQlFBRGdnRUJBSm5TaFp2QTI5TEc1NTdpYWFKMEhCS2ZEZkJwU0JrSQpsNy80RGxwU1VLQU5CN0VpaVZHS1VLWTVvQ2lOWW9zYytva1ozdElUYlJ5Z2ZoTnJxV0dSeGdNdlNHWHRpUWVBCnRzRzJRWUxNMGZzazA4R0dNcDl4QmlpdnhoSXRqNW5oaUNCdGRpMTJPejk0dUtlTVZqNkFNQ0I5WFAxRDBLQm0KYjl2MXFVS0FXSnBVdncyZWJ2Ykh3a0grZnRzNEJhcERBaERXdERuaTc1Z0liYldZWlhWbUZhUlZjdHFrUG9mVgpISFhqaUpOLzcrb0dpdGFSN0xSRTNBUXh0NlBGZkp5TkdMMnFRMkw3clBiTXlhNHNIZWFvV0tReTJMeDF0ZzBnCkxIMllNb0JyOFRLdU4rU0ZBd21OWjVCWEpOUHZPcWpLaXlvVFRKZWUxTTY0MkRBcVExU3g3UTQ9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K
  client.key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSVBtbEd6SWxRNjNkYVM2clFCeTN6bHNDOU9wc3BWaEtCaUQwNzJnNGpQMTdvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFa210ZnE5cmppZ2RKZkhDemZNeXdsSUJNZjRyVDBmcml4MzRNc1NtMnR0TjJBV0dPdi9raAp3SlBnTGluL2d3c3Zlc0lxWnVIWVlQQUZIdDBwcDBwS2lRPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=
kind: Secret
metadata:
  name: ingress-nginx-main-auth-tls
  namespace: d8-ingress-nginx
type: Opaque
`

			f.BindingContexts.Set(f.KubeStateSet(tlsAuthSecret))

			var secret *v1.Secret
			err := yaml.Unmarshal([]byte(tlsAuthSecret), &secret)
			if err != nil {
				fmt.Printf("yaml unmarshal error: %v", err)
			}

			_, _ = f.KubeClient().CoreV1().Secrets("d8-ingress-nginx").Create(context.TODO(), secret, metav1.CreateOptions{})

			f.BindingContexts.Set(f.GenerateScheduleContext("42 4 * * *"))

			f.RunHook()
		})

		It(":: certificate must be updated", func() {
			Expect(f).To(ExecuteSuccessfully())

			certFirst := f.ValuesGet("ingressNginx.internal.nginxAuthTLS.0")
			Expect(certFirst.Exists()).To(BeTrue())
			Expect(certFirst.Get("controllerName").String()).To(Equal("main"))
			Expect(certFirst.Get("data.key").Exists()).To(BeTrue())
			Expect(certFirst.Get("data.cert").Exists()).To(BeTrue())

			certFromValues := certFirst.Get("data.cert").String()
			parsedCert, err := helpers.ParseCertificatePEM([]byte(certFromValues))
			if err != nil {
				fmt.Printf("certificate parsing error: %v", err)
			}

			Expect(time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC).Equal(parsedCert.NotBefore)).To(BeFalse())
			Expect(time.Now().Before(parsedCert.NotAfter.AddDate(0, 0, -10))).To(BeTrue())
			Expect(parsedCert.Issuer.CommonName).To(Equal("kube-rbac-proxy-ca-key-pair"))
		})
	})

})

func addIndentsToMultilineString(s string, indentsCount int) string {
	var newString string
	indent := strings.Repeat(" ", indentsCount)
	for _, line := range strings.Split(s, "\n") {
		newString = fmt.Sprintf("%s%s%s\n", newString, indent, line)
	}
	return newString
}
