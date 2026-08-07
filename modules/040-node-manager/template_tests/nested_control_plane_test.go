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

// In a virtual control plane tenant Deckhouse manages the cluster from the parent, which is what
// global.deckhouseSelfHosted: false means. There the module owns the objects nodes bootstrap
// against and nothing else — the workloads run in the parent, so a Pod rendered here would have
// nowhere to run and would fight control-plane-manager for the same objects.
var _ = Describe("Module :: node-manager :: helm template :: nested control plane", func() {
	f := SetupHelmConfig(``)

	const bashibleContext = "clusterDomain: cluster.virtual\n"

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerValues)
		setBashibleAPIServerTLSValues(f)
	})

	Context("Tenant cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("global.deckhouseSelfHosted", false)
			f.ValuesSet("nodeManager.internal.bashibleContext", bashibleContext)
			f.HelmRender()
		})

		It("renders the objects bashible needs", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:node-manager:bashible:node-bootstrapped-nodes").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", "d8-cloud-instance-manager", "bashible-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Role", "default", "node-manager:kubernetes-api-proxy").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster").Exists()).To(BeTrue())
		})

		// The CRDs the module installs exist in a tenant, so the roles describing who may read and
		// write them have to exist there too. They also feed the role matrix in the documentation.
		It("renders the access levels for its own resources", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:node-manager:user").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:manage:permission:module:node-manager:view").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:manage:permission:module:node-manager:edit").Exists()).To(BeTrue())
		})

		It("renders no workload of its own", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "bashible-apiserver").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Service", "d8-cloud-instance-manager", "bashible-api").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-api-server-tls").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "node-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "node-group-exporter").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "capi-controller-manager").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "machine-controller-manager").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-cloud-instance-manager", "deckhouse-registry").Exists()).To(BeFalse())
		})

		// The context is assembled by bashible_context_vcp.go from the contract the host
		// publishes, because a tenant runs no node-controller Pod to write the Secret itself.
		It("renders the bashible context published by the hook", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			secret := f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-apiserver-context")
			Expect(secret.Exists()).To(BeTrue())
			Expect(getDecodedSecretValue(&secret, `input\.yaml`)).To(Equal(bashibleContext))
			Expect(secret.Field(`metadata.annotations.node-manager\.deckhouse\.io/context-owner`).String()).To(Equal("node-manager"))
		})

		// A virtual control plane authenticates bashible-apiserver as a User and issues its join
		// token with its own bootstrap group, neither of which exists in a normal cluster.
		It("grants the virtual control plane identities the same access", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:bashible-apiserver:auth-reader").
				Field("subjects").String()).To(MatchYAML(`
- kind: ServiceAccount
  name: bashible-apiserver
  namespace: d8-cloud-instance-manager
- kind: User
  name: bashible-apiserver
  apiGroup: rbac.authorization.k8s.io
`))

			Expect(f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible").
				Field("subjects").String()).To(MatchYAML(`
- kind: Group
  name: system:bootstrappers:d8-node-manager
  apiGroup: rbac.authorization.k8s.io
- kind: Group
  name: system:nodes
  apiGroup: rbac.authorization.k8s.io
- kind: Group
  name: system:bootstrappers:d8:vcp
  apiGroup: rbac.authorization.k8s.io
`))
		})
	})

	// Every failure path of bashible_context_vcp.go leaves the value unset, and the Secret must
	// not be rendered empty: bashible-apiserver would serve an empty context to every node.
	Context("Tenant cluster with no context published", func() {
		BeforeEach(func() {
			f.ValuesSet("global.deckhouseSelfHosted", false)
			f.HelmRender()
		})

		It("renders no context Secret and keeps the rest", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-apiserver-context").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-bashbooster").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Role", "d8-cloud-instance-manager", "bashible").Exists()).To(BeTrue())
		})
	})

	// The other half of the gate: a self-hosted cluster must see none of the above, whatever the
	// hook happens to have published.
	Context("Self-hosted cluster", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.internal.bashibleContext", bashibleContext)
			f.HelmRender()
		})

		It("keeps the context Secret to node-controller and the workloads in place", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("Secret", "d8-cloud-instance-manager", "bashible-apiserver-context").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "bashible-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "d8-cloud-instance-manager", "node-controller").Exists()).To(BeTrue())
		})

		It("grants access to the in-cluster identities only", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:bashible-apiserver:auth-reader").
				Field("subjects").String()).To(MatchYAML(`
- kind: ServiceAccount
  name: bashible-apiserver
  namespace: d8-cloud-instance-manager
`))

			Expect(f.KubernetesResource("RoleBinding", "d8-cloud-instance-manager", "bashible").
				Field("subjects").String()).To(MatchYAML(`
- kind: Group
  name: system:bootstrappers:d8-node-manager
  apiGroup: rbac.authorization.k8s.io
- kind: Group
  name: system:nodes
  apiGroup: rbac.authorization.k8s.io
`))
		})
	})
})
