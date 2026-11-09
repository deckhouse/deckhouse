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

// With the registry module off or in Unmanaged mode, node-controller arbitrates every
// RegistryUpstream, tells each its verdict and hands the accepted ones to the nodes in their
// NodeConfig; bashible-apiserver reads them for the nodes on containerd v1, which bashible
// configures inline. The nodes themselves never read one: the objects carry credentials, and
// a node has what it needs in its own NodeConfig. Nobody here writes a spec: the objects
// belong to modules and operators.
var _ = Describe("Module :: node-manager :: helm template :: RegistryUpstream rights", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
	})

	It("lets node-controller read registry upstreams and module sources and patch only the status", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		controller := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:node-controller")
		Expect(controller.Exists()).To(BeTrue())
		Expect(roleVerbs(controller, "deckhouse.io", "registryupstreams")).To(ConsistOf("get", "list", "watch"))
		Expect(roleVerbs(controller, "deckhouse.io", "registryupstreams/status")).To(ConsistOf("patch"))
		Expect(roleVerbs(controller, "deckhouse.io", "modulesources")).To(ContainElements("get", "list", "watch"))
	})

	It("gives a node no registry upstream at all", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		agent := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:node-agent")
		Expect(agent.Exists()).To(BeTrue())
		Expect(roleVerbs(agent, "deckhouse.io", "registryupstreams")).To(BeEmpty())
	})

	It("lets bashible-apiserver only read registry upstreams", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		reader := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:bashible-apiserver:nodeusers")
		Expect(reader.Exists()).To(BeTrue())
		Expect(roleVerbs(reader, "deckhouse.io", "registryupstreams")).To(ConsistOf("get", "list", "watch"))
	})
})
