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
	"context"
	"errors"
	"fmt"

	"github.com/flant/addon-operator/sdk"
	"github.com/flant/shell-operator/pkg/metric_storage/operation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: nodeManager :: hooks :: update_approval_draining ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster node is draining", func() {
		BeforeEach(func() {
			st := f.KubeStateSet(`
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker
  labels:
    node.deckhouse.io/group: "master"
  annotations:
    update.node.deckhouse.io/draining: ""
---
apiVersion: v1
kind: Node
metadata:
  name: wor-ker-2
  labels:
    node.deckhouse.io/group: "master"
  annotations:
    update.node.deckhouse.io/draining: "user"
`)
			f.BindingContexts.Set(st)
			testMoveNodesToStaticClient(f)
			f.RunHook()
		})

		It("Must be drained", func() {
			Expect(f).To(ExecuteSuccessfully())
			node := f.KubernetesGlobalResource("Node", "wor-ker")
			Expect(node.Field("metadata.annotations.update\\.node\\.deckhouse\\.io/drained").String()).To(Equal("bashible"))
			Expect(node.Field("metadata.annotations.update\\.node\\.deckhouse\\.io/draining").Exists()).To(BeFalse())
			k8sClient := f.BindingContextController.FakeCluster().Client
			node1Core, _ := k8sClient.CoreV1().Nodes().Get(context.Background(), "wor-ker", v1.GetOptions{})
			Expect(node1Core.Spec.Unschedulable).To(BeTrue())

			node2 := f.KubernetesGlobalResource("Node", "wor-ker-2")
			Expect(node2.Field("metadata.annotations.update\\.node\\.deckhouse\\.io/drained").String()).To(Equal("user"))
			Expect(node2.Field("metadata.annotations.update\\.node\\.deckhouse\\.io/draining").Exists()).To(BeFalse())
		})
	})

	Context("draining_nodes", func() {
		var initialState = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: Static
status:
  desired: 1
  ready: 1
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: undisruptable-worker
spec:
  nodeType: Static
  disruptions:
    approvalMode: Manual
status:
  desired: 1
  ready: 1
---
apiVersion: v1
kind: Secret
metadata:
  name: configuration-checksums
  namespace: d8-cloud-instance-manager
data:
  worker: dXBkYXRlZA== # updated
  undisruptable-worker: dXBkYXRlZA== # updated
`
		var nodeNames = []string{"worker-1", "worker-2", "worker-3"}
		for _, gDraining := range []bool{true, false} {
			for _, gUnschedulable := range []bool{true, false} {
				Context(fmt.Sprintf("Draining: %t, Unschedulable: %t", gDraining, gUnschedulable), func() {
					draining := gDraining
					unschedulable := gUnschedulable
					BeforeEach(func() {
						st := f.KubeStateSet(initialState + generateStateToTestDrainingNodes(nodeNames, draining, unschedulable))
						f.BindingContexts.Set(st)
						testMoveNodesToStaticClient(f)
						f.RunHook()
					})

					It("Works as expected", func() {
						Expect(f).To(ExecuteSuccessfully())
						for _, nodeName := range nodeNames {
							if draining {
								By(fmt.Sprintf("%s must have /drained", nodeName), func() {
									Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeTrue())
									Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).String()).To(Equal("bashible"))
								})

								By(fmt.Sprintf("%s must not have /draining", nodeName), func() {
									Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeFalse())
								})
							} else {
								By(fmt.Sprintf("%s must not have /drained", nodeName), func() {
									Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeFalse())
								})

								if unschedulable {
									By(fmt.Sprintf("%s must be unschedulable", nodeName), func() {
										Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).Exists()).To(BeTrue())
									})
								} else {
									By(fmt.Sprintf("%s must not be unschedulable", nodeName), func() {
										Expect(f.KubernetesGlobalResource("Node", nodeName).Field(`spec.unschedulable`).Exists()).To(BeFalse())
									})
								}
							}
						}
					})
				})
			}
		}
	})

	Context("simulate error", func() {
		var event *eventsv1.Event

		BeforeEach(func() {

			st := f.KubeStateSet("")
			f.BindingContexts.Set(st)

			dnode := drainedNodeRes{
				NodeName:       "foo-1",
				DrainingSource: "bashible",
				Err:            errors.New("foo-bar-error"),
			}

			event = dnode.buildEvent()
			unst, _ := sdk.ToUnstructured(event)
			_, _ = f.BindingContextController.FakeCluster().Client.Dynamic().Resource(schema.GroupVersionResource{Resource: "events", Group: "events.k8s.io", Version: "v1"}).Namespace("default").Create(context.Background(), unst, v1.CreateOptions{})
		})

		It("Should generate event", func() {
			ev := f.KubernetesResource("Event", "default", event.Name)
			Expect(ev.Field("note").String()).To(Equal("foo-bar-error"))
			Expect(ev.Field("reason").String()).To(Equal("DrainFailed"))
			Expect(ev.Field("type").String()).To(Equal("Warning"))
			Expect(ev.Field("regarding.kind").String()).To(Equal("Node"))
			Expect(ev.Field("regarding.name").String()).To(Equal("foo-1"))
		})
	})

	Context("simulate error metrics", func() {
		BeforeEach(func() {

			st := f.KubeStateSet(`
---
apiVersion: v1
kind: Node
metadata:
  name: foo-2
  labels:
    node.deckhouse.io/group: "master"
  annotations:
    update.node.deckhouse.io/draining: "bashible"
`)
			f.BindingContexts.Set(st)
			testMoveNodesToStaticClient(f)
			f.RunHook()
		})

		It("Should generate metrics", func() {
			k8sClient := f.BindingContextController.FakeCluster().Client
			node1Core, _ := k8sClient.CoreV1().Nodes().Get(context.Background(), "foo-2", v1.GetOptions{})
			Expect(node1Core.Spec.Unschedulable).To(BeTrue())
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("Node", "foo-2").Field(`metadata.annotations.update\.node\.deckhouse\.io/drained`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("Node", "foo-2").Field(`metadata.annotations.update\.node\.deckhouse\.io/draining`).Exists()).To(BeTrue())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(2))
			Expect(m[0]).To(BeEquivalentTo(operation.MetricOperation{
				Group:  "d8_node_draining",
				Action: "expire",
			}))

			Expect(m[1]).To(BeEquivalentTo(operation.MetricOperation{
				Name:   "d8_node_draining",
				Action: "set",
				Value:  pointer.Float64(1.0),
				Labels: map[string]string{
					"node":    "foo-2",
					"message": "unable to parse requirement: <nil>: Invalid value: \"a:\": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')",
				},
			}))
		})
	})
})

// BindingContexts work with Dynamic client but drainHelper works with CoreV1 from kubernetes.Interface client
// copy nodes to the static client for appropriate testing
func testMoveNodesToStaticClient(f *HookExecutionConfig) {
	k8sClient := f.BindingContextController.FakeCluster().Client

	nodesList, _ := k8sClient.Dynamic().Resource(schema.GroupVersionResource{Resource: "nodes", Version: "v1"}).List(context.Background(), v1.ListOptions{})
	for _, obj := range nodesList.Items {
		var n corev1.Node
		_ = sdk.FromUnstructured(&obj, &n)
		_ = k8sClient.CoreV1().Nodes().Delete(context.Background(), n.Name, v1.DeleteOptions{})
		_, _ = k8sClient.CoreV1().Nodes().Create(context.Background(), &n, v1.CreateOptions{})
	}
}
