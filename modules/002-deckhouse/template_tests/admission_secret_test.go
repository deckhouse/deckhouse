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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// admission/secret.yaml holds the certificate the Deckhouse controller serves the webhooks of
// deckhouse-webhook with, from the directory the Secret is mounted to. The API server verifies them
// with the caBundle of each webhook, which has to be the CA of the same pair.

var _ = Describe("Module :: deckhouse :: admission webhook certificate ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues+clusterIsBootstrapped)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("carries the certificate the module issued for the admission webhooks", func() {
		secret := f.KubernetesResource("Secret", "d8-system", "admission-webhook-certs")
		Expect(secret.Exists()).To(BeTrue())
		Expect(secret.Field("type").String()).To(Equal("kubernetes.io/tls"))
		Expect(secret.Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"))

		for key, value := range map[string]string{
			`tls\.crt`: "deckhouse.internal.admissionWebhookCert.crt",
			`tls\.key`: "deckhouse.internal.admissionWebhookCert.key",
			`ca\.crt`:  "deckhouse.internal.admissionWebhookCert.ca",
		} {
			decoded, err := base64.StdEncoding.DecodeString(secret.Field("data." + key).String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(decoded)).To(Equal(f.ValuesGet(value).String()), key)
		}
	})

	It("is mounted where the controller serves the webhooks from", func() {
		deployment := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
		Expect(deployment.Field(`spec.template.spec.volumes.#(name=="certs").secret.secretName`).String()).To(Equal("admission-webhook-certs"))

		container := deployment.Field(`spec.template.spec.containers.#(name=="deckhouse")`)
		Expect(container.Get(`volumeMounts.#(name=="certs").mountPath`).String()).
			To(Equal(container.Get(`env.#(name=="ADDON_OPERATOR_ADMISSION_SERVER_CERTS_DIR").value`).String()))
	})

	It("is the CA every webhook of deckhouse-webhook is verified with", func() {
		ca := base64.StdEncoding.EncodeToString([]byte(f.ValuesGet("deckhouse.internal.admissionWebhookCert.ca").String()))

		webhooks := f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "deckhouse-webhook").Field("webhooks").Array()
		Expect(webhooks).ToNot(BeEmpty())
		for _, webhook := range webhooks {
			Expect(webhook.Get("clientConfig.caBundle").String()).To(Equal(ca), webhook.Get("name").String())
		}
	})
})
