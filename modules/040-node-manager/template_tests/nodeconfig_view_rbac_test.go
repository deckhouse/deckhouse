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

// A NodeStaticPodRequest carries counts and no node names, so "which node lags,
// and why" is answered by that node's NodeConfig. The console reads it directly.
var _ = Describe("Module :: node-manager :: helm template :: NodeConfig view role", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
	})

	It("lets a viewer read NodeConfig and never its status subresource", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		view := f.KubernetesGlobalResource("ClusterRole", "d8:manage:permission:module:node-manager:view")
		Expect(view.Exists()).To(BeTrue())

		Expect(roleVerbs(view, "internal.deckhouse.io", "nodeconfigs")).To(ConsistOf("get", "list", "watch"),
			"the view role must name internal.deckhouse.io/nodeconfigs")

		// The NodeConfig above answers "where did it land"; these two are the
		// objects that question is about, and neither was in any user-facing role.
		Expect(roleVerbs(view, "deckhouse.io", "nodestaticpodrequests")).To(ConsistOf("get", "list", "watch"))
		Expect(roleVerbs(view, "deckhouse.io", "nodeextensionrequests")).To(ConsistOf("get", "list", "watch"))

		// status.maintenanceToken is a credential for a root-equivalent config
		// push. The API masks it on the resource only, never on the subresource.
		rules := view.Field("rules").String()
		Expect(rules).NotTo(ContainSubstring("nodeconfigs/status"))
		Expect(rules).NotTo(ContainSubstring("nodeconfigs/sensitive"))
	})

	It("keeps NodeConfig out of the role that writes", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		edit := f.KubernetesGlobalResource("ClusterRole", "d8:manage:permission:module:node-manager:edit")
		Expect(edit.Exists()).To(BeTrue())

		// node-controller and the node itself write NodeConfig; a user never does.
		Expect(roleVerbs(edit, "internal.deckhouse.io", "nodeconfigs")).To(BeEmpty())

		// Creating one of these runs a privileged pod on every node of a group,
		// so the grant is the cluster owner's to make, not this role's.
		Expect(roleVerbs(edit, "deckhouse.io", "nodestaticpodrequests")).To(BeEmpty())
		Expect(roleVerbs(edit, "deckhouse.io", "nodeextensionrequests")).To(BeEmpty())
	})
})
