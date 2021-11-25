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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cert-manager :: hooks :: migrate_legacy_d8_certificates_and_secrets ::", func() {
	const (
		stateNamespace = `
apiVersion: v1
kind: Namespace
metadata:
  name: d8-dashboard
`
		stateCertificates = `
---
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
  name: dashboard
  namespace: d8-dashboard
---
apiVersion: certmanager.k8s.io/v1alpha1
kind: Certificate
metadata:
  name: nginx
  namespace: default
`
		stateSecrets = `
---
apiVersion: v1
kind: Secret
metadata:
  annotations:
    certmanager.k8s.io/certificate-name: dashboard
    app.domain.com/test: custom
  labels:
    certmanager.k8s.io/certificate-name: dashboard
    test: custom
  name: ingress-tls
  namespace: d8-dashboard
type: kubernetes.io/tls
data:
  ca.crt: ""
  tls.crt: LS0tLS1C
  tls.key: LS0tLS1C
---
apiVersion: v1
kind: Secret
metadata:
  annotations:
    certmanager.k8s.io/certificate-name: nginx
  labels:
    certmanager.k8s.io/certificate-name: nginx
  name: nginx-tls
  namespace: default # out of scope of the migration
type: kubernetes.io/tls
data:
  ca.crt: ""
  tls.crt: LS0tLS1C
  tls.key: LS0tLS1C
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD("certmanager.k8s.io", "v1alpha1", "Certificate", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})
	Context("Secrets in cluster, Certificate in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespace + stateCertificates + stateSecrets))
			f.RunHook()
		})

		It("removes d8 Certificate, migrates its Secret, resources in ns/default should be untouched", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesResource("Certificate", "d8-dashboard", "dashboard").Exists()).To(BeFalse())
			d8Secret := f.KubernetesResource("Secret", "d8-dashboard", "ingress-tls")
			Expect(d8Secret.Field(`metadata.annotations`).String()).To(MatchYAML(`app.domain.com/test: custom`))
			Expect(d8Secret.Field(`metadata.labels`).String()).To(MatchYAML(`test: custom`))

			Expect(f.KubernetesResource("Certificate", "default", "nginx").Exists()).To(BeTrue())
			otherSecret := f.KubernetesResource("Secret", "default", "nginx-tls")
			Expect(otherSecret.Field(`metadata.annotations`).String()).To(MatchYAML(`certmanager.k8s.io/certificate-name: nginx`))
			Expect(otherSecret.Field(`metadata.labels`).String()).To(MatchYAML(`certmanager.k8s.io/certificate-name: nginx`))
		})
	})
})
