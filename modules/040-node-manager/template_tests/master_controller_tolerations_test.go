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

var masterControllers = []string{"node-controller", "capi-controller-manager", "machine-controller-manager", "bashible-apiserver"}

var quorumReplicaControllers = []string{"capi-controller-manager", "machine-controller-manager", "bashible-apiserver"}

func tolerationsByKey(deployment object_store.KubeObject) map[string][]string {
	byKey := map[string][]string{}
	for _, toleration := range deployment.Field("spec.template.spec.tolerations").Array() {
		key := toleration.Get("key").String()
		byKey[key] = append(byKey[key], toleration.Raw)
	}
	return byKey
}

var _ = Describe("Module :: node-manager :: helm template :: master controllers", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
		f.ValuesSet("nodeManager.internal.capiControllerManagerEnabled", true)
		setBashibleAPIServerTLSValues(f)
	})

	assertMasterControllers := func(singleMaster bool) {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		maxUnavailable := int64(1)
		if singleMaster {
			maxUnavailable = 0
		}

		for _, name := range masterControllers {
			pdb := f.KubernetesResource("PodDisruptionBudget", nodeManagerNamespace, name)
			Expect(pdb.Exists()).To(BeTrue(), name)
			Expect(pdb.Field("spec.maxUnavailable").Int()).To(Equal(maxUnavailable), name)
		}

		for _, name := range masterControllers {
			deployment := f.KubernetesResource("Deployment", nodeManagerNamespace, name)
			Expect(deployment.Exists()).To(BeTrue(), name)

			tolerations := tolerationsByKey(deployment)
			for _, key := range []string{
				"node-role.kubernetes.io/control-plane",
				"node.deckhouse.io/uninitialized",
				"node.deckhouse.io/csi-not-bootstrapped",
				"node.kubernetes.io/not-ready",
			} {
				Expect(tolerations).To(HaveKey(key), name)
			}
			for _, key := range []string{"", "node.deckhouse.io/etcd-arbiter", "node.kubernetes.io/out-of-disk", "ToBeDeletedTaint"} {
				Expect(tolerations).NotTo(HaveKey(key), name)
			}

			Expect(tolerations["node.kubernetes.io/unreachable"]).To(ConsistOf(MatchJSON(`{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":60}`)), name)
			Expect(tolerations).NotTo(HaveKey("node.kubernetes.io/unschedulable"), name)
		}
	}

	assertReplicas := func(replicas int64) {
		for _, name := range quorumReplicaControllers {
			deployment := f.KubernetesResource("Deployment", nodeManagerNamespace, name)
			Expect(deployment.Exists()).To(BeTrue(), name)
			Expect(deployment.Field("spec.replicas").Int()).To(Equal(replicas), name)
		}
	}

	Context("With the master count not discovered", func() {
		BeforeEach(func() {
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("lets a controller replica move and keeps the helm_lib replica count", func() {
			assertMasterControllers(false)
			assertReplicas(3)
		})
	})

	Context("With a single master", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 1)
			f.HelmRender()
		})

		It("keeps the single-master PDB while leaving pod tolerations invariant", func() {
			assertMasterControllers(true)
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
			assertMasterControllers(true)
			assertReplicas(1)
		})
	})

	Context("When additional masters join during bootstrap", func() {
		renderTemplates := func(masters int, disableHA, bootstrapped bool, expectedReplicas int64) map[string]string {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.clusterIsBootstrapped", bootstrapped)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("global.discovery.clusterMasterCount", masters)
			f.ValuesSet("global.discovery.clusterControlPlaneIsHighlyAvailable", masters > 1)
			f.ValuesSet("global.discovery.d8SpecificNodeCountByRole.master", masters)

			if disableHA {
				f.ValuesSet("global.highAvailability", false)
			}

			f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+nodeManagerAWS)
			f.ValuesSet("nodeManager.internal.capiControllerManagerEnabled", true)
			setBashibleAPIServerTLSValues(f)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			for _, name := range quorumReplicaControllers {
				deployment := f.KubernetesResource("Deployment", nodeManagerNamespace, name)
				Expect(deployment.Exists()).To(BeTrue(), name)
				term := deployment.Field("spec.template.spec.affinity.podAntiAffinity.requiredDuringSchedulingIgnoredDuringExecution.0")
				Expect(term.Get("topologyKey").String()).To(Equal("kubernetes.io/hostname"), name)
				Expect(term.Get("labelSelector.matchLabels.app").String()).To(Equal(name), name)
				Expect(deployment.Field("spec.replicas").Int()).To(Equal(expectedReplicas), name)
				Expect(deployment.Field("spec.strategy.type").String()).To(Equal("RollingUpdate"), name)
				Expect(deployment.Field("spec.strategy.rollingUpdate.maxSurge").Raw).To(Equal("0"), name)
			}

			checked := append([]string(nil), masterControllers...)

			if bootstrapped {
				exporter := f.KubernetesResource("Deployment", nodeManagerNamespace, "node-group-exporter")
				Expect(exporter.Exists()).To(BeTrue())
				Expect(exporter.Field("spec.template.spec.affinity.podAntiAffinity").Exists()).To(BeFalse())
				Expect(exporter.Field("spec.strategy.rollingUpdate.maxSurge").Raw).To(Equal("0"))
				Expect(exporter.Field("spec.strategy.rollingUpdate.maxUnavailable").Int()).To(Equal(int64(1)))

				checked = append(checked, "node-group-exporter")
			}

			templates := make(map[string]string, len(checked))
			for _, name := range checked {
				deployment := f.KubernetesResource("Deployment", nodeManagerNamespace, name)
				Expect(deployment.Exists()).To(BeTrue(), name)
				templates[name] = deployment.Field("spec.template").Raw
			}

			return templates
		}

		for _, tc := range []struct {
			name         string
			disableHA    bool
			bootstrapped bool
		}{
			{"pre-bootstrap when HA is disabled", true, false},
			{"pre-bootstrap when HA is unset", false, false},
			{"post-bootstrap when HA is disabled", true, true},
			{"post-bootstrap when HA is unset", false, true},
		} {
			It("keeps spec.template identical across 1, 2, and 3 masters "+tc.name, func() {
				one := renderTemplates(1, tc.disableHA, tc.bootstrapped, 1)

				for _, masters := range []int{2, 3} {
					expectedReplicas := int64(1)
					if !tc.disableHA {
						expectedReplicas = 2
					}

					got := renderTemplates(masters, tc.disableHA, tc.bootstrapped, expectedReplicas)
					for name, want := range one {
						Expect(got[name]).To(MatchJSON(want), name)
					}
				}
			})
		}

		It("runs one replica before the master count is discovered when HA is disabled", func() {
			renderTemplates(0, true, false, 1)
		})
	})

	Context("With two masters", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 2)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("keeps the replica count of three masters", func() {
			assertMasterControllers(false)
			assertReplicas(2)
		})
	})

	Context("With four masters", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 4)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("keeps the replica count of five masters", func() {
			assertReplicas(3)
		})
	})

	Context("With three masters", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 3)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("leaves cluster-autoscaler on the helm_lib tolerations", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			autoscaler := f.KubernetesResource("Deployment", nodeManagerNamespace, "cluster-autoscaler")
			Expect(autoscaler.Exists()).To(BeTrue())
			Expect(tolerationsByKey(autoscaler)).To(HaveKey("node.deckhouse.io/etcd-arbiter"))
		})

		It("lets one controller replica move to another master", func() {
			assertMasterControllers(false)
		})

		It("runs a quorum of replicas and replaces them one at a time", func() {
			assertReplicas(2)

			for _, name := range quorumReplicaControllers {
				deployment := f.KubernetesResource("Deployment", nodeManagerNamespace, name)
				Expect(deployment.Field("spec.strategy.rollingUpdate.maxSurge").Raw).To(Equal("0"), name)
				Expect(deployment.Field("spec.strategy.rollingUpdate.maxUnavailable").Int()).To(Equal(int64(1)), name)
			}
		})

		It("keeps three node-controller replicas and prefers one per master", func() {
			nodeController := f.KubernetesResource("Deployment", nodeManagerNamespace, "node-controller")
			Expect(nodeController.Field("spec.replicas").Int()).To(Equal(int64(3)))

			antiAffinity := nodeController.Field("spec.template.spec.affinity.podAntiAffinity")
			Expect(antiAffinity.Get("requiredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			preferred := antiAffinity.Get("preferredDuringSchedulingIgnoredDuringExecution.0.podAffinityTerm")
			Expect(preferred.Get("topologyKey").String()).To(Equal("kubernetes.io/hostname"))
			Expect(preferred.Get("labelSelector.matchLabels.app").String()).To(Equal("node-controller"))
		})
	})

	Context("With five masters", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 5)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("runs a quorum of replicas", func() {
			assertMasterControllers(false)
			assertReplicas(3)
		})
	})

	Context("With three masters and high availability disabled", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 3)
			f.ValuesSet("global.highAvailability", false)
			f.HelmRender()
		})

		It("goes by the master count, not the HA flag", func() {
			assertMasterControllers(false)
		})

		It("runs a single replica of every controller", func() {
			assertReplicas(1)

			nodeController := f.KubernetesResource("Deployment", nodeManagerNamespace, "node-controller")
			Expect(nodeController.Field("spec.replicas").Int()).To(Equal(int64(1)))
			Expect(nodeController.Field("spec.template.spec.containers.0.args").String()).To(ContainSubstring(`"--leader-elect=true"`))
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

			for _, name := range masterControllers {
				deployment := rendered.KubernetesResource("Deployment", nodeManagerNamespace, name)
				Expect(deployment.Exists()).To(BeTrue(), name)

				Expect(tolerationsByKey(deployment)).NotTo(HaveKey("example.com/custom"), name)
			}
		})
	})
})
