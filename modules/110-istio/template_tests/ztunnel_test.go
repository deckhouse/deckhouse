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

var _ = Describe("Module :: istio :: helm template :: ztunnel", func() {
	f := SetupHelmConfig(``)

	Context("Ambient mode enabled with global version 1.25.2 (supports ambient)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.HelmRender()
		})

		It("ztunnel resources should be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ztunnelDs := f.KubernetesResource("DaemonSet", "d8-istio", "ztunnel")
			Expect(ztunnelDs.Exists()).To(BeTrue())
			Expect(ztunnelDs.Field("spec.template.spec.serviceAccountName").String()).To(Equal("ztunnel"))

			ztunnelVpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ztunnel")
			Expect(ztunnelVpa.Exists()).To(BeTrue())
			Expect(ztunnelVpa.Field("spec.targetRef.name").String()).To(Equal("ztunnel"))
			Expect(ztunnelVpa.Field("spec.targetRef.kind").String()).To(Equal("DaemonSet"))

			ztunnelSa := f.KubernetesResource("ServiceAccount", "d8-istio", "ztunnel")
			Expect(ztunnelSa.Exists()).To(BeTrue())

			ztunnelPodMonitor := f.KubernetesResource("PodMonitor", "d8-monitoring", "ztunnel")
			Expect(ztunnelPodMonitor.Exists()).To(BeTrue())
			Expect(ztunnelPodMonitor.Field("spec.podMetricsEndpoints.0.port").String()).To(Equal("ztunnel-stats"))
		})
	})

	Context("Ambient mode enabled with global version 1.21.6 (does not support ambient)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.21.6")
			f.ValuesSet("istio.ambient.enabled", true)
			f.HelmRender()
		})

		It("ztunnel resources should NOT be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ztunnel").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ztunnel").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "ztunnel").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "ztunnel").Exists()).To(BeFalse())
		})
	})

	Context("Ambient mode disabled with global version 1.25.2", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", false)
			f.HelmRender()
		})

		It("ztunnel resources should NOT be created", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("DaemonSet", "d8-istio", "ztunnel").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-istio", "ztunnel").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", "d8-istio", "ztunnel").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "ztunnel").Exists()).To(BeFalse())
		})
	})

	Context("Ambient mode enabled with CRL and global version 1.29.6", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.29.6")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("istio.internal.crl", "-----BEGIN X509 CRL-----\nCRL\n-----END X509 CRL-----")
			f.HelmRender()
		})

		It("ztunnel should mount CRL and set CRL_PATH", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ztunnelDs := f.KubernetesResource("DaemonSet", "d8-istio", "ztunnel")
			Expect(ztunnelDs.Exists()).To(BeTrue())

			env := ztunnelDs.Field("spec.template.spec.containers.0.env").String()
			Expect(env).To(ContainSubstring(`"name":"CRL_PATH"`))
			Expect(env).To(ContainSubstring(`/var/run/secrets/istio/crl/ca-crl.pem`))

			mounts := ztunnelDs.Field("spec.template.spec.containers.0.volumeMounts").String()
			Expect(mounts).To(ContainSubstring(`"mountPath":"/var/run/secrets/istio/crl"`))
			Expect(mounts).To(ContainSubstring(`"name":"crl-volume"`))

			volumes := ztunnelDs.Field("spec.template.spec.volumes").String()
			Expect(volumes).To(ContainSubstring(`"name":"crl-volume"`))
			Expect(volumes).To(ContainSubstring(`"name":"istio-ca-crl"`))
			Expect(volumes).To(ContainSubstring(`"optional":true`))
		})

		It("cacerts secret should include ca-crl.pem", func() {
			secretCacerts := f.KubernetesResource("Secret", "d8-istio", "cacerts")
			Expect(secretCacerts.Exists()).To(BeTrue())
			Expect(secretCacerts.Field("data.ca-crl\\.pem").Exists()).To(BeTrue())
		})
	})

	Context("Ambient mode enabled with CRL and global version 1.25.2 (no peer CA CRL support)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.25.2")
			f.ValuesSet("istio.ambient.enabled", true)
			f.ValuesSet("istio.internal.crl", "-----BEGIN X509 CRL-----\nCRL\n-----END X509 CRL-----")
			f.HelmRender()
		})

		It("ztunnel should NOT mount CRL", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ztunnelDs := f.KubernetesResource("DaemonSet", "d8-istio", "ztunnel")
			Expect(ztunnelDs.Exists()).To(BeTrue())

			env := ztunnelDs.Field("spec.template.spec.containers.0.env").String()
			Expect(env).NotTo(ContainSubstring(`"name":"CRL_PATH"`))

			mounts := ztunnelDs.Field("spec.template.spec.containers.0.volumeMounts").String()
			Expect(mounts).NotTo(ContainSubstring(`"mountPath":"/var/run/secrets/istio/crl"`))

			volumes := ztunnelDs.Field("spec.template.spec.volumes").String()
			Expect(volumes).NotTo(ContainSubstring(`"name":"crl-volume"`))
		})

		It("cacerts secret should still include ca-crl.pem", func() {
			secretCacerts := f.KubernetesResource("Secret", "d8-istio", "cacerts")
			Expect(secretCacerts.Exists()).To(BeTrue())
			Expect(secretCacerts.Field("data.ca-crl\\.pem").Exists()).To(BeTrue())
		})
	})

	Context("Ambient mode enabled without CRL and global version 1.29.6", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYamlWithOpenAPIDefaults("istio", istioValues)
			f.ValuesSet("istio.internal.globalVersion", "1.29.6")
			f.ValuesSet("istio.ambient.enabled", true)
			f.HelmRender()
		})

		It("ztunnel should NOT mount CRL", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ztunnelDs := f.KubernetesResource("DaemonSet", "d8-istio", "ztunnel")
			Expect(ztunnelDs.Exists()).To(BeTrue())

			env := ztunnelDs.Field("spec.template.spec.containers.0.env").String()
			Expect(env).NotTo(ContainSubstring(`"name":"CRL_PATH"`))

			volumes := ztunnelDs.Field("spec.template.spec.volumes").String()
			Expect(volumes).NotTo(ContainSubstring(`"name":"crl-volume"`))
		})
	})
})
