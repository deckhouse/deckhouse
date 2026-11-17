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

var _ = Describe("Module :: multitenancy-manager :: helm template :: priorityclasses grants ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSetFromYaml("multitenancyManager.internal.admissionWebhookCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ValuesSetFromYaml("multitenancyManager.internal.quotaLedgerCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ApplyOpenAPIDefaults()
		f.HelmRender()
	})

	It("registers PriorityClass, available by default, without the platform classes and with the reserved ones closed", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())
		def := f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "priorityclasses")
		Expect(def.Exists()).To(BeTrue())
		Expect(def.Field("metadata.labels.module").String()).To(Equal("multitenancy-manager"))
		Expect(def.Field("spec.grantedResource.apiGroup").String()).To(Equal("scheduling.k8s.io"))
		Expect(def.Field("spec.grantedResource.kind").String()).To(Equal("PriorityClass"))
		Expect(def.Field("spec.enforcement").String()).To(Equal("Managed"))
		Expect(def.Field("spec.defaultAvailability").String()).To(Equal("All"))
		Expect(def.Field("spec.defaultFrom").Exists()).To(BeFalse())
		Expect(def.Field("spec.excluded").String()).To(MatchJSON(`[{"names": [
  "system-node-critical", "system-cluster-critical", "cluster-medium", "cluster-low", "standby"
]}]`))
		Expect(def.Field("spec.reserved").String()).To(MatchJSON(`[{"names": ["production-high", "deployment-machinery"]}]`))
	})

	It("registers the pod and every built-in pod template", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())
		for name, rule := range map[string]string{
			"priorityclasses-pod":   `{"apiGroups": [""], "apiVersions": ["v1"], "resources": ["pods", "replicationcontrollers", "podtemplates"]}`,
			"priorityclasses-apps":  `{"apiGroups": ["apps"], "apiVersions": ["v1"], "resources": ["deployments", "statefulsets", "daemonsets", "replicasets"]}`,
			"priorityclasses-batch": `{"apiGroups": ["batch"], "apiVersions": ["v1"], "resources": ["jobs", "cronjobs"]}`,
		} {
			ref := f.KubernetesGlobalResource("GrantableClusterResourceReference", name)
			Expect(ref.Exists()).To(BeTrue(), name)
			Expect(ref.Field("spec.grantableClusterResourceName").String()).To(Equal("priorityclasses"), name)
			Expect(ref.Field("spec.rule").String()).To(MatchJSON(rule), name)
		}
	})
})
