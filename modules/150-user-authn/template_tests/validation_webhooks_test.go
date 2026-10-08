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
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/webhooks/validationwebhook"
)

// The ValidationWebhooks of the module and the files of webhooks/validating their handler is made of.
var userAuthnValidationWebhooks = map[string][]string{
	"user-authn-group": {"webhooks/validating/group.py"},
	"user-authn-user":  {"webhooks/validating/user.py"},
}

var _ = Describe("Module :: user-authn :: helm template :: validation webhooks", func() {
	f := SetupHelmConfig(``)

	_, thisFile, _, _ := runtime.Caller(0)
	moduleDir := filepath.Join(filepath.Dir(thisFile), "..")

	resource := func(name string) object_store.KubeObject {
		object := f.KubernetesGlobalResource("ValidationWebhook", name)
		Expect(object.Exists()).To(BeTrue(), name)
		return object
	}

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

	It("Should check every ValidationWebhook the module declares", func() {
		declared, err := validationwebhook.ManifestNames(moduleDir)
		Expect(err).ShouldNot(HaveOccurred())
		checked := make([]string, 0, len(userAuthnValidationWebhooks))
		for name := range userAuthnValidationWebhooks {
			checked = append(checked, name)
		}
		Expect(checked).To(ConsistOf(declared))
	})

	It("Should render the handlers from webhooks/validating", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name, files := range userAuthnValidationWebhooks {
			handler := resource(name).Field("handler.python").String()
			Expect(validationwebhook.HandlerMatchesFiles(moduleDir, handler, files...)).To(Succeed(), name)
		}
	})

	It("Should render only the fields the CRD and the operator template accept", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name := range userAuthnValidationWebhooks {
			object := resource(name)
			undeclared, err := validationwebhook.UndeclaredFields(object)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(undeclared).To(BeEmpty(), name)
			Expect(validationwebhook.LiteralHazards(object)).To(BeEmpty(), name)
		}
	})

	It("Should keep the wrapper of the python test helper in sync with the operator template", func() {
		Expect(validationwebhook.TestHelperMatchesTemplate(moduleDir)).To(Succeed())
	})
})
