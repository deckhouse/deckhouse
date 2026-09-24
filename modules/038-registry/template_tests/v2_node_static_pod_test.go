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
	"gopkg.in/yaml.v3"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

// The agent on an Engine node, which has no bashible step to install it.
//
// What is asserted here is mostly what would be silent if it were wrong. A node hands
// containerd's registry.d to the agent and stops writing it itself; if the pod that was
// supposed to take over cannot start — a name node-controller does not recognise, an
// image that has to be pulled, a directory mounted read-only that the agent writes — the
// node cannot pull anything at all, and cannot pull the thing that would fix it either.
var _ = Describe("Module :: registry :: helm template :: v2 node static pod", func() {
	f := SetupHelmConfig(``)

	renderWith := func(registryValues string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryValues)
		f.HelmRender()
	}

	// podSpec is the manifest the object carries, parsed.
	podSpec := func() map[string]any {
		request := f.KubernetesGlobalResource("NodeStaticPodRequest", "registry-agent")
		Expect(request.Exists()).To(BeTrue())

		var pod map[string]any
		Expect(yaml.Unmarshal([]byte(request.Field("spec.manifest").String()), &pod)).To(Succeed())
		return pod["spec"].(map[string]any)
	}

	container := func() map[string]any {
		containers := podSpec()["containers"].([]any)
		Expect(containers).To(HaveLen(1))
		return containers[0].(map[string]any)
	}

	Context("the module manages nothing", func() {
		BeforeEach(func() { renderWith(v2EnabledUnmanaged) })

		It("asks for no agent", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("NodeStaticPodRequest", "registry-agent").Exists()).To(BeFalse())
		})
	})

	Context("the previous implementation still owns the cluster", func() {
		BeforeEach(func() { renderWith(v2Disabled) })

		It("asks for no agent", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("NodeStaticPodRequest", "registry-agent").Exists()).To(BeFalse())
		})
	})

	// This module is 038 and the kind belongs to 040, so on the convergence of the release
	// that introduces it this one renders first. Failing there would take down every object
	// of the registry module, not just this one.
	Context("the kind does not exist in the cluster yet", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("global.discovery.apiVersions", `[]`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", v2Enabled)
			f.HelmRender()
		})

		It("renders the rest of the module and skips the request", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("NodeStaticPodRequest", "registry-agent").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-system", "registry-controller").Exists()).To(BeTrue(),
				"one missing kind must not take the whole release with it")
		})
	})

	Context("the module owns the pull path", func() {
		BeforeEach(func() { renderWith(v2Enabled) })

		It("publishes one request, under the name the platform recognises", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			request := f.KubernetesGlobalResource("NodeStaticPodRequest", "registry-agent")
			Expect(request.Exists()).To(BeTrue())

			// node-controller releases registry.d only to a node whose config carries a
			// static pod called exactly this (registryAgentStaticPodName). A rename here
			// leaves every Engine node writing its own registry.d forever, with an agent
			// running beside it that nothing routes through.
			Expect(request.Field("metadata.name").String()).To(Equal("registry-agent"))

			// No selector: node-controller already ignores the object for every group
			// bashible configures.
			Expect(request.Field("spec.nodeGroupSelector").Exists()).To(BeFalse())
		})

		It("names the image the containerd extension imports, never one to pull", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// The runtime has no registry configured until this pod writes one, so an
			// image it had to pull could never arrive. The extension imports this ref on
			// every boot: modules/007-registrypackages/images/containerd.
			Expect(container()["image"]).To(Equal("deckhouse.local/images:registry-agent"))
			Expect(container()["imagePullPolicy"]).To(Equal("IfNotPresent"))
		})

		It("serves the runtime on the loopback, in the host's namespace", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(podSpec()["hostNetwork"]).To(BeTrue())
			// Cluster DNS cannot start until its image is pulled, which is what this pod
			// makes possible — so it must not depend on it.
			Expect(podSpec()["dnsPolicy"]).To(Equal("Default"))

			args := container()["args"].([]any)
			Expect(args).To(ContainElement("--listen-address=" + constant.ProxyHost))
		})

		It("points the agent at the paths an Engine node actually has", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			args := container()["args"].([]any)
			// /run, not /etc/containerd: this is what nodelet renders into containerd's
			// config_path, and a drop-in written anywhere else is read by nobody.
			Expect(args).To(ContainElement("--containerd-registry-dir=/run/etc/containerd/registry.d"))
			Expect(args).To(ContainElement("--pki-dir=" + constant.AgentPKIPath))
			// The seed and the API credentials both come out of this document, because
			// nothing else on such a node can carry them privately.
			Expect(args).To(ContainElement("--bootstrap-node-config=/config/nodeconfig.yaml"))
		})

		It("lets the agent write the material it now generates itself", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			mounts := map[string]map[string]any{}
			for _, raw := range container()["volumeMounts"].([]any) {
				mount := raw.(map[string]any)
				mounts[mount["mountPath"].(string)] = mount
			}

			// Read-only on a bashible node, where the step generates it before the pod
			// starts. Here the agent is the writer, and a read-only mount is a node whose
			// agent cannot start at all.
			Expect(mounts).To(HaveKey("/etc/kubernetes/registry-agent"))
			Expect(mounts["/etc/kubernetes/registry-agent"]).ToNot(HaveKey("readOnly"))

			Expect(mounts).To(HaveKey("/run/etc/containerd/registry.d"))
			Expect(mounts["/run/etc/containerd/registry.d"]).ToNot(HaveKey("readOnly"))

			// The two it only reads.
			Expect(mounts["/config/nodeconfig.yaml"]["readOnly"]).To(BeTrue())
			Expect(mounts["/var/lib/kubelet/pki"]["readOnly"]).To(BeTrue())
		})

		It("mounts nothing that would expose the cluster authority's key", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, raw := range container()["volumeMounts"].([]any) {
				path := raw.(map[string]any)["mountPath"].(string)
				// On a master this directory holds ca.key. The agent takes the cluster
				// authority out of the node config instead.
				Expect(path).ToNot(Equal("/etc/kubernetes/pki"))
			}
		})

		It("creates the directories that do not exist before the agent runs", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			volumes := map[string]map[string]any{}
			for _, raw := range podSpec()["volumes"].([]any) {
				volume := raw.(map[string]any)
				volumes[volume["name"].(string)] = volume["hostPath"].(map[string]any)
			}

			// /etc/kubernetes is a tmpfs here and the agent is the only writer under it,
			// so Directory — what the bashible node's manifest uses — would leave the pod
			// pending forever on the first boot.
			Expect(volumes["agent-material"]["type"]).To(Equal("DirectoryOrCreate"))
			Expect(volumes["containerd-registry-d"]["type"]).To(Equal("DirectoryOrCreate"))
			Expect(volumes["layout-cache"]["type"]).To(Equal("DirectoryOrCreate"))

			// File, not FileOrCreate: an empty document would read as a node naming no
			// registry and no API server, and the agent would route from nothing while
			// reporting success.
			Expect(volumes["node-config"]["type"]).To(Equal("File"))
		})
	})
})
