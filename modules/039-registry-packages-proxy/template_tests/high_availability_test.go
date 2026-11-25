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
	"github.com/flant/kube-client/manifest/releaseutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/sjson"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

const proxyNamespace = "d8-cloud-instance-manager"

var _ = Describe("Module :: registry-packages-proxy :: helm template :: high availability", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValuesBootstrapped)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.discovery.clusterMasterCount", 3)
		f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
	})

	assertRenderedState := func(replicas int64) {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		deployment := f.KubernetesResource("Deployment", proxyNamespace, "registry-packages-proxy")
		Expect(deployment.Exists()).To(BeTrue())
		Expect(deployment.Field("spec.replicas").Int()).To(BeEquivalentTo(replicas))
		Expect(deployment.Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution.0.topologyKey").String()).To(Equal("kubernetes.io/hostname"))
		Expect(deployment.Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution.0.labelSelector.matchLabels.app").String()).To(Equal("registry-packages-proxy"))

		pdb := f.KubernetesResource("PodDisruptionBudget", proxyNamespace, "registry-packages-proxy")
		Expect(pdb.Exists()).To(BeTrue())
		Expect(pdb.Field("spec.maxUnavailable").Int()).To(BeEquivalentTo(1))
	}

	Context("With globally disabled high availability", func() {
		BeforeEach(func() {
			f.ValuesSet("global.highAvailability", false)
		})

		It("Should render one replica and allow its eviction by default", func() {
			f.HelmRender()

			assertRenderedState(1)
		})

		It("Should enable high availability for this module", func() {
			f.ValuesSet("registryPackagesProxy.highAvailability", true)
			f.HelmRender()

			assertRenderedState(2)
		})
	})

	Context("With globally enabled high availability", func() {
		BeforeEach(func() {
			f.ValuesSet("global.highAvailability", true)
		})

		It("Should inherit high availability by default", func() {
			f.HelmRender()

			assertRenderedState(2)
		})

		It("Should disable high availability for this module", func() {
			f.ValuesSet("registryPackagesProxy.highAvailability", false)
			f.HelmRender()

			assertRenderedState(1)
		})
	})
})

func tolerationsByKey(deployment object_store.KubeObject) map[string][]string {
	byKey := map[string][]string{}
	for _, toleration := range deployment.Field("spec.template.spec.tolerations").Array() {
		key := toleration.Get("key").String()
		byKey[key] = append(byKey[key], toleration.Raw)
	}
	return byKey
}

