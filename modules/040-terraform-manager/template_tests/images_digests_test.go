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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const imagesDigestsFile = "/images-digests/images_digests.json"

var _ = Describe("Module :: terraform-manager :: helm template :: images digests", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("terraformManager.autoConvergerEnabled", true)
		f.ValuesSet("terraformManager.autoConvergerPeriod", "1h")
		f.HelmRender()
	})

	// The dhctl baked into the terraform-manager images embeds no real digests, so every
	// container that runs it must read them from the mounted ConfigMap.
	It("Every dhctl container reads images digests from the ConfigMap", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		dhctlContainers := 0
		for _, name := range []string{"terraform-auto-converger", "terraform-state-exporter"} {
			deployment := f.KubernetesResource("Deployment", "d8-system", name)
			Expect(deployment.Exists()).To(BeTrue(), name)

			podSpec := deployment.Field("spec.template.spec")
			containers := append(podSpec.Get("initContainers").Array(), podSpec.Get("containers").Array()...)
			for _, container := range containers {
				if !strings.Contains(container.Get("image").String(), "terraformManager") {
					continue
				}
				dhctlContainers++

				containerName := name + "/" + container.Get("name").String()
				Expect(container.Get(`env.#(name=="DHCTL_IMAGES_DIGESTS_FILE").value`).String()).To(Equal(imagesDigestsFile), containerName)
				Expect(container.Get(`volumeMounts.#(name=="images-digests").mountPath`).String()).To(Equal("/images-digests"), containerName)
			}
		}
		Expect(dhctlContainers).To(Equal(3))
	})
})
