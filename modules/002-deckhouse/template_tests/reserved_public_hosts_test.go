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

const (
	reservedHostsConfigMapName = "d8-reserved-public-hosts"
	reservedHostsIngressPolicy = "reserved-public-hosts-ingress.deckhouse.io"

	validatingAdmissionPolicyAPI        = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"
	validatingAdmissionPolicyBindingAPI = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding"
)

var _ = Describe("Module :: deckhouse :: reserved public hosts ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	It("does not render admission policies or the ConfigMap", func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		f.ValuesSetFromYaml("deckhouse.reservedPublicHosts", `{mode: Template}`)
		f.HelmRender(WithAPIVersions(validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI))

		Expect(f.RenderError).ShouldNot(HaveOccurred())
		Expect(f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName).Exists()).To(BeFalse())
		Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy).Exists()).To(BeFalse())
		Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsIngressPolicy).Exists()).To(BeFalse())
	})
})
