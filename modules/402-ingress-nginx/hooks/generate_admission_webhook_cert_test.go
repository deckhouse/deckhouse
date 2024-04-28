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

var _ = Describe("ingress-nginx :: hooks :: generate_admission_webhook_cert ::", func() {
	f := HookExecutionConfigInit(`{"ingressNginx":{"defaultControllerVersion": "1.6", "internal": {"admissionCertificate": {}}}}`, "")
	f.RegisterCRD("deckhouse.io", "v1", "IngressNginxController", false)

	Context("Fresh cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})
		It("Should run and create internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.admissionCertificate.ca").String()).ToNot(BeEmpty())
			Expect(f.ValuesGet("ingressNginx.internal.admissionCertificate.cert").String()).ToNot(BeEmpty())
			Expect(f.ValuesGet("ingressNginx.internal.admissionCertificate.key").String()).ToNot(BeEmpty())
		})
	})

	Context("Cluster with existing secret", func() {
		BeforeEach(func() {
			f.KubeStateSet(secretWithCertificate)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should not generate new cert", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("ingressNginx.internal.admissionCertificate.ca").String()).To(BeEquivalentTo("certtest"))
			Expect(f.ValuesGet("ingressNginx.internal.admissionCertificate.cert").String()).To(BeEquivalentTo("certtest"))
			Expect(f.ValuesGet("ingressNginx.internal.admissionCertificate.key").String()).To(BeEquivalentTo("certtest"))
		})
	})
})

const (
	secretWithCertificate = `
---
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: ingress-admission-certificate
  namespace: d8-ingress-nginx
data:
  ca.crt: Y2VydHRlc3Q=
  tls.crt: Y2VydHRlc3Q=
  tls.key: Y2VydHRlc3Q=
`
)
