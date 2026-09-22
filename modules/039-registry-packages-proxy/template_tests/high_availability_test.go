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

var _ = Describe("Module :: registry-packages-proxy :: helm template :: high availability", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValuesBootstrapped)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.discovery.clusterMasterCount", 3)
		f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
	})

	assertRenderedState := func(replicas int64, haEnabled bool) {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		deployment := f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "registry-packages-proxy")
		Expect(deployment.Exists()).To(BeTrue())
		Expect(deployment.Field("spec.replicas").Int()).To(BeEquivalentTo(replicas))
		Expect(deployment.Field("spec.template.spec.affinity.podAntiAffinity").Exists()).To(Equal(haEnabled))

		pdb := f.KubernetesResource("PodDisruptionBudget", "d8-cloud-instance-manager", "registry-packages-proxy")
		Expect(pdb.Exists()).To(BeTrue())
		Expect(pdb.Field("spec.maxUnavailable").Int()).To(BeEquivalentTo(1))
	}

	Context("With globally disabled high availability", func() {
		BeforeEach(func() {
			f.ValuesSet("global.highAvailability", false)
		})

		It("Should render one replica and allow its eviction by default", func() {
			f.HelmRender()

			assertRenderedState(1, false)
		})

		It("Should enable high availability for this module", func() {
			f.ValuesSet("registryPackagesProxy.highAvailability", true)
			f.HelmRender()

			assertRenderedState(3, true)
		})
	})

	Context("With globally enabled high availability", func() {
		BeforeEach(func() {
			f.ValuesSet("global.highAvailability", true)
		})

		It("Should inherit high availability by default", func() {
			f.HelmRender()

			assertRenderedState(3, true)
		})

		It("Should disable high availability for this module", func() {
			f.ValuesSet("registryPackagesProxy.highAvailability", false)
			f.HelmRender()

			assertRenderedState(1, false)
		})
	})
})
