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

var _ = Describe("Module :: node-manager :: helm template :: master controller tolerations", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
		f.ValuesSet("nodeManager.internal.capiControllerManagerEnabled", true)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
	})

	It("tolerates unreachable masters for 60 seconds only", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for _, name := range []string{"node-controller", "capi-controller-manager", "machine-controller-manager"} {
			deployment := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", name)
			Expect(deployment.Exists()).To(BeTrue(), name)

			var keys []string
			var unreachable []string
			for _, toleration := range deployment.Field("spec.template.spec.tolerations").Array() {
				key := toleration.Get("key").String()
				keys = append(keys, key)
				if key == "node.kubernetes.io/unreachable" {
					unreachable = append(unreachable, toleration.Raw)
				}
			}

			// etcd-arbiter comes only from the helm_lib "any-node" base, which also renders
			// customTolerationKeys. HelmRender resets global.modules.placement, so they can't be set here.
			Expect(keys).To(ContainElements(
				"node-role.kubernetes.io/control-plane",
				"node.deckhouse.io/etcd-arbiter",
				"node.deckhouse.io/uninitialized",
				"node.deckhouse.io/csi-not-bootstrapped",
				"node.kubernetes.io/not-ready",
			), name)
			for _, unneeded := range []string{"node.kubernetes.io/out-of-disk", "ToBeDeletedTaint"} {
				Expect(keys).NotTo(ContainElement(unneeded), name)
			}

			Expect(unreachable).To(HaveLen(1), name)
			Expect(unreachable[0]).To(MatchJSON(`{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":60}`), name)
		}
	})
})
