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
	sigs_yaml "sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// joiningSeedValues provides the minimum registry module values needed for the
// joining-node seed tests: PKI CA cert + takeover phase.
const joiningSeedBaseValues = `
cache:
  enabled: false
internal:
  cache:
    enabled: false
    upstream:
      scheme: HTTPS
      host: registry.example.com
      path: /deckhouse/ee
      username: u
      password: p
      hasCA: false
  pki:
    hash: "h"
    httpSecret: HTTP_SECRET
    ca:
      cert: "TEST_CA_CERT\n"
      key: CA_KEY
    token: {cert: TOKEN_CERT, key: TOKEN_KEY}
    agent: {cert: AGENT_CERT, key: AGENT_KEY}
    distribution: {cert: DIST_CERT, key: DIST_KEY}
    auth: {cert: AUTH_CERT, key: AUTH_KEY}
    users:
      - {name: ro, password: ro-pass, passwordHash: ro-hash, role: ReadOnly}
`

var _ = Describe("Module :: registry :: helm template :: joining-node seed", func() {
	f := SetupHelmConfig(``)

	Context("phase New + cache enabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", joiningSeedBaseValues)
			f.ValuesSet("registry.internal.takeover.phase", "New")
			f.ValuesSet("registry.cache.enabled", true)
			f.HelmRender()
		})

		It("renders the registry-bashible-config secret", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-bashible-config")
			Expect(s.Exists()).To(BeTrue())
		})

		It("config decodes to valid YAML with registryModuleEnable=true and correct imagesBase", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-bashible-config")
			Expect(s.Exists()).To(BeTrue())

			raw, err := base64.StdEncoding.DecodeString(s.Field("data.config").String())
			Expect(err).ShouldNot(HaveOccurred())

			var ctx map[string]interface{}
			Expect(sigs_yaml.Unmarshal(raw, &ctx)).ShouldNot(HaveOccurred())

			Expect(ctx["registryModuleEnable"]).To(BeTrue())
			Expect(ctx["imagesBase"]).To(Equal("registry.d8-system.svc:5001/system/deckhouse"))
			// mode + version are required by bashible.Config.Validate(); the
			// node-manager StateController rejects the seed without them.
			Expect(ctx["mode"]).NotTo(BeEmpty(), "mode required by bashible.Config.Validate()")
			Expect(ctx["version"]).NotTo(BeEmpty(), "version required by bashible.Config.Validate()")
		})

		It("config has 2 mirrors (127.0.0.1 + cache) with scheme https and non-empty ca", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-bashible-config")
			Expect(s.Exists()).To(BeTrue())

			raw, err := base64.StdEncoding.DecodeString(s.Field("data.config").String())
			Expect(err).ShouldNot(HaveOccurred())

			var ctx map[string]interface{}
			Expect(sigs_yaml.Unmarshal(raw, &ctx)).ShouldNot(HaveOccurred())

			hosts, ok := ctx["hosts"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "hosts should be a map")

			hostEntry, ok := hosts["registry.d8-system.svc:5001"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "hosts[registry.d8-system.svc:5001] should be a map")

			mirrors, ok := hostEntry["mirrors"].([]interface{})
			Expect(ok).To(BeTrue(), "mirrors should be a list")
			Expect(mirrors).To(HaveLen(2))

			var mirrorHosts []string
			for _, m := range mirrors {
				mmap := m.(map[string]interface{})
				mirrorHosts = append(mirrorHosts, mmap["host"].(string))
				Expect(mmap["scheme"]).To(Equal("https"))
				Expect(mmap["ca"]).NotTo(BeEmpty())
			}
			Expect(mirrorHosts).To(ConsistOf("127.0.0.1:5001", "registry-cache.d8-system.svc:5001"))
		})
	})

	Context("phase New + cache disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", joiningSeedBaseValues)
			f.ValuesSet("registry.internal.takeover.phase", "New")
			f.ValuesSet("registry.cache.enabled", false)
			f.HelmRender()
		})

		It("renders registry-bashible-config with only 1 mirror (127.0.0.1:5001)", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			s := f.KubernetesResource("Secret", "d8-system", "registry-bashible-config")
			Expect(s.Exists()).To(BeTrue())

			raw, err := base64.StdEncoding.DecodeString(s.Field("data.config").String())
			Expect(err).ShouldNot(HaveOccurred())

			var ctx map[string]interface{}
			Expect(sigs_yaml.Unmarshal(raw, &ctx)).ShouldNot(HaveOccurred())

			hosts := ctx["hosts"].(map[string]interface{})
			hostEntry := hosts["registry.d8-system.svc:5001"].(map[string]interface{})
			mirrors := hostEntry["mirrors"].([]interface{})
			Expect(mirrors).To(HaveLen(1))

			m0 := mirrors[0].(map[string]interface{})
			Expect(m0["host"]).To(Equal("127.0.0.1:5001"))
			Expect(m0["scheme"]).To(Equal("https"))
		})
	})

	Context("phase Legacy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", joiningSeedBaseValues)
			f.ValuesSet("registry.internal.takeover.phase", "Legacy")
			f.HelmRender()
		})

		It("does not render registry-bashible-config (new writer is gated off in legacy phase)", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-bashible-config").Exists()).To(BeFalse())
		})
	})
})
