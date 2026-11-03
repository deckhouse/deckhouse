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

const (
	stayOnUnreachable   = `{"key":"node.kubernetes.io/unreachable"}`
	leaveUnreachable    = `{"key":"node.kubernetes.io/unreachable","operator":"Exists","effect":"NoExecute","tolerationSeconds":60}`
	stayOnUnschedulable = `{"key":"node.kubernetes.io/unschedulable","operator":"Exists","effect":"NoSchedule"}`
)

type masterWorkload struct {
	kind string
	name string
	// A Terminating registry-storage replica counts as absent, and the storage update
	// controller then refuses to replace any other one, so it stays on an unreachable master.
	staysOnUnreachableMaster bool
}

var masterWorkloads = []masterWorkload{
	{"Deployment", "registry-controller", false},
	{"StatefulSet", "registry-storage", true},
}

func tolerationsByKey(workload object_store.KubeObject) map[string][]string {
	byKey := map[string][]string{}
	for _, toleration := range workload.Field("spec.template.spec.tolerations").Array() {
		key := toleration.Get("key").String()
		byKey[key] = append(byKey[key], toleration.Raw)
	}
	return byKey
}

// What decides is the number of masters, not the high-availability flag: on a single master
// there is nowhere else to run, so a cordon must not drain these workloads off it.
var _ = Describe("Module :: registry :: helm template :: placement on masters", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
	})

	assertPlacement := func(singleMaster bool) {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		maxUnavailable := int64(1)
		if singleMaster {
			maxUnavailable = 0
		}

		for _, w := range masterWorkloads {
			pdb := f.KubernetesResource("PodDisruptionBudget", "d8-system", w.name)
			Expect(pdb.Exists()).To(BeTrue(), w.name)
			Expect(pdb.Field("spec.maxUnavailable").Int()).To(Equal(maxUnavailable), w.name)

			workload := f.KubernetesResource(w.kind, "d8-system", w.name)
			Expect(workload.Exists()).To(BeTrue(), w.name)

			tolerations := tolerationsByKey(workload)
			for _, key := range []string{
				"node-role.kubernetes.io/control-plane",
				"node.deckhouse.io/uninitialized",
				"node.deckhouse.io/csi-not-bootstrapped",
				"node.kubernetes.io/not-ready",
			} {
				Expect(tolerations).To(HaveKey(key), w.name)
			}
			for _, key := range []string{"", "node.deckhouse.io/etcd-arbiter", "node.kubernetes.io/out-of-disk", "ToBeDeletedTaint", "ToBeDeletedByClusterAutoscaler"} {
				Expect(tolerations).NotTo(HaveKey(key), w.name)
			}

			if singleMaster {
				Expect(tolerations["node.kubernetes.io/unreachable"]).To(ConsistOf(MatchJSON(stayOnUnreachable)), w.name)
				Expect(tolerations["node.kubernetes.io/unschedulable"]).To(ConsistOf(MatchJSON(stayOnUnschedulable)), w.name)
				continue
			}

			Expect(tolerations).NotTo(HaveKey("node.kubernetes.io/unschedulable"), w.name)
			if w.staysOnUnreachableMaster {
				Expect(tolerations["node.kubernetes.io/unreachable"]).To(ConsistOf(MatchJSON(stayOnUnreachable)), w.name)
			} else {
				Expect(tolerations["node.kubernetes.io/unreachable"]).To(ConsistOf(MatchJSON(leaveUnreachable)), w.name)
			}
		}
	}

	Context("With the master count not discovered", func() {
		BeforeEach(func() {
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("lets the controller leave an unreachable master", func() {
			assertPlacement(false)
		})
	})

	Context("With a single master", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 1)
			f.HelmRender()
		})

		It("never evicts the workloads and lets them back onto the cordoned master", func() {
			assertPlacement(true)
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
		})
	})

	Context("With three masters", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 3)
			f.ValuesSet("global.highAvailability", true)
			f.HelmRender()
		})

		It("moves the controller off an unreachable master and leaves the storage replica on it", func() {
			assertPlacement(false)
		})

		It("prefers one controller replica per master", func() {
			controller := f.KubernetesResource("Deployment", "d8-system", "registry-controller")
			antiAffinity := controller.Field("spec.template.spec.affinity.podAntiAffinity")
			Expect(antiAffinity.Get("requiredDuringSchedulingIgnoredDuringExecution").Exists()).To(BeFalse())
			preferred := antiAffinity.Get("preferredDuringSchedulingIgnoredDuringExecution.0.podAffinityTerm")
			Expect(preferred.Get("topologyKey").String()).To(Equal("kubernetes.io/hostname"))
			Expect(preferred.Get("labelSelector.matchLabels.app").String()).To(Equal("registry-controller"))
		})
	})

	Context("With three masters and high availability disabled", func() {
		BeforeEach(func() {
			f.ValuesSet("global.discovery.clusterMasterCount", 3)
			f.ValuesSet("global.highAvailability", false)
			f.HelmRender()
		})

		It("goes by the master count, not the HA flag", func() {
			assertPlacement(false)
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

			for _, w := range masterWorkloads {
				workload := rendered.KubernetesResource(w.kind, "d8-system", w.name)
				Expect(workload.Exists()).To(BeTrue(), w.name)

				Expect(tolerationsByKey(workload)).NotTo(HaveKey("example.com/custom"), w.name)
			}
		})
	})
})
