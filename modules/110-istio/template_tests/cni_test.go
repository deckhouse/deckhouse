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

// install-cni runs with a read-only root filesystem, and legacy iptables fails without a
// writable /run for its lock file /run/xtables.lock.
var _ = Describe("Module :: istio :: helm template :: cni DaemonSet", func() {
	f := SetupHelmConfig(``)

	assertWritableRun := func(ambientEnabled bool) {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.ambient.enabled", ambientEnabled)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("mounts an emptyDir at /run in install-cni", func() {
			ds := f.KubernetesResource("DaemonSet", "d8-istio", "istio-cni-node")
			Expect(ds.Exists()).To(BeTrue())

			container := ds.Field("spec.template.spec.containers.#(name==install-cni)")
			Expect(container.Get("securityContext.readOnlyRootFilesystem").Bool()).To(BeTrue())
			volumeName := container.Get("volumeMounts.#(mountPath==/run).name").String()
			Expect(volumeName).To(Equal("install-cni-run"))

			volume := ds.Field("spec.template.spec.volumes.#(name==install-cni-run)")
			Expect(volume.Get("emptyDir").Exists()).To(BeTrue())
			Expect(volume.Get("emptyDir.medium").String()).To(Equal("Memory"))
		})
	}

	Context("ambient disabled", func() {
		assertWritableRun(false)
	})

	Context("ambient enabled", func() {
		assertWritableRun(true)
	})
})
