/*
Copyright 2025 Flant JSC

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

// The d8hostnetwork constraint treats `network.hostNetwork.allowedValue` as an exact
// match against the pod spec, not as an upper bound: granting `true` to a workload that
// runs with `hostNetwork: false` is a deny. The CNI SecurityPolicyException must
// therefore follow the DaemonSet and grant hostNetwork only when ambient is on.
var _ = Describe("Module :: istio :: helm template :: cni SecurityPolicyException", func() {
	f := SetupHelmConfig(``)

	const cniGlobalValues = globalValues + `
  apiVersions:
    - deckhouse.io/v1alpha1/SecurityPolicyException
`

	assertRendered := func(ambientEnabled bool) {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", cniGlobalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.ambient.enabled", ambientEnabled)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})
	}

	Context("ambient disabled", func() {
		assertRendered(false)

		It("grants no hostNetwork and no ambient-only hostPaths", func() {
			ds := f.KubernetesResource("DaemonSet", "d8-istio", "istio-cni-node")
			Expect(ds.Exists()).To(BeTrue())
			Expect(ds.Field("spec.template.spec.hostNetwork").Exists()).To(BeFalse())

			spe := f.KubernetesResource("SecurityPolicyException", "d8-istio", "istio-cni-node")
			Expect(spe.Exists()).To(BeTrue())
			Expect(spe.Field("spec.network").Exists()).To(BeFalse())

			paths := spe.Field("spec.volumes.hostPath.allowedValues").Array()
			Expect(paths).ToNot(BeEmpty())
			for _, p := range paths {
				Expect(p.Get("path").String()).ToNot(BeElementOf("/proc", "/var/run/ztunnel"))
			}
		})
	})

	Context("ambient enabled", func() {
		assertRendered(true)

		It("grants hostNetwork and the ambient-only hostPaths", func() {
			ds := f.KubernetesResource("DaemonSet", "d8-istio", "istio-cni-node")
			Expect(ds.Exists()).To(BeTrue())
			Expect(ds.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())

			spe := f.KubernetesResource("SecurityPolicyException", "d8-istio", "istio-cni-node")
			Expect(spe.Exists()).To(BeTrue())
			Expect(spe.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())

			var got []string
			for _, p := range spe.Field("spec.volumes.hostPath.allowedValues").Array() {
				got = append(got, p.Get("path").String())
			}
			Expect(got).To(ContainElements("/proc", "/var/run/ztunnel"))
		})
	})
})
