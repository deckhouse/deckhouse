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

const takeoverModuleValues = `
cache: {}
internal: {}
`

const takeoverProxyMigrationValues = `
cache:
  enabled: true
internal:
  cache:
    enabled: true
    upstream:
      scheme: HTTPS
      host: registry.example.com
      path: /deckhouse/ee
      username: u
      password: p
      hasCA: false
  pki:
    hash: "h"
    httpSecret: HS
    ca: {cert: CA, key: K}
    token: {cert: C, key: K}
    agent: {cert: C, key: K}
    distribution: {cert: C, key: K}
    auth: {cert: C, key: K}
    users:
      - {name: ro, password: p, passwordHash: h, role: ReadOnly}
  orchestrator:
    ready: true
    hash: "x"
    state:
      mode: Proxy
      target_mode: Proxy
      registry_service: node-services
      node_services:
        run: true
`

const takeoverNewStackValues = `
cache:
  enabled: false
internal:
  pki:
    hash: "h"
    httpSecret: HS
    ca: {cert: CA, key: K}
    token: {cert: C, key: K}
    agent: {cert: C, key: K}
    distribution: {cert: C, key: K}
    auth: {cert: C, key: K}
    users:
      - {name: ro, password: p, passwordHash: h, role: ReadOnly}
`

var _ = Describe("Module :: registry :: helm template :: takeover phase", func() {
	f := SetupHelmConfig(``)

	Context("phase New (fresh install)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", takeoverModuleValues)
			f.ValuesSet("registry.internal.takeover.phase", "New")
			f.HelmRender()
		})

		It("renders without error", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("renders the takeover secret with phase New", func() {
			s := f.KubernetesResource("Secret", "d8-system", "registry-takeover")
			Expect(s.Exists()).To(BeTrue())
			Expect(s.Field("stringData.phase").String()).To(Equal("New"))
		})

		It("does not render the legacy registry-state secret", func() {
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-state").Exists()).To(BeFalse())
		})
	})

	Context("phase Legacy (upgraded cluster)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", takeoverModuleValues)
			f.ValuesSet("registry.internal.takeover.phase", "Legacy")
			f.HelmRender()
		})

		It("renders the takeover secret with phase Legacy", func() {
			s := f.KubernetesResource("Secret", "d8-system", "registry-takeover")
			Expect(s.Exists()).To(BeTrue())
			Expect(s.Field("stringData.phase").String()).To(Equal("Legacy"))
		})
	})

	Context("phase TakingOver renders the new agent stack + seed, not the legacy bashible config", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", takeoverNewStackValues)
			f.ValuesSet("registry.internal.takeover.phase", "TakingOver")
			f.HelmRender()
		})
		It("renders the agent DaemonSet and the seed registry-bashible-config", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("DaemonSet", "d8-system", "registry-agent").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Service", "d8-system", "registry").Exists()).To(BeTrue())
			// The seed config carries the agent mirror.
			s := f.KubernetesResource("Secret", "d8-system", "registry-bashible-config")
			Expect(s.Exists()).To(BeTrue())
		})
	})

	Context("phase Legacy does NOT render the agent DaemonSet", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", takeoverNewStackValues)
			f.ValuesSet("registry.internal.takeover.phase", "Legacy")
			f.HelmRender()
		})
		It("agent DaemonSet absent in Legacy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("DaemonSet", "d8-system", "registry-agent").Exists()).To(BeFalse())
		})
	})

	Context("TakingOver with a live orchestrator (Proxy migration): agent service wins, legacy node-level gone, no collision", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", takeoverProxyMigrationValues)
			f.ValuesSet("registry.internal.takeover.phase", "TakingOver")
			f.HelmRender()
		})
		It("renders exactly one registry Service (the agent's) and no nodeservices DaemonSet", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			svc := f.KubernetesResource("Service", "d8-system", "registry")
			Expect(svc.Exists()).To(BeTrue())
			Expect(svc.Field("spec.selector.app").String()).To(Equal("registry-agent"))
			// nodeservices DaemonSet must be gone during TakingOver
			Expect(f.KubernetesResource("DaemonSet", "d8-system", "registry-nodeservices-manager").Exists()).To(BeFalse())
		})
	})

	Context("pure Legacy with a live orchestrator: legacy registry Service + nodeservices render", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", takeoverProxyMigrationValues)
			f.ValuesSet("registry.internal.takeover.phase", "Legacy")
			f.HelmRender()
		})
		It("renders the legacy registry Service (orchestrator) and the nodeservices DaemonSet", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Service", "d8-system", "registry").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("DaemonSet", "d8-system", "registry-nodeservices-manager").Exists()).To(BeTrue())
		})
	})

})
