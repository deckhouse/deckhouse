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
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// Reboot and Drain are an operator's tool, so both RBAC systems have to hand
// NodeOperation out the way they hand out NodeGroup.
var _ = Describe("Module :: node-manager :: helm template :: NodeOperation rights", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
		f.HelmRender()
	})

	It("lets the manage roles read, create and approve operations", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		view := f.KubernetesGlobalResource("ClusterRole", "d8:system-capability:node-manager:view")
		edit := f.KubernetesGlobalResource("ClusterRole", "d8:system-capability:node-manager:edit")
		Expect(view.Exists()).To(BeTrue())
		Expect(edit.Exists()).To(BeTrue())

		Expect(nodeOperationVerbs(view)).To(ConsistOf("get", "list", "watch"))
		// patch approves a Disruption; the rest of the spec is immutable.
		Expect(nodeOperationVerbs(edit)).To(ConsistOf("create", "patch", "delete"))
	})

	It("lets the user-authz roles read, create and approve operations", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		user := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user")
		clusterEditor := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-editor")
		clusterAdmin := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:cluster-admin")
		Expect(user.Exists()).To(BeTrue())
		Expect(clusterEditor.Exists()).To(BeTrue())
		Expect(clusterAdmin.Exists()).To(BeTrue())

		Expect(nodeOperationVerbs(user)).To(ConsistOf("get", "list", "watch"))
		Expect(nodeOperationVerbs(clusterEditor)).To(ConsistOf("create", "patch", "delete"))
		Expect(nodeOperationVerbs(clusterAdmin)).To(ConsistOf("get", "list", "watch", "create", "patch", "delete"))
	})

	// The node reports with an update that carries the resourceVersion it read;
	// a patch would land on an operation that has meanwhile ended.
	It("lets a node only read operations and update their status", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		agent := f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:node-agent")
		Expect(agent.Exists()).To(BeTrue())
		Expect(nodeOperationVerbs(agent)).To(ConsistOf("get", "list", "watch"))
		Expect(roleVerbs(agent, "deckhouse.io", "nodeoperations/status")).To(ConsistOf("update"))
	})

	// Operations of one node are found by spec.nodeName, a selectable field;
	// a policy guarding a label nobody reads any more would only mislead.
	It("does not guard the retired node label", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", "nodeoperations-own-node-only.deckhouse.io")
		Expect(policy.Exists()).To(BeTrue())
		Expect(policy.Field("spec.validations").String()).NotTo(ContainSubstring("node-manager.deckhouse.io/node"))
	})

	// node-manager creates its Disruptions as this service account: a policy naming
	// any other refuses every one of them, and no node is interrupted for its config.
	It("leaves creating a Disruption to the service account node-controller runs as", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		Expect(f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "node-controller").Exists()).To(BeTrue())
		policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", "nodeoperations-disruption-by-node-manager.deckhouse.io")
		Expect(policy.Exists()).To(BeTrue())
		Expect(policy.Field("spec.validations.0.expression").String()).
			To(ContainSubstring(`"system:serviceaccount:d8-cloud-instance-manager:node-controller"`))
		binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "nodeoperations-disruption-by-node-manager.deckhouse.io")
		Expect(binding.Field("spec.validationActions").String()).To(ContainSubstring("Deny"))
	})

	// node-manager creates the Drain of a deleted machine as this service account.
	// A policy naming any other refuses that Drain, and the machine waits on its
	// pre-drain hook for good.
	It("leaves the label of a deleted machine's Drain to the service account node-controller runs as", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		Expect(f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "node-controller").Exists()).To(BeTrue())
		policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", "nodeoperations-machine-drain-by-node-manager.deckhouse.io")
		Expect(policy.Exists()).To(BeTrue())
		Expect(policy.Field("spec.matchConditions.0.expression").String()).
			To(ContainSubstring(`"node-manager.deckhouse.io/machine-uid"`))
		Expect(policy.Field("spec.validations.0.expression").String()).
			To(ContainSubstring(`"system:serviceaccount:d8-cloud-instance-manager:node-controller"`))
		binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "nodeoperations-machine-drain-by-node-manager.deckhouse.io")
		Expect(binding.Field("spec.policyName").String()).To(Equal("nodeoperations-machine-drain-by-node-manager.deckhouse.io"))
		Expect(binding.Field("spec.validationActions").String()).To(ContainSubstring("Deny"))
	})
})

func nodeOperationVerbs(role object_store.KubeObject) []string {
	return roleVerbs(role, "deckhouse.io", "nodeoperations")
}
