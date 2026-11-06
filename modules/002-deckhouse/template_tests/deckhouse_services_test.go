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

// service.yaml and deckhouse-leader-service.yaml put the leader among the Deckhouse pods behind the
// Services deckhouse and deckhouse-leader. The controller labels its pod leader: "true" once it holds
// the lease, and the admission webhooks of deckhouse-webhook call the deckhouse Service.

var _ = Describe("Module :: deckhouse :: Services of the leader ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues+clusterIsBootstrapped)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	services := []string{"deckhouse", "deckhouse-leader"}

	It("send to the leader only, ready or not", func() {
		for _, name := range services {
			service := f.KubernetesResource("Service", "d8-system", name)
			Expect(service.Exists()).To(BeTrue(), "the %s Service should be rendered", name)
			Expect(service.Field("spec.selector").String()).To(MatchJSON(`{"app":"deckhouse","leader":"true"}`),
				"the followers serve no webhooks")
			Expect(service.Field("spec.publishNotReadyAddresses").Bool()).To(BeTrue(),
				"the leader serves before its pod is ready")
		}
		Expect(f.KubernetesResource("Deployment", "d8-system", "deckhouse").Field("spec.template.metadata.labels.app").String()).
			To(Equal("deckhouse"))
	})

	It("send each port to the named port of the Deckhouse container", func() {
		container := f.KubernetesResource("Deployment", "d8-system", "deckhouse").Field(`spec.template.spec.containers.#(name=="deckhouse")`)
		Expect(container.Exists()).To(BeTrue())

		for _, name := range services {
			service := f.KubernetesResource("Service", "d8-system", name)
			Expect(service.Field(`spec.ports.#(name=="self").port`).Int()).To(Equal(int64(8080)), name)
			Expect(service.Field(`spec.ports.#(name=="webhook").port`).Int()).To(Equal(int64(4223)), name)

			for _, port := range service.Field("spec.ports").Array() {
				targetPort := port.Get("targetPort").String()
				Expect(container.Get(`ports.#(name=="`+targetPort+`")`).Exists()).To(BeTrue(),
					"the %s port of the %s Service should send to a port of the container", targetPort, name)
			}
		}
	})

	It("serve every webhook of deckhouse-webhook on a port of the deckhouse Service", func() {
		ports := map[int64]bool{}
		for _, port := range f.KubernetesResource("Service", "d8-system", "deckhouse").Field("spec.ports").Array() {
			ports[port.Get("port").Int()] = true
		}

		webhooks := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "deckhouse-webhook").Field("webhooks").Array()
		Expect(webhooks).ToNot(BeEmpty())
		for _, webhook := range webhooks {
			service := webhook.Get("clientConfig.service")
			Expect(service.Get("name").String()).To(Equal("deckhouse"))
			Expect(service.Get("namespace").String()).To(Equal("d8-system"))
			Expect(ports).To(HaveKey(service.Get("port").Int()), webhook.Get("name").String())
		}
	})

	It("keeps the deckhouse Service when Helm removes it from the release", func() {
		Expect(f.KubernetesResource("Service", "d8-system", "deckhouse").Field(`metadata.annotations.helm\.sh/resource-policy`).String()).
			To(Equal("keep"))
	})
})
