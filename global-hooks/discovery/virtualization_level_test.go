// Copyright 2022 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const noLabel = "noLabel"

var _ = Describe("Global hooks :: virtualization_level", func() {

	f := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)
	Context("Cluster without master nodes (unmanaged)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook runs successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with 1 hardware master node and 2 virtual ones", func() {
		nodesLevels := map[string]any{
			"kube-master-1": 0,
			"kube-master-2": 1,
			"kube-master-3": 1,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels)))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel to 0", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("0"))

			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("0"))
		})
	})

	Context("Cluster with 3 virtual master nodes", func() {
		nodesLevels := map[string]any{
			"kube-master-1": 1,
			"kube-master-2": 1,
			"kube-master-3": 1,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels)))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("1"))
			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("1"))
		})
	})

	Context("Cluster with 3 virtual master nodes and one node has a label with a faulty value", func() {
		nodesLevels := map[string]any{
			"kube-master-1": 3,
			"kube-master-2": "x",
			"kube-master-3": 4,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels)))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("3"))
			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("3"))
		})
	})

	Context("Cluster with 3 virtual master nodes and all nodes have falty labels", func() {
		nodesLevels := map[string]any{
			"kube-master-1": "x",
			"kube-master-2": "x",
			"kube-master-3": "x",
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels)))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("0"))
			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("0"))
		})
	})

	Context("Cluster with 3 virtual master nodes without labels", func() {
		nodesLevels := map[string]any{
			"kube-master-1": noLabel,
			"kube-master-2": noLabel,
			"kube-master-3": noLabel,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels)))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("0"))
			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("0"))
		})
	})

	Context("Cluster with 3 virtual master nodes and an empty configmap", func() {
		nodesLevels := map[string]any{
			"kube-master-1": 1,
			"kube-master-2": 1,
			"kube-master-3": 1,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels) + generateCMManigest("")))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("1"))
			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("1"))
		})
	})

	Context("Cluster with 3 virtual master nodes and a configmap overriding the nodes' labels", func() {
		nodesLevels := map[string]any{
			"kube-master-1": 1,
			"kube-master-2": 1,
			"kube-master-3": 1,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels) + generateCMManigest("2")))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("2"))
			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("2"))
		})
	})

	Context("Cluster with 3 virtual master nodes and a configmap with a faulty value", func() {
		nodesLevels := map[string]any{
			"kube-master-1": 3,
			"kube-master-2": 3,
			"kube-master-3": 3,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels) + generateCMManigest("x")))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("3"))
			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("3"))
		})
	})

	Context("Cluster with 3 virtual master nodes and a configmap having less value", func() {
		nodesLevels := map[string]any{
			"kube-master-1": 1,
			"kube-master-2": 1,
			"kube-master-3": 1,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels) + generateCMManigest("0")))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("1"))
			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("1"))
		})
	})

	Context("Cluster with 3 virtual master nodes and a configmap having greater value", func() {
		nodesLevels := map[string]any{
			"kube-master-1": 1,
			"kube-master-2": 1,
			"kube-master-3": 1,
		}

		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(generateMasterNodesManifests(nodesLevels) + generateCMManigest("2")))
			f.RunHook()
		})

		It("Create configmap, set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("2"))
			configmap := f.KubernetesResource("ConfigMap", "d8-system", "d8-virtualization-level")
			Expect(configmap.Field("data.level").String()).To(Equal("2"))
		})
	})
})

func generateCMManigest(level string) string {
	var result strings.Builder
	result.WriteString(`
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-virtualization-level
  namespace: d8-system
data:`)

	if len(level) > 0 {
		result.WriteString(fmt.Sprintf(`
  level: "%s"`, level))
	}

	return result.String()
}

func generateMasterNodesManifests(nodeLevels map[string]any) string {
	var manifests strings.Builder

	for name, level := range nodeLevels {
		node := v1.Node{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		}

		labels := make(map[string]string)
		labels[fmt.Sprintf("node-role.kubernetes.io/%s", nodeRole)] = ""
		labels[masterNodeGroup] = nodeRole

		switch l := level.(type) {
		case int:
			labels[virtualizationLevelKey] = fmt.Sprintf("%d", l)
		case string:
			if l != noLabel {
				labels[virtualizationLevelKey] = l
			}
		default:
			labels[virtualizationLevelKey] = "unknown value"
		}

		node.SetLabels(labels)

		y, err := yaml.Marshal(node)
		if err != nil {
			panic(err)
		}

		manifests.WriteString("\n---\n")
		manifests.Write(y)
	}

	return manifests.String()
}
