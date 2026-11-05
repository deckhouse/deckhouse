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
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
enabledModules: ["vertical-pod-autoscaler", "prometheus", "operator-prometheus", "admission-policy-engine"]
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
  kubernetesVersion: "1.33.5"
  podSubnet: 10.111.0.0/16
  d8SpecificNodeCountByRole:
    system: 1
modules:
  placement: {}
`

var _ = Describe("Module :: cniSimpleBridge :: helm template :: ipv4-options-guard ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("cniSimpleBridge", `internal: {ipv4OptionsGuardMode: Audit, ipv4OptionsGuardDefaultMode: Audit}`)
	})

	It("runs the sidecar with its own metrics endpoint", func() {
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		ds := f.KubernetesResource("DaemonSet", "d8-cni-simple-bridge", "simple-bridge")
		Expect(ds.Field(`spec.template.spec.containers.#(name=="ipv4-options-guard").securityContext.privileged`).Bool()).To(BeTrue())
		Expect(ds.Field(`spec.template.spec.containers.#(name=="ipv4-options-guard").volumeMounts.#(name=="ipv4-options-guard-netns").mountPropagation`).String()).To(Equal("HostToContainer"))
		// The state is kept on the host /run, and nothing comes from the host /etc: it is read-only
		// on Deckhouse Engine, where a hostPath there keeps the whole CNI agent pod from starting.
		Expect(ds.Field(`spec.template.spec.volumes.#(name=="ipv4-options-guard-state").hostPath.path`).String()).To(Equal("/run/ipv4-options-guard"))
		Expect(ds.Field(`spec.template.spec.containers.#(name=="ipv4-options-guard").volumeMounts.#(name=="ipv4-options-guard-state").mountPath`).String()).To(Equal("/run/ipv4-options-guard"))
		var allowedHostPaths []string
		for _, allowed := range f.KubernetesResource("SecurityPolicyException", "d8-cni-simple-bridge", "simple-bridge").Field("spec.volumes.hostPath.allowedValues.#.path").Array() {
			allowedHostPaths = append(allowedHostPaths, allowed.String())
		}
		for _, mount := range ds.Field(`spec.template.spec.containers.#(name=="ipv4-options-guard").volumeMounts.#.name`).Array() {
			hostPath := ds.Field(`spec.template.spec.volumes.#(name=="` + mount.String() + `").hostPath.path`)
			if !hostPath.Exists() {
				continue
			}
			Expect(hostPath.String()).NotTo(HavePrefix("/etc"), "volume %s", mount.String())
			Expect(allowedHostPaths).To(ContainElement(hostPath.String()), "volume %s", mount.String())
		}
		Expect(ds.Field(`spec.template.spec.volumes.#(name=="ipv4-options-guard-netns").hostPath.path`).String()).To(Equal("/run/netns"))
		// The guard reuses the iptables-wrapper directory of the pod.
		Expect(ds.Field(`spec.template.spec.containers.#(name=="ipv4-options-guard").volumeMounts.#(name=="sbin").mountPath`).String()).To(Equal("/sbin"))
		Expect(ds.Field(`spec.template.spec.containers.#(name=="ipv4-options-guard-kube-rbac-proxy").ports.0.containerPort`).Int()).To(Equal(int64(4261)))

		Expect(f.KubernetesResource("ConfigMap", "d8-cni-simple-bridge", "ipv4-options-guard").Field("data.settings\\.json").String()).To(MatchJSON(`{"mode":"Audit"}`))
		Expect(f.KubernetesResource("ConfigMap", "d8-cni-simple-bridge", "ipv4-options-guard").Field("data.defaultMode").String()).To(Equal("Audit"))
		Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:cni-simple-bridge:ipv4-options-guard:rbac-proxy").Exists()).To(BeTrue())
		Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "cni-simple-bridge-ipv4-options-guard").Exists()).To(BeTrue())
		Expect(f.KubernetesResource("SecurityPolicyException", "d8-cni-simple-bridge", "simple-bridge").Field("spec.network.hostPorts.#(port==4261)").Exists()).To(BeTrue())
	})

	It("passes the settings to the sidecar", func() {
		f.ValuesSetFromYaml("cniSimpleBridge.ipv4OptionsGuard", `{mode: Enforce, disableMPTCP: false, allowKernelModuleLoad: true}`)
		f.ValuesSet("cniSimpleBridge.internal.ipv4OptionsGuardMode", "Enforce")
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
		Expect(f.KubernetesResource("ConfigMap", "d8-cni-simple-bridge", "ipv4-options-guard").Field("data.settings\\.json").String()).
			To(MatchJSON(`{"mode":"Enforce","disableMPTCP":false,"allowKernelModuleLoad":true}`))
	})
})
