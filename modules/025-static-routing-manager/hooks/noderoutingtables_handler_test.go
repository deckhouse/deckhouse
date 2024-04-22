/*
Copyright 2024 Flant JSC

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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("StatisRouteMgr hooks :: noderoutingtables_handler ::", func() {

	const (
		rt1YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: test1
spec:
  ipRouteTableID: 500
  routes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.0.0/24
    gateway: 192.168.0.1
  nodeSelector:
    node-role: testrole1
`
		rt2YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: test2
spec:
  ipRouteTableID: 300
  routes:
  - destination: 0.0.0.0/0
    gateway: 2.2.2.1
  nodeSelector:
    node-role: testrole1
`
		nrt1YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: NodeRoutingTables
metadata:
  name: kube-worker-3
spec:
  routingTables:
    "500":
      routes:
      - destination: 0.0.0.0/0
        gateway: 1.2.3.4
      - destination: 192.168.0.0/24
        gateway: 192.168.0.1
    "300":
      routes:
      - destination: 0.0.0.0/0
        gateway: 2.2.2.1
`
		node1YAML = `
---
apiVersion: v1
kind: Node
metadata:
  name: kube-worker-3
  labels:
    node-role: "testrole1"
`
	)

	var (
		// rtGVR = schema.GroupVersionResource{
		// 	Group:    "network.deckhouse.io",
		// 	Version:  "v1alpha1",
		// 	Resource: "routingtables",
		// }
		rtGVK = schema.GroupVersionKind{
			Group:   "network.deckhouse.io",
			Version: "v1alpha1",
			Kind:    "RoutingTable",
		}
		// nrtGVR = schema.GroupVersionResource{
		// 	Group:    "network.deckhouse.io",
		// 	Version:  "v1alpha1",
		// 	Resource: "noderoutingtables",
		// }
		nrtGVK = schema.GroupVersionKind{
			Group:   "network.deckhouse.io",
			Version: "v1alpha1",
			Kind:    "NodeRoutingTables",
		}
		// rt1   *v1alpha1.RoutingTable
		rt1u *unstructured.Unstructured
		// rt2   *v1alpha1.RoutingTable
		rt2u *unstructured.Unstructured
		// nrt1  *v1alpha1.NodeRoutingTables
		nrt1u *unstructured.Unstructured
		node1 *v1.Node
	)
	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(rt1YAML), &rt1u)
		_ = yaml.Unmarshal([]byte(rt2YAML), &rt2u)
		_ = yaml.Unmarshal([]byte(nrt1YAML), &nrt1u)
		_ = yaml.Unmarshal([]byte(node1YAML), &node1)
	})

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD(rtGVK.Group, rtGVK.Version, rtGVK.Kind, false)
	f.RegisterCRD(nrtGVK.Group, nrtGVK.Version, nrtGVK.Kind, false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.KubeStateSet(``)
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))
		})
	})

	Context("Checking the creation operation of a CR NodeRoutingTables", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + rt1YAML + rt2YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeRoutingTables", "kube-worker-3").Exists()).To(BeTrue())
			nrtspec := f.KubernetesGlobalResource("NodeRoutingTables", "kube-worker-3").Field("spec").String()
			Expect(nrtspec).To(MatchYAML(`
routingTables:
  "500":
    routes:
    - destination: 0.0.0.0/0
      gateway: 1.2.3.4
    - destination: 192.168.0.0/24
      gateway: 192.168.0.1
  "300":
    routes:
    - destination: 0.0.0.0/0
      gateway: 2.2.2.1
`))
		})
	})

	Context("Checking the deletion operation of a CR NodeRoutingTables", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt1YAML + rt2YAML + nrt1YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeRoutingTables", "kube-worker-3").Exists()).To(BeFalse())
		})
	})

	Context("Checking the updating operation of a CR NodeRoutingTables", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + rt1YAML + nrt1YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeRoutingTables", "kube-worker-3").Exists()).To(BeTrue())
			nrtspec := f.KubernetesGlobalResource("NodeRoutingTables", "kube-worker-3").Field("spec").String()
			Expect(nrtspec).To(MatchYAML(`
routingTables:
  "500":
    routes:
    - destination: 0.0.0.0/0
      gateway: 1.2.3.4
    - destination: 192.168.0.0/24
      gateway: 192.168.0.1
`))
		})
	})

	Context("Checking case when node was deleted", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + nrt1YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeRoutingTables", "kube-worker-3").Exists()).To(BeFalse())
		})
	})

})
