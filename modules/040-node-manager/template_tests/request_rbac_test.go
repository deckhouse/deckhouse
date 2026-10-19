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

// NodeExtensionRequest is an operator's tool and is handed out like NodeGroup.
// A NodeStaticPodRequest runs any manifest as root on the selected nodes, so
// users only read it, and only cluster administrators in user-authz.
var _ = Describe("Module :: node-manager :: helm template :: request rights", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
	})

	It("hands the requests out through the manage roles", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		view := f.KubernetesGlobalResource("ClusterRole", "d8:system-capability:node-manager:view")
		edit := f.KubernetesGlobalResource("ClusterRole", "d8:system-capability:node-manager:edit")
		Expect(view.Exists()).To(BeTrue())
		Expect(edit.Exists()).To(BeTrue())

		Expect(roleVerbs(view, "deckhouse.io", "nodeextensionrequests")).To(ConsistOf("get", "list", "watch"))
		Expect(roleVerbs(edit, "deckhouse.io", "nodeextensionrequests")).To(ConsistOf("create", "update", "patch", "delete", "deletecollection"))

		Expect(roleVerbs(view, "deckhouse.io", "nodestaticpodrequests")).To(ConsistOf("get", "list", "watch"))
		Expect(roleVerbs(edit, "deckhouse.io", "nodestaticpodrequests")).To(BeEmpty())
	})

	It("hands the requests out through the user-authz roles", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		user := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
		clusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
		clusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")
		Expect(user.Exists()).To(BeTrue())
		Expect(clusterEditor.Exists()).To(BeTrue())
		Expect(clusterAdmin.Exists()).To(BeTrue())

		Expect(roleVerbs(user, "deckhouse.io", "nodeextensionrequests")).To(ConsistOf("get", "list", "watch"))
		Expect(roleVerbs(clusterEditor, "deckhouse.io", "nodeextensionrequests")).To(ConsistOf("create", "update", "patch", "delete", "deletecollection"))
		Expect(roleVerbs(clusterAdmin, "deckhouse.io", "nodeextensionrequests")).To(ConsistOf("get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"))

		Expect(roleVerbs(user, "deckhouse.io", "nodestaticpodrequests")).To(BeEmpty())
		Expect(roleVerbs(clusterEditor, "deckhouse.io", "nodestaticpodrequests")).To(BeEmpty())
		Expect(roleVerbs(clusterAdmin, "deckhouse.io", "nodestaticpodrequests")).To(ConsistOf("get", "list", "watch"))
	})
})
