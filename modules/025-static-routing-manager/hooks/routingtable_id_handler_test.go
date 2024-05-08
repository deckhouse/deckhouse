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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("StatisRouteMgr hooks :: routingtable_id_handler ::", func() {

	const (
		rt1YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: test1
spec:
  ipRoutingTableID: 500
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
  routes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.0.0/24
    gateway: 192.168.0.1
  nodeSelector:
    node-role: testrole1
`
		rt3YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: test3
spec:
  ipRoutingTableID: 500
  routes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.0.0/24
    gateway: 192.168.0.1
  nodeSelector:
    node-role: testrole1
status:
  ipRoutingTableID: 300
`
	)

	var (
		rtGVK = schema.GroupVersionKind{
			Group:   "network.deckhouse.io",
			Version: "v1alpha1",
			Kind:    "RoutingTable",
		}
		rt1u *unstructured.Unstructured
		rt2u *unstructured.Unstructured
		rt3u *unstructured.Unstructured
	)
	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(rt1YAML), &rt1u)
		_ = yaml.Unmarshal([]byte(rt2YAML), &rt2u)
		_ = yaml.Unmarshal([]byte(rt3YAML), &rt3u)

	})

	f := HookExecutionConfigInit(`{}`, `{}`)
	f.RegisterCRD(rtGVK.Group, rtGVK.Version, rtGVK.Kind, false)

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

	Context("Checking setting id in status(from spec) of a CR RoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt1YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("RoutingTable", "test1").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("RoutingTable", "test1").Field("status.ipRoutingTableID").Exists()).To(BeTrue())
			rtstatus := f.KubernetesGlobalResource("RoutingTable", "test1").Field("status").String()
			Expect(rtstatus).To(MatchYAML(`ipRoutingTableID: 500`))
		})
	})

	Context("Checking generating and setting id in status of a CR RoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt1YAML + rt2YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("RoutingTable", "test2").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("RoutingTable", "test2").Field("status.ipRoutingTableID").Exists()).To(BeTrue())
			rtstatus := f.KubernetesGlobalResource("RoutingTable", "test2").Field("status").String()
			Expect(rtstatus).NotTo(MatchYAML(`ipRoutingTableID: 500`))
		})
	})

	Context("Checking setting id in status(from spec) (overwrite) of a CR RoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt3YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("RoutingTable", "test3").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("RoutingTable", "test3").Field("status.ipRoutingTableID").Exists()).To(BeTrue())
			rtstatus := f.KubernetesGlobalResource("RoutingTable", "test3").Field("status").String()
			Expect(rtstatus).To(MatchYAML(`ipRoutingTableID: 500`))
		})
	})

})
