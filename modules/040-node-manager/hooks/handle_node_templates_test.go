package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"fmt"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: nodeManager :: hooks :: handle_node_templates_test ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "NodeGroup", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("NG without nodeTemplate and Cloud Node", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Cloud
---
apiVersion: v1
kind: Node
metadata:
  name: worker
  labels:
    node.deckhouse.io/group: worker
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
			        "node.deckhouse.io/group": "worker"
			      },
			      "name": "worker"
			    }
			  }
			`
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "worker").Parse()).To(MatchJSON(expectedJSON))
		})
	})

	Context("NG without nodeTemplate and minimal Static Node", func() {
		BeforeEach(func() {
			state := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
---
apiVersion: v1
kind: Node
metadata:
  name: worker
  labels:
    node.deckhouse.io/group: worker
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
			              "node.deckhouse.io/group": "worker",
			              "node-role.kubernetes.io/worker": ""
			            },
			            "name": "worker"
			          },
			          "spec": {}
			        }
			`
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "worker").Parse()).To(MatchJSON(expectedJSON))
		})
	})

	Context("Updated NG nodeTemplate and minimal Static Node", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
  nodeTemplate:
    annotations:
      new: new
    labels:
      new: new
      node.deckhouse.io/group: worker
    taints:
    - effect: NoSchedule
      key: new
---
apiVersion: v1
kind: Node
metadata:
  name: worker
  labels:
    node.deckhouse.io/group: worker
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
						"node.deckhouse.io/group": "worker"
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
			lastApplied := f.KubernetesGlobalResource("Node", "worker").Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).String()
			// fmt.Printf("LOG:\n🔥🔥🔥\n%v\n🔥🔥🔥\n", f.ValuesGet("nodeManager.test").String())
			// fmt.Printf("NODE:\n🔥🔥🔥\n%v\n🔥🔥🔥\n", lastApplied)
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))
		})
	})

	Context("Minimal NG, Static Node with old labels, taints, annotations", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
---
apiVersion: v1
kind: Node
metadata:
  name: worker
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
    node.deckhouse.io/group: worker
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
			lastApplied := f.KubernetesGlobalResource("Node", "worker").Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).String()
			node := f.KubernetesGlobalResource("Node", "worker").Parse()
			fmt.Printf("LOG:\n🔥🔥🔥\n%v\n🔥🔥🔥\n", f.ValuesGet("nodeManager.test").String())
			fmt.Printf("NODE:\n🔥🔥🔥\n%v\n🔥🔥🔥\n", node)
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))
		})
	})

	Context("Cluster with NG and Static Node", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: worker
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
  name: worker
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
    node.deckhouse.io/group: worker
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
			lastApplied := f.KubernetesGlobalResource("Node", "worker").Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).String()
			node := f.KubernetesGlobalResource("Node", "worker").Parse()
			fmt.Printf("LOG:\n🔥🔥🔥\n%v\n🔥🔥🔥\n", f.ValuesGet("nodeManager.test").String())
			fmt.Printf("NODE:\n🔥🔥🔥\n%v\n🔥🔥🔥\n", node)
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))
		})
	})

	Context("Cluster with NG and Static Node", func() {
		BeforeEach(func() {

			state := `
---
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroup
metadata:
  name: worker
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
  name: worker
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
    node.deckhouse.io/group: worker
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
  name: worker
  annotations:
    a: a
    node-manager.deckhouse.io/last-applied-node-template: '{"annotations":{"a":"a"},"labels":{"a":"a"},"taints":[{"effect":"NoSchedule","key":"a"}]}'
  labels:
    a: a
    node.deckhouse.io/group: worker
    node-role.kubernetes.io/worker: ''
spec:
  taints:
  - key: a
    effect: NoSchedule`

			lastApplied := f.KubernetesGlobalResource("Node", "worker").Field(`metadata.annotations.node-manager\.deckhouse\.io/last-applied-node-template`).String()
			node := f.KubernetesGlobalResource("Node", "worker").Parse()
			fmt.Printf("LOG:\n🔥🔥🔥\n%v\n🔥🔥🔥\n", f.ValuesGet("nodeManager.test").String())
			fmt.Printf("NODE:\n🔥🔥🔥\n%v\n🔥🔥🔥\n", node)
			Expect(f).To(ExecuteSuccessfully())
			Expect(lastApplied).To(MatchJSON(expectedLastApplied))
			Expect(node).To(MatchYAML(expectedYAML))
		})
	})

})
