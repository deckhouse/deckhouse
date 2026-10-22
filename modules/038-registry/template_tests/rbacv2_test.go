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
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// The platform contract test checks the shape of the capabilities. What it cannot
// check is which level reads what: ModuleConfig/registry, RegistryConfig,
// RegistryUpstream and RegistryStorage carry credentials inline, and the viewer
// of the subsystem reads no Secrets.

// grantedVerbs maps every resource a ClusterRole names to the verbs it gets on it.
func grantedVerbs(role object_store.KubeObject) map[string][]string {
	granted := map[string][]string{}
	role.Field("rules").ForEach(func(_, rule gjson.Result) bool {
		for _, resource := range rule.Get("resources").Array() {
			for _, verb := range rule.Get("verbs").Array() {
				granted[resource.String()] = append(granted[resource.String()], verb.String())
			}
		}
		return true
	})
	return granted
}

var readVerbs = []string{"get", "list", "watch"}

var _ = Describe("Module :: registry :: helm template :: rbacv2", func() {
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

			// The CRDs are installed either way, so the rights to them are too.
			It("gives the viewer only the per-node settings, which hold no credentials", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				view := f.KubernetesGlobalResource("ClusterRole", "d8:system-capability:registry:view")
				Expect(view.Exists()).To(BeTrue())
				Expect(view.Field(`metadata.labels.rbac\.deckhouse\.io/aggregate-to-cluster-as`).String()).To(Equal("viewer"))
				Expect(grantedVerbs(view)).To(Equal(map[string][]string{"registrynodes": readVerbs}))
			})

			It("lets the manager read the configuration and write only what the module does not own", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				edit := f.KubernetesGlobalResource("ClusterRole", "d8:system-capability:registry:edit")
				Expect(edit.Exists()).To(BeTrue())
				Expect(edit.Field(`metadata.labels.rbac\.deckhouse\.io/aggregate-to-cluster-as`).String()).To(Equal("manager"))

				granted := grantedVerbs(edit)
				// A deleted ModuleConfig falls back to the defaults and can switch the mode.
				Expect(granted["moduleconfigs"]).To(ConsistOf("get", "list", "watch", "create", "update", "patch"))
				// Rendered by the module and written by the controller.
				Expect(granted["registryconfigs"]).To(ConsistOf(readVerbs))
				Expect(granted["registrystorages"]).To(ConsistOf(readVerbs))
				Expect(granted["registryupstreams"]).To(ConsistOf(
					"get", "list", "watch", "create", "update", "patch", "delete", "deletecollection"))
				Expect(granted).To(HaveLen(4), "no status subresource and nothing outside the module")

				edit.Field("rules").ForEach(func(_, rule gjson.Result) bool {
					for _, resource := range rule.Get("resources").Array() {
						if resource.String() == "moduleconfigs" {
							Expect(rule.Get("resourceNames").String()).To(MatchJSON(`["registry"]`))
						}
					}
					return true
				})
			})
		})
	}
})
