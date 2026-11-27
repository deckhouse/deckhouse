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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"sigs.k8s.io/yaml"

	constant "github.com/deckhouse/deckhouse/go_lib/registry/const"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

// A pod reaches the key, registry.d8-system.svc:5001, through the Service: the cache with it, the proxy
// for pods without it. Nodes never go there; they go through the agent.
var _ = Describe("Module :: registry :: helm template :: the key for pods", func() {
	f := SetupHelmConfig(``)

	render := func(registryValues string, set map[string]any) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryValues)
		for path, value := range set {
			f.ValuesSet(path, value)
		}
		f.HelmRender()
	}

	secretField := func(name, key string) string {
		decoded, err := base64.StdEncoding.DecodeString(
			f.KubernetesResource("Secret", "d8-system", name).Field("data." + gjson.Escape(key)).String())
		Expect(err).ShouldNot(HaveOccurred())
		return string(decoded)
	}

	service := func() gjson.Result {
		return f.KubernetesResource("Service", "d8-system", constant.ServiceName).Field("spec").Result
	}

	expectRoutable := func(selects string) {
		Expect(service().Get("clusterIP").Exists()).To(BeFalse(), "routable: a headless name dials the masters' 5001")
		Expect(service().Get("type").String()).To(Equal("ClusterIP"))
		if selects == "" {
			Expect(service().Get("selector").Exists()).To(BeFalse(),
				"the controller points it at the leader: a follower lacks what is published beside the image set")
		} else {
			Expect(service().Get("selector.app").String()).To(Equal(selects))
		}
		Expect(service().Get(`ports.#(port==5001).targetPort`).Int()).To(BeEquivalentTo(constant.StoragePort),
			"the key leads a pod to the store")
		Expect(service().Get(`ports.#(port==4281).targetPort`).Int()).To(BeEquivalentTo(constant.StoragePort),
			"StorageHost stays where its readers dial it")
	}

	Context("with the cache", func() {
		BeforeEach(func() { render(v2Enabled, nil) })

		It("leads the key to the cache, through the leader the controller names", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			expectRoutable("")
			Expect(f.KubernetesResource("Deployment", "d8-system", "registry-storage-proxy").Exists()).To(BeFalse())
		})
	})

	Context("without the cache", func() {
		BeforeEach(func() {
			render(v2ManagedNoCache, map[string]any{
				// The upstream the controller holds, with its credentials as it stores them.
				"registry.internal.v2.proxyUpstream.auth": base64.StdEncoding.EncodeToString(
					[]byte("license-token:dh-test-secret-0123456789abcdef")),
				"registry.internal.v2.proxyUpstream.ca": "UPSTREAM-CA",
				// The configured one, which a failed probe leaves unused: not what the proxy reads.
				"registry.internal.v2.registryConfig.primary.upstream.host": "typo.example.com",
			})
		})

		It("leads the key to the proxy for pods", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			expectRoutable("registry-storage-proxy")
		})

		It("is not rendered without an upstream to fetch from", func() {
			f.ValuesDelete("registry.internal.v2.proxyUpstream")
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Deployment", "d8-system", "registry-storage-proxy").Exists()).To(BeFalse())
		})

		It("runs the proxy in the pod network, keeping nothing on the node", func() {
			pod := f.KubernetesResource("Deployment", "d8-system", "registry-storage-proxy").Field("spec.template.spec").Result
			Expect(pod.Get("hostNetwork").Bool()).To(BeFalse())
			Expect(pod.Get(`volumes.#(hostPath)`).Exists()).To(BeFalse())
			Expect(pod.Get(`volumes.#(name=="data").emptyDir.sizeLimit`).String()).NotTo(BeEmpty())
			Expect(pod.Get("securityContext.runAsNonRoot").Bool()).To(BeTrue())
			Expect(pod.Get(`containers.#(name=="syncer")`).Exists()).To(BeFalse(), "nothing to fill")
		})

		It("proxies to the upstream with its account, keeping a miss briefly, and takes no writes", func() {
			var wrapper map[string]any
			Expect(yaml.Unmarshal([]byte(secretField("registry-storage-proxy-config", "deckhouse.yaml")), &wrapper)).To(Succeed())
			upstream := wrapper["upstream"].(map[string]any)
			Expect(upstream["address"]).To(Equal("registry.deckhouse.io"))
			Expect(upstream["path"]).To(Equal("/deckhouse/ee"))
			Expect(upstream["scheme"]).To(Equal("https"))
			Expect(upstream["username"]).To(Equal("license-token"))
			Expect(upstream["password"]).To(Equal("dh-test-secret-0123456789abcdef"))
			Expect(upstream["ca"]).To(Equal("/config/upstream-registry-ca.crt"))
			Expect(upstream["ttl"]).NotTo(BeEmpty())
			Expect(wrapper["scope"]).To(Equal("system/deckhouse"))
			Expect(wrapper).NotTo(HaveKey("writeEndpoint"))

			Expect(secretField("registry-storage-proxy-config", "upstream-registry-ca.crt")).To(Equal("UPSTREAM-CA"))

			var distribution map[string]any
			Expect(yaml.Unmarshal([]byte(secretField("registry-storage-proxy-config", "config.yaml")), &distribution)).To(Succeed())
			Expect(distribution).NotTo(HaveKey("proxy"), "the wrapper builds it from the upstream")
			Expect(distribution["http"].(map[string]any)["addr"]).To(Equal(":4281"))
		})

		It("reaches the upstream through the cluster's proxy, and the loopback directly", func() {
			render(v2ManagedNoCache, map[string]any{
				"global.clusterConfiguration": map[string]any{
					"apiVersion":        "deckhouse.io/v1",
					"kind":              "ClusterConfiguration",
					"clusterType":       "Static",
					"clusterDomain":     "cluster.local",
					"podSubnetCIDR":     "10.111.0.0/16",
					"serviceSubnetCIDR": "10.222.0.0/16",
					"proxy":             map[string]any{"httpsProxy": "http://proxy.example.com:3128"},
				},
			})
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			env := f.KubernetesResource("Deployment", "d8-system", "registry-storage-proxy").
				Field(`spec.template.spec.containers.#(name=="distribution").env`).Result
			Expect(env.Get(`#(name=="HTTPS_PROXY").value`).String()).To(Equal("http://proxy.example.com:3128"))
			Expect(env.Get(`#(name=="NO_PROXY").value`).String()).To(ContainSubstring("127.0.0.1"),
				"the token service and the upstream rewriter are on the loopback")
		})

		It("keeps the store's PKI, so its authority survives a restart, and nothing of the cache's readers", func() {
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-storage-pki-state").Exists()).To(BeTrue(),
				"regenerated on every restart, the authority in the global values would converge every module")
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-storage-auth-config").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-storage-client").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", "registry-storage-push").Exists()).To(BeFalse())
		})
	})

	Context("while the previous implementation's Service holds the name", func() {
		It("leaves it alone with the cache too: a node not on the agent yet pulls through it", func() {
			render(v2Enabled, map[string]any{"registry.internal.v2.legacyService": true})
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Service", "d8-system", constant.ServiceName).Exists()).To(BeFalse())
		})

		It("leaves it alone without the cache", func() {
			render(v2ManagedNoCache, map[string]any{"registry.internal.v2.legacyService": true})
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Service", "d8-system", constant.ServiceName).Exists()).To(BeFalse())
		})
	})

	Context("not managing the pull path", func() {
		BeforeEach(func() { render(v2EnabledUnmanaged, nil) })

		It("renders neither the Service nor the proxy", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Service", "d8-system", constant.ServiceName).Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Deployment", "d8-system", "registry-storage-proxy").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Secret", "d8-system", constant.InClusterAccessSecretName).Exists()).To(BeFalse())
		})
	})

	Context("the account and authority for the key", func() {
		published := map[string]any{
			"registry.internal.v2.imageAddress":    constant.HostWithPath,
			"registry.internal.v2.inClusterAccess": true,
		}

		It("are published once the Service serves, with the pods' account and the store's authority", func() {
			render(v2Enabled, published)
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(secretField(constant.InClusterAccessSecretName, "ca")).To(Equal("CA-CERT"))

			dockercfg := secretField(constant.InClusterAccessSecretName, "dockercfg")
			auth := gjson.Get(dockercfg, "auths."+gjson.Escape(constant.Host)+".auth").String()
			Expect(auth).To(Equal(base64.StdEncoding.EncodeToString([]byte("registry-pods:PODS-PASSWORD"))),
				"the pods' account, not the nodes' one, which also reads the module sources")
			Expect(dockercfg).NotTo(ContainSubstring("RO-PASSWORD"))
			Expect(dockercfg).NotTo(ContainSubstring("SYNC-PASSWORD"))
			Expect(dockercfg).NotTo(ContainSubstring("RW-PASSWORD"))
		})

		It("are published without the cache too", func() {
			render(v2ManagedNoCache, published)
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-system", constant.InClusterAccessSecretName).Exists()).To(BeTrue())
		})

		It("are not published before the Service serves", func() {
			render(v2Enabled, map[string]any{"registry.internal.v2.imageAddress": constant.HostWithPath})
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-system", constant.InClusterAccessSecretName).Exists()).To(BeFalse())
		})

		It("are not published before the image address", func() {
			render(v2Enabled, map[string]any{"registry.internal.v2.inClusterAccess": true})
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Secret", "d8-system", constant.InClusterAccessSecretName).Exists()).To(BeFalse())
		})
	})
})
