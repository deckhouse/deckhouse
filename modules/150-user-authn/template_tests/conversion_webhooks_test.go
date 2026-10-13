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
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/webhooks/conversionwebhook"
)

// The ConversionWebhooks of the module, named after their CRDs: the file of webhooks/conversion that
// holds their methods and the from/to pairs of their conversions, in the order the chart lists them.
var userAuthnConversionWebhooks = map[string]struct {
	file        string
	conversions [][2]string
}{
	"dexauthenticators.deckhouse.io": {file: "webhooks/conversion/dex_authenticator_conversion.py", conversions: [][2]string{{"v1", "v2alpha1"}, {"v2alpha1", "v1"}, {"v1", "v1alpha1"}, {"v1alpha1", "v1"}}},
	"dexproviders.deckhouse.io":      {file: "webhooks/conversion/dex_provider_conversion.py", conversions: [][2]string{{"v1alpha1", "v1"}, {"v1", "v1alpha1"}}},
}

var _ = Describe("Module :: user-authn :: helm template :: conversion webhooks", func() {
	f := SetupHelmConfig(``)

	_, thisFile, _, _ := runtime.Caller(0)
	moduleDir := filepath.Join(filepath.Dir(thisFile), "..")

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", `
discovery:
  kubernetesVersion: 1.32.0
  d8SpecificNodeCountByRole:
    system: 2
  kubernetesCA: plainstring
modules:
  publicDomainTemplate: "%s.example.com"
  https:
    mode: Disabled
`)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		f.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		f.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		f.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
		f.HelmRender()
	})

	It("Should render the conversions and the handler from webhooks/conversion", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name, expected := range userAuthnConversionWebhooks {
			object := f.KubernetesGlobalResource("ConversionWebhook", name)
			Expect(object.Exists()).To(BeTrue(), name)

			var pairs [][2]string
			var conversions []conversionwebhook.Conversion
			for _, item := range object.Field("conversions").Array() {
				pairs = append(pairs, [2]string{item.Get("from").String(), item.Get("to").String()})
				conversions = append(conversions, conversionwebhook.Conversion{
					From:   item.Get("from").String(),
					To:     item.Get("to").String(),
					Python: item.Get("handler.python").String(),
				})
			}
			Expect(pairs).To(Equal(expected.conversions), name)
			Expect(conversionwebhook.HandlerMatchesFile(moduleDir, expected.file, conversions)).To(Succeed(), name)
		}
	})

})
