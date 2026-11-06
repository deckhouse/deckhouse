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
	"regexp"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// Two ValidationWebhooks of webhook-handler/validating/ that check ModuleConfigs. The heritage one is
// checked in heritage_subresources_test.go.

// handlerSnapshot matches a snapshot the Python handler of a ValidationWebhook reads.
var handlerSnapshot = regexp.MustCompile(`ctx\.snapshots\.get\(\s*"([^"]+)"`)

var _ = Describe("Module :: deckhouse :: ModuleConfig validation webhooks ::", func() {
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

	const moduleConfigRules = `[{"apiGroups":["deckhouse.io"],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["moduleconfigs"],"scope":"Cluster"}]`

	It("reviews every ModuleConfig written, to keep the sds-drbd module from being enabled", func() {
		render()

		webhook := f.KubernetesGlobalResource("ValidationWebhook", "disable-sds-drbd-module")
		Expect(webhook.Exists()).To(BeTrue())
		Expect(webhook.Field("validationObject.rules").String()).To(MatchJSON(moduleConfigRules))
		Expect(webhook.Field("handler.python").String()).To(ContainSubstring(`mc_name == "sds-drbd"`))
	})

	Context("publicDomainTemplate", func() {
		webhook := func() string {
			render()

			webhook := f.KubernetesGlobalResource("ValidationWebhook", "public-domain-template")
			Expect(webhook.Exists()).To(BeTrue())
			Expect(webhook.Field("validationObject.rules").String()).To(MatchJSON(moduleConfigRules))
			return webhook.Field("handler.python").String()
		}

		It("compares the template with the cluster domain the cluster resolved when no configuration names one", func() {
			f.ValuesSet("global.discovery.clusterDomain", "corp.example")

			Expect(webhook()).To(ContainSubstring(`dns_domains.append("corp.example")`))
		})

		It("reads only the snapshots the webhook subscribes to", func() {
			handler := webhook()

			contexts := map[string]bool{}
			for _, context := range f.KubernetesGlobalResource("ValidationWebhook", "public-domain-template").Field("context").Array() {
				contexts[context.Get("name").String()] = true
			}

			read := handlerSnapshot.FindAllStringSubmatch(handler, -1)
			Expect(read).ToNot(BeEmpty())
			for _, match := range read {
				Expect(contexts).To(HaveKey(match[1]), "a snapshot without a context is always empty, so the handler would never see it")
			}
		})

		It("subscribes to the objects the cluster domain is kept in", func() {
			webhook()

			contexts := f.KubernetesGlobalResource("ValidationWebhook", "public-domain-template").Field("context")
			for name, source := range map[string]struct{ apiVersion, kind, name, namespace string }{
				"d8-kube-dns-cm-wh":                      {"v1", "ConfigMap", "d8-kube-dns", "kube-system"},
				"d8-cluster-configuration":               {"v1", "Secret", "d8-cluster-configuration", "kube-system"},
				"kube-dns-module":                        {"deckhouse.io/v1alpha1", "Module", "kube-dns", ""},
				"d8-control-plane-manager-module-config": {"deckhouse.io/v1alpha1", "ModuleConfig", "control-plane-manager", ""},
			} {
				kubernetes := contexts.Get(`#(name=="` + name + `").kubernetes`)
				Expect(kubernetes.Exists()).To(BeTrue(), "the %s context should be declared", name)
				Expect(kubernetes.Get("apiVersion").String()).To(Equal(source.apiVersion), name)
				Expect(kubernetes.Get("kind").String()).To(Equal(source.kind), name)
				Expect(kubernetes.Get("nameSelector.matchNames").String()).To(MatchJSON(`["`+source.name+`"]`), name)
				Expect(kubernetes.Get("namespace.nameSelector.matchNames.0").String()).To(Equal(source.namespace), name)
			}
		})
	})
})
