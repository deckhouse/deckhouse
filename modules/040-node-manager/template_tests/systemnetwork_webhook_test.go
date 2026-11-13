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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const systemNetworkWebhookName = "systemnetwork.validation.node-controller.deckhouse.io"

// The expression the envtest of the webhook runs: node-controller/.../moduleconfig/systemnetwork.
const systemNetworkMatchExpression = `object.metadata.name == 'node-manager' &&
object.?spec.?settings.?fencing.?systemNetworkFallback.?systemNetworkName.orValue('') != '' &&
object.?spec.?settings.?fencing.?systemNetworkFallback.?systemNetworkName.orValue('') != null &&
object.?spec.?settings.?fencing.?systemNetworkFallback.?systemNetworkName.orValue('') !=
  (oldObject == null ? '' : oldObject.?spec.?settings.?fencing.?systemNetworkFallback.?systemNetworkName.orValue(''))
`

var _ = Describe("Module :: node-manager :: helm template :: webhook of the SystemNetwork", func() {
	f := SetupHelmConfig(``)

	render := func(edition string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.deckhouseEdition", edition)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	systemNetworkWebhook := func() gjson.Result {
		webhooks := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "node-controller-validating-webhook-configuration").Field("webhooks")
		Expect(webhooks.Exists()).To(BeTrue())
		return webhooks.Get(`#(name=="` + systemNetworkWebhookName + `")`)
	}

	DescribeTable("the element is rendered in the editions that have the setting",
		func(edition string, want bool) {
			render(edition)

			Expect(systemNetworkWebhook().Exists()).To(Equal(want))
			// The other elements of the configuration stay in every edition.
			webhooks := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "node-controller-validating-webhook-configuration").Field("webhooks")
			Expect(webhooks.Get(`#(name=="nodegroup.validation.node-controller.deckhouse.io")`).Exists()).To(BeTrue())
		},
		Entry("CE", "CE", false),
		Entry("BE", "BE", false),
		Entry("SE", "SE", false),
		Entry("SE-plus", "SE-plus", false),
		Entry("EE", "EE", true),
		Entry("FE", "FE", true),
		Entry("CSE", "CSE", true),
	)

	Context("EE", func() {
		BeforeEach(func() { render("EE") })

		It("checks ModuleConfig on create and update and refuses when node-controller does not answer", func() {
			element := systemNetworkWebhook()

			Expect(element.Get("clientConfig.service.path").String()).To(Equal("/validate-deckhouse-io-v1alpha1-moduleconfig-node-manager"))
			Expect(element.Get("failurePolicy").String()).To(Equal("Fail"))
			Expect(element.Get("timeoutSeconds").Int()).To(Equal(int64(10)))
			Expect(element.Get("sideEffects").String()).To(Equal("None"))
			Expect(element.Get("matchPolicy").String()).To(Equal("Equivalent"))
			Expect(element.Get("rules").String()).To(MatchJSON(`[{
				"apiGroups": ["deckhouse.io"],
				"apiVersions": ["v1alpha1"],
				"operations": ["CREATE", "UPDATE"],
				"resources": ["moduleconfigs"],
				"scope": "Cluster"
			}]`))
		})

		It("calls the webhook only when the network changes to a value that is not empty", func() {
			conditions := systemNetworkWebhook().Get("matchConditions").Array()

			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0].Get("expression").String()).To(Equal(systemNetworkMatchExpression))
		})

		It("keeps the deploy dependencies of the configuration", func() {
			configuration := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "node-controller-validating-webhook-configuration")

			Expect(configuration.Field(`metadata.annotations.werf\.io/deploy-dependency-deployment`).String()).To(ContainSubstring("name=node-controller"))
			Expect(configuration.Field(`metadata.annotations.werf\.io/deploy-dependency-service`).String()).To(ContainSubstring("name=node-controller-webhook"))
		})

		It("lets node-controller read SystemNetworks, and only read one at a time", func() {
			role := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:node-controller")

			Expect(roleVerbs(role, "network.deckhouse.io", "systemnetworks")).To(ConsistOf("get"))
		})
	})
})
