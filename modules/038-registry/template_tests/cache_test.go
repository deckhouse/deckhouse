// Copyright 2026 Flant JSC
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

package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const cacheOnValues = `
upstream:
  host: registry.example.com
  path: /deckhouse/ee
  scheme: HTTPS
  ca: UPSTREAM_CA_PEM
cache:
  enabled: true
  ttl: 168h
  storageSize: 50Gi
internal:
  takeover:
    phase: "New"
  cache:
    enabled: true
    upstream:
      scheme: HTTPS
      host: registry.example.com
      path: /deckhouse/ee
      username: u
      password: p
      hasCA: true
      ttl: 168h
  pki:
    hash: "h"
    httpSecret: HTTP_SECRET
    ca: {cert: CA_CERT, key: CA_KEY}
    token: {cert: TOKEN_CERT, key: TOKEN_KEY}
    agent: {cert: AGENT_CERT, key: AGENT_KEY}
    distribution: {cert: DIST_CERT, key: DIST_KEY}
    auth: {cert: AUTH_CERT, key: AUTH_KEY}
    users:
      - {name: ro, password: ro-pass, passwordHash: ro-hash, role: ReadOnly}
      - {name: rw, password: rw-pass, passwordHash: rw-hash, role: ReadWrite}
`

const cacheOffValues = `
cache:
  enabled: false
internal:
  takeover:
    phase: "New"
  cache:
    enabled: false
  pki:
    hash: "h"
    httpSecret: HTTP_SECRET
    ca: {cert: CA_CERT, key: CA_KEY}
    token: {cert: TOKEN_CERT, key: TOKEN_KEY}
    agent: {cert: AGENT_CERT, key: AGENT_KEY}
    distribution: {cert: DIST_CERT, key: DIST_KEY}
    auth: {cert: AUTH_CERT, key: AUTH_KEY}
    users:
      - {name: ro, password: ro-pass, passwordHash: ro-hash, role: ReadOnly}
`

var _ = Describe("Module :: registry :: helm template :: cache secrets", func() {
	f := SetupHelmConfig(``)

	Context("cache enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", cacheOnValues)
			f.HelmRender()
		})

		It("renders the cache config secret with distribution + auth config", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-cache-config")
			Expect(s.Exists()).To(BeTrue())
			dist := s.Field("stringData.distribution-config\\.yaml").String()
			Expect(dist).To(ContainSubstring(`remoteurl: "HTTPS://registry.example.com"`))
			Expect(dist).To(ContainSubstring(`remotepathonly: "/deckhouse/ee"`))
			Expect(dist).To(ContainSubstring(`localpathalias: "/system/deckhouse"`))
			Expect(dist).To(ContainSubstring("ca: /pki/upstream-registry-ca.crt"))
			Expect(dist).To(ContainSubstring(`ttl: "168h"`))
			Expect(dist).To(ContainSubstring(`secret: "HTTP_SECRET"`))
			authc := s.Field("stringData.auth-config\\.yaml").String()
			Expect(authc).To(ContainSubstring(`"ro":`))
			Expect(authc).To(ContainSubstring(`password: "ro-hash"`))
			Expect(authc).To(ContainSubstring(`actions: ["*"]`))    // rw → full access
			Expect(authc).To(ContainSubstring(`actions: ["pull"]`)) // ro → read-only
		})

		It("renders the cache PKI secret with certs + upstream CA", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-cache-pki")
			Expect(s.Exists()).To(BeTrue())
			Expect(s.Field(`data.distribution\.crt`).String()).To(Equal(b64("DIST_CERT")))
			Expect(s.Field(`data.token\.key`).String()).To(Equal(b64("TOKEN_KEY")))
			Expect(s.Field(`data.upstream-registry-ca\.crt`).String()).To(Equal(b64("UPSTREAM_CA_PEM")))
		})

		It("renders the cache DaemonSet on masters with three containers + hostPath store", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("DaemonSet", "d8-system", "registry-cache")
			Expect(s.Exists()).To(BeTrue())
			names := []string{
				s.Field("spec.template.spec.containers.0.name").String(),
				s.Field("spec.template.spec.containers.1.name").String(),
				s.Field("spec.template.spec.containers.2.name").String(),
			}
			Expect(names).To(ConsistOf("distribution", "auth", "cache-agent"))
			// node-local hostPath store, not a PVC.
			vols := s.Field("spec.template.spec.volumes").Array()
			var dataPath string
			for _, v := range vols {
				if v.Get("name").String() == "data" {
					dataPath = v.Get("hostPath.path").String()
				}
			}
			Expect(dataPath).To(Equal("/var/lib/deckhouse/registry-cache"))
		})

		It("renders read and leader Services (no peers headless Service)", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			read := f.KubernetesResource("Service", "d8-system", "registry-cache")
			Expect(read.Exists()).To(BeTrue())
			Expect(read.Field("spec.ports.0.targetPort").String()).To(Equal("distribution"))
			leader := f.KubernetesResource("Service", "d8-system", "registry-cache-leader")
			Expect(leader.Exists()).To(BeTrue())
			Expect(leader.Field("spec.selector.registry-cache-role").String()).To(Equal("leader"))
			// No StatefulSet → no headless peers Service.
			Expect(f.KubernetesResource("Service", "d8-system", "registry-cache-peers").Exists()).To(BeFalse())
		})

		It("renders the agent config secret with ro/rw creds and CA", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-cache-agent")
			Expect(s.Exists()).To(BeTrue())
			cfg := s.Field("stringData.config\\.yaml").String()
			Expect(cfg).To(ContainSubstring("leaderAddress: registry-cache-leader.d8-system.svc:5001"))
			Expect(cfg).To(ContainSubstring("name: \"ro\""))
			Expect(cfg).To(ContainSubstring("name: \"rw\""))
		})

		It("renders RBAC: SA + Role (leases + pods/patch) + RoleBinding", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("ServiceAccount", "d8-system", "registry-cache").Exists()).To(BeTrue())
			role := f.KubernetesResource("Role", "d8-system", "registry-cache")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(ContainSubstring("leases"))
			Expect(role.Field("rules").String()).To(ContainSubstring("pods"))
			Expect(f.KubernetesResource("RoleBinding", "d8-system", "registry-cache").Exists()).To(BeTrue())
		})
	})

	Context("cache disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", cacheOffValues)
			f.HelmRender()
		})

		It("renders no cache secrets", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-cache-config").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-cache-pki").Exists()).To(BeFalse())
		})

		It("renders no cache workload", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("DaemonSet", "d8-system", "registry-cache").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-system", "registry-cache").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-system", "registry-cache-leader").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-cache-agent").Exists()).To(BeFalse())
		})
	})
})
