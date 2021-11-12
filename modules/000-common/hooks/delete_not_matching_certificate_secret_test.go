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

var _ = Describe("Modules :: common :: hooks :: delete_not_matching_certificate_secret ::", func() {
	const (
		stateNamespaces = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-system
`
		stateSecrets = `
---
apiVersion: v1
data:
  tls.crt: Q1JUQ1JUQ1JUCg== # CRTCRTCRT
  tls.key: S0VZS0VZS0VZCg== # KEYKEYKEY
kind: Secret
metadata:
  name: ingress-tls
  namespace: d8-system
  annotations:
    cert-manager.io/issuer-name: letsencrypt
type: kubernetes.io/tls
`
	)

	f := HookExecutionConfigInit(`{"common":{"internal": {}}}`, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Namespace and secret are in cluster, https mode set to CertManager, ClusterIssues letsencrypt", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaces + stateSecrets))
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
			f.RunHook()
		})

		It("Secret should still exist", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "d8-system", "ingress-tls")
			Expect(secret.Exists()).To(BeTrue())
		})

	})

	Context("Namespace and secret are in cluster, https mode set to CertManager, ClusterIssues selfsigned", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNamespaces + stateSecrets))
			f.ValuesSet("global.modules.https.mode", "CertManager")
			f.ValuesSet("global.modules.https.certManager.clusterIssuerName", "selfsigned")
			f.RunHook()
		})

		It("Hook must delete secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "d8-system", "ingress-tls")
			Expect(secret.Exists()).To(BeFalse())
		})

	})

})
