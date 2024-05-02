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
  name: testrt1
spec:
  ipRouteTableID: 500
  routes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.0.0/24
    gateway: 192.168.0.1
  nodeSelector:
    node-role: testrole1
status:
  ipRouteTableID: 500
`
		desiredRT1SpecYAML = `
ipRouteTableID: 500
routes:
- destination: 0.0.0.0/0
  gateway: 1.2.3.4
- destination: 192.168.0.0/24
  gateway: 192.168.0.1
`
		rt1upYAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: testrt1
spec:
  ipRouteTableID: 500
  routes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.1.0/24
    gateway: 192.168.2.1
  nodeSelector:
    node-role: testrole1
status:
  ipRouteTableID: 500
`
		desiredRT1upSpecYAML = `
ipRouteTableID: 500
routes:
- destination: 0.0.0.0/0
  gateway: 1.2.3.4
- destination: 192.168.1.0/24
  gateway: 192.168.2.1
`
		rt2YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: testrt2
spec:
  routes:
  - destination: 0.0.0.0/0
    gateway: 2.2.2.1
  nodeSelector:
    node-role: testrole1
status:
  ipRouteTableID: 300
`
		desiredRT2SpecYAML = `
ipRouteTableID: 300
routes:
- destination: 0.0.0.0/0
  gateway: 2.2.2.1
`
		rt3YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: testrt3
spec:
  ipRouteTableID: 300
  routes:
  - destination: 0.0.0.0/0
    gateway: 2.2.2.1
  nodeSelector:
    node-role: testrole1
`
		nrt11YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: NodeRoutingTable
metadata:
  name: kube-worker-1-testrt1
spec:
  ipRouteTableID: 500
  routes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.0.0/24
    gateway: 192.168.0.1
`
		node1YAML = `
---
apiVersion: v1
kind: Node
metadata:
  name: kube-worker-1
  labels:
    node-role: "testrole1"
`
		node2YAML = `
---
apiVersion: v1
kind: Node
metadata:
  name: kube-worker-2
  labels:
    node-role: "testrole1"
`
	)

	var (
		rtGVK = schema.GroupVersionKind{
			Group:   "network.deckhouse.io",
			Version: "v1alpha1",
			Kind:    "RoutingTable",
		}
		nrtGVK = schema.GroupVersionKind{
			Group:   "network.deckhouse.io",
			Version: "v1alpha1",
			Kind:    "NodeRoutingTable",
		}
		rt1u  *unstructured.Unstructured
		rt2u  *unstructured.Unstructured
		rt3u  *unstructured.Unstructured
		nrt1u *unstructured.Unstructured
		node1 *v1.Node
		node2 *v1.Node
	)
	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(rt1YAML), &rt1u)
		_ = yaml.Unmarshal([]byte(rt2YAML), &rt2u)
		_ = yaml.Unmarshal([]byte(rt3YAML), &rt3u)
		_ = yaml.Unmarshal([]byte(nrt11YAML), &nrt1u)
		_ = yaml.Unmarshal([]byte(node1YAML), &node1)
		_ = yaml.Unmarshal([]byte(node2YAML), &node2)
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

	Context("Checking the creation operation of a CR NodeRoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + node2YAML + rt1YAML + rt2YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-1-testrt1").Exists()).To(BeTrue())
			nrt11spec := f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-1-testrt1").Field("spec").String()
			Expect(nrt11spec).To(MatchYAML(desiredRT1SpecYAML))
			Expect(f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-1-testrt2").Exists()).To(BeTrue())
			nrt12spec := f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-1-testrt2").Field("spec").String()
			Expect(nrt12spec).To(MatchYAML(desiredRT2SpecYAML))
			Expect(f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-2-testrt1").Exists()).To(BeTrue())
			nrt21spec := f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-2-testrt1").Field("spec").String()
			Expect(nrt21spec).To(MatchYAML(desiredRT1SpecYAML))
			Expect(f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-2-testrt2").Exists()).To(BeTrue())
			nrt22spec := f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-2-testrt2").Field("spec").String()
			Expect(nrt22spec).To(MatchYAML(desiredRT2SpecYAML))
		})
	})

	Context("Checking the creation operation of a CR NodeRoutingTable from CR RoutingTable without ipRouteTableID in status", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + rt3YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-1-testrt3").Exists()).To(BeFalse())
		})
	})

	Context("Checking the deletion operation of a CR NodeRoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + nrt11YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-1-testrt1").Exists()).To(BeFalse())
		})
	})

	Context("Checking case when node was deleted", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt1YAML + nrt11YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-1-testrt1").Exists()).To(BeFalse())
		})
	})

	Context("Checking the updating operation of a CR NodeRoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + rt1upYAML + nrt11YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-1-testrt1").Exists()).To(BeTrue())
			nrt11spec := f.KubernetesGlobalResource("NodeRoutingTable", "kube-worker-1-testrt1").Field("spec").String()
			Expect(nrt11spec).To(MatchYAML(desiredRT1upSpecYAML))
		})
	})

})
