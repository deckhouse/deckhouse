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

var _ = Describe("Module :: prometheus-metrics-adapter :: admission-policy-engine compatibility", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", `
enabledModules: ["prometheus-metrics-adapter", "admission-policy-engine", "admission-policy-engine-crd"]
modules:
  placement: {}
discovery:
  prometheusScrapeInterval: 30
  clusterDomain: cluster.local
  clusterMasterCount: 1
  d8SpecificNodeCountByRole:
    master: 1
    system: 1
  extensionAPIServerAuthenticationRequestheaderClientCA: test-ca
  apiVersions:
    - deckhouse.io/v1alpha1/SecurityPolicyException
`)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("prometheusMetricsAdapter", `
internal:
  customMetrics: {}
  adapterCert:
    ca: test-ca
    crt: test-crt
    key: test-key
`)
		f.HelmRender()
	})

	It("must not render SecurityPolicyException resources or exception pod labels", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		adapterDeployment := f.KubernetesResource("Deployment", "d8-monitoring", "prometheus-metrics-adapter")
		Expect(adapterDeployment.Exists()).To(BeTrue())
		Expect(adapterDeployment.Field("spec.template.metadata.labels.security\\.deckhouse\\.io/security-policy-exception").Exists()).To(BeFalse())

		Expect(f.KubernetesResource("SecurityPolicyException", "d8-monitoring", "prometheus-metrics-adapter").Exists()).To(BeFalse())
	})
})
