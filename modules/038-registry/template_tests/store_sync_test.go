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

// storeSyncBase holds the minimum registry values needed for the store-sync
// Job to render: cache.enabled, Local orchestrator mode, full PKI (with both
// ReadOnly and ReadWrite users), and phase Legacy.
const storeSyncBase = `
cache:
  enabled: true
internal:
  takeover:
    phase: "Legacy"
  cache:
    enabled: true
    upstream:
      scheme: HTTPS
      host: registry.example.com
      path: /deckhouse/ee
      username: u
      password: p
      hasCA: false
      ttl: 168h
  orchestrator:
    ready: true
    hash: "abc"
    state:
      mode: Local
      target_mode: Local
      registry_service: node-services
      node_services:
        run: false
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

var _ = Describe("Module :: registry :: helm template :: store-sync Job", func() {
	f := SetupHelmConfig(``)

	Context("cache.enabled + mode Local + phase Legacy → Job and Secret render", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", storeSyncBase)
			f.HelmRender()
		})

		It("renders without error", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("renders the store-sync Secret", func() {
			s := f.KubernetesResource("Secret", "d8-system", "registry-cache-store-sync")
			Expect(s.Exists()).To(BeTrue())
		})

		It("renders the store-sync Job", func() {
			j := f.KubernetesResource("Job", "d8-system", "registry-cache-store-sync")
			Expect(j.Exists()).To(BeTrue())
		})

		It("Job uses the mirrorer image", func() {
			j := f.KubernetesResource("Job", "d8-system", "registry-cache-store-sync")
			img := j.Field("spec.template.spec.containers.0.image").String()
			Expect(img).To(ContainSubstring("mirrorer"))
		})

		It("Job command is positional-arg (no --once flag)", func() {
			j := f.KubernetesResource("Job", "d8-system", "registry-cache-store-sync")
			cmd0 := j.Field("spec.template.spec.containers.0.command.0").String()
			cmd1 := j.Field("spec.template.spec.containers.0.command.1").String()
			Expect(cmd0).To(Equal("/mirrorer"))
			Expect(cmd1).To(Equal("/config/config.yaml"))
		})

		It("config secret has once:true, correct local and remote addresses", func() {
			s := f.KubernetesResource("Secret", "d8-system", "registry-cache-store-sync")
			cfg := s.Field("stringData.config\\.yaml").String()
			Expect(cfg).To(ContainSubstring("once: true"))
			Expect(cfg).To(ContainSubstring("local: registry-cache-leader.d8-system.svc:5001"))
			Expect(cfg).To(ContainSubstring("registry.d8-system.svc:5001"))
		})

		It("config secret maps ReadOnly user -> puller, ReadWrite user -> pusher", func() {
			s := f.KubernetesResource("Secret", "d8-system", "registry-cache-store-sync")
			cfg := s.Field("stringData.config\\.yaml").String()
			// puller reads the legacy registry (ReadOnly), pusher writes the cache (ReadWrite).
			Expect(cfg).To(MatchRegexp(`(?s)puller:.*name:\s*ro`))
			Expect(cfg).To(MatchRegexp(`(?s)pusher:.*name:\s*rw`))
		})

		It("config secret has CA file path reference", func() {
			s := f.KubernetesResource("Secret", "d8-system", "registry-cache-store-sync")
			cfg := s.Field("stringData.config\\.yaml").String()
			Expect(cfg).To(ContainSubstring("ca: /ca/ca.crt"))
		})

		It("Job has helm.sh/resource-policy: keep annotation", func() {
			j := f.KubernetesResource("Job", "d8-system", "registry-cache-store-sync")
			Expect(j.Field("metadata.annotations.helm\\.sh/resource-policy").String()).To(Equal("keep"))
		})
	})

	Context("cache.enabled + mode Proxy → Job does NOT render", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", storeSyncBase)
			f.ValuesSet("registry.internal.orchestrator.state.mode", "Proxy")
			f.HelmRender()
		})

		It("renders without error", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("does NOT render the store-sync Job for Proxy mode", func() {
			Expect(f.KubernetesResource("Job", "d8-system", "registry-cache-store-sync").Exists()).To(BeFalse())
		})

		It("does NOT render the store-sync Secret for Proxy mode", func() {
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-cache-store-sync").Exists()).To(BeFalse())
		})
	})

	Context("cache.enabled + mode Local + phase New → Job does NOT render", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", storeSyncBase)
			f.ValuesSet("registry.internal.takeover.phase", "New")
			f.HelmRender()
		})

		It("renders without error", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("does NOT render the store-sync Job for phase New", func() {
			Expect(f.KubernetesResource("Job", "d8-system", "registry-cache-store-sync").Exists()).To(BeFalse())
		})

		It("does NOT render the store-sync Secret for phase New", func() {
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-cache-store-sync").Exists()).To(BeFalse())
		})
	})

	Context("cache.enabled + mode Local + phase CleanupPending → Job does NOT render", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", storeSyncBase)
			f.ValuesSet("registry.internal.takeover.phase", "CleanupPending")
			f.HelmRender()
		})

		It("renders without error", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		// CleanupPending = new arch in control, legacy registry (the source) gone.
		It("does NOT render the store-sync Job for phase CleanupPending", func() {
			Expect(f.KubernetesResource("Job", "d8-system", "registry-cache-store-sync").Exists()).To(BeFalse())
		})
	})
})
