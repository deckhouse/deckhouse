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
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

const certManagerCARotation = `
internal:
  enableCAInjector: false
  selfSignedCA:
    cert: string
    key: string
  webhookCert:
    ca: test-webhook-ca
    key: test-webhook-key
    crt: test-webhook-crt
    caBundle: test-webhook-ca-bundle
    caRotationInProgress: true
`

var _ = Describe("Module :: cert-manager :: helm template :: webhook resilience ::", func() {
	f := SetupHelmConfig(``)

	webhookConfigurations := func() []object_store.KubeObject {
		return []object_store.KubeObject{
			f.KubernetesGlobalResource("ValidatingWebhookConfiguration", "cert-manager-webhook"),
			f.KubernetesGlobalResource("MutatingWebhookConfiguration", "cert-manager-webhook"),
		}
	}

	Context("HA", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("surges the webhook and replaces at most one replica at a time", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			webhook := f.KubernetesResource("Deployment", "d8-cert-manager", "webhook")
			Expect(webhook.Field("spec.replicas").Int()).To(BeEquivalentTo(5))
			Expect(webhook.Field("spec.strategy").String()).To(MatchYAML(`
type: RollingUpdate
rollingUpdate:
  maxSurge: 1
  maxUnavailable: 1
`))
			Expect(webhook.Field("spec.template.spec.affinity").String()).To(MatchYAML(`
podAntiAffinity:
  requiredDuringSchedulingIgnoredDuringExecution:
  - labelSelector:
      matchLabels:
        app: webhook
    matchLabelKeys:
    - pod-template-hash
    topologyKey: kubernetes.io/hostname
`))
			Expect(webhook.Field("spec.template.spec.containers.0.lifecycle").String()).To(MatchYAML(`
preStop:
  sleep:
    seconds: 10
`))
			// The webhook_ca_bundle hook compares this annotation with caChecksum() of the CA in values.
			sum := sha256.Sum256([]byte("string"))
			Expect(webhook.Field(`spec.template.metadata.annotations.checksum/ca`).String()).To(Equal(hex.EncodeToString(sum[:])))
		})

		It("restarts the controller lease checks every 5 seconds", func() {
			controller := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			Expect(controller.Field("spec.template.spec.containers.0.args").String()).To(ContainSubstring(`--leader-election-retry-period=5s`))
		})

		It("gives the controller five minutes to reach the API server at startup", func() {
			controller := f.KubernetesResource("Deployment", "d8-cert-manager", "cert-manager")
			probe := controller.Field("spec.template.spec.containers.0.startupProbe")
			Expect(probe.Get("httpGet.path").String()).To(Equal("/livez"))
			Expect(probe.Get("periodSeconds").Int() * probe.Get("failureThreshold").Int()).To(BeNumerically(">", 300))
		})

		It("waits for the webhook Deployment and trusts the current CA outside a rotation", func() {
			for _, wc := range webhookConfigurations() {
				Expect(wc.Field(`metadata.annotations.werf\.io/deploy-dependency-deployment`).Exists()).To(BeTrue())
				Expect(wc.Field("webhooks.0.clientConfig.caBundle").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("string"))))
			}
		})
	})

	Context("Single master", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManager)
			f.HelmRender()
		})

		It("keeps the single webhook replica serving during a rollout", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			webhook := f.KubernetesResource("Deployment", "d8-cert-manager", "webhook")
			Expect(webhook.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(webhook.Field("spec.strategy.rollingUpdate.maxSurge").Int()).To(BeEquivalentTo(1))
			Expect(webhook.Field("spec.strategy.rollingUpdate.maxUnavailable").Int()).To(BeEquivalentTo(0))
			Expect(webhook.Field("spec.template.spec.affinity").Exists()).To(BeFalse())
		})
	})

	Context("CA rotation in progress", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValuesHa)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("certManager", certManagerCARotation)
			f.HelmRender()
		})

		It("applies the webhook configurations at once with both CAs", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, wc := range webhookConfigurations() {
				Expect(wc.Field(`metadata.annotations.werf\.io/deploy-dependency-deployment`).Exists()).To(BeFalse())
				Expect(wc.Field(`metadata.annotations.werf\.io/deploy-dependency-service`).Exists()).To(BeTrue())
				Expect(wc.Field("webhooks.0.clientConfig.caBundle").String()).To(Equal(base64.StdEncoding.EncodeToString([]byte("test-webhook-ca-bundle"))))
			}
		})
	})
})
