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

// The ClusterIssuer grant registration consists of objects of the multitenancy-manager CRDs, so
// these tests check that the cert-manager release carries them exactly while that module is enabled
// and its CRDs are served, under the names a ClusterResourceGrantPolicy refers to.

const (
	grantableDefinitionAPI = "multitenancy.deckhouse.io/v1alpha1/GrantableClusterResourceDefinition"
	grantableReferenceAPI  = "multitenancy.deckhouse.io/v1alpha1/GrantableClusterResourceReference"
)

var _ = Describe("Module :: cert-manager :: ClusterIssuer grants ::", func() {
	f := SetupHelmConfig(``)

	render := func(enabledModules string, apiVersions ...string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSetFromYaml("global.enabledModules", enabledModules)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("certManager", certManager)
		f.HelmRender(WithAPIVersions(apiVersions...))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	rendered := func() []bool {
		return []bool{
			f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers").Exists(),
			f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-certificate").Exists(),
			f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-ingress").Exists(),
		}
	}

	Context("multitenancy-manager is enabled and its CRDs are served", func() {
		BeforeEach(func() {
			render(`["vertical-pod-autoscaler", "multitenancy-manager"]`, grantableDefinitionAPI, grantableReferenceAPI)
		})

		It("registers ClusterIssuer with every issuer available by default", func() {
			definition := f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "clusterissuers")
			Expect(definition.Exists()).To(BeTrue())
			Expect(definition.Field("metadata.labels.module").String()).To(Equal("cert-manager"))
			Expect(definition.Field("spec.grantedResource").String()).To(MatchJSON(`{"apiGroup":"cert-manager.io","kind":"ClusterIssuer"}`))
			Expect(definition.Field("spec.enforcement").String()).To(Equal("Managed"))
			Expect(definition.Field("spec.defaultAvailability").String()).To(Equal("All"),
				"the registration alone must not take issuers away from projects")
		})

		It("checks and defaults the issuer of a Certificate that refers to a ClusterIssuer", func() {
			reference := f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-certificate")
			Expect(reference.Exists()).To(BeTrue())
			Expect(reference.Field("spec.grantableClusterResourceName").String()).To(Equal("clusterissuers"))
			Expect(reference.Field("spec.rule").String()).To(MatchJSON(`{"apiGroups":["cert-manager.io"],"apiVersions":["v1"],"resources":["certificates"]}`))
			Expect(reference.Field("spec.fieldPaths").String()).To(MatchJSON(`[{
				"path":"$.spec.issuerRef.name",
				"match":{"fieldPath":"$.spec.issuerRef.kind","equals":"ClusterIssuer"},
				"defaulting":"FillEmpty"
			}]`))
		})

		It("checks the cluster-issuer annotation of an Ingress without defaulting it", func() {
			reference := f.KubernetesGlobalResource("GrantableClusterResourceReference", "clusterissuers-ingress")
			Expect(reference.Exists()).To(BeTrue())
			Expect(reference.Field("spec.grantableClusterResourceName").String()).To(Equal("clusterissuers"))
			Expect(reference.Field("spec.rule").String()).To(MatchJSON(`{"apiGroups":["networking.k8s.io"],"apiVersions":["*"],"resources":["ingresses"]}`))
			Expect(reference.Field("spec.fieldPaths").String()).To(MatchJSON(`[{
				"path":"$.metadata.annotations['cert-manager.io/cluster-issuer']",
				"defaulting":"None"
			}]`))
		})
	})

	It("renders nothing while multitenancy-manager is disabled, even with its CRDs left in the cluster", func() {
		render(`["vertical-pod-autoscaler"]`, grantableDefinitionAPI, grantableReferenceAPI)
		Expect(rendered()).To(Equal([]bool{false, false, false}))
	})

	It("renders nothing while the multitenancy-manager CRDs are not served yet", func() {
		render(`["vertical-pod-autoscaler", "multitenancy-manager"]`)
		Expect(rendered()).To(Equal([]bool{false, false, false}))
	})
})
