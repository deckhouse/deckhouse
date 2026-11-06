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

// d8-deckhouse-version-info tells the documentation module and the node controller which release
// channel, version and edition the cluster runs. Both parse data.json as JSON.

var _ = Describe("Module :: deckhouse :: version info ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.deckhouseVersion", "v1.77.0")
		f.ValuesSet("global.deckhouseEdition", "EE")
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
	})

	versionInfo := func() string {
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		configMap := f.KubernetesResource("ConfigMap", "d8-system", "d8-deckhouse-version-info")
		Expect(configMap.Exists()).To(BeTrue())
		return configMap.Field(`data.data\.json`).String()
	}

	It("publishes the release channel, the version and the edition", func() {
		f.ValuesSet("deckhouse.releaseChannel", "Stable")

		Expect(versionInfo()).To(MatchJSON(`{"channel":"Stable","version":"v1.77.0","edition":"EE"}`))
	})

	It("reports the release channel as unknown while none is set", func() {
		Expect(versionInfo()).To(MatchJSON(`{"channel":"unknown","version":"v1.77.0","edition":"EE"}`))
	})
})
