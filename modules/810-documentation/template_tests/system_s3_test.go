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

var _ = Describe("Module :: documentation :: helm template :: module versions in the system S3 storage", func() {
	f := SetupHelmConfig(``)

	renderBootstrapped := func(s3 string, bootstrapped bool, enabledModules ...string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.clusterIsBootstrapped", bootstrapped)
		f.ValuesSet("global.enabledModules", append([]string{"vertical-pod-autoscaler", "documentation", "operator-prometheus-crd"}, enabledModules...))
		f.ValuesSet("documentation.internal.auth.password", "plainstring")
		f.ValuesSet("global.modules.ingress.enabled", false)
		if s3 != "" {
			f.ValuesSet("documentation.moduleStorage.s3.type", s3)
		}
		f.HelmRender()
	}
	render := func(s3 string, enabledModules ...string) {
		renderBootstrapped(s3, true, enabledModules...)
	}

	builderArgs := func() string {
		return f.KubernetesResource("Deployment", "d8-system", "documentation").Field(`spec.template.spec.containers.#(name=="builder").args`).String()
	}

	It("Keeps versions in the pod only by default", func() {
		render("", "system-s3")
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		Expect(f.KubernetesResource("SystemBucket", "d8-system", "documentation").Exists()).To(BeFalse())
		Expect(builderArgs()).NotTo(ContainSubstring("--ondemand-s3-dir"))
		// Without S3 every replica reads the registry itself.
		Expect(builderArgs()).NotTo(ContainSubstring("--ondemand-follow-interval"))
	})

	It("Asks system-s3 for a bucket and mounts its connection and the STS token", func() {
		render("System", "system-s3")
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		bucket := f.KubernetesResource("SystemBucket", "d8-system", "documentation")
		Expect(bucket.Exists()).To(BeTrue())
		Expect(bucket.Field("spec.size").String()).To(Equal("5Gi"))

		role := f.KubernetesResource("Role", "d8-system", "documentation:system-bucket")
		Expect(role.Field("rules").String()).To(MatchJSON(`[{"apiGroups":["storage.deckhouse.io"],"resources":["systembuckets/read","systembuckets/write"],"resourceNames":["documentation"],"verbs":["get"]}]`))
		binding := f.KubernetesResource("RoleBinding", "d8-system", "documentation:system-bucket")
		Expect(binding.Field("subjects").String()).To(MatchJSON(`[{"kind":"ServiceAccount","name":"documentation","namespace":"d8-system"}]`))

		Expect(builderArgs()).To(ContainSubstring("--ondemand-s3-dir=/etc/system-s3"))
		// With S3 one replica reads the registry and the other follows it.
		Expect(builderArgs()).To(ContainSubstring("--ondemand-follow-interval=1m"))
		// The eviction goes by the size of the bucket.
		Expect(builderArgs()).To(ContainSubstring("--ondemand-s3-size=5Gi"))
		pod := f.KubernetesResource("Deployment", "d8-system", "documentation").Field("spec.template.spec")
		Expect(pod.Get(`volumes.#(name=="s3-connection").configMap`).String()).To(MatchJSON(`{"name":"documentation-s3-connection","optional":true}`))
		Expect(pod.Get(`volumes.#(name=="s3-token").projected.sources.0.serviceAccountToken.audience`).String()).To(Equal("system-s3-sts"))
	})

	It("Keeps versions in the pod and raises an alert without the system-s3 module", func() {
		render("System")
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		Expect(f.KubernetesResource("SystemBucket", "d8-system", "documentation").Exists()).To(BeFalse())
		Expect(builderArgs()).NotTo(ContainSubstring("--ondemand-s3-dir"))

		rules := f.KubernetesResource("PrometheusRule", "d8-monitoring", "documentation-system-s3").Field("spec.groups").String()
		Expect(rules).To(ContainSubstring("D8DocumentationSystemS3Disabled"))
	})

	It("Asks for no bucket while there is no documentation pod", func() {
		renderBootstrapped("System", false, "system-s3")
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		Expect(f.KubernetesResource("Deployment", "d8-system", "documentation").Exists()).To(BeFalse())
		Expect(f.KubernetesResource("SystemBucket", "d8-system", "documentation").Exists()).To(BeFalse())
		Expect(f.KubernetesResource("Role", "d8-system", "documentation:system-bucket").Exists()).To(BeFalse())
	})

	It("Raises no alert with the system-s3 module", func() {
		render("System", "system-s3")
		Expect(f.KubernetesResource("PrometheusRule", "d8-monitoring", "documentation-system-s3").Exists()).To(BeFalse())
	})

	It("Restarts the pod on a new nginx config without pod-reloader", func() {
		render("")
		deployment := f.KubernetesResource("Deployment", "d8-system", "documentation")
		Expect(deployment.Field("metadata.annotations").String()).NotTo(ContainSubstring("pod-reloader"))
		nginx := deployment.Field(`spec.template.metadata.annotations.checksum/nginx-config`).String()
		Expect(nginx).NotTo(BeEmpty())
		Expect(deployment.Field(`spec.template.metadata.annotations.checksum/s3-connection`).Exists()).To(BeFalse())

		// The same values keep the pod: no restart on every helm run.
		render("")
		Expect(f.KubernetesResource("Deployment", "d8-system", "documentation").Field(`spec.template.metadata.annotations.checksum/nginx-config`).String()).To(Equal(nginx))
	})

	It("Restarts the pod on a new ConfigMap of system-s3", func() {
		f.ValuesSet("documentation.internal.s3ConnectionChecksum", "0d2b3f4c")
		render("System", "system-s3")
		annotation := f.KubernetesResource("Deployment", "d8-system", "documentation").Field(`spec.template.metadata.annotations.checksum/s3-connection`)
		Expect(annotation.String()).To(Equal("0d2b3f4c"))
	})

	It("Has no alert of an S3 storage not ready without S3", func() {
		render("")
		Expect(f.KubernetesResource("PrometheusRule", "d8-monitoring", "documentation-s3-not-ready").Exists()).To(BeFalse())
	})

	Context("An S3 storage of the user", func() {
		renderExternal := func(external map[string]any, enabledModules ...string) {
			f.ValuesSet("documentation.moduleStorage.s3.external", external)
			f.ApplyOpenAPIDefaults()
			render("External", enabledModules...)
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		}
		secretRef := map[string]any{"namespace": "d8-system", "name": "documentation-s3"}

		It("Mounts the copy of the Secret and keeps to the default prefix and size", func() {
			renderExternal(map[string]any{"secretRef": secretRef})

			args := builderArgs()
			Expect(args).To(ContainSubstring("--ondemand-s3-dir=/etc/documentation-s3"))
			Expect(args).To(ContainSubstring("--ondemand-s3-size=5Gi"))
			Expect(args).To(ContainSubstring("--ondemand-s3-prefix=documentation/"))

			pod := f.KubernetesResource("Deployment", "d8-system", "documentation").Field("spec.template.spec")
			// Optional: without the Secret the pod starts and keeps versions in itself.
			Expect(pod.Get(`volumes.#(name=="s3-connection").secret`).String()).To(MatchJSON(`{"secretName":"documentation-s3-connection","optional":true}`))
			Expect(pod.Get(`volumes.#(name=="s3-token")`).Exists()).To(BeFalse())
			Expect(pod.Get(`containers.#(name=="builder").volumeMounts.#(name=="s3-connection").mountPath`).String()).To(Equal("/etc/documentation-s3/connection"))
		})

		It("Copies the Secret the hook read into d8-system", func() {
			renderExternal(map[string]any{"secretRef": map[string]any{"namespace": "documentation-storage", "name": "documentation-s3"}})
			// The hook has not read the Secret yet.
			Expect(f.KubernetesResource("Secret", "d8-system", "documentation-s3-connection").Exists()).To(BeFalse())
			checksum := func() string {
				return f.KubernetesResource("Deployment", "d8-system", "documentation").Field(`spec.template.metadata.annotations.checksum/s3-connection`).String()
			}
			before := checksum()

			f.ValuesSet("documentation.internal.s3Connection", map[string]any{"S3_BUCKET": "ZG9jdW1lbnRhdGlvbg==", "AWS_ACCESS_KEY_ID": "QUtJRA=="})
			render("External")
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			copied := f.KubernetesResource("Secret", "d8-system", "documentation-s3-connection")
			Expect(copied.Field("data").String()).To(MatchJSON(`{"S3_BUCKET":"ZG9jdW1lbnRhdGlvbg==","AWS_ACCESS_KEY_ID":"QUtJRA=="}`))
			// docs-builder reads the connection once: a new copy restarts the pod.
			Expect(checksum()).NotTo(BeEmpty())
			Expect(checksum()).NotTo(Equal(before))
		})

		It("Asks for no system bucket and needs no system-s3 module", func() {
			renderExternal(map[string]any{"secretRef": secretRef})

			Expect(f.KubernetesResource("SystemBucket", "d8-system", "documentation").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Role", "d8-system", "documentation:system-bucket").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PrometheusRule", "d8-monitoring", "documentation-system-s3").Exists()).To(BeFalse())
		})

		It("Keeps to the storage of the user with the system-s3 module enabled", func() {
			renderExternal(map[string]any{"secretRef": secretRef}, "system-s3")

			Expect(f.KubernetesResource("SystemBucket", "d8-system", "documentation").Exists()).To(BeFalse())
			Expect(builderArgs()).To(ContainSubstring("--ondemand-s3-prefix=documentation/"))
		})

		It("Raises an alert when the storage stays not ready", func() {
			renderExternal(map[string]any{"secretRef": secretRef})

			rules := f.KubernetesResource("PrometheusRule", "d8-monitoring", "documentation-s3-not-ready").Field("spec.groups").String()
			Expect(rules).To(ContainSubstring("D8DocumentationS3NotReady"))
		})

		It("Passes the prefix and the size of the user", func() {
			renderExternal(map[string]any{"secretRef": secretRef, "prefix": "clusters/main/documentation/", "size": "20Gi"})

			Expect(builderArgs()).To(ContainSubstring("--ondemand-s3-prefix=clusters/main/documentation/"))
			Expect(builderArgs()).To(ContainSubstring("--ondemand-s3-size=20Gi"))
		})
	})
})
