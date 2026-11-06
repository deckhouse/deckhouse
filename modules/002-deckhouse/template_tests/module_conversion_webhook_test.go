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
	"os"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// With Module v2 the Modules are stored in v1beta1, which carries no properties, and the webhook fills
// v1alpha1 from it for the readers of that version. webhook-operator reads the CRD name from the name
// of the ConversionWebhook and pastes the handlers of all conversions into one class, which is why the
// whole of webhooks/conversion/modules.py is the handler of the first conversion.

const moduleConversionWebhook = "modules.deckhouse.io"

var _ = Describe("Module :: deckhouse :: Module conversion webhook ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	render := func(enableModuleV2 bool) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.ValuesSet("deckhouse.enableModuleV2", enableModuleV2)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	It("is not rendered without Module v2, when the Modules are stored in v1alpha1", func() {
		render(false)

		Expect(f.KubernetesGlobalResource("ConversionWebhook", moduleConversionWebhook).Exists()).To(BeFalse())
	})

	It("converts the Modules between v1beta1 and v1alpha1 with Module v2", func() {
		render(true)

		webhook := f.KubernetesGlobalResource("ConversionWebhook", moduleConversionWebhook)
		Expect(webhook.Exists()).To(BeTrue())

		conversions := webhook.Field("conversions").Array()
		Expect(conversions).To(HaveLen(2))
		Expect(conversions[0].Get("from").String()).To(Equal("v1beta1"))
		Expect(conversions[0].Get("to").String()).To(Equal("v1alpha1"))
		Expect(conversions[1].Get("from").String()).To(Equal("v1alpha1"))
		Expect(conversions[1].Get("to").String()).To(Equal("v1beta1"))
		Expect(conversions[1].Get("handler").String()).To(MatchJSON(`{}`),
			"the methods of both conversions come with the handler of the first one")
	})

	It("carries the handler from webhooks/conversion/modules.py", func() {
		render(true)

		handler, err := os.ReadFile("../webhooks/conversion/modules.py")
		Expect(err).ShouldNot(HaveOccurred())

		webhook := f.KubernetesGlobalResource("ConversionWebhook", moduleConversionWebhook)
		// The block scalar keeps one trailing newline whatever the file ends with.
		Expect(strings.TrimSpace(webhook.Field("conversions.0.handler.python").String())).
			To(Equal(strings.TrimSpace(string(handler))))
	})
})
