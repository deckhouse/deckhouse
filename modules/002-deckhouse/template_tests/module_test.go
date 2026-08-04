/*
Copyright 2021 Flant JSC

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
	"encoding/base64"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
deckhouseVersion: test
enabledModules: ["vertical-pod-autoscaler", "prometheus", "operator-prometheus"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterDomain: cluster.local
  clusterType: Static
  kubernetesVersion: "Automatic"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
discovery:
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  kubernetesVersion: "1.32.0"
  d8SpecificNodeCountByRole:
    system: 1
modules:
  placement: {}
`

	globalValues2 = `
deckhouseVersion: test
enabledModules: ["vertical-pod-autoscaler", "prometheus", "operator-prometheus", "control-plane-manager"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterDomain: cluster.local
  clusterType: Static
  kubernetesVersion: "Automatic"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
discovery:
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  kubernetesVersion: "1.32.0"
  d8SpecificNodeCountByRole:
    system: 1
modules:
  placement: {}
`

	clusterIsBootstrapped = `
clusterIsBootstrapped: true
`

	moduleValuesForMasterNode = `
bundle: Default
logLevel: Info
internal:
  namespaces:
    - test
  webhookHandlerCert:
    crt: a
    key: b
    ca: c
  admissionWebhookCert:
    crt: a
    key: b
    ca: c
  currentReleaseImageName: test
registry:
  mode: Unmanaged
`

	moduleValuesForDeckhouseNode = `
bundle: Default
logLevel: Info
nodeSelector:
  node-role.kubernetes.io/deckhouse: ""
tolerations:
- key: testkey
  operator: Exists
internal:
  namespaces:
    - test
  webhookHandlerCert:
    crt: a
    key: b
    ca: c
  admissionWebhookCert:
    crt: a
    key: b
    ca: c
  currentReleaseImageName: test
registry:
  mode: Unmanaged
`
)

var _ = Describe("Module :: deckhouse :: helm template ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	Context("Cluster with deckhouse on master node", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, chartName)
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.template.spec.nodeSelector").String()).To(MatchYAML(`node-role.kubernetes.io/control-plane: ""`))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
  - operator: Exists
`))
		})
	})

	// What the Deckhouse controller fetches over HTTP must name a registry its own client can
	// reach, which is not necessarily the one image references point at.
	//
	// `global.modulesImages.registry.base` is the address container image references are
	// rendered from, and the registry module may point it at the in-cluster registry — an
	// address only the container runtime resolves, because a node agent stands in that path.
	// Nothing stands in the path of the controller's HTTP client. Reading `base` here worked
	// only for as long as the two were always the same value, and taking them apart is what
	// lets the pull path move without the release check and the module source moving with it.
	//
	// The fixture gives them different values on purpose, so this is observable at all.
	Context("What the controller fetches itself", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues+clusterIsBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.HelmRender()
		})

		It("names the registry that client can reach, not the one images render from", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			reachable := "registry.deckhouse.io/deckhouse/fe"
			renderedFrom := "registry.example.com"

			secret := f.KubernetesResource("Secret", "d8-system", "deckhouse-registry")
			Expect(secret.Exists()).To(BeTrue())
			imagesRegistry, err := base64.StdEncoding.DecodeString(secret.Field("data.imagesRegistry").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(imagesRegistry)).To(Equal(reachable))
			Expect(string(imagesRegistry)).ToNot(ContainSubstring(renderedFrom))

			source := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(source.Exists()).To(BeTrue())
			Expect(source.Field("spec.registry.repo").String()).To(Equal(reachable + "/modules"))
			Expect(source.Field("spec.registry.repo").String()).ToNot(ContainSubstring(renderedFrom))
		})
	})

	Context("Cluster with deckhouse on system node", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, chartName)
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.template.spec.nodeSelector").String()).To(MatchYAML(`node-role.kubernetes.io/deckhouse: ""`))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: testkey
  operator: Exists
- key: node.deckhouse.io/bashible-uninitialized
  operator: Exists
  effect: NoSchedule
- key: node.deckhouse.io/uninitialized
  operator: Exists
  effect: NoSchedule
`))
		})
	})

	Context("Control-plane is managed by third-party team: use service by default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(dp.Exists()).To(BeTrue())
			serviceHostValue := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_HOST\").value",
			)
			Expect(serviceHostValue.Exists()).To(BeFalse())
			servicePort := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_PORT\").value",
			)
			Expect(servicePort.Exists()).To(BeFalse())
		})
	})

	Context("Managed by Deckhouse: use API-proxy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues2+clusterIsBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(dp.Exists()).To(BeTrue())
			serviceHostValue := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_HOST\").value",
			).String()
			Expect(serviceHostValue).To(Equal("127.0.0.1"))
			servicePort := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_PORT\").value",
			).String()
			Expect(servicePort).To(Equal("6445"))
		})
	})

	Context("Bootstrap phase: use direct connection", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues2)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(dp.Exists()).To(BeTrue())
			serviceHostValue := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_HOST\")." +
				"valueFrom.fieldRef.fieldPath",
			).String()
			Expect(serviceHostValue).To(Equal("status.hostIP"))
			servicePort := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_PORT\").value",
			).String()
			Expect(servicePort).To(Equal("6443"))
		})
	})

	Context("SecurityPolicyException for the deckhouse pod", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Pod label must point to an existing exception", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, chartName)
			speName := dp.Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).String()
			Expect(speName).NotTo(BeEmpty())
			Expect(f.KubernetesResource("SecurityPolicyException", nsName, speName).Exists()).To(BeTrue())
		})

		It("Every container must drop capabilities, otherwise the exception cannot cover it", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, chartName)
			for _, path := range []string{"spec.template.spec.containers", "spec.template.spec.initContainers"} {
				for _, container := range dp.Field(path).Array() {
					Expect(container.Get("securityContext.capabilities.drop").Array()).
						NotTo(BeEmpty(), "container %s", container.Get("name").String())
				}
			}
		})

		It("Every container port must be listed in the exception, the pod runs in the host network", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(dp.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())

			spe := f.KubernetesResource("SecurityPolicyException", nsName, chartName)
			allowedPorts := make(map[int64]bool)
			for _, port := range spe.Field("spec.network.hostPorts").Array() {
				allowedPorts[port.Get("port").Int()] = true
			}
			for _, container := range dp.Field("spec.template.spec.containers").Array() {
				for _, port := range container.Get("ports").Array() {
					Expect(allowedPorts).To(HaveKey(port.Get("containerPort").Int()))
				}
			}
		})
	})
})
