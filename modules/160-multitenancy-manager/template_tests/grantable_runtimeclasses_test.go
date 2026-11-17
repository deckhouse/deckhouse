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

var _ = Describe("Module :: multitenancy-manager :: helm template :: runtimeclasses grant registration ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSetFromYaml("multitenancyManager.internal.admissionWebhookCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ValuesSetFromYaml("multitenancyManager.internal.quotaLedgerCert", `{"ca": "test-ca", "crt": "test", "key": "test"}`)
		f.ApplyOpenAPIDefaults()
		f.HelmRender()
	})

	It("registers RuntimeClass available to every project, without exclusions", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())
		def := f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "runtimeclasses")
		Expect(def.Exists()).To(BeTrue())
		Expect(def.Field("metadata.labels.module").String()).To(Equal("multitenancy-manager"))
		Expect(def.Field("spec.grantedResource").String()).To(MatchJSON(`{"apiGroup": "node.k8s.io", "kind": "RuntimeClass"}`))
		Expect(def.Field("spec.enforcement").String()).To(Equal("Managed"))
		Expect(def.Field("spec.defaultAvailability").String()).To(Equal("All"))
		Expect(def.Field("spec.excluded").Exists()).To(BeFalse())
		Expect(def.Field("spec.defaultFrom").Exists()).To(BeFalse())
		Expect(def.Field("spec.catalogFields").String()).To(MatchJSON(`[
  {"name": "handler", "path": "$.handler"},
  {"name": "overhead", "path": "$.overhead.podFixed"},
  {"name": "nodeSelector", "path": "$.scheduling.nodeSelector"}
]`))
	})

	It("checks spec.runtimeClassName of pods and of the pod template of every built-in workload", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		core := f.KubernetesGlobalResource("GrantableClusterResourceReference", "runtimeclasses-core")
		Expect(core.Field("spec.grantableClusterResourceName").String()).To(Equal("runtimeclasses"))
		Expect(core.Field("spec.rule").String()).To(MatchJSON(`{
  "apiGroups": [""], "apiVersions": ["v1"], "resources": ["pods", "replicationcontrollers", "podtemplates"]
}`))
		Expect(core.Field("spec.fieldPaths").String()).To(MatchJSON(`[
  {"resources": ["pods"], "path": "$.spec.runtimeClassName", "defaulting": "None"},
  {"resources": ["replicationcontrollers"], "path": "$.spec.template.spec.runtimeClassName", "defaulting": "None"},
  {"resources": ["podtemplates"], "path": "$.template.spec.runtimeClassName", "defaulting": "None"}
]`))

		apps := f.KubernetesGlobalResource("GrantableClusterResourceReference", "runtimeclasses-apps")
		Expect(apps.Field("spec.grantableClusterResourceName").String()).To(Equal("runtimeclasses"))
		Expect(apps.Field("spec.rule").String()).To(MatchJSON(`{
  "apiGroups": ["apps"], "apiVersions": ["v1"], "resources": ["deployments", "statefulsets", "daemonsets", "replicasets"]
}`))
		Expect(apps.Field("spec.fieldPaths").String()).To(MatchJSON(`[
  {"path": "$.spec.template.spec.runtimeClassName", "defaulting": "None"}
]`))

		batch := f.KubernetesGlobalResource("GrantableClusterResourceReference", "runtimeclasses-batch")
		Expect(batch.Field("spec.grantableClusterResourceName").String()).To(Equal("runtimeclasses"))
		Expect(batch.Field("spec.rule").String()).To(MatchJSON(`{
  "apiGroups": ["batch"], "apiVersions": ["v1"], "resources": ["jobs", "cronjobs"]
}`))
		Expect(batch.Field("spec.fieldPaths").String()).To(MatchJSON(`[
  {"resources": ["jobs"], "path": "$.spec.template.spec.runtimeClassName", "defaulting": "None"},
  {"resources": ["cronjobs"], "path": "$.spec.jobTemplate.spec.template.spec.runtimeClassName", "defaulting": "None"}
]`))
	})

	It("does not depend on any other module being enabled", func() {
		f.ValuesSetFromYaml("global.enabledModules", `[]`)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
		Expect(f.KubernetesGlobalResource("GrantableClusterResourceDefinition", "runtimeclasses").Exists()).To(BeTrue())
		for _, name := range []string{"runtimeclasses-core", "runtimeclasses-apps", "runtimeclasses-batch"} {
			Expect(f.KubernetesGlobalResource("GrantableClusterResourceReference", name).Exists()).To(BeTrue(), name)
		}
	})
})
