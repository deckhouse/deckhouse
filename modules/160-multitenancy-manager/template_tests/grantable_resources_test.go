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

var storageClassTemplateReferences = []string{
	"storageclasses-statefulset-claims",
	"storageclasses-statefulset-claims-annotation",
	"storageclasses-pod-ephemeral",
	"storageclasses-pod-ephemeral-annotation",
	"storageclasses-apps-ephemeral",
	"storageclasses-apps-ephemeral-annotation",
	"storageclasses-batch-ephemeral",
	"storageclasses-batch-ephemeral-annotation",
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: grantable resources ::", func() {
	f := SetupHelmConfig(``)

	render := func(enabledModules string, storageFoundationShipsStorageClassGrant bool) {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSetFromYaml("global.enabledModules", enabledModules)
		f.ValuesSet("multitenancyManager.internal.storageFoundationShipsStorageClassGrant", storageFoundationShipsStorageClassGrant)
		f.ValuesSetFromYaml("multitenancyManager.internal.admissionWebhookCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ValuesSetFromYaml("multitenancyManager.internal.quotaLedgerCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ApplyOpenAPIDefaults()
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	Context("while storage-foundation is disabled", func() {
		BeforeEach(func() {
			render(`["cert-manager", "multitenancy-manager"]`, false)
		})

		It("registers the cluster resources of Kubernetes itself", func() {
			for _, name := range []string{"storageclasses", "loadbalancerclasses", "clusterroles"} {
				Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", name).Exists()).To(BeTrue(), name)
			}
			for _, name := range []string{"storageclasses-pvc", "loadbalancerclasses-service", "clusterroles-rolebinding"} {
				Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", name).Exists()).To(BeTrue(), name)
			}
		})

		It("keeps the StorageClass objects when the release stops rendering them", func() {
			for _, obj := range []struct{ kind, name string }{
				{"GrantableClusterResourceDefinition", "storageclasses"},
				{"GrantableClusterResourceReference", "storageclasses-pvc"},
			} {
				Expect(f.KubernetesGlobalResource(obj.kind, obj.name).Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"), obj.name)
			}
			for _, name := range storageClassTemplateReferences {
				Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", name).Field(`metadata.annotations.helm\.sh/resource-policy`).String()).To(Equal("keep"), name)
			}
		})

		It("finds the cluster default StorageClass by both annotations Kubernetes reads", func() {
			defaultFrom := f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "storageclasses").Field("spec.defaultFrom")
			Expect(defaultFrom.Get("annotationKey").String()).To(Equal("storageclass.kubernetes.io/is-default-class"))
			Expect(defaultFrom.Get("additionalAnnotationKeys").String()).To(MatchJSON(`["storageclass.beta.kubernetes.io/is-default-class"]`))
		})

		It("leaves ClusterIssuer to the cert-manager module, even while it is enabled", func() {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-certificate").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-ingress").Exists()).To(BeFalse())
		})
	})

	Context("while storage-foundation is enabled at a version that does not register the StorageClass grant", func() {
		BeforeEach(func() {
			render(`["cert-manager", "storage-foundation", "multitenancy-manager"]`, false)
		})

		It("registers the StorageClass objects itself", func() {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "storageclasses").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "storageclasses-pvc").Exists()).To(BeTrue())
			for _, name := range storageClassTemplateReferences {
				Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", name).Exists()).To(BeTrue(), name)
			}
		})

		It("checks the class of the beta annotation before the data source entry of storageclasses-pvc", func() {
			ref := f.KubernetesGlobalResource("GrantableClusterResourceReference", "storageclasses-pvc")
			Expect(ref.Field("spec.fieldPaths.1.path").String()).To(Equal("$.metadata.annotations['volume.beta.kubernetes.io/storage-class']"))
			Expect(ref.Field("spec.fieldPaths.1.match.fieldPath").String()).To(Equal("$.metadata.annotations['volume.beta.kubernetes.io/storage-class']"))
			Expect(ref.Field("spec.fieldPaths.1.defaulting").String()).To(Equal("None"))
			Expect(ref.Field("spec.fieldPaths.2.match.fieldPath").String()).To(Equal("$.spec['dataSource','dataSourceRef'].kind"))
		})
	})

	Context("while storage-foundation registers the StorageClass grant", func() {
		BeforeEach(func() {
			render(`["cert-manager", "storage-foundation", "multitenancy-manager"]`, true)
		})

		It("leaves StorageClass to the storage-foundation module", func() {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "storageclasses").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", "storageclasses-pvc").Exists()).To(BeFalse())
			for _, name := range storageClassTemplateReferences {
				Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", name).Exists()).To(BeFalse(), name)
			}
		})

		It("still registers the other cluster resources of Kubernetes", func() {
			for _, name := range []string{"loadbalancerclasses", "clusterroles"} {
				Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", name).Exists()).To(BeTrue(), name)
			}
			for _, name := range []string{"loadbalancerclasses-service", "clusterroles-rolebinding"} {
				Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", name).Exists()).To(BeTrue(), name)
			}
		})
	})
})
