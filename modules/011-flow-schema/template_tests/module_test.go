/*
Copyright 2023 Flant JSC

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

const (
	globalValuesK8s126 = `
discovery:
  kubernetesVersion: "1.26.1"
`

	globalValuesK8s127 = `
discovery:
  kubernetesVersion: "1.27.5"
`
	globalValuesK8s128 = `
discovery:
  kubernetesVersion: "1.28.1"
`

	globalValuesK8s129 = `
discovery:
  kubernetesVersion: "1.29.1"
`

	globalValuesK8s130 = `
discovery:
  kubernetesVersion: "1.30.0"
`

	moduleEmptyValuesForFlowSchemaModule = `
internal:
  namespaces: []
`

	moduleValuesForFlowSchemaModule = `
internal:
  namespaces:
  - test1
  - test2
`
)

var _ = Describe("Module :: flow-schema :: helm template ::", func() {
	f := SetupHelmConfig(``)

	Context("Cluster without deckhouse namespaces, kubernetes 1.27", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesK8s127)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("flowSchema", moduleEmptyValuesForFlowSchemaModule)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			fs := f.KubernetesResource("FlowSchema", "", "d8-serviceaccounts")
			pl := f.KubernetesResource("PriorityLevelConfiguration", "", "d8-serviceaccounts")
			Expect(fs.Exists()).To(BeFalse())
			Expect(pl.Exists()).To(BeTrue())
		})
	})

	Context("Cluster with deckhouse namespaces, kubernetes 1.28", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesK8s128)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("flowSchema", moduleValuesForFlowSchemaModule)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			fs := f.KubernetesResource("FlowSchema", "", "d8-serviceaccounts")
			pl := f.KubernetesResource("PriorityLevelConfiguration", "", "d8-serviceaccounts")
			Expect(fs.Exists()).To(BeTrue())
			Expect(pl.Exists()).To(BeTrue())
			Expect(fs.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1beta3"))
			Expect(fs.Field("spec.rules.0.subjects").String()).To(MatchYAML(`
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test1
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test2
`))
			Expect(pl.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1beta3"))
			Expect(pl.Field("spec.limited.nominalConcurrencyShares").String()).To(Equal("5"))
		})
	})

	Context("Cluster with deckhouse namespaces, kubernetes 1.27", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesK8s127)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("flowSchema", moduleValuesForFlowSchemaModule)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			fs := f.KubernetesResource("FlowSchema", "", "d8-serviceaccounts")
			pl := f.KubernetesResource("PriorityLevelConfiguration", "", "d8-serviceaccounts")
			Expect(fs.Exists()).To(BeTrue())
			Expect(pl.Exists()).To(BeTrue())
			Expect(fs.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1beta3"))
			Expect(fs.Field("spec.rules.0.subjects").String()).To(MatchYAML(`
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test1
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test2
`))
			Expect(pl.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1beta3"))
			Expect(pl.Field("spec.limited.nominalConcurrencyShares").String()).To(Equal("5"))
		})
	})

	Context("Cluster with deckhouse namespaces, kubernetes 1.26", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesK8s126)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("flowSchema", moduleValuesForFlowSchemaModule)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			fs := f.KubernetesResource("FlowSchema", "", "d8-serviceaccounts")
			pl := f.KubernetesResource("PriorityLevelConfiguration", "", "d8-serviceaccounts")
			Expect(fs.Exists()).To(BeTrue())
			Expect(pl.Exists()).To(BeTrue())
			Expect(fs.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1beta2"))
			Expect(fs.Field("spec.rules.0.subjects").String()).To(MatchYAML(`
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test1
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test2
`))
			Expect(pl.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1beta2"))
			Expect(pl.Field("spec.limited.nominalConcurrencyShares").String()).To(Equal("5"))
		})
	})

	Context("Cluster with deckhouse namespaces, kubernetes 1.29", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesK8s129)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("flowSchema", moduleValuesForFlowSchemaModule)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			fs := f.KubernetesResource("FlowSchema", "", "d8-serviceaccounts")
			pl := f.KubernetesResource("PriorityLevelConfiguration", "", "d8-serviceaccounts")
			Expect(fs.Exists()).To(BeTrue())
			Expect(pl.Exists()).To(BeTrue())
			Expect(fs.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1beta3"))
			Expect(fs.Field("spec.rules.0.subjects").String()).To(MatchYAML(`
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test1
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test2
`))
			Expect(pl.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1beta3"))
			Expect(pl.Field("spec.limited.nominalConcurrencyShares").String()).To(Equal("5"))
		})
	})

	Context("Cluster with deckhouse namespaces, kubernetes 1.30", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesK8s130)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("flowSchema", moduleValuesForFlowSchemaModule)
			f.HelmRender()
		})

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			fs := f.KubernetesResource("FlowSchema", "", "d8-serviceaccounts")
			pl := f.KubernetesResource("PriorityLevelConfiguration", "", "d8-serviceaccounts")
			Expect(fs.Exists()).To(BeTrue())
			Expect(pl.Exists()).To(BeTrue())
			Expect(fs.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1"))
			Expect(fs.Field("spec.rules.0.subjects").String()).To(MatchYAML(`
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test1
- kind: ServiceAccount
  serviceAccount:
    name: '*'
    namespace: test2
`))
			Expect(pl.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1"))
			Expect(pl.Field("spec.limited.nominalConcurrencyShares").String()).To(Equal("5"))
		})
	})

})
