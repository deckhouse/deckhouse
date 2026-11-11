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

// The default FlowSchema service-accounts of kube-apiserver, which the storage requests of Dex
// matched before.
const serviceAccountsFlowSchemaPrecedence = 9000

var _ = Describe("Module :: user-authn :: helm template :: Dex API priority and fairness", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
		hec.HelmRender()
	})

	It("Should put the storage requests of Dex, and only them, on a level of their own", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		schema := hec.KubernetesGlobalResource("FlowSchema", "d8-user-authn-dex")
		Expect(schema.Exists()).To(BeTrue())
		Expect(schema.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1"))
		Expect(schema.Field("spec.priorityLevelConfiguration.name").String()).To(Equal("d8-user-authn-dex"))
		Expect(schema.Field("spec.matchingPrecedence").Int()).To(BeNumerically("<", serviceAccountsFlowSchemaPrecedence),
			"the schema must match before the default service-accounts schema")

		rules := schema.Field("spec.rules").Array()
		Expect(rules).To(HaveLen(1))
		Expect(rules[0].Get("nonResourceRules").Exists()).To(BeFalse())

		subjects := rules[0].Get("subjects").Array()
		Expect(subjects).To(HaveLen(1))
		Expect(subjects[0].Get("kind").String()).To(Equal("ServiceAccount"))
		Expect(subjects[0].Get("serviceAccount.name").String()).To(Equal("dex"))
		Expect(subjects[0].Get("serviceAccount.namespace").String()).To(Equal("d8-user-authn"))

		resourceRules := rules[0].Get("resourceRules").Array()
		Expect(resourceRules).To(HaveLen(1))
		Expect(resourceRules[0].Get("apiGroups").Array()).To(HaveLen(1))
		Expect(resourceRules[0].Get("apiGroups.0").String()).To(Equal("dex.coreos.com"))
		Expect(resourceRules[0].Get("clusterScope").Bool()).To(BeFalse())
		Expect(resourceRules[0].Get("namespaces").Array()).To(HaveLen(1))
		Expect(resourceRules[0].Get("namespaces.0").String()).To(Equal("d8-user-authn"))
		Expect(verbsOf(resourceRules[0])).To(ConsistOf("get", "list", "create", "update", "delete"))
	})

	It("Should give the level a bounded share and queue what exceeds it", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		level := hec.KubernetesGlobalResource("PriorityLevelConfiguration", "d8-user-authn-dex")
		Expect(level.Exists()).To(BeTrue())
		Expect(level.Field("apiVersion").String()).To(Equal("flowcontrol.apiserver.k8s.io/v1"))
		Expect(level.Field("spec.type").String()).To(Equal("Limited"))
		Expect(level.Field("spec.limited.nominalConcurrencyShares").Int()).To(Equal(int64(20)))
		Expect(level.Field("spec.limited.lendablePercent").Int()).To(Equal(int64(50)))
		Expect(level.Field("spec.limited.limitResponse.type").String()).To(Equal("Queue"))

		queuing := level.Field("spec.limited.limitResponse.queuing")
		Expect(queuing.Get("handSize").Int()).To(BeNumerically("<=", queuing.Get("queues").Int()))
		Expect(queuing.Get("queueLengthLimit").Int()).To(BeNumerically(">", 0))
	})
})
