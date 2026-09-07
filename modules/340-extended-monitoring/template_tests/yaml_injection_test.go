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

// image-availability-exporter arguments are built from free-form ModuleConfig strings.
// Every argument must be rendered as a single quoted scalar, so a value carrying a
// newline and a "---" separator cannot add documents to the module release.
var _ = Describe("Module :: extendedMonitoring :: helm template :: yaml injection", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSetFromYaml("global.clusterConfiguration", `
apiVersion: deckhouse.io/v1
cloud:
  prefix: dev
  provider: OpenStack
clusterDomain: cluster.local
clusterType: Cloud
defaultCRI: Containerd
kind: ClusterConfiguration
kubernetesVersion: "1.31"
podSubnetCIDR: 10.111.0.0/16
podSubnetNodeCIDRPrefix: "24"
serviceSubnetCIDR: 10.222.0.0/16
`)
		hec.ValuesSet("extendedMonitoring.imageAvailability.exporterEnabled", true)
		hec.ValuesSetFromYaml("extendedMonitoring.imageAvailability.registry.tlsConfig", `{}`)
		hec.ValuesSetFromYaml("extendedMonitoring.certificates", `{}`)
		hec.ValuesSetFromYaml("extendedMonitoring.events", `{}`)
		hec.ValuesSetFromYaml("extendedMonitoring.imageAvailability", `
exporterEnabled: true
registry:
  tlsConfig: {}
imageCheckInterval: |-
  30s
  ---
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: pwned-check-interval
    namespace: d8-monitoring
defaultRegistry: |-
  index.docker.io
  ---
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: pwned-default-registry
    namespace: d8-monitoring
ignoredImages:
- |-
  evil'
  ---
  apiVersion: v1
  kind: ConfigMap
  metadata:
    name: pwned-ignored-images
    namespace: d8-monitoring
mirrors:
- original: |-
    evil'
    ---
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: pwned-mirror
      namespace: d8-monitoring
  mirror: mirror.example.com
`)
		hec.HelmRender()
	})

	It("Must render and must not produce injected documents", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())
		Expect(hec.KubernetesResource("Deployment", "d8-monitoring", "image-availability-exporter").Exists()).To(BeTrue())

		for _, name := range []string{
			"pwned-check-interval",
			"pwned-default-registry",
			"pwned-ignored-images",
			"pwned-mirror",
		} {
			Expect(hec.KubernetesResource("ConfigMap", "d8-monitoring", name).Exists()).To(BeFalse(), name)
		}
	})
})
