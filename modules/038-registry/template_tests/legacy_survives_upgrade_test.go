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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// The objects that serve the in-cluster address must outlive the release that stops rendering them.
//
// This is the backport's purpose. On a `Direct` cluster the nodes pull through the in-cluster
// address; if the Service and the proxy behind it go away on the first reconciliation of the release
// that no longer renders them, nothing answers that address until the node agent takes it over —
// including the pulls the takeover itself depends on.
//
// `helm.sh/resource-policy: keep` is read by Helm off the object ALREADY IN THE CLUSTER, which is
// why it has to be here, in the release installed before the upgrade, and not in the one that
// removes the templates. Asserted per object rather than in one loop so a failure names which one
// lost its annotation.
var _ = Describe("Module :: registry :: helm template :: the legacy pull path survives an upgrade", func() {
	f := SetupHelmConfig(``)

	const orchestratorInDirect = `
internal:
  orchestrator:
    ready: true
    hash: "deadbeef"
    state:
      mode: Direct
      # target_mode and conditions are rendered by the state Secret unconditionally, so the values
      # have to carry them even though this test is about annotations.
      target_mode: Direct
      conditions: []
      registry_service: incluster-proxy
      pki:
        ca:
          cert: CA-CERT
          key: CA-KEY
        token:
          cert: TOKEN-CERT
          key: TOKEN-KEY
      in_cluster_proxy:
        config:
          version: "1"
          config:
            ca: CA-CERT
            auth_cert: AUTH-CERT
            auth_key: AUTH-KEY
            token_cert: TOKEN-CERT
            token_key: TOKEN-KEY
            distribution_cert: DIST-CERT
            distribution_key: DIST-KEY
            http_secret: HTTP-SECRET
            upstream:
              scheme: HTTPS
              host: dev-registry.example.com
              path: /sys/deckhouse-oss
              user:
                name: upstream-user
                password: upstream-password
                password_hash: upstream-hash
`

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", orchestratorInDirect)
		f.HelmRender()
	})

	It("renders without error", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("keeps the Service every image reference is built from", func() {
		svc := f.KubernetesResource("Service", "d8-system", "registry")
		Expect(svc.Exists()).To(BeTrue(), "the Service is what the nodes resolve; without it there is nothing to keep")
		Expect(svc.Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"))
	})

	It("keeps the proxy that answers that Service", func() {
		deploy := f.KubernetesResource("Deployment", "d8-system", "registry-incluster-proxy")
		Expect(deploy.Exists()).To(BeTrue())
		Expect(deploy.Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"))
	})

	It("keeps the proxy's own configuration, without which it cannot start", func() {
		secret := f.KubernetesResource("Secret", "d8-system", "registry-incluster-proxy-config")
		Expect(secret.Exists()).To(BeTrue())
		Expect(secret.Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"))
	})

	It("keeps the PKI the proxy serves TLS with", func() {
		secret := f.KubernetesResource("Secret", "d8-system", "registry-pki")
		Expect(secret.Exists()).To(BeTrue())
		Expect(secret.Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"))
	})
})
