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

var perMasterReplicaControllers = []string{"capi-controller-manager", "machine-controller-manager", "bashible-apiserver"}

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

			if singleMaster {
				Expect(tolerations["node.kubernetes.io/unreachable"]).To(ConsistOf(MatchJSON(`{"key":"node.kubernetes.io/unreachable"}`)), name)
				Expect(tolerations["node.kubernetes.io/unschedulable"]).To(ConsistOf(MatchJSON(`{"key":"node.kubernetes.io/unschedulable","operator":"Exists","effect":"NoSchedule"}`)), name)
				continue
			}

			Expect(tolerations["node.kubernetes.io/unreachable"]).To(ConsistOf(MatchJSON(`{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":60}`)), name)
			Expect(tolerations).NotTo(HaveKey("node.kubernetes.io/unschedulable"), name)
		}
	}

	assertReplicas := func(replicas int64) {
		for _, name := range perMasterReplicaControllers {
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

		It("never evicts the controllers and lets them back onto the cordoned master", func() {
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

	Context("With two masters", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 2)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("runs a replica per master", func() {
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

		It("runs a replica per master", func() {
			assertReplicas(4)
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

		It("runs a replica per master and rolls out as helm_lib does", func() {
			assertReplicas(3)

			for _, name := range perMasterReplicaControllers {
				deployment := f.KubernetesResource("Deployment", nodeManagerNamespace, name)
				Expect(deployment.Field("spec.strategy.rollingUpdate.maxSurge").Int()).To(BeZero(), name)
				Expect(deployment.Field("spec.strategy.rollingUpdate.maxUnavailable").Int()).To(Equal(int64(2)), name)
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

		It("runs a replica per master", func() {
			assertMasterControllers(false)
			assertReplicas(5)
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
			Expect(nodeController.Field("spec.template.spec.containers.0.args").String()).To(ContainSubstring(`"--leader-elect=false"`))
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
