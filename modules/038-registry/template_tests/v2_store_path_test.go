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
	"gopkg.in/yaml.v3"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

// Where the store keeps its blobs on a node.
//
// Three things have to name the same directory and they are rendered in three different
// places: the store's own hostPath, the path the controller is told to report so an
// operator knows which directory to reclaim, and the one the node agent measures. The
// hook decides it once — a node whose root filesystem is read-only has nowhere to put
// blobs under /opt — and what is asserted here is that all three follow it.
var _ = Describe("Module :: registry :: helm template :: v2 store path", func() {
	f := SetupHelmConfig(``)

	renderWith := func(storePath string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		if storePath != "" {
			f.ValuesSet("registry.internal.v2.storePath", storePath)
		}
		f.HelmRender()
	}

	dataHostPath := func() string {
		sts := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
		Expect(sts.Exists()).To(BeTrue())

		var spec map[string]any
		Expect(yaml.Unmarshal([]byte(sts.Field("spec.template.spec").String()), &spec)).To(Succeed())
		for _, raw := range spec["volumes"].([]any) {
			volume := raw.(map[string]any)
			if volume["name"] == "data" {
				return volume["hostPath"].(map[string]any)["path"].(string)
			}
		}
		Fail("the store has no data volume")
		return ""
	}

	controllerArgs := func() []any {
		deployment := f.KubernetesResource("Deployment", "d8-system", "registry-controller")
		Expect(deployment.Exists()).To(BeTrue())

		var containers []any
		Expect(yaml.Unmarshal(
			[]byte(deployment.Field("spec.template.spec.containers").String()), &containers)).To(Succeed())
		return containers[0].(map[string]any)["args"].([]any)
	}

	agentArgs := func() []any {
		request := f.KubernetesGlobalResource("NodeStaticPodRequest", "registry-agent")
		Expect(request.Exists()).To(BeTrue())

		var pod map[string]any
		Expect(yaml.Unmarshal([]byte(request.Field("spec.manifest").String()), &pod)).To(Succeed())
		containers := pod["spec"].(map[string]any)["containers"].([]any)
		return containers[0].(map[string]any)["args"].([]any)
	}

	Context("a cluster whose masters bashible configures", func() {
		BeforeEach(func() { renderWith(constant.StorePath) })

		It("keeps the blobs where the previous implementation left them", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// Not merely a default: the store adopts what is already in
			// StorePath/local_data instead of refetching every image in the cluster.
			Expect(dataHostPath()).To(Equal(constant.StorePath + "/local_data"))
			Expect(controllerArgs()).To(ContainElement("--store-path=" + constant.StorePath))
		})
	})

	Context("a cluster whose masters have a read-only root filesystem", func() {
		BeforeEach(func() { renderWith(constant.StorePathImmutable) })

		It("moves all three off /opt together", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(dataHostPath()).To(Equal(constant.StorePathImmutable + "/local_data"))
			// Reported, so that an operator reclaiming the disk is told the directory
			// that actually holds it rather than one that cannot exist on this node.
			Expect(controllerArgs()).To(ContainElement("--store-path=" + constant.StorePathImmutable))
			// Measured by the agent, which reports it as stale data when the cache is off.
			Expect(agentArgs()).To(ContainElement("--store-path=" + constant.StorePathImmutable))
		})
	})

	// A values tree written before the hook that decides this has run. The store keeps the
	// path every existing cluster already uses, which is the answer that breaks nothing.
	Context("the hook has not decided yet", func() {
		BeforeEach(func() { renderWith("") })

		It("falls back to the path of a bashible node", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(dataHostPath()).To(Equal(constant.StorePath + "/local_data"))
			Expect(controllerArgs()).To(ContainElement("--store-path=" + constant.StorePath))
		})
	})
})
