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

const webhookPresentValues = `
cache:
  enabled: false
internal:
  takeover:
    phase: "New"
  pki:
    hash: "pki-hash-abc"
    httpSecret: HTTP_SECRET
    ca: {cert: CA_CERT, key: CA_KEY}
    token: {cert: TOKEN_CERT, key: TOKEN_KEY}
    agent: {cert: AGENT_CERT, key: AGENT_KEY}
    distribution: {cert: DIST_CERT, key: DIST_KEY}
    auth: {cert: AUTH_CERT, key: AUTH_KEY}
    users:
      - {name: ro, password: ro-pass, passwordHash: ro-hash, role: ReadOnly}
  webhook:
    ca: WEBHOOK_CA
    crt: WEBHOOK_CRT
    key: WEBHOOK_KEY
`

const webhookAbsentValues = `
cache:
  enabled: false
internal:
  takeover:
    phase: "New"
  pki:
    hash: "pki-hash-abc"
    httpSecret: HTTP_SECRET
    ca: {cert: CA_CERT, key: CA_KEY}
    token: {cert: TOKEN_CERT, key: TOKEN_KEY}
    agent: {cert: AGENT_CERT, key: AGENT_KEY}
    distribution: {cert: DIST_CERT, key: DIST_KEY}
    auth: {cert: AUTH_CERT, key: AUTH_KEY}
    users:
      - {name: ro, password: ro-pass, passwordHash: ro-hash, role: ReadOnly}
`

var _ = Describe("Module :: registry :: helm template :: webhook", func() {
	f := SetupHelmConfig(``)

	Context("webhook cert present", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", webhookPresentValues)
			f.HelmRender()
		})

		It("renders without error", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("renders MutatingWebhookConfiguration with correct caBundle", func() {
			mwc := f.KubernetesResource("MutatingWebhookConfiguration", "", "registry-modulesource.deckhouse.io")
			Expect(mwc.Exists()).To(BeTrue())
			caBundle := mwc.Field("webhooks.0.clientConfig.caBundle").String()
			Expect(caBundle).To(Equal(b64("WEBHOOK_CA")))
		})

		It("renders MutatingWebhookConfiguration with failurePolicy Ignore", func() {
			mwc := f.KubernetesResource("MutatingWebhookConfiguration", "", "registry-modulesource.deckhouse.io")
			Expect(mwc.Exists()).To(BeTrue())
			Expect(mwc.Field("webhooks.0.failurePolicy").String()).To(Equal("Ignore"))
		})

		It("renders MutatingWebhookConfiguration with modulesources CREATE/UPDATE rule", func() {
			mwc := f.KubernetesResource("MutatingWebhookConfiguration", "", "registry-modulesource.deckhouse.io")
			Expect(mwc.Exists()).To(BeTrue())
			Expect(mwc.Field("webhooks.0.rules.0.resources.0").String()).To(Equal("modulesources"))
			ops := mwc.Field("webhooks.0.rules.0.operations").String()
			Expect(ops).To(ContainSubstring("CREATE"))
			Expect(ops).To(ContainSubstring("UPDATE"))
		})

		It("renders MutatingWebhookConfiguration with objectSelector heritage NotIn [deckhouse]", func() {
			mwc := f.KubernetesResource("MutatingWebhookConfiguration", "", "registry-modulesource.deckhouse.io")
			Expect(mwc.Exists()).To(BeTrue())
			key := mwc.Field("webhooks.0.objectSelector.matchExpressions.0.key").String()
			Expect(key).To(Equal("heritage"))
			op := mwc.Field("webhooks.0.objectSelector.matchExpressions.0.operator").String()
			Expect(op).To(Equal("NotIn"))
			values := mwc.Field("webhooks.0.objectSelector.matchExpressions.0.values").String()
			Expect(values).To(ContainSubstring("deckhouse"))
		})

		It("renders Service registry-webhook with port 443 -> 9443", func() {
			svc := f.KubernetesResource("Service", "d8-system", "registry-webhook")
			Expect(svc.Exists()).To(BeTrue())
			Expect(svc.Field("spec.ports.0.port").Int()).To(Equal(int64(443)))
			Expect(svc.Field("spec.ports.0.targetPort").String()).To(Equal("webhook"))
		})

		It("renders Deployment registry-webhook with non-empty image (registryWebhook key)", func() {
			dep := f.KubernetesResource("Deployment", "d8-system", "registry-webhook")
			Expect(dep.Exists()).To(BeTrue())
			image := dep.Field("spec.template.spec.containers.0.image").String()
			Expect(image).NotTo(BeEmpty())
		})

		It("renders Deployment with /certs and /etc/registry-module-pki volume mounts", func() {
			dep := f.KubernetesResource("Deployment", "d8-system", "registry-webhook")
			Expect(dep.Exists()).To(BeTrue())
			mounts := dep.Field("spec.template.spec.containers.0.volumeMounts").String()
			Expect(mounts).To(ContainSubstring("/certs"))
			Expect(mounts).To(ContainSubstring("/etc/registry-module-pki"))
		})

		It("renders Deployment with checksum/creds annotation", func() {
			dep := f.KubernetesResource("Deployment", "d8-system", "registry-webhook")
			Expect(dep.Exists()).To(BeTrue())
			anno := dep.Field("spec.template.metadata.annotations").String()
			Expect(anno).To(ContainSubstring("checksum/creds"))
			Expect(anno).To(ContainSubstring("pki-hash-abc"))
		})

		It("renders ServiceAccount registry-webhook", func() {
			sa := f.KubernetesResource("ServiceAccount", "d8-system", "registry-webhook")
			Expect(sa.Exists()).To(BeTrue())
		})
	})

	Context("webhook cert absent (internal.webhook unset)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", webhookAbsentValues)
			f.HelmRender()
		})

		It("renders without error", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("does not render MutatingWebhookConfiguration", func() {
			mwc := f.KubernetesResource("MutatingWebhookConfiguration", "", "registry-modulesource.deckhouse.io")
			Expect(mwc.Exists()).To(BeFalse())
		})

		It("does not render Deployment registry-webhook", func() {
			dep := f.KubernetesResource("Deployment", "d8-system", "registry-webhook")
			Expect(dep.Exists()).To(BeFalse())
		})

		It("still renders Service registry-webhook (always-on)", func() {
			svc := f.KubernetesResource("Service", "d8-system", "registry-webhook")
			Expect(svc.Exists()).To(BeTrue())
		})
	})
})
