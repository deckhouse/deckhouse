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

const registryConfigValues = `
upstream:
  host: registry.example.com
  path: /deckhouse/ee
  scheme: HTTPS
  credentials:
    username: u
    password: p
cache:
  enabled: true
  ttl: 24h
additionalRegistries:
  - host: docker.io
    upstream:
      host: registry-1.docker.io
      scheme: HTTPS
auth:
  users:
    - name: ro-user
      role: ReadOnly
internal:
  takeover:
    phase: TakingOver
  cache:
    enabled: true
    upstream:
      scheme: HTTPS
      host: registry.example.com
      path: /deckhouse/ee
      username: u
      password: p
      hasCA: false
      ttl: 24h
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

const registryMinimalValues = `
cache: {}
internal:
  takeover:
    phase: TakingOver
`

const registryModuleSourceOrderingValues = `
upstream:
  host: up.example.com
  scheme: HTTPS
cache: {}
additionalRegistries:
  - host: extra.example.com
internal:
  takeover:
    phase: TakingOver
  moduleSourceEntries:
    - host: modules.example.com
      upstream:
        host: up.example.com
        scheme: HTTPS
`

// Derived-config fixtures (7b-2). phase TakingOver keeps isLegacy truthy so the
// New-only, pki-gated templates (e.g. agent/bashible-config-secret) stay
// short-circuited in this hook-less render — the derived branch of
// registry-config.yaml depends only on derived.present, not on the phase value.
const registryDerivedDirectValues = `
upstream:
  host: operator.example.com
  path: /operator
  scheme: HTTPS
cache:
  enabled: false
internal:
  takeover:
    phase: TakingOver
    derived:
      present: true
      upstream:
        host: legacy.example.com
        path: /deckhouse/ee
        scheme: HTTPS
        credentials:
          username: lu
          password: lp
      cache:
        enabled: true
        ttl: 24h
`

const registryDerivedAirgapValues = `
upstream:
  host: operator.example.com
  scheme: HTTPS
cache: {}
internal:
  takeover:
    phase: TakingOver
    derived:
      present: true
      cache:
        enabled: true
`

const registryDerivedAbsentValues = `
upstream:
  host: operator.example.com
  path: /operator
  scheme: HTTPS
cache:
  enabled: false
internal:
  takeover:
    phase: TakingOver
    derived:
      present: false
`

var _ = Describe("Module :: registry :: helm template :: registry-config", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryConfigValues)
		f.HelmRender()
	})

	It("renders a singleton RegistryConfig with primary + additional entries and auth", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		rc := f.KubernetesGlobalResource("RegistryConfig", "registry")
		Expect(rc.Exists()).To(BeTrue())

		Expect(rc.Field("spec.registries.0.host").String()).To(Equal("registry.d8-system.svc:5001"))
		Expect(rc.Field("spec.registries.0.source").String()).To(Equal("Primary"))
		Expect(rc.Field("spec.registries.0.upstream.host").String()).To(Equal("registry.example.com"))
		Expect(rc.Field("spec.registries.0.cache.enabled").Bool()).To(BeTrue())

		Expect(rc.Field("spec.registries.1.host").String()).To(Equal("docker.io"))
		Expect(rc.Field("spec.registries.1.source").String()).To(Equal("Additional"))
		Expect(rc.Field("spec.registries.1.upstream.host").String()).To(Equal("registry-1.docker.io"))

		Expect(rc.Field("spec.auth.users.0.name").String()).To(Equal("ro-user"))
		Expect(rc.Field("spec.auth.users.0.role").String()).To(Equal("ReadOnly"))
	})

	Context("minimal / nil-safe render (no upstream, no cache, no additionalRegistries, no auth)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", registryMinimalValues)
			f.HelmRender()
		})

		It("renders exactly one Primary entry with no upstream, no cache, and no auth", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			rc := f.KubernetesGlobalResource("RegistryConfig", "registry")
			Expect(rc.Exists()).To(BeTrue())

			// Exactly one entry.
			Expect(rc.Field("spec.registries").Array()).To(HaveLen(1))

			// Primary entry has correct host and source.
			Expect(rc.Field("spec.registries.0.host").String()).To(Equal("registry.d8-system.svc:5001"))
			Expect(rc.Field("spec.registries.0.source").String()).To(Equal("Primary"))

			// No upstream or cache emitted (nil-safety: Fix A).
			Expect(rc.Field("spec.registries.0.upstream").Exists()).To(BeFalse())
			Expect(rc.Field("spec.registries.0.cache").Exists()).To(BeFalse())

			// No second entry.
			Expect(rc.Field("spec.registries.1").Exists()).To(BeFalse())

			// No auth block.
			Expect(rc.Field("spec.auth").Exists()).To(BeFalse())
		})
	})

	Context("ModuleSource ordering (Primary → Additional → ModuleSource)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", registryModuleSourceOrderingValues)
			f.HelmRender()
		})

		It("places Primary at index 0, Additional at index 1, ModuleSource at index 2", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			rc := f.KubernetesGlobalResource("RegistryConfig", "registry")
			Expect(rc.Exists()).To(BeTrue())

			Expect(rc.Field("spec.registries.0.source").String()).To(Equal("Primary"))
			Expect(rc.Field("spec.registries.0.host").String()).To(Equal("registry.d8-system.svc:5001"))

			Expect(rc.Field("spec.registries.1.source").String()).To(Equal("Additional"))
			Expect(rc.Field("spec.registries.1.host").String()).To(Equal("extra.example.com"))

			Expect(rc.Field("spec.registries.2.source").String()).To(Equal("ModuleSource"))
			Expect(rc.Field("spec.registries.2.host").String()).To(Equal("modules.example.com"))
		})
	})

	Context("prefers derived upstream/cache when derived.present (Direct-from-legacy)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", registryDerivedDirectValues)
			f.HelmRender()
		})
		It("uses the derived legacy upstream, not the operator upstream", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			rc := f.KubernetesGlobalResource("RegistryConfig", "registry")
			Expect(rc.Field("spec.registries.0.upstream.host").String()).To(Equal("legacy.example.com"))
			Expect(rc.Field("spec.registries.0.upstream.credentials.username").String()).To(Equal("lu"))
			Expect(rc.Field("spec.registries.0.cache.enabled").Bool()).To(BeTrue())
		})
	})

	Context("derived air-gap (Local-from-legacy): no upstream, cache on", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", registryDerivedAirgapValues)
			f.HelmRender()
		})
		It("emits no upstream and cache enabled", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			rc := f.KubernetesGlobalResource("RegistryConfig", "registry")
			Expect(rc.Field("spec.registries.0.upstream").Exists()).To(BeFalse())
			Expect(rc.Field("spec.registries.0.cache.enabled").Bool()).To(BeTrue())
		})
	})

	Context("derived not present: operator config flows through (Unmanaged)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", registryDerivedAbsentValues)
			f.HelmRender()
		})
		It("uses the operator upstream", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			rc := f.KubernetesGlobalResource("RegistryConfig", "registry")
			Expect(rc.Field("spec.registries.0.upstream.host").String()).To(Equal("operator.example.com"))
		})
	})
})
