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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
enabledModules: ["vertical-pod-autoscaler"]
modules:
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  kubernetesVersion: 1.15.4
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
`

const globalValuesHa = `
enabledModules: ["vertical-pod-autoscaler"]
modules:
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  clusterControlPlaneIsHighlyAvailable: true
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  kubernetesVersion: 1.15.4
  d8SpecificNodeCountByRole:
    system: 3
    master: 5
`

const globalValuesManaged = `
enabledModules: ["vertical-pod-autoscaler"]
modules:
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  kubernetesVersion: 1.15.4
  d8SpecificNodeCountByRole:
    system: 3
`

const globalValuesManagedHa = `
highAvailability: true
enabledModules: ["vertical-pod-autoscaler"]
modules:
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  clusterUUID: f49dd1c3-a63a-4565-a06c-625e35587eab
  kubernetesVersion: 1.15.4
  d8SpecificNodeCountByRole:
    system: 3
`

const dashboard = `
internal:
  auth: {}
auth: {}
https:
  mode: OnlyInURI
`

var _ = Describe("Module :: dashboard :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("dashboard", dashboard)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-dashboard")
			registrySecret := f.KubernetesResource("Secret", "d8-dashboard", "deckhouse-registry")

			metricsScraper := f.KubernetesResource("Deployment", "d8-dashboard", "metrics-scraper")
			web := f.KubernetesResource("Deployment", "d8-dashboard", "web")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())

			Expect(metricsScraper.Exists()).To(BeTrue())
			Expect(metricsScraper.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(metricsScraper.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "dashboard"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
`))
			Expect(metricsScraper.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(metricsScraper.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(metricsScraper.Field("spec.template.spec.affinity").Exists()).To(BeFalse())

			Expect(web.Exists()).To(BeTrue())
			Expect(web.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(web.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "dashboard"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
`))
			Expect(web.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(web.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(web.Field("spec.template.spec.affinity").Exists()).To(BeFalse())
		})
	})

	Context("DefaultHA", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("dashboard", dashboard)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster with ha", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-dashboard")
			registrySecret := f.KubernetesResource("Secret", "d8-dashboard", "deckhouse-registry")

			metricsScraper := f.KubernetesResource("Deployment", "d8-dashboard", "metrics-scraper")
			web := f.KubernetesResource("Deployment", "d8-dashboard", "web")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(metricsScraper.Exists()).To(BeTrue())
			Expect(metricsScraper.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(metricsScraper.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "dashboard"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
`))
			Expect(metricsScraper.Field("spec.replicas").Int()).To(BeEquivalentTo(2))
			Expect(metricsScraper.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 1
`))
			Expect(metricsScraper.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: metrics-scraper
    topologyKey: kubernetes.io/hostname
`))
			Expect(web.Exists()).To(BeTrue())
			Expect(web.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(web.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "dashboard"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
`))
			Expect(web.Field("spec.replicas").Int()).To(BeEquivalentTo(2))
			Expect(web.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 1
`))
			Expect(web.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: web
    topologyKey: kubernetes.io/hostname
`))
		})
	})

	Context("Managed", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManaged)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("dashboard", dashboard)
			f.HelmRender()
		})

		It("Everything must render properly for managed cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-dashboard")
			registrySecret := f.KubernetesResource("Secret", "d8-dashboard", "deckhouse-registry")

			metricsScraper := f.KubernetesResource("Deployment", "d8-dashboard", "metrics-scraper")
			web := f.KubernetesResource("Deployment", "d8-dashboard", "web")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(metricsScraper.Exists()).To(BeTrue())
			Expect(metricsScraper.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(metricsScraper.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "dashboard"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
`))
			Expect(metricsScraper.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(metricsScraper.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(metricsScraper.Field("spec.template.spec.affinity").Exists()).To(BeFalse())

			Expect(web.Exists()).To(BeTrue())
			Expect(web.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(web.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "dashboard"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
`))
			Expect(web.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(web.Field("spec.strategy").Exists()).To(BeFalse())
			Expect(web.Field("spec.template.spec.affinity").Exists()).To(BeFalse())
		})
	})

	Context("ManagedHa", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesManagedHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("dashboard", dashboard)
			f.HelmRender()
		})

		It("Everything must render properly for managed cluster with ha", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			namespace := f.KubernetesGlobalResource("Namespace", "d8-dashboard")
			registrySecret := f.KubernetesResource("Secret", "d8-dashboard", "deckhouse-registry")

			metricsScraper := f.KubernetesResource("Deployment", "d8-dashboard", "metrics-scraper")
			web := f.KubernetesResource("Deployment", "d8-dashboard", "web")

			Expect(namespace.Exists()).To(BeTrue())
			Expect(registrySecret.Exists()).To(BeTrue())
			Expect(metricsScraper.Exists()).To(BeTrue())
			Expect(metricsScraper.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(metricsScraper.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "dashboard"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
`))
			Expect(metricsScraper.Field("spec.replicas").Int()).To(BeEquivalentTo(2))
			Expect(metricsScraper.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 1
`))
			Expect(metricsScraper.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: metrics-scraper
    topologyKey: kubernetes.io/hostname
`))
			Expect(web.Exists()).To(BeTrue())
			Expect(web.Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.deckhouse.io/system": ""}`))
			Expect(web.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: dedicated.deckhouse.io
  operator: Equal
  value: "dashboard"
- key: dedicated.deckhouse.io
  operator: Equal
  value: "system"
- key: drbd.linbit.com/lost-quorum
- key: drbd.linbit.com/force-io-error
- key: drbd.linbit.com/ignore-fail-over
`))
			Expect(web.Field("spec.replicas").Int()).To(BeEquivalentTo(2))
			Expect(web.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 0
  maxUnavailable: 1
`))
			Expect(web.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: web
    topologyKey: kubernetes.io/hostname
`))
		})
	})
})
