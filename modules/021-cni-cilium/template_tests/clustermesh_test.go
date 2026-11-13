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
	"slices"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const clusterMeshGlobalValues = `
deckhouseVersion: test
deckhouseEdition: EE
enabledModules: ["vertical-pod-autoscaler", "admission-policy-engine"]
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  kind: ClusterConfiguration
  clusterDomain: cluster.local
  clusterType: Static
  kubernetesVersion: "Automatic"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
discovery:
  clusterUUID: 6a1b8f3e-0d4c-4e2a-9b7f-1c2d3e4f5a6b
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  kubernetesVersion: "1.31.0"
  podSubnet: 10.111.0.0/16
  d8SpecificNodeCountByRole:
    system: 1
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
`

const clusterMeshCniCiliumValues = `
tunnelMode: VXLAN
resourcesManagement:
  mode: VPA
  vpa:
    mode: Auto
    cpu: {min: "50m", max: "2"}
    memory: {min: "256Mi", max: "2Gi"}
clusterMesh:
  enabled: true
  clusterID: 11
internal:
  mode: VXLAN
  ipv4OptionsGuardMode: Audit
  tunnelPortVXLAN: 4298
  masqueradeMode: BPF
  hubble:
    certs:
      ca: {cert: CERT, key: KEY}
      server: {ca: CA, key: KEY, cert: CERT}
    settings:
      extendedMetrics: {enabled: false, collectors: []}
      flowLogs: {enabled: false, allowFilterList: [], denyFilterList: [], fieldMaskList: [], fileMaxSizeMB: 10}
  clusterMesh:
    certs:
      ca: {cert: CA-CERT, key: CA-KEY}
      server: {cert: SERVER-CERT, key: SERVER-KEY}
      admin: {cert: ADMIN-CERT, key: ADMIN-KEY}
      local: {cert: LOCAL-CERT, key: LOCAL-KEY}
    hostAliases:
    - hostname: spb.peering.network.deckhouse.io
      ips: [10.0.0.1, 10.0.0.2]
`