// What decides PDB eviction is the number of masters, not the high-availability flag: on a single master
// there is nowhere else to run, so a drain cannot evict the proxy. Pod tolerations and anti-affinity stay
// fixed across master counts so joining masters do not roll the proxy mid-bootstrap.
var _ = Describe("Module :: registry-packages-proxy :: helm template :: placement on masters", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValuesBootstrapped)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.discovery.d8SpecificNodeCountByRole.master", 3)
		f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
	})

	deployment := func() object_store.KubeObject {
		deployment := f.KubernetesResource("Deployment", proxyNamespace, "registry-packages-proxy")
		Expect(deployment.Exists()).To(BeTrue())
		return deployment
	}

	assertPlacement := func(singleMaster bool) {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		maxUnavailable := int64(1)
		if singleMaster {
			maxUnavailable = 0
		}
		pdb := f.KubernetesResource("PodDisruptionBudget", proxyNamespace, "registry-packages-proxy")
		Expect(pdb.Exists()).To(BeTrue())
		Expect(pdb.Field("spec.maxUnavailable").Int()).To(Equal(maxUnavailable))

		tolerations := tolerationsByKey(deployment())
		for _, key := range []string{
			"node-role.kubernetes.io/control-plane",
			"node.deckhouse.io/uninitialized",
			"node.deckhouse.io/csi-not-bootstrapped",
			"node.kubernetes.io/not-ready",
		} {
			Expect(tolerations).To(HaveKey(key))
		}
		for _, key := range []string{"", "node.deckhouse.io/etcd-arbiter", "node.kubernetes.io/out-of-disk", "ToBeDeletedTaint", "ToBeDeletedByClusterAutoscaler"} {
			Expect(tolerations).NotTo(HaveKey(key))
		}

		Expect(tolerations["node.kubernetes.io/unreachable"]).To(ConsistOf(MatchJSON(`{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":60}`)))
		Expect(tolerations).NotTo(HaveKey("node.kubernetes.io/unschedulable"))
	}

	assertReplicas := func(replicas int64) {
		Expect(deployment().Field("spec.replicas").Int()).To(Equal(replicas))
	}

	Context("With the master count not discovered", func() {
		BeforeEach(func() {
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("lets the proxy move and keeps the helm_lib replica count", func() {
			assertPlacement(false)
			assertReplicas(3)
		})
	})

	Context("With a single master", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 1)
			f.HelmRender()
		})

		It("keeps the single-master PDB while leaving pod tolerations invariant", func() {
			assertPlacement(true)
			assertReplicas(1)
		})
	})

	Context("With a single master and high availability enabled", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 1)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("goes by the master count, not the HA flag", func() {
			assertPlacement(true)
			assertReplicas(1)
		})
	})

	Context("When additional masters join during bootstrap", func() {
		renderTemplate := func(masters int, disableHA bool, expectedReplicas int64) string {
			f.ValuesSetFromYaml("global", globalValuesNotBootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("global.discovery.clusterMasterCount", masters)
			f.ValuesSet("global.discovery.clusterControlPlaneIsHighlyAvailable", masters > 1)
			f.ValuesSet("global.discovery.d8SpecificNodeCountByRole.master", masters)

			if disableHA {
				f.ValuesSet("global.highAvailability", false)
			}

			f.ValuesSetFromYaml("registryPackagesProxy", customCertificatePresent)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			proxy := deployment()
			Expect(proxy.Field("spec.replicas").Int()).To(Equal(expectedReplicas))
			Expect(proxy.Field("spec.strategy.type").String()).To(Equal("RollingUpdate"))
			Expect(proxy.Field("spec.strategy.rollingUpdate.maxSurge").Raw).To(Equal("0"))
			Expect(proxy.Field("spec.strategy.rollingUpdate.maxUnavailable").Int()).To(Equal(int64(1)))
			Expect(proxy.Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution.0.labelSelector").Raw).To(MatchJSON(`{ "matchLabels": { "app": "registry-packages-proxy" } }`))
			Expect(proxy.Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution.0.topologyKey").String()).To(Equal("kubernetes.io/hostname"))

			return proxy.Field("spec.template").Raw
		}

		It("keeps spec.template identical across 1, 2, and 3 masters when HA is disabled", func() {
			one := renderTemplate(1, true, 1)
			Expect(renderTemplate(2, true, 1)).To(MatchJSON(one))
			Expect(renderTemplate(3, true, 1)).To(MatchJSON(one))
		})

		It("keeps spec.template identical across 1, 2, and 3 masters when HA is unset", func() {
			one := renderTemplate(1, false, 1)
			Expect(renderTemplate(2, false, 2)).To(MatchJSON(one))
			Expect(renderTemplate(3, false, 2)).To(MatchJSON(one))
		})

		It("runs one replica before the master count is discovered when HA is disabled", func() {
			renderTemplate(0, true, 1)
		})
	})

	Context("With two masters", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 2)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("keeps the replica count of three masters", func() {
			assertPlacement(false)
			assertReplicas(2)
		})
	})

	Context("With three masters", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 3)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("runs a quorum of replicas and replaces them one at a time", func() {
			assertPlacement(false)
			assertReplicas(2)
			Expect(deployment().Field("spec.strategy.rollingUpdate.maxSurge").Raw).To(Equal("0"))
			Expect(deployment().Field("spec.strategy.rollingUpdate.maxUnavailable").Int()).To(Equal(int64(1)))
		})
	})

	Context("With five masters", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 5)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("runs a quorum of replicas", func() {
			assertPlacement(false)
			assertReplicas(3)
		})
	})

	Context("With three masters and high availability disabled", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 3)
			f.ValuesSet("global.highAvailability", false)
			f.HelmRender()
		})

		It("goes by the master count and runs a single replica", func() {
			assertPlacement(false)
			assertReplicas(1)
		})
	})

	Context("With custom toleration keys", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 3)
		})

		It("ignores custom keys", func() {
			// HelmRender resets global.modules.placement, so the chart is rendered again directly.
			values, err := sjson.Set(f.ValuesGet("@this").Raw, "global.modules.placement.customTolerationKeys", []string{"example.com/custom"})
			Expect(err).ShouldNot(HaveOccurred())
			files, err := helm.Renderer{}.RenderChartFromDir("..", values)
			Expect(err).ShouldNot(HaveOccurred())

			rendered := make(object_store.ObjectStore)
			for _, manifests := range files {
				for _, doc := range releaseutil.SplitManifests(manifests) {
					obj := ManifestStringToUnstructed(doc)
					if obj == nil {
						continue
					}
					rendered.PutObject(obj.Object, object_store.NewMetaIndex(obj.GetKind(), obj.GetNamespace(), obj.GetName()))
				}
			}

			proxy := rendered.KubernetesResource("Deployment", proxyNamespace, "registry-packages-proxy")
			Expect(proxy.Exists()).To(BeTrue())
			Expect(tolerationsByKey(proxy)).NotTo(HaveKey("example.com/custom"))
		})
	})
})
