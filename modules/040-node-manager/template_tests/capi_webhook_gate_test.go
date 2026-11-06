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

var _ = Describe("Module :: node-manager :: helm template :: capi webhook gate", func() {
	f := SetupHelmConfig(``)

	const (
		mutatingKind   = "MutatingWebhookConfiguration"
		mutatingName   = "capi-mutating-webhook-configuration"
		validatingKind = "ValidatingWebhookConfiguration"
		validatingName = "capi-validating-webhook-configuration"
		checksumField  = `metadata.annotations.node-manager\.deckhouse\.io/capi-pod-template`
		dependency     = `metadata.annotations.werf\.io/deploy-dependency-deployment`
	)

	render := func(masters int, digest string) string {
		f.ValuesSet("global.discovery.clusterMasterCount", masters)
		f.ValuesSet("global.modulesImages.digests.nodeManager.capiControllerManager", digest)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		checksum := f.KubernetesGlobalResource(mutatingKind, mutatingName).Field(checksumField).String()
		Expect(checksum).NotTo(BeEmpty())
		Expect(f.KubernetesGlobalResource(validatingKind, validatingName).Field(checksumField).String()).To(Equal(checksum))

		return checksum
	}

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.highAvailability", true)
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
		f.ValuesSet("nodeManager.internal.capiControllerManagerEnabled", true)
		setBashibleAPIServerTLSValues(f)
	})

	It("waits for capi only when the webhooks change", func() {
		render(3, "sha256:one")

		for kind, name := range map[string]string{mutatingKind: mutatingName, validatingKind: validatingName} {
			Expect(f.KubernetesGlobalResource(kind, name).Field(dependency).String()).To(
				Equal("state=ready,group=apps,version=v1,kind=Deployment,name=capi-controller-manager,namespace=d8-cloud-instance-manager,external=true"), kind)
		}

		deployment := f.KubernetesResource("Deployment", nodeManagerNamespace, "capi-controller-manager")
		Expect(deployment.Field(`metadata.annotations.werf\.io/track-termination-mode`).String()).To(Equal("WaitUntilResourceReady"))
		Expect(deployment.Field(`metadata.annotations.werf\.io/fail-mode`).String()).To(Equal("FailWholeDeployProcessImmediately"))
	})

	It("changes the webhooks with the capi image", func() {
		Expect(render(3, "sha256:one")).NotTo(Equal(render(3, "sha256:two")))
	})

	It("keeps the webhooks when only the replica count changes", func() {
		three := render(3, "sha256:one")
		replicasOfThree := f.KubernetesResource("Deployment", nodeManagerNamespace, "capi-controller-manager").Field("spec.replicas").Int()

		five := render(5, "sha256:one")
		replicasOfFive := f.KubernetesResource("Deployment", nodeManagerNamespace, "capi-controller-manager").Field("spec.replicas").Int()

		Expect(replicasOfFive).NotTo(Equal(replicasOfThree))
		Expect(five).To(Equal(three))
	})
})