var _ = Describe("Module :: cniCilium :: ClusterMesh render ::", func() {
	f := SetupHelmConfig(``)

	Context("ClusterMesh enabled, peering installed", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", clusterMeshGlobalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cniCilium", clusterMeshCniCiliumValues)
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "admission-policy-engine", "peering"]`)
			f.HelmRender(WithAPIVersions("peering.deckhouse.io/v1alpha1/PeeringCapability"))
		})

		It("renders the mesh", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-cni-cilium", "cilium-config")
			Expect(cm.Field("data.cluster-name").String()).To(Equal("6a1b8f3e0d4c4e2a9b7f1c2d3e4f5a6b"))
			Expect(cm.Field("data.cluster-id").String()).To(Equal("11"))
			Expect(cm.Field("data.clustermesh-config").String()).To(Equal("/var/lib/cilium/clustermesh/"))

			ds := f.KubernetesResource("DaemonSet", "d8-cni-cilium", "clustermesh-apiserver")
			Expect(ds.Exists()).To(BeTrue())
			Expect(ds.Field(`spec.template.spec.containers.#(name=="etcd").ports.0.hostPort`).Int()).To(Equal(int64(4294)))
			Expect(ds.Field(`spec.template.spec.containers.#(name=="etcd").image`).String()).To(Equal("registry.example.com@imageHash-cniCilium-clustermeshApiserver"))
			Expect(ds.Field(`spec.template.spec.initContainers.0.env.#(name=="INITIAL_CLUSTER_TOKEN").valueFrom.fieldRef.fieldPath`).String()).To(Equal("metadata.uid"))
			Expect(ds.Field("spec.template.spec.nodeSelector").String()).To(ContainSubstring("control-plane"))
			Expect(ds.Field("spec.template.spec.hostAliases").String()).To(MatchJSON(
				`[{"ip":"10.0.0.1","hostnames":["spb.peering.network.deckhouse.io"]},{"ip":"10.0.0.2","hostnames":["spb.peering.network.deckhouse.io"]}]`))
			for _, c := range []string{"apiserver", "kvstoremesh"} {
				args := ds.Field(`spec.template.spec.containers.#(name=="` + c + `").args`).String()
				Expect(args).NotTo(ContainSubstring("--enable-cilium-endpoint-slice"), c)
				Expect(args).NotTo(ContainSubstring("--enable-heartbeat"), c)
			}

			ctrl := f.KubernetesResource("Deployment", "d8-cni-cilium", "clustermesh-controller")
			Expect(ctrl.Exists()).To(BeTrue())
			Expect(ctrl.Field(`spec.template.spec.containers.0.env.#(name=="VXLAN_PORT").value`).String()).To(Equal("4298"))
			Expect(ctrl.Field(`spec.template.spec.containers.0.env.#(name=="POD_CIDRS").value`).String()).To(Equal("10.111.0.0/16"))
			Expect(ctrl.Field(`spec.template.spec.containers.0.env.#(name=="CLUSTER_ID").value`).String()).To(Equal("11"))
			Expect(ctrl.Field(`spec.template.spec.containers.0.env.#(name=="CLUSTER_NAME").value`).String()).To(Equal("6a1b8f3e0d4c4e2a9b7f1c2d3e4f5a6b"))
			Expect(ctrl.Field(`spec.template.spec.containers.0.env.#(name=="PEERING_CAPABILITY").value`).String()).To(Equal("network"))
			Expect(ctrl.Field(`spec.template.spec.containers.0.env.#(name=="PEERING_MODULE_ENABLED").value`).String()).To(Equal("true"))

			for _, s := range []string{"clustermesh-ca", "clustermesh-apiserver-server-cert", "clustermesh-apiserver-admin-cert", "clustermesh-apiserver-local-cert"} {
				Expect(f.KubernetesResource("Secret", "d8-cni-cilium", s).Exists()).To(BeTrue(), s)
			}
			Expect(f.KubernetesResource("Service", "d8-cni-cilium", "clustermesh-apiserver").Field("spec.sessionAffinity").String()).To(Equal("ClientIP"))
			Expect(f.KubernetesResource("SecurityPolicyException", "d8-cni-cilium", "clustermesh-apiserver").Field("spec.network.hostPorts.0.port").Int()).To(Equal(int64(4294)))

			fc := f.KubernetesGlobalResource("PeeringCapability", "network")
			Expect(fc.Exists()).To(BeTrue())
			Expect(fc.Field("spec.consumers.0.name").String()).To(Equal("clustermesh-controller"))

			agent := f.KubernetesResource("DaemonSet", "d8-cni-cilium", "agent")
			Expect(agent.Field(`spec.template.spec.volumes.#(name=="clustermesh-secrets").projected.sources.0.secret.name`).String()).To(Equal("cilium-clustermesh"))
			Expect(agent.Field(`spec.template.spec.containers.#(name=="cilium-agent").volumeMounts.#(name=="clustermesh-secrets").mountPath`).String()).To(Equal("/var/lib/cilium/clustermesh"))

			// EndpointSlice synchronization: the operator reads the peers like the agents and writes EndpointSlices.
			Expect(cm.Field("data.clustermesh-enable-endpoint-sync").String()).To(Equal("true"))
			operator := f.KubernetesResource("Deployment", "d8-cni-cilium", "operator")
			Expect(operator.Field(`spec.template.spec.volumes.#(name=="clustermesh-secrets").projected.sources.0.secret.name`).String()).To(Equal("cilium-clustermesh"))
			Expect(operator.Field(`spec.template.spec.containers.#(name=="operator").volumeMounts.#(name=="clustermesh-secrets").mountPath`).String()).To(Equal("/var/lib/cilium/clustermesh"))
			// The secrets are mounted 0400 root: without fsGroup the operator, running as deckhouse,
			// cannot read them and Cilium skips an unreadable config without a word.
			Expect(operator.Field(`spec.template.spec.securityContext.fsGroup`).Int()).To(Equal(int64(64535)))
			operatorRole := f.KubernetesGlobalResource("ClusterRole", "d8:cni-cilium:operator")
			Expect(operatorRole.Field(`rules.#(resources.0=="endpointslices").verbs`).String()).To(ContainSubstring(`"create"`))
			Expect(operatorRole.Field(`rules.#(resources.0=="events").verbs`).String()).To(ContainSubstring(`"create"`))
		})

		It("grants the components their RBAC", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			// Named after templates/clustermesh/<component>/rbac-for-us.yaml: dmt requires it for the
			// ServiceAccount and the Role, the cluster-wide names follow the same path by convention.
			for _, component := range []string{"apiserver", "controller"} {
				sa := "clustermesh-" + component
				Expect(f.KubernetesResource("ServiceAccount", "d8-cni-cilium", sa).Exists()).To(BeTrue(), sa)
				Expect(f.KubernetesGlobalResource("ClusterRole", "d8:cni-cilium:clustermesh:"+component).Exists()).To(BeTrue(), component)
				Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cni-cilium:clustermesh:"+component).Field("subjects.0.name").String()).To(Equal(sa))
			}
			Expect(f.KubernetesResource("Role", "d8-cni-cilium", "clustermesh:controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("RoleBinding", "d8-cni-cilium", "clustermesh:controller").Field("roleRef.name").String()).To(Equal("clustermesh:controller"))
			Expect(f.KubernetesResource("RoleBinding", "d8-cni-cilium", "clustermesh:controller").Field("subjects.0.name").String()).To(Equal("clustermesh-controller"))
		})
	})

	Context("user roles", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", clusterMeshGlobalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cniCilium", clusterMeshCniCiliumValues)
			f.ValuesSet("cniCilium.clusterMesh.enabled", false)
			f.HelmRender()
		})

		// rulesOn returns the verbs a ClusterRole grants on a network.deckhouse.io resource; the role must exist.
		rulesOn := func(name, resource string) []string {
			role := f.KubernetesGlobalResource("ClusterRole", name)
			Expect(role.Exists()).To(BeTrue(), name)
			var verbs []string
			for _, rule := range role.Field("rules").Array() {
				if !slices.ContainsFunc(rule.Get("apiGroups").Array(), func(g gjson.Result) bool { return g.String() == "network.deckhouse.io" }) {
					continue
				}
				if !slices.ContainsFunc(rule.Get("resources").Array(), func(r gjson.Result) bool { return r.String() == resource }) {
					continue
				}
				for _, v := range rule.Get("verbs").Array() {
					verbs = append(verbs, v.String())
				}
			}
			return verbs
		}
		read := []string{"get", "list", "watch"}
		write := []string{"create", "delete", "deletecollection", "patch", "update"}

		// The roles ship with the module whether the mesh is on or not, in every edition: without the CRDs of
		// the EE editions they grant nothing.
		It("let users read the mesh and cluster editors consent to peers", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			for _, resource := range []string{"clustermeshes", "clusternetworkpeers"} {
				Expect(rulesOn("d8:user-authz:cni-cilium:user", resource)).To(ConsistOf(read), resource)
				Expect(rulesOn("d8:system-capability:cni-cilium:view", resource)).To(ConsistOf(read), resource)
				Expect(rulesOn("d8:namespace-capability:cni-cilium:view", resource)).To(BeEmpty(), "cluster-scoped "+resource)
			}
			Expect(rulesOn("d8:user-authz:cni-cilium:cluster-editor", "clusternetworkpeers")).To(ConsistOf(write))
			Expect(rulesOn("d8:system-capability:cni-cilium:edit", "clusternetworkpeers")).To(ConsistOf(write))
			// ClusterMesh is written by clustermesh-controller only.
			for _, role := range []string{"d8:user-authz:cni-cilium:cluster-editor", "d8:user-authz:cni-cilium:cluster-admin", "d8:system-capability:cni-cilium:edit"} {
				Expect(rulesOn(role, "clustermeshes")).To(BeEmpty(), role)
			}
		})
	})

	// CSE gets the EE overlay, but not the peering module ClusterMesh needs.
	for _, edition := range []string{"SE-plus", "CSE"} {
		Context("ClusterMesh enabled in "+edition, func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global", clusterMeshGlobalValues)
				f.ValuesSet("global.modulesImages", GetModulesImages())
				f.ValuesSet("global.deckhouseEdition", edition)
				f.ValuesSetFromYaml("cniCilium", clusterMeshCniCiliumValues)
				f.HelmRender(WithAPIVersions("peering.deckhouse.io/v1alpha1/PeeringCapability"))
			})
			It("renders nothing of the mesh", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				cm := f.KubernetesResource("ConfigMap", "d8-cni-cilium", "cilium-config")
				Expect(cm.Field("data.cluster-name").String()).To(Equal("default"))
				Expect(cm.Field("data.cluster-id").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("DaemonSet", "d8-cni-cilium", "clustermesh-apiserver").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("Deployment", "d8-cni-cilium", "clustermesh-controller").Exists()).To(BeFalse())
				Expect(f.KubernetesGlobalResource("PeeringCapability", "network").Exists()).To(BeFalse())
				Expect(f.KubernetesResource("DaemonSet", "d8-cni-cilium", "agent").Field(`spec.template.spec.volumes.#(name=="clustermesh-secrets")`).Exists()).To(BeFalse())
			})
		})
	}

	Context("ClusterMesh enabled, no peering", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", clusterMeshGlobalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cniCilium", clusterMeshCniCiliumValues)
			f.HelmRender()
		})
		It("renders no capability and tells the controller", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("PeeringCapability", "network").Exists()).To(BeFalse())
			ctrl := f.KubernetesResource("Deployment", "d8-cni-cilium", "clustermesh-controller")
			Expect(ctrl.Field(`spec.template.spec.containers.0.env.#(name=="PEERING_MODULE_ENABLED").value`).String()).To(Equal("false"))
		})
	})

	Context("ClusterMesh enabled without VXLAN", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", clusterMeshGlobalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cniCilium", clusterMeshCniCiliumValues)
			f.ValuesSet("cniCilium.internal.mode", "DirectWithNodeRoutes")
			f.ValuesSet("cniCilium.bpfLBMode", "Hybrid")
			f.HelmRender()
		})
		It("fails with the reason", func() {
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).To(ContainSubstring("requires Cilium in VXLAN mode"))
		})
	})

	Context("ClusterMesh disabled", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", clusterMeshGlobalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("cniCilium", clusterMeshCniCiliumValues)
			f.ValuesSet("cniCilium.clusterMesh.enabled", false)
			f.HelmRender(WithAPIVersions("peering.deckhouse.io/v1alpha1/PeeringCapability"))
		})
		It("renders nothing of the mesh and keeps cluster-name default", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("ConfigMap", "d8-cni-cilium", "cilium-config").Field("data.cluster-name").String()).To(Equal("default"))
			Expect(f.KubernetesResource("ConfigMap", "d8-cni-cilium", "cilium-config").Field("data.cluster-id").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("DaemonSet", "d8-cni-cilium", "clustermesh-apiserver").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("PeeringCapability", "network").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("DaemonSet", "d8-cni-cilium", "agent").Field(`spec.template.spec.volumes.#(name=="clustermesh-secrets")`).Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ConfigMap", "d8-cni-cilium", "cilium-config").Field("data.clustermesh-enable-endpoint-sync").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cni-cilium", "operator").Field(`spec.template.spec.volumes.#(name=="clustermesh-secrets")`).Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-cni-cilium", "operator").Field(`spec.template.spec.securityContext.fsGroup`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:cni-cilium:operator").Field(`rules.#(resources.0=="endpointslices").verbs`).String()).NotTo(ContainSubstring(`"create"`))
		})
	})
})
