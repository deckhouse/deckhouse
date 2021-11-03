// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Cert Manager hooks :: discover email for clusterissuers ::", func() {
	const (
		clusterissuerWithEmail = `
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt
  labels:
    heritage: deckhouse
spec:
  acme:
    email: test+letsencrypt-test-dev@notice.flant.com
    solvers:
    - http01:
        ingress: {}
    privateKeySecretRef:
      name: cert-manager-letsencrypt-private-key
    server: https://acme-v02.api.letsencrypt.org/directory
`
		clusterissuerWithoutEmail = `
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt
  labels:
    heritage: deckhouse
spec:
  acme:
    solvers:
    - http01:
        ingress: {}
    privateKeySecretRef:
      name: cert-manager-letsencrypt-private-key
    server: https://acme-v02.api.letsencrypt.org/directory
`
	)

	f := HookExecutionConfigInit(`{"certManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("cert-manager.io", "v1", "ClusterIssuer", false)
	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should run", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with ClusterIssuer resource, but without set config values", func() {
		BeforeEach(func() {
			f.KubeStateSet(clusterissuerWithEmail)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should run, internal values must be set from clusterissuer", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("certManager.internal.email").String()).To(Equal("test+letsencrypt-test-dev@notice.flant.com"))
		})
	})

	Context("Cluster with ClusterIssuer resource, with set config values", func() {
		BeforeEach(func() {
			f.KubeStateSet(clusterissuerWithEmail)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("certManager.email", "test@test.com")
			f.RunHook()
		})

		It("Hook should run, internal values must be set from config value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("certManager.internal.email").String()).To(Equal("test@test.com"))
		})
	})

	Context("Cluster with ClusterIssuer resource without email, but without set config values", func() {
		BeforeEach(func() {
			f.KubeStateSet(clusterissuerWithoutEmail)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook should run, internal values must be set from clusterissuer", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("certManager.internal.email").String()).Should(BeEmpty())
		})
	})

	Context("Cluster with ClusterIssuer resource without email, with set config values", func() {
		BeforeEach(func() {
			f.KubeStateSet(clusterissuerWithoutEmail)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet("certManager.email", "test@test.com")
			f.RunHook()
		})

		It("Hook should run, internal values must be set from config value", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("certManager.internal.email").String()).To(Equal("test@test.com"))
		})
	})
})
