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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const registryPKIValues = `
cache:
  enabled: false
internal:
  takeover:
    phase: "New"
  cache:
    enabled: false
  pki:
    hash: "deadbeef"
    httpSecret: HTTP_SECRET
    ca:
      cert: CA_CERT
      key: CA_KEY
    token:
      cert: TOKEN_CERT
      key: TOKEN_KEY
    agent:
      cert: AGENT_CERT
      key: AGENT_KEY
    distribution:
      cert: DIST_CERT
      key: DIST_KEY
    auth:
      cert: AUTH_CERT
      key: AUTH_KEY
    users:
      - name: ro
        password: ro-pw
        passwordHash: ro-hash
        role: ReadOnly
      - name: rw
        password: rw-pw
        passwordHash: rw-hash
        role: ReadWrite
`

const registryPKIEmptyValues = `
cache:
  enabled: false
internal:
  takeover:
    phase: "New"
  cache:
    enabled: false
  pki: {}
`

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

var _ = Describe("Module :: registry :: helm template :: pki", func() {
	f := SetupHelmConfig(``)

	Context("with PKI values present", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", registryPKIValues)
			f.HelmRender()
		})

		It("renders the registry-module-pki persistent store", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-module-pki")
			Expect(s.Exists()).To(BeTrue())
			Expect(s.Field("type").String()).To(Equal("registry/pki"))
			Expect(s.Field(`data.ca\.crt`).String()).To(Equal(b64("CA_CERT")))
			Expect(s.Field(`data.agent\.key`).String()).To(Equal(b64("AGENT_KEY")))
			Expect(s.Field(`data.distribution\.crt`).String()).To(Equal(b64("DIST_CERT")))
			Expect(s.Field("data.users\\.json").String()).NotTo(BeEmpty())
		})

		It("renders registry-agent-pki with ca.crt/tls.crt/tls.key from the agent leaf", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-agent-pki")
			Expect(s.Exists()).To(BeTrue())
			Expect(s.Field("type").String()).To(Equal("kubernetes.io/tls"))
			Expect(s.Field(`data.ca\.crt`).String()).To(Equal(b64("CA_CERT")))
			Expect(s.Field(`data.tls\.crt`).String()).To(Equal(b64("AGENT_CERT")))
			Expect(s.Field(`data.tls\.key`).String()).To(Equal(b64("AGENT_KEY")))
		})

		It("renders registry-agent-users users.yaml with bcrypt hashes and roles", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-agent-users")
			Expect(s.Exists()).To(BeTrue())
			// The template uses stringData (plaintext), so the harness exposes
			// the value under stringData, NOT data. No base64 decode needed.
			users := s.Field("stringData.users\\.yaml").String()
			Expect(users).To(ContainSubstring(`name: "ro"`))
			Expect(users).To(ContainSubstring(`passwordHash: "ro-hash"`))
			Expect(users).To(ContainSubstring(`role: "ReadOnly"`))
			Expect(users).To(ContainSubstring(`role: "ReadWrite"`))
			// Plaintext passwords must NOT leak into the agent users secret.
			Expect(users).ShouldNot(ContainSubstring("ro-pw"))
			Expect(users).ShouldNot(ContainSubstring("rw-pw"))
		})
	})

	Context("with empty PKI values (hook not yet run)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", registryPKIEmptyValues)
			f.HelmRender()
		})

		It("renders none of the PKI secrets", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-module-pki").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-agent-pki").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-agent-users").Exists()).To(BeFalse())
		})
	})
})
