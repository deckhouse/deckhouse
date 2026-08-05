/*
Copyright 2021 Flant JSC

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
	"encoding/base64"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	globalValues = `
deckhouseVersion: test
enabledModules: ["vertical-pod-autoscaler", "prometheus", "operator-prometheus"]
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
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  kubernetesVersion: "1.32.0"
  d8SpecificNodeCountByRole:
    system: 1
modules:
  placement: {}
`

	globalValues2 = `
deckhouseVersion: test
enabledModules: ["vertical-pod-autoscaler", "prometheus", "operator-prometheus", "control-plane-manager"]
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
  clusterMasterCount: 3
  prometheusScrapeInterval: 30
  kubernetesVersion: "1.32.0"
  d8SpecificNodeCountByRole:
    system: 1
modules:
  placement: {}
`

	clusterIsBootstrapped = `
clusterIsBootstrapped: true
`

	moduleValuesForMasterNode = `
bundle: Default
logLevel: Info
internal:
  namespaces:
    - test
  webhookHandlerCert:
    crt: a
    key: b
    ca: c
  admissionWebhookCert:
    crt: a
    key: b
    ca: c
  currentReleaseImageName: test
registry:
  mode: Unmanaged
`

	moduleValuesForDeckhouseNode = `
bundle: Default
logLevel: Info
nodeSelector:
  node-role.kubernetes.io/deckhouse: ""
tolerations:
- key: testkey
  operator: Exists
internal:
  namespaces:
    - test
  webhookHandlerCert:
    crt: a
    key: b
    ca: c
  admissionWebhookCert:
    crt: a
    key: b
    ca: c
  currentReleaseImageName: test
registry:
  mode: Unmanaged
`
)

var _ = Describe("Module :: deckhouse :: helm template ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	Context("Cluster with deckhouse on master node", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, chartName)
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.template.spec.nodeSelector").String()).To(MatchYAML(`node-role.kubernetes.io/control-plane: ""`))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
  - operator: Exists
`))
		})
	})

	// What the Deckhouse controller fetches over HTTP must name something its own client can
	// dial, which is not the address image references point at.
	//
	// `base` is where image references point, and those are resolved by the container runtime,
	// which the registry module hands a drop-in redirecting any registry to its node agent —
	// so `base` may name a service nothing ever dials. A process has no such redirection.
	// Reading `base` here worked only for as long as the two were always the same value, and
	// taking them apart is what lets the pull path move without the release check and the
	// module source moving with it.
	//
	// The fixture gives them different values on purpose, so this is observable at all.
	Context("What the controller fetches itself", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues+clusterIsBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.HelmRender()
		})

		It("names what that client can dial, not the address images render from", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dialable := "registry.deckhouse.io/deckhouse/fe"
			renderedFrom := "registry.example.com"

			secret := f.KubernetesResource("Secret", "d8-system", "deckhouse-registry")
			Expect(secret.Exists()).To(BeTrue())

			// What an out-of-cluster caller reads is composed from `address` and `path`, never
			// from `base` — the fixture gives them different values so that a template reading
			// the wrong one shows up here.
			outside, err := base64.StdEncoding.DecodeString(secret.Field("data.imagesRegistry").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(outside)).To(Equal(dialable))
			Expect(string(outside)).ToNot(ContainSubstring(renderedFrom))

			// And what the controller in the cluster reads follows `base`, because that is
			// the address the registry module can move.
			inside, err := base64.StdEncoding.DecodeString(secret.Field("data.fetchRegistry").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(inside)).To(Equal(renderedFrom))

			// The module source follows `base`, because the images of every module under it
			// are rendered from this repository as well as fetched from it.
			source := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(source.Exists()).To(BeTrue())
			Expect(source.Field("spec.registry.repo").String()).To(Equal(renderedFrom + "/modules"))
		})
	})

	// The pull path having been taken over by the registry module.
	//
	// This is the case the whole separation exists for: a registry change made through that
	// module reconfigures the node agents and writes no registry address anywhere, so a
	// controller that read an address out of this secret would keep fetching from the registry
	// the cluster was installed with — forever, and silently, because the old registry usually
	// still answers.
	Context("When the registry module manages the pull path", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues+clusterIsBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			// As the global hook sets it once the module has published its address.
			f.ValuesSet("global.modulesImages.registry.base", constant.HostWithPath)
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.HelmRender()
		})

		It("fetches through the node agent, while images render from the in-cluster address", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			secret := f.KubernetesResource("Secret", "d8-system", "deckhouse-registry")
			fetchRegistry, err := base64.StdEncoding.DecodeString(secret.Field("data.fetchRegistry").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(fetchRegistry)).To(Equal(constant.HostWithPath))

			// The in-cluster address, not the loopback one the controller will dial. Nothing
			// that is recorded may name the loopback: it is node-local, and the translation
			// belongs to the one party that has to connect. See `utils.Dial`.
			Expect(string(fetchRegistry)).ToNot(ContainSubstring(constant.ProxyHost))

			// And the field that names the registry as seen from OUTSIDE the cluster keeps
			// doing that, whatever the module does to the pull path.
			//
			// dhctl reads it from wherever it is run and refuses to touch a cluster unless
			// the docker config carries credentials for the host it names. Pointed at the
			// loopback address it made the cluster unmanageable: `dhctl destroy` stopped at
			// "docker config doesn't contain 127.0.0.1:5001/system/deckhouse registry
			// credentials" before doing anything at all, and there is no address inside the
			// cluster that an out-of-cluster caller could use instead.
			imagesRegistry, err := base64.StdEncoding.DecodeString(secret.Field("data.imagesRegistry").String())
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(imagesRegistry)).To(Equal("registry.deckhouse.io/deckhouse/fe"))
			Expect(string(imagesRegistry)).ToNot(ContainSubstring(constant.ProxyHost))

			// The module source names the in-cluster address as well, and that matters more
			// here than anywhere else: this repository is not only fetched, it is what the
			// images of every module under it are rendered from. Naming the loopback address
			// put it into pod specs, where those pods ran only on content their nodes already
			// held — the agent answers 502 to a request naming itself as its own registry.
			source := f.KubernetesGlobalResource("ModuleSource", "deckhouse")
			Expect(source.Field("spec.registry.repo").String()).
				To(Equal(constant.HostWithPath + "/modules"))
			Expect(source.Field("spec.registry.repo").String()).ToNot(ContainSubstring(constant.ProxyHost))

			// And the images themselves keep naming the in-cluster address, which is the one
			// thing that must not become the loopback one: it is written into every pod spec
			// in the cluster, and a pod is not tied to the node whose agent answered for it.
			deployment := f.KubernetesResource("Deployment", "d8-system", "deckhouse")
			Expect(deployment.Field("spec.template.spec.containers.0.image").String()).
				ToNot(ContainSubstring(constant.ProxyHost))
		})

		// Without this the move is rendered and never applied. The annotation exists because
		// changes to fields other than `.dockerconfigjson` are otherwise not noticed, and the
		// address the controller fetches from is exactly such a field.
		It("changes the checksum, so the move is actually applied", func() {
			checksum := func() string {
				return f.KubernetesResource("Secret", "d8-system", "deckhouse-registry").
					Field("metadata.annotations.checksum").String()
			}

			moved := checksum()
			Expect(moved).ToNot(BeEmpty())

			// The same cluster before the module took the pull path over: only the address
			// images render from differs between the two renders.
			f.ValuesSet("global.modulesImages.registry.base", "registry.deckhouse.io/deckhouse/fe")
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(moved).ToNot(Equal(checksum()))
		})

		It("mounts the authority the agent is verified with from the node", func() {
			deployment := f.KubernetesResource("Deployment", "d8-system", "deckhouse")

			var mounted string
			for _, mount := range deployment.Field("spec.template.spec.containers.0.volumeMounts").Array() {
				if mount.Get("name").String() == "registry-agent-pki" {
					mounted = mount.Get("mountPath").String()
					Expect(mount.Get("readOnly").Bool()).To(BeTrue())
				}
			}
			// The same path inside the pod as on the node, so that one constant describes
			// both sides. The controller opens it by that constant.
			Expect(mounted).To(Equal(constant.AgentPKIPath),
				"the controller reads the agent authority by an absolute path, so the mount "+
					"has to put it exactly there")

			var source string
			for _, volume := range deployment.Field("spec.template.spec.volumes").Array() {
				if volume.Get("name").String() == "registry-agent-pki" {
					source = volume.Get("hostPath.path").String()
					// Not `File` and not `Directory`: on a cluster where the module manages
					// nothing this path does not exist, and a host path that names a missing
					// file is created as a directory — which would then be in the way of the
					// material the node generates later.
					Expect(volume.Get("hostPath.type").String()).To(Equal("DirectoryOrCreate"))
				}
			}
			Expect(source).To(Equal(constant.AgentPKIPath))
		})
	})

	Context("Cluster with deckhouse on system node", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			sa := f.KubernetesResource("ServiceAccount", nsName, chartName)
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(sa.Exists()).To(BeTrue())
			Expect(dp.Exists()).To(BeTrue())
			Expect(dp.Field("spec.template.spec.nodeSelector").String()).To(MatchYAML(`node-role.kubernetes.io/deckhouse: ""`))
			Expect(dp.Field("spec.template.spec.tolerations").String()).To(MatchYAML(`
- key: testkey
  operator: Exists
- key: node.deckhouse.io/bashible-uninitialized
  operator: Exists
  effect: NoSchedule
- key: node.deckhouse.io/uninitialized
  operator: Exists
  effect: NoSchedule
`))
		})
	})

	Context("Control-plane is managed by third-party team: use service by default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(dp.Exists()).To(BeTrue())
			serviceHostValue := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_HOST\").value",
			)
			Expect(serviceHostValue.Exists()).To(BeFalse())
			servicePort := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_PORT\").value",
			)
			Expect(servicePort.Exists()).To(BeFalse())
		})
	})

	Context("Managed by Deckhouse: use API-proxy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues2+clusterIsBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(dp.Exists()).To(BeTrue())
			serviceHostValue := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_HOST\").value",
			).String()
			Expect(serviceHostValue).To(Equal("127.0.0.1"))
			servicePort := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_PORT\").value",
			).String()
			Expect(servicePort).To(Equal("6445"))
		})
	})

	Context("Bootstrap phase: use direct connection", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues2)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForDeckhouseNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Everything must render properly", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(dp.Exists()).To(BeTrue())
			serviceHostValue := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_HOST\")." +
				"valueFrom.fieldRef.fieldPath",
			).String()
			Expect(serviceHostValue).To(Equal("status.hostIP"))
			servicePort := dp.Field("spec.template.spec.containers.0.env." +
				"#(name==\"KUBERNETES_SERVICE_PORT\").value",
			).String()
			Expect(servicePort).To(Equal("6443"))
		})
	})

	Context("SecurityPolicyException for the deckhouse pod", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.HelmRender()
		})

		nsName := "d8-system"
		chartName := "deckhouse"

		It("Pod label must point to an existing exception", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, chartName)
			speName := dp.Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).String()
			Expect(speName).NotTo(BeEmpty())
			Expect(f.KubernetesResource("SecurityPolicyException", nsName, speName).Exists()).To(BeTrue())
		})

		It("Every container must drop capabilities, otherwise the exception cannot cover it", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, chartName)
			for _, path := range []string{"spec.template.spec.containers", "spec.template.spec.initContainers"} {
				for _, container := range dp.Field(path).Array() {
					Expect(container.Get("securityContext.capabilities.drop").Array()).
						NotTo(BeEmpty(), "container %s", container.Get("name").String())
				}
			}
		})

		It("Every container port must be listed in the exception, the pod runs in the host network", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			dp := f.KubernetesResource("Deployment", nsName, chartName)
			Expect(dp.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())

			spe := f.KubernetesResource("SecurityPolicyException", nsName, chartName)
			allowedPorts := make(map[int64]bool)
			for _, port := range spe.Field("spec.network.hostPorts").Array() {
				allowedPorts[port.Get("port").Int()] = true
			}
			for _, container := range dp.Field("spec.template.spec.containers").Array() {
				for _, port := range container.Get("ports").Array() {
					Expect(allowedPorts).To(HaveKey(port.Get("containerPort").Int()))
				}
			}
		})
	})
})
