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
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"

	"github.com/deckhouse/deckhouse/go_lib/certificate"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

func generateSelfSignedCert(fullchain bool, options ...interface{}) string {
	var log = logrus.New()
	log.Level = logrus.InfoLevel
	log.Out = os.Stdout
	var logEntry = log.WithContext(context.TODO())
	selfSignedCA, _ := certificate.GenerateCA(logEntry, "test")
	cert, _ := certificate.GenerateSelfSignedCert(logEntry,
		"dashboard.test",
		selfSignedCA,
		options...,
	)
	selfSignedCertificate := cert.Cert
	if fullchain {
		selfSignedCertificate += selfSignedCA.Cert
	}
	return base64.StdEncoding.EncodeToString([]byte(selfSignedCertificate))
}

var _ = Describe("Modules :: cert-manager :: hooks :: orphan_secrets_cleaner ::", func() {
	const (
		stateCertificate = `
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  annotations:
    meta.helm.sh/release-name: dashboard
    meta.helm.sh/release-namespace: d8-system
  labels:
    app: dashboard
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: dashboard
  name: dashboard
  namespace: d8-dashboard
spec:
  acme:
    config:
    - domains:
      - dashboard.test
      http01:
        ingressClass: nginx
  dnsNames:
  - dashboard.test
  issuerRef:
    kind: ClusterIssuer
    name: letsencrypt
  secretName: ingress-tls
`
		stateSecretTemplate = `
---
apiVersion: v1
kind: Secret
metadata:
  annotations:
    cert-manager.io/alt-names: dashboard.test
    cert-manager.io/certificate-name: dashboard
    cert-manager.io/common-name: dashboard.test
    cert-manager.io/ip-sans: ""
    cert-manager.io/issuer-kind: ClusterIssuer
    cert-manager.io/issuer-name: letsencrypt
  name: ingress-tls
  namespace: d8-dashboard
type: kubernetes.io/tls
data:
  ca.crt: ""
  tls.crt: %s
  tls.key: ""
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("cert-manager.io", "v1", "Certificate", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Certificate in cluster, Secret in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCertificate + fmt.Sprintf(stateSecretTemplate, "LS0tLS1C")))
			f.RunHook()
		})

		It("The Secret should not be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Secret", "d8-dashboard", "ingress-tls").Exists()).To(BeTrue())
		})
	})

	Context("Certificate in cluster, Secret in cluster containing expired certificate", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCertificate + fmt.Sprintf(stateSecretTemplate, generateSelfSignedCert(false, certificate.WithSigningDefaultExpiry(-1)))))
			f.RunHook()
		})

		It("The Secret should not be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Secret", "d8-dashboard", "ingress-tls").Exists()).To(BeTrue())
		})
	})

	Context("Certificate not in cluster, Secret in cluster containing expired certificate", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(stateSecretTemplate, generateSelfSignedCert(false, certificate.WithSigningDefaultExpiry(-1)))))
			f.RunHook()
		})

		It("The Secret should be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Secret", "d8-dashboard", "ingress-tls").Exists()).To(BeFalse())
		})
	})

	Context("Certificate not in cluster, Secret in cluster containing valid certificate", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(fmt.Sprintf(stateSecretTemplate, generateSelfSignedCert(true))))
			f.RunHook()
		})

		It("The Secret should not be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Secret", "d8-dashboard", "ingress-tls").Exists()).To(BeTrue())
		})
	})
})
