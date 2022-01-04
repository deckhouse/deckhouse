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

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/set"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: nodeManager :: hooks :: handle_node_templates_test ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.MetricsCollector.CollectedMetrics()).To(HaveLen(1), "expire group should exist on empty cluster")
		})
	})

	Context("NG without nodeTemplate and Cloud Node", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: CloudEphemeral
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  labels:
    node.deckhouse.io/group: wor-ker
    node-role.kubernetes.io/wor-ker: ""
spec:
  taints:
  - effect: NoSchedule
    key: node.deckhouse.io/uninitialized
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully; last-applied-node-template must be match to expectedJSON", func() {
			expectedJSON := `
			  {
			    "apiVersion": "v1",
			    "kind": "Node",
			    "metadata": {
			      "labels": {
			        "node.deckhouse.io/group": "wor-ker",
			        "node-role.kubernetes.io/wor-ker": ""
			      },
			      "name": "wor-ker"
			    },
                "spec": {}
			  }
			`
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "wor-ker").Parse().DropFields("status", "metadata.creationTimestamp")).
				To(MatchJSON(expectedJSON))
			Expect(f.MetricsCollector.CollectedMetrics()).Should(HaveLen(1), "should have only expire metric for managed node")
		})
	})

	Context("NG without nodeTemplate and minimal Static Node", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: Static
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  labels:
    node.deckhouse.io/group: wor-ker
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully; last-applied-node-template must be match to expectedJSON", func() {
			expectedJSON := `
			        {
			          "apiVersion": "v1",
			          "kind": "Node",
			          "metadata": {
			            "annotations": {
			              "node-manager.deckhouse.io/last-applied-node-template": "{\"annotations\":{},\"labels\":{},\"taints\":[]}"
			            },
			            "labels": {
			              "node.deckhouse.io/group": "wor-ker",
			              "node-role.kubernetes.io/wor-ker": "",
			              "node.deckhouse.io/type": "Static"
			            },
			            "name": "wor-ker"
			          },
			          "spec": {}
			        }
			`
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "wor-ker").Parse().DropFields("status", "metadata.creationTimestamp")).
				To(MatchJSON(expectedJSON))
		})
	})

	Context("Updated NG nodeTemplate and minimal Static Node", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: Static
  nodeTemplate:
    annotations:
      new: new
    labels:
      new: new
      node.deckhouse.io/group: wor-ker
    taints:
    - effect: NoSchedule
      key: new
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  labels:
    node.deckhouse.io/group: wor-ker
spec:
  taints:
  - key: node.deckhouse.io/uninitialized
    effect: NoSchedule
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully; new labels and annotations must be set", func() {
			expectedLastApplied := `
				{
					"labels": {
						"new": "new",
						"node.deckhouse.io/group": "wor-ker"
					},
					"annotations": {
						"new": "new"
					},
					"taints": [
						{
							"key": "new",
							"effect": "NoSchedule"
						}
					]
				}
			`
			lastApplied := f.KubernetesGlobalResource("Node", "wor-ker").Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).String()
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))
		})
	})

	Context("Minimal NG, Static Node with old labels, taints, annotations", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: Static
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  annotations:
    node-manager.deckhouse.io/last-applied-node-template: |
      {
        "labels": {
          "old-old": "old"
        },
        "annotations": {
          "old-old": "old"
        },
        "taints": [
          {
            "key": "old-old",
            "effect": "NoSchedule"
          }
        ]
      }
  labels:
    node.deckhouse.io/group: wor-ker
spec:
  taints:
  - key: node.deckhouse.io/uninitialized
    effect: NoSchedule
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully; annotations, labels and taints must be deleted", func() {
			expectedLastApplied := `
				{
					"annotations": {},
					"labels": {},
					"taints": []
				}
			`
			lastApplied := f.KubernetesGlobalResource("Node", "wor-ker").Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).String()
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))
		})
	})

	Context("Cluster with NG and Static Node", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: Static
  nodeTemplate:
    annotations:
      a: a
      new: new
    labels:
      a: a
      new: new
    taints:
    - key: a
      effect: NoSchedule
    - key: new
      effect: NoSchedule
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  annotations:
    node-manager.deckhouse.io/last-applied-node-template: |
      {
        "labels": {
          "a": "a"
        },
        "annotations": {
          "a": "a"
        },
        "taints": [
          {
            "key": "a",
            "effect": "NoSchedule"
          }
        ]
      }
  labels:
    node.deckhouse.io/group: wor-ker
spec:
  taints:
  - key: node.deckhouse.io/uninitialized
    effect: NoSchedule
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully; annotations, labels, taints must be updated", func() {
			expectedLastApplied := `
				{
					"annotations": {
						"a": "a",
						"new": "new"
					},
					"labels": {
						"a": "a",
						"new": "new"
					},
					"taints": [
						{
							"key": "a",
							"effect": "NoSchedule"
						},
						{
							"key": "new",
							"effect": "NoSchedule"
						}
					]
				}
			`
			lastApplied := f.KubernetesGlobalResource("Node", "wor-ker").Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).String()
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))
		})
	})

	Context("Cluster with NG and Static Node", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: Static
  nodeTemplate:
    annotations:
      a: a
    labels:
      a: a
    taints:
    - key: a
      effect: NoSchedule
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  annotations:
    node-manager.deckhouse.io/last-applied-node-template: |
      {
        "labels": {
          "a": "a",
          "old": "old"
        },
        "annotations": {
          "a": "a",
          "old": "old"
        },
        "taints": [
          {
            "key": "a",
            "effect": "NoSchedule"
          },
          {
            "key": "old",
            "effect": "NoSchedule"
          }
        ]
      }
  labels:
    node.deckhouse.io/group: wor-ker
spec:
  taints:
  - key: node.deckhouse.io/uninitialized
    effect: NoSchedule
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully; annotations, labels, taints must be updated; uninitialized taint must be deleted", func() {
			expectedLastApplied := `
				{
					"annotations": {
						"a": "a"
					},
					"labels": {
						"a": "a"
					},
					"taints": [
						{
							"key": "a",
							"effect": "NoSchedule"
						}
					]
				}
			`
			expectedYAML := `
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  annotations:
    a: a
    node-manager.deckhouse.io/last-applied-node-template: '{"annotations":{"a":"a"},"labels":{"a":"a"},"taints":[{"key":"a","effect":"NoSchedule"}]}'
  labels:
    a: a
    node.deckhouse.io/group: wor-ker
    node-role.kubernetes.io/wor-ker: ''
    node.deckhouse.io/type: Static
spec:
  taints:
  - key: a
    effect: NoSchedule`

			lastApplied := f.KubernetesGlobalResource("Node", "wor-ker").
				Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).
				String()
			node := f.KubernetesGlobalResource("Node", "wor-ker").Parse()
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))
			Expect(node.DropFields("status", "metadata.creationTimestamp")).To(MatchYAML(expectedYAML))
		})
	})

	Context("NG with label node-role.deckhouse.io/system and minimal Static Node", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: Static
  nodeTemplate:
    labels:
      node.deckhouse.io/group: wor-ker
      node-role.deckhouse.io/system: ""
      node-role.deckhouse.io/stateful: ""
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  labels:
    node.deckhouse.io/group: wor-ker
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully; new node-role.deckhouse.io label must be set", func() {
			expectedLastApplied := `
				{
					"labels": {
						"node-role.deckhouse.io/system": "",
						"node-role.deckhouse.io/stateful": "",
						"node.deckhouse.io/group": "wor-ker"
					},
					"annotations": {},
					"taints": []
				}
			`
			lastApplied := f.KubernetesGlobalResource("Node", "wor-ker").Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).String()
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))
		})
	})

	Context("Unmanaged nodes in cluster", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  labels:
    node.deckhouse.io/group: wor-ker
---
apiVersion: v1
kind: Node
metadata:
  name: unmanaged-wor-ker
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully; metric should exported", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0].Action).Should(Equal("expire"))
			Expect(m[1].Labels["node"]).Should(Equal("unmanaged-wor-ker"))
		})
	})

	Context("NodeGroup without taints and Node with taints", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: Static
  nodeTemplate:
    labels:
      node.deckhouse.io/group: wor-ker
      node-role.deckhouse.io/system: ""
      node-role.deckhouse.io/stateful: ""
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  labels:
    node.deckhouse.io/group: wor-ker
spec:
  taints:
  - key: a
    effect: NoSchedule
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must delete the taints completely", func() {
			Expect(f).To(ExecuteSuccessfully())

			node := f.KubernetesGlobalResource("Node", "wor-ker").Parse()
			Expect(node.Get("spec.taints").Array()).To(HaveLen(0))
		})
	})

	Context("Update NG: NodeGroup with labels adding annotation", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(`
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: Static
  nodeTemplate:
    annotations:
      test: test
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  labels:
    node.deckhouse.io/type: Static
    node.deckhouse.io/group: wor-ker
    node-role.kubernetes.io/wor-ker: ""
spec:
  taints:
  - key: a
    effect: NoSchedule
`))
			f.RunHook()
		})

		It("Must add annotation", func() {
			Expect(f).To(ExecuteSuccessfully())

			node := f.KubernetesGlobalResource("Node", "wor-ker").Parse()
			Expect(node.Get("metadata.annotations").Map()["test"].String()).To(Equal("test"))
		})
	})

	Context("Update NG: set empty nodeTemplate", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: wor-ker
spec:
  nodeType: Static
  nodeTemplate: {}
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  annotations:
    a: a
    node-manager.deckhouse.io/last-applied-node-template: '{"annotations":{"a":"a"},"labels":{"a":"a"},"taints":[{"key":"a","effect":"NoSchedule"}]}'
  labels:
    a: a
    node.deckhouse.io/group: wor-ker
    node-role.kubernetes.io/wor-ker: ''
    node.deckhouse.io/type: Static
spec:
  taints:
  - key: a
    effect: NoSchedule
  - key: node.deckhouse.io/uninitialized
    effect: NoSchedule
`
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("Must be executed successfully; labels and annotations must be removed", func() {
			expectedLastApplied := `
				{
					"labels": {},
					"annotations": {},
					"taints": []
				}
			`
			lastApplied := f.KubernetesGlobalResource("Node", "wor-ker").Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).String()
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))

			labels := f.KubernetesGlobalResource("Node", "wor-ker").Field(`metadata.labels`).Map()
			Expect(labels).ToNot(HaveKey("a"), "label 'a' should be removed")

			annotations := f.KubernetesGlobalResource("Node", "wor-ker").Field(`metadata.annotations`).Map()
			Expect(annotations).ToNot(HaveKey("a"), "annotation 'a' should be removed")

			taints := f.KubernetesGlobalResource("Node", "wor-ker").Field(`spec.taints`).Array()
			taintKeys := set.New()
			for _, taint := range taints {
				taintKeys.Add(taint.Get(`key`).String())
			}
			Expect(taintKeys).ToNot(HaveKey("a"), "taint with key 'a' should be removed")
		})
	})
})
