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
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
	"github.com/deckhouse/deckhouse/testing/webhooks/validationwebhook"
)

// The ValidationWebhooks of the module and the files of webhooks/validating their handler is made of.
var userAuthnValidationWebhooks = map[string][]string{
	"user-authn-dex-authenticator":           {"webhooks/validating/dex_authenticator_validation.py"},
	"user-authn-dex-client":                  {"webhooks/validating/dex_client_validation.py"},
	"user-authn-group":                       {"webhooks/validating/group.py"},
	"user-authn-group-prefix-dex-provider":   {"webhooks/validating/group_prefix_dex_provider.py"},
	"user-authn-group-prefix-token-exchange": {"webhooks/validating/group_prefix_token_exchange.py"},
	"user-authn-two-factor-dex-provider":     {"webhooks/validating/two_factor_dex_provider.py"},
	"user-authn-two-factor-module-config":    {"webhooks/validating/two_factor_module_config.py"},
	"user-authn-user":                        {"webhooks/validating/user.py"},
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

	Context("dex-authenticator", func() {
		const webhook = "user-authn-dex-authenticator"

		It("Should validate creates and updates of DexAuthenticators of every version", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			object := resource(webhook)
			Expect(object.Field("validationObject.name").String()).To(Equal("dexauthenticators-unique.deckhouse.io"))
			Expect(object.Field("validationObject.rules").String()).To(MatchJSON(`[{
				"apiGroups": ["deckhouse.io"], "apiVersions": ["v1", "v1alpha1", "v2alpha1"],
				"operations": ["CREATE", "UPDATE"], "resources": ["dexauthenticators"], "scope": "Namespaced"}]`))
		})

		It("Should read the existing DexAuthenticators through v1 with the jqFilter the python tests run", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			contexts := resource(webhook).Field("context").Array()
			Expect(contexts).To(HaveLen(2))
			Expect(contexts[0].Get("name").String()).To(Equal("dexauthenticators"))
			Expect(contexts[0].Get("kubernetes.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(contexts[0].Get("kubernetes.kind").String()).To(Equal("DexAuthenticator"))

			// dex_authenticator_validation_test.py builds the snapshots with a copy of the jqFilter.
			test, err := os.ReadFile(filepath.Join(moduleDir, "webhooks/validating/dex_authenticator_validation_test.py"))
			Expect(err).ShouldNot(HaveOccurred())
			copied := regexp.MustCompile(`(?s)\nJQ_FILTER = '''(.*?)'''`).FindSubmatch(test)
			Expect(copied).NotTo(BeNil())
			Expect(strings.TrimSpace(contexts[0].Get("kubernetes.jqFilter").String())).To(Equal(strings.TrimSpace(string(copied[1]))))
		})

		It("Should read the DexProviders through v1 to warn about allowedProviders that name none", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			contexts := resource(webhook).Field("context").Array()
			Expect(contexts[1].Get("name").String()).To(Equal("dexproviders"))
			Expect(contexts[1].Get("kubernetes.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(contexts[1].Get("kubernetes.kind").String()).To(Equal("DexProvider"))
			// The hook reads item["filterResult"]["name"].
			Expect(strings.TrimSpace(contexts[1].Get("kubernetes.jqFilter").String())).To(Equal(`{"name": .metadata.name}`))
		})
	})

	Context("group-prefix-dex-provider", func() {
		const webhook = "user-authn-group-prefix-dex-provider"

		It("Should check the right requests", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			object := resource(webhook)
			Expect(object.Field("validationObject.name").String()).To(Equal("groupprefix-dex-providers.deckhouse.io"))
			Expect(object.Field("validationObject.rules").String()).To(MatchJSON(`[{
				"apiGroups": ["deckhouse.io"], "apiVersions": ["v1", "v1alpha1"],
				"operations": ["CREATE", "UPDATE"], "resources": ["dexproviders"], "scope": "Cluster"}]`))
		})

		It("Should read the objects with the jqFilters the python tests run", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			contexts := resource(webhook).Field("context").Array()
			Expect(contexts).To(HaveLen(2))
			Expect(contexts[0].Get("name").String()).To(Equal("dexproviders"))
			Expect(contexts[0].Get("kubernetes.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(contexts[0].Get("kubernetes.kind").String()).To(Equal("DexProvider"))
			Expect(contexts[1].Get("name").String()).To(Equal("tokenexchangeproviders"))
			Expect(contexts[1].Get("kubernetes.kind").String()).To(Equal("TokenExchangeProvider"))

			test, err := os.ReadFile(filepath.Join(moduleDir, "webhooks/validating/group_prefix_dex_provider_test.py"))
			Expect(err).ShouldNot(HaveOccurred())
			dexProviders := regexp.MustCompile(`(?s)\nJQ_FILTER = '''(.*?)'''`).FindSubmatch(test)
			Expect(dexProviders).NotTo(BeNil())
			Expect(strings.TrimSpace(contexts[0].Get("kubernetes.jqFilter").String())).To(Equal(strings.TrimSpace(string(dexProviders[1]))))
			exchanges := regexp.MustCompile(`(?s)\nTEP_JQ_FILTER = '''(.*?)'''`).FindSubmatch(test)
			Expect(exchanges).NotTo(BeNil())
			Expect(strings.TrimSpace(contexts[1].Get("kubernetes.jqFilter").String())).To(Equal(strings.TrimSpace(string(exchanges[1]))))
		})
	})

	Context("group-prefix-token-exchange", func() {
		const webhook = "user-authn-group-prefix-token-exchange"

		It("Should check the right requests", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			object := resource(webhook)
			Expect(object.Field("validationObject.name").String()).To(Equal("groupprefix-token-exchange-providers.deckhouse.io"))
			Expect(object.Field("validationObject.rules").String()).To(MatchJSON(`[{
				"apiGroups": ["deckhouse.io"], "apiVersions": ["v1alpha1"],
				"operations": ["CREATE", "UPDATE"], "resources": ["tokenexchangeproviders"], "scope": "Cluster"}]`))
		})

		It("Should read the DexProviders with the jqFilter the python tests run", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			contexts := resource(webhook).Field("context").Array()
			Expect(contexts).To(HaveLen(1))
			Expect(contexts[0].Get("name").String()).To(Equal("dexproviders"))
			Expect(contexts[0].Get("kubernetes.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(contexts[0].Get("kubernetes.kind").String()).To(Equal("DexProvider"))

			test, err := os.ReadFile(filepath.Join(moduleDir, "webhooks/validating/group_prefix_token_exchange_test.py"))
			Expect(err).ShouldNot(HaveOccurred())
			copied := regexp.MustCompile(`(?s)\nJQ_FILTER = '''(.*?)'''`).FindSubmatch(test)
			Expect(copied).NotTo(BeNil())
			Expect(strings.TrimSpace(contexts[0].Get("kubernetes.jqFilter").String())).To(Equal(strings.TrimSpace(string(copied[1]))))
		})
	})

	Context("two-factor-module-config", func() {
		const webhook = "user-authn-two-factor-module-config"

		It("Should check the right requests", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			object := resource(webhook)
			Expect(object.Field("validationObject.name").String()).To(Equal("twofactor-basic-auth-settings.deckhouse.io"))
			Expect(object.Field("validationObject.rules").String()).To(MatchJSON(`[{
				"apiGroups": ["deckhouse.io"], "apiVersions": ["*"],
				"operations": ["CREATE", "UPDATE"], "resources": ["moduleconfigs"], "scope": "Cluster"}]`))
		})

		It("Should read the objects with the jqFilter the python tests run", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			contexts := resource(webhook).Field("context").Array()
			Expect(contexts).To(HaveLen(1))
			Expect(contexts[0].Get("name").String()).To(Equal("dexproviders"))
			Expect(contexts[0].Get("kubernetes.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(contexts[0].Get("kubernetes.kind").String()).To(Equal("DexProvider"))

			test, err := os.ReadFile(filepath.Join(moduleDir, "webhooks/validating/two_factor_module_config_test.py"))
			Expect(err).ShouldNot(HaveOccurred())
			copied := regexp.MustCompile(`(?s)\nJQ_FILTER = '''(.*?)'''`).FindSubmatch(test)
			Expect(copied).NotTo(BeNil())
			Expect(strings.TrimSpace(contexts[0].Get("kubernetes.jqFilter").String())).To(Equal(strings.TrimSpace(string(copied[1]))))
		})
	})

	Context("two-factor-dex-provider", func() {
		const webhook = "user-authn-two-factor-dex-provider"

		It("Should check the right requests", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			object := resource(webhook)
			Expect(object.Field("validationObject.name").String()).To(Equal("twofactor-basic-auth-providers.deckhouse.io"))
			Expect(object.Field("validationObject.rules").String()).To(MatchJSON(`[{
				"apiGroups": ["deckhouse.io"], "apiVersions": ["v1", "v1alpha1"],
				"operations": ["CREATE", "UPDATE"], "resources": ["dexproviders"], "scope": "Cluster"}]`))
		})

		It("Should read the objects with the jqFilter the python tests run", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			contexts := resource(webhook).Field("context").Array()
			Expect(contexts).To(HaveLen(1))
			Expect(contexts[0].Get("name").String()).To(Equal("modulesettings"))
			Expect(contexts[0].Get("kubernetes.kind").String()).To(Equal("ModuleConfig"))
			Expect(contexts[0].Get("kubernetes.nameSelector.matchNames").Value()).To(Equal([]any{"user-authn"}))

			test, err := os.ReadFile(filepath.Join(moduleDir, "webhooks/validating/two_factor_dex_provider_test.py"))
			Expect(err).ShouldNot(HaveOccurred())
			copied := regexp.MustCompile(`(?s)\nJQ_FILTER = '''(.*?)'''`).FindSubmatch(test)
			Expect(copied).NotTo(BeNil())
			Expect(strings.TrimSpace(contexts[0].Get("kubernetes.jqFilter").String())).To(Equal(strings.TrimSpace(string(copied[1]))))
		})
	})

	Context("dex-client", func() {
		const webhook = "user-authn-dex-client"

		It("Should warn on creates and updates of DexClients of every version, and never on deletes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			object := resource(webhook)
			Expect(object.Field("validationObject.name").String()).To(Equal("dexclients-providers.deckhouse.io"))
			Expect(object.Field("validationObject.rules").String()).To(MatchJSON(`[{
				"apiGroups": ["deckhouse.io"], "apiVersions": ["v1", "v1alpha1"],
				"operations": ["CREATE", "UPDATE"], "resources": ["dexclients"], "scope": "Namespaced"}]`))
		})

		It("Should read the DexProviders through v1", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			contexts := resource(webhook).Field("context").Array()
			Expect(contexts).To(HaveLen(1))
			Expect(contexts[0].Get("name").String()).To(Equal("dexproviders"))
			Expect(contexts[0].Get("kubernetes.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(contexts[0].Get("kubernetes.kind").String()).To(Equal("DexProvider"))
			Expect(strings.TrimSpace(contexts[0].Get("kubernetes.jqFilter").String())).To(Equal(`{"name": .metadata.name}`))
		})
	})
})
