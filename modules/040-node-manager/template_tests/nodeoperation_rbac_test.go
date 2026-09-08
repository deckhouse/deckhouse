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
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// Reboot and Drain are an operator's tool, so both RBAC systems have to hand
// NodeOperation out the way they hand out NodeGroup.
var _ = Describe("Module :: node-manager :: helm template :: NodeOperation rights", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
	})

	It("lets the manage roles read and create operations", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		view := f.KubernetesGlobalResource("ClusterRole", "d8:manage:permission:module:node-manager:view")
		edit := f.KubernetesGlobalResource("ClusterRole", "d8:manage:permission:module:node-manager:edit")
		Expect(view.Exists()).To(BeTrue())
		Expect(edit.Exists()).To(BeTrue())

		Expect(nodeOperationVerbs(view)).To(ConsistOf("get", "list", "watch"))
		// The spec is immutable, so update and patch would grant nothing.
		Expect(nodeOperationVerbs(edit)).To(ConsistOf("create", "delete"))
	})

	It("lets the user-authz roles read and create operations", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		user := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
		clusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
		clusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")
		Expect(user.Exists()).To(BeTrue())
		Expect(clusterEditor.Exists()).To(BeTrue())
		Expect(clusterAdmin.Exists()).To(BeTrue())

		Expect(nodeOperationVerbs(user)).To(ConsistOf("get", "list", "watch"))
		Expect(nodeOperationVerbs(clusterEditor)).To(ConsistOf("create", "delete"))
		Expect(nodeOperationVerbs(clusterAdmin)).To(ConsistOf("get", "list", "watch", "create", "delete"))
	})
})

func nodeOperationVerbs(role object_store.KubeObject) []string {
	return roleVerbs(role, "deckhouse.io", "nodeoperations")
}
