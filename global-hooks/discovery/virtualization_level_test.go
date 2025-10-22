// Copyright 2025 Flant JSC
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

		It("Set global value dvpNestingLevel to 0", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("0"))
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

		It("Set global value dvpNestingLevel to 0", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("0"))
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

		It("Set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("1"))
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

		It("Set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("3"))
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

		It("Set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("0"))
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

		It("set global value dvpNestingLevel", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.dvpNestingLevel").String()).To(Equal("0"))
		})
	})
})

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
