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

// flow-schema/ puts the list requests of the service accounts of the Deckhouse namespaces into a
// priority level of their own, so that the components listing whole collections cannot starve the
// rest of the API server. The namespaces are those with the heritage: deckhouse label, collected by
// hooks/discover_d8_ns.go into deckhouse.internal.namespaces.

const deckhouseFlowControl = "d8-serviceaccounts"

var _ = Describe("Module :: deckhouse :: API priority and fairness ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
	})

	render := func() {
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	It("sends the list requests of the service accounts of every Deckhouse namespace to their own priority level", func() {
		f.ValuesSetFromYaml("deckhouse.internal.namespaces", `["d8-system", "d8-monitoring"]`)
		render()

		schema := f.KubernetesGlobalResource("FlowSchema", deckhouseFlowControl)
		Expect(schema.Exists()).To(BeTrue())
		Expect(schema.Field("spec.priorityLevelConfiguration.name").String()).To(Equal(deckhouseFlowControl))
		Expect(f.KubernetesGlobalResource("PriorityLevelConfiguration", deckhouseFlowControl).Exists()).To(BeTrue())
		Expect(schema.Field("spec.distinguisherMethod.type").String()).To(Equal("ByUser"),
			"each service account queues on its own, so one of them cannot take the whole level")

		Expect(schema.Field("spec.rules").Array()).To(HaveLen(1))
		Expect(schema.Field("spec.rules.0.subjects").String()).To(MatchJSON(`[
			{"kind":"ServiceAccount","serviceAccount":{"name":"*","namespace":"d8-system"}},
			{"kind":"ServiceAccount","serviceAccount":{"name":"*","namespace":"d8-monitoring"}}
		]`))

		Expect(schema.Field("spec.rules.0.resourceRules").Array()).To(HaveLen(1))
		rule := schema.Field("spec.rules.0.resourceRules.0")
		Expect(rule.Get("verbs").String()).To(MatchJSON(`["list"]`))
		Expect(rule.Get("resources").String()).To(MatchJSON(`["*"]`))
		Expect(rule.Get("namespaces").String()).To(MatchJSON(`["*"]`))
		Expect(rule.Get("clusterScope").Bool()).To(BeTrue())
		Expect(schema.Field("spec.rules.0.resourceRules.0.apiGroups").AsStringSlice()).
			To(ContainElements("", "deckhouse.io", "cilium.io"))
	})

	It("is not rendered while no namespace is known, since a FlowSchema rule needs a subject", func() {
		f.ValuesSetFromYaml("deckhouse.internal.namespaces", `[]`)
		render()

		Expect(f.KubernetesGlobalResource("FlowSchema", deckhouseFlowControl).Exists()).To(BeFalse())
	})

	It("limits the priority level by its nominal concurrency shares and queues what exceeds them", func() {
		render()

		level := f.KubernetesGlobalResource("PriorityLevelConfiguration", deckhouseFlowControl)
		Expect(level.Exists()).To(BeTrue())
		Expect(level.Field("spec.type").String()).To(Equal("Limited"))
		Expect(level.Field("spec.limited.nominalConcurrencyShares").Int()).To(Equal(int64(5)))
		Expect(level.Field("spec.limited.assuredConcurrencyShares").Exists()).To(BeFalse())
		Expect(level.Field("spec.limited.limitResponse.type").String()).To(Equal("Queue"))
	})

	It("names the shares assuredConcurrencyShares before Kubernetes 1.26", func() {
		f.ValuesSet("global.discovery.kubernetesVersion", "1.25.0")
		render()

		level := f.KubernetesGlobalResource("PriorityLevelConfiguration", deckhouseFlowControl)
		Expect(level.Field("spec.limited.assuredConcurrencyShares").Int()).To(Equal(int64(5)))
		Expect(level.Field("spec.limited.nominalConcurrencyShares").Exists()).To(BeFalse())
	})
})
