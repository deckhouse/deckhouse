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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const fencingSingleMasterVAPName = "fencing-single-master.deckhouse.io"

var _ = Describe("Module :: node-manager :: helm template :: fencing in a cluster with one master", func() {
	f := SetupHelmConfig(``)

	render := func(masters, systemMasters int) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.clusterIsBootstrapped", true)
		f.ValuesSet("global.discovery.clusterMasterCount", masters)
		f.ValuesSet("global.discovery.d8SpecificNodeCountByRole.master", systemMasters)
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		f.ValuesSetFromYaml("nodeManager.internal.fencing", `nodeGroups: {master: {mode: Disabled}, worker: {mode: Disabled}}`)
		f.ApplyOpenAPIDefaults()
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	// Without masters with the control plane label, the system nodes with the master role count, as in helm_lib.
	DescribeTable("checks the fencing mode of NodeGroups only when the cluster has one master",
		func(masters, systemMasters int, want bool) {
			render(masters, systemMasters)
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", fencingSingleMasterVAPName).Exists()).To(Equal(want))
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "fencing-single-master-update.deckhouse.io").Exists()).To(Equal(want))
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "fencing-single-master-create.deckhouse.io").Exists()).To(Equal(want))
		},
		Entry("one master", 1, 0, true),
		Entry("one master and other system nodes with the master role", 1, 3, true),
		Entry("three masters", 3, 0, false),
		Entry("no masters and one system master", 0, 1, true),
		Entry("no masters and two system masters", 0, 2, false),
		Entry("no masters and no system masters", 0, 0, false),
	)

	Context("one master", func() {
		BeforeEach(func() { render(1, 0) })

		admin := &user.DefaultInfo{Name: "admin@example.com"}

		DescribeTable("admits a NodeGroup",
			func(operation admission.Operation, oldMode, mode string, want bool) {
				gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "NodeGroup"}
				policy := compilePolicy(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", fencingSingleMasterVAPName).ToYaml(), gvk, "nodegroups")
				var oldObject map[string]any
				if operation == admission.Update {
					oldObject = testNodeGroup(oldMode)
				}
				Expect(policy.admits(admin, operation, "", oldObject, testNodeGroup(mode))).To(Equal(want))
			},
			Entry("turn on from Disabled", admission.Update, "Disabled", "Normal", false),
			Entry("turn on from no fencing", admission.Update, "", "Critical", false),
			Entry("turn on Watchdog", admission.Update, "Disabled", "Watchdog", false),
			Entry("turn on from an old value that works as Disabled", admission.Update, "Notify", "Slow", false),
			Entry("change the mode of a group with fencing", admission.Update, "Normal", "Slow", true),
			Entry("change another field of a group with fencing", admission.Update, "Normal", "Normal", true),
			Entry("edit a group with the deprecated Watchdog", admission.Update, "Watchdog", "Slow", true),
			Entry("turn off", admission.Update, "Slow", "Disabled", true),
			Entry("keep fencing off", admission.Update, "", "Disabled", true),
			Entry("create with fencing", admission.Create, "", "Normal", false),
			Entry("create without fencing", admission.Create, "", "", true),
		)

		It("denies the update in a bootstrapped cluster and only warns on the create", func() {
			update := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "fencing-single-master-update.deckhouse.io")
			Expect(update.Field("spec.policyName").String()).To(Equal(fencingSingleMasterVAPName))
			Expect(update.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))
			Expect(update.Field("spec.matchResources.resourceRules.#.operations|@flatten").String()).To(MatchJSON(`["UPDATE"]`))

			create := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "fencing-single-master-create.deckhouse.io")
			Expect(create.Field("spec.policyName").String()).To(Equal(fencingSingleMasterVAPName))
			Expect(create.Field("spec.validationActions").String()).To(MatchJSON(`["Warn"]`))
			Expect(create.Field("spec.matchResources.resourceRules.#.operations|@flatten").String()).To(MatchJSON(`["CREATE"]`))
		})

		// The installation applies its resources over the NodeGroups that node-controller already created.
		It("only warns on the update until the cluster is bootstrapped", func() {
			f.ValuesSet("global.clusterIsBootstrapped", false)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			update := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "fencing-single-master-update.deckhouse.io")
			Expect(update.Field("spec.validationActions").String()).To(MatchJSON(`["Warn"]`))
			Expect(update.Field("spec.matchResources.resourceRules.#.operations|@flatten").String()).To(MatchJSON(`["UPDATE"]`))
			create := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "fencing-single-master-create.deckhouse.io")
			Expect(create.Field("spec.validationActions").String()).To(MatchJSON(`["Warn"]`))
		})
	})
})
