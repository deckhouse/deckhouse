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

// The basic role model follows the RBACv2 capabilities: what the cluster manager gets goes to
// ClusterAdmin, what the cluster viewer reads goes to User, except RegistryNode. User is granted to
// application developers, and a RegistryNode lays out the upstreams, mirrors and CAs of the cluster.
var _ = Describe("Module :: registry :: helm template :: user-authz", func() {
	f := SetupHelmConfig(``)

	renderWith := func(registryValues string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryValues)
		f.HelmRender()
	}

	for name, values := range map[string]string{"discriminator on": v2Enabled, "discriminator off": v2Disabled} {
		Context(name, func() {
			BeforeEach(func() { renderWith(values) })

			It("gives User only the operations and the cached images: no credentials, no registry layout", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				user := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:registry:user")
				Expect(user.Exists()).To(BeTrue())
				Expect(user.Field(`metadata.annotations.user-authz\.deckhouse\.io/access-level`).String()).To(Equal("User"))
				Expect(grantedVerbs(user)).To(Equal(map[string][]string{
					"registryoperations": readVerbs,
					// A virtual resource: no watch is served.
					"registryimages": {"get", "list"},
				}))
			})

			It("gives ClusterAdmin what the RBACv2 manager adds to the viewer, and RegistryNode", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				admin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:registry:cluster-admin")
				Expect(admin.Exists()).To(BeTrue())
				Expect(admin.Field(`metadata.annotations.user-authz\.deckhouse\.io/access-level`).String()).To(Equal("ClusterAdmin"))

				granted := grantedVerbs(admin)
				// Rendered by the module and written by the controller.
				Expect(granted["registryconfigs"]).To(ConsistOf(readVerbs))
				Expect(granted["registryconfigs/sensitive"]).To(ConsistOf(readVerbs))
				Expect(granted["registrystorages"]).To(ConsistOf(readVerbs))
				// Compiled by the controller; kept from User.
				Expect(granted["registrynodes"]).To(ConsistOf(readVerbs))
				Expect(granted["registryupstreams"]).To(ConsistOf(
					"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"))
				Expect(granted["registryupstreams/sensitive"]).To(ConsistOf(readVerbs))
				// Reading comes from User; the status is the controller's and the replicas'.
				Expect(granted["registryoperations"]).To(ConsistOf("create", "update", "patch", "delete"))
				// ClusterAdmin writes every ModuleConfig through the deckhouse module.
				Expect(granted).To(HaveLen(7), "no status subresource and nothing outside the module")
			})
		})
	}
})
