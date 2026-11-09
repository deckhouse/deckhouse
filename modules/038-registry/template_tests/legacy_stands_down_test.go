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

// Nothing of the previous implementation renders, on either side of the switch gate.
//
// Its templates are gone, and these names are what came out of them. On an upgraded cluster the
// leftovers are removed by Helm or, for the objects the previous release kept, by hooks/v2/cleanup.go
// and the controller's legacy cleanup. A template that rendered one again would fight that cleanup on
// every pass.
var _ = Describe("Module :: registry :: helm template :: the previous implementation renders nothing", func() {
	type legacyObject struct {
		kind      string
		namespace string
		name      string
	}

	// Deliberately not `Service/registry` or `registry-push`: the current implementation creates
	// both, and their ownership is asserted in v2_controller_test.go.
	legacyObjects := []legacyObject{
		{"Secret", "d8-system", "registry-state"},
		{"Secret", "d8-system", "registry-pki"},
		{"Secret", "d8-system", "registry-user-ro"},
		{"Secret", "d8-system", "registry-user-rw"},
		{"Secret", "d8-system", "registry-checker-state"},
		{"ServiceAccount", "d8-system", "registry-nodeservices"},
		{"Role", "d8-system", "registry:nodeservices"},
		{"RoleBinding", "d8-system", "registry:nodeservices"},
		{"DaemonSet", "d8-system", "registry-nodeservices-manager"},
		{"Secret", "d8-system", "registry-node-config-master-0"},
		{"Deployment", "d8-system", "registry-incluster-proxy"},
		{"Secret", "d8-system", "registry-incluster-proxy-config"},
		{"Ingress", "d8-system", "registry"},
	}

	for name, values := range map[string]string{"switch off": v2Disabled, "switch on": v2Enabled} {
		Context(name, func() {
			f := SetupHelmConfig(``)

			BeforeEach(func() {
				f.ValuesSetFromYaml("global", globalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSetFromYaml("registry", values)
				f.HelmRender()
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("deploys nothing of the previous implementation", func() {
				for _, object := range legacyObjects {
					Expect(f.KubernetesResource(object.kind, object.namespace, object.name).Exists()).
						To(BeFalse(), "%s/%s belongs to the previous implementation", object.kind, object.name)
				}

				Expect(f.KubernetesGlobalResource("ClusterRole", "d8:registry:nodeservices").Exists()).To(BeFalse())
				Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:registry:nodeservices").Exists()).To(BeFalse())
			})
		})
	}
})
