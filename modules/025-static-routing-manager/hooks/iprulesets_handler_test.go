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
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib"
	"github.com/deckhouse/deckhouse/modules/025-static-routing-manager/hooks/lib/v1alpha1"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("StatisRouteMgr hooks :: iprulesets_handler ::", func() {

	const (
		initValuesString       = `{"staticRoutingManager":{"internal": {}}}`
		initConfigValuesString = `{"staticRoutingManager":{}}`
	)

	const (
		rt1YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: testrt1
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
  ipRoutingTableID: 500
`
		irs1YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: IPRuleSet
metadata:
  name: testirs1
spec:
  rules:
  - actions:
      lookup:
        routingTableName: testrt1
    selectors:
      not: true
      from:
      - 192.168.111.0/24
      - 192.168.222.0/24
      to:
      - 3.0.0.0/8
      - 4.0.0.0/8
      ipproto: 6
      dport: 300-400
      sport: 100-200
      iif: eth1
      oif: cilium_net
      fwmark: 0x42/0xff
      tos: "0x10"
      uidrange: 1001-2000
  nodeSelector:
    node-role: testrole1
`
		irs1upYAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: IPRuleSet
metadata:
  name: testirs1
spec:
  rules:
  - actions:
      lookup:
        routingTableName: testrt1
    selectors:
      not: true
      from:
      - 192.168.111.0/24
      to:
      - 3.0.0.0/8
      ipproto: 6
      dport: 300-400
      sport: 100-200
      iif: eth1
      oif: cilium_net
      fwmark: 0x42/0xff
      tos: "0x10"
      uidrange: 1001-2000
  nodeSelector:
    node-role: testrole1
`
		irs2YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: IPRuleSet
metadata:
  name: testirs2
spec:
  rules:
  - actions:
      lookup:
        ipRoutingTableID: 300
    selectors:
      not: true
      from:
      - 192.168.111.0/24
      - 192.168.222.0/24
      to:
      - 3.0.0.0/8
      - 4.0.0.0/8
      ipproto: 6
      dport: 300-400
      sport: 100-200
      iif: eth1
      oif: cilium_net
      fwmark: 0x42/0xff
      tos: "0x10"
      uidrange: 1001-2000
  nodeSelector:
    node-role: testrole1
`
		nirs11YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: NodeIPRuleSet
metadata:
  finalizers:
  - routing-tables-manager.network.deckhouse.io
  generation: 4
  labels:
    routing-manager.network.deckhouse.io/node-name: kube-worker-1
  name: testirs1-29c8b10d14
  ownerReferences:
  - apiVersion: network.deckhouse.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: IPRuleSet
    name: testirs666
spec:
  nodeName: kube-worker-1
  rules:
  - actions:
      lookup:
        ipRoutingTableID: 500
    selectors:
      not: true
      from:
      - 192.168.111.0/24
      - 192.168.222.0/24
      to:
      - 3.0.0.0/8
      - 4.0.0.0/8
      ipproto: 6
      dport: 300-400
      sport: 100-200
      iif: eth1
      oif: cilium_net
      fwmark: 0x42/0xff
      tos: "0x10"
      uidrange: 1001-2000
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
		irs666YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: IPRuleSet
metadata:
  generation: 4
  name: testirs666
spec:
  rules:
  - actions:
      lookup:
        routingTableName: testrt1
    selectors:
      not: true
      from:
      - 192.168.111.0/24
      - 192.168.222.0/24
      to:
      - 3.0.0.0/8
      - 4.0.0.0/8
      ipproto: 6
      dport: 300-400
      sport: 100-200
      iif: eth1
      oif: cilium_net
      fwmark: 0x42/0xff
      tos: "0x10"
      uidrange: 1001-2000
  nodeSelector:
    node-role: testrole1
status:
  affectedNodeIPRuleSets: 2
  conditions:
  - lastHeartbeatTime: "2024-05-29T18:42:03Z"
    lastTransitionTime: "2024-05-29T18:35:23Z"
    message: ""
    reason: ReconciliationSucceed
    status: "True"
    type: Ready
  observedGeneration: 4
  readyNodeIPRuleSets: 2
`
		nirs666YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: NodeIPRuleSet
metadata:
  finalizers:
  - routing-tables-manager.network.deckhouse.io
  generation: 4
  labels:
    routing-manager.network.deckhouse.io/node-name: kube-worker-1
  name: testirs666-4795340ecf
  ownerReferences:
  - apiVersion: network.deckhouse.io/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: IPRuleSet
    name: testirs666
spec:
  nodeName: kube-worker-1
  rules:
  - actions:
      lookup:
        ipRoutingTableID: 500
    selectors:
      not: true
      from:
      - 192.168.111.0/24
      - 192.168.222.0/24
      to:
      - 3.0.0.0/8
      - 4.0.0.0/8
      ipproto: 6
      dport: 300-400
      sport: 100-200
      iif: eth1
      oif: cilium_net
      fwmark: 0x42/0xff
      tos: "0x10"
      uidrange: 1001-2000
status:
  appliedRoutes:
  - actions:
      lookup:
        ipRoutingTableID: 500
    selectors:
      not: true
      from:
      - 192.168.111.0/24
      - 192.168.222.0/24
      to:
      - 3.0.0.0/8
      - 4.0.0.0/8
      ipproto: 6
      dport: 300-400
      sport: 100-200
      iif: eth1
      oif: cilium_net
      fwmark: 0x42/0xff
      tos: "0x10"
      uidrange: 1001-2000
  conditions:
  - lastHeartbeatTime: "2024-05-29T18:55:16Z"
    lastTransitionTime: "2024-05-29T18:35:23Z"
    message: ""
    reason: ReconciliationSucceed
    status: "True"
    type: Ready
  observedGeneration: 4
`
	)

	var (
		rtGVK = schema.GroupVersionKind{
			Group:   "network.deckhouse.io",
			Version: "v1alpha1",
			Kind:    "RoutingTable",
		}
		irsGVK = schema.GroupVersionKind{
			Group:   "network.deckhouse.io",
			Version: "v1alpha1",
			Kind:    "IPRuleSet",
		}
		nirsGVK = schema.GroupVersionKind{
			Group:   "network.deckhouse.io",
			Version: "v1alpha1",
			Kind:    "NodeIPRuleSet",
		}
		irs1u   *unstructured.Unstructured
		irs2u   *unstructured.Unstructured
		nirs11u *unstructured.Unstructured
		node1   *v1.Node
		node2   *v1.Node
	)
	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(irs1YAML), &irs1u)
		_ = yaml.Unmarshal([]byte(irs2YAML), &irs2u)
		_ = yaml.Unmarshal([]byte(nirs11YAML), &nirs11u)
		_ = yaml.Unmarshal([]byte(node1YAML), &node1)
		_ = yaml.Unmarshal([]byte(node2YAML), &node2)
	})

	f := HookExecutionConfigInit(initValuesString, "")
	f.RegisterCRD(irsGVK.Group, irsGVK.Version, irsGVK.Kind, false)
	f.RegisterCRD(nirsGVK.Group, nirsGVK.Version, nirsGVK.Kind, false)
	f.RegisterCRD(rtGVK.Group, rtGVK.Version, rtGVK.Kind, false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))
		})
	})

	Context("Checking the creation operation of a CR NodeRoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(irs1YAML + irs2YAML + node1YAML + node2YAML + rt1YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			nirs11Name := "testirs1" + "-" + lib.GenerateShortHash("testirs1"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nirsKeyPath + ".1.name").String()).To(Equal(nirs11Name))
			Expect(f.ValuesGet(nirsKeyPath + ".1.nodeName").String()).To(Equal("kube-worker-1"))
			Expect(f.ValuesGet(nirsKeyPath + ".1.ownerIRSName").String()).To(Equal("testirs1"))
			Expect(f.ValuesGet(nirsKeyPath + ".1.rules").String()).To(MatchYAML(`
- actions:
    lookup:
      ipRoutingTableID: 500
      routingTableName: testrt1
  selectors:
    not: true
    from:
    - 192.168.111.0/24
    - 192.168.222.0/24
    to:
    - 3.0.0.0/8
    - 4.0.0.0/8
    ipproto: 6
    dport: 300-400
    sport: 100-200
    iif: eth1
    oif: cilium_net
    fwmark: 0x42/0xff
    tos: "0x10"
    uidrange: 1001-2000
`))
			nirs12Name := "testirs1" + "-" + lib.GenerateShortHash("testirs1"+"#"+"kube-worker-2")
			Expect(f.ValuesGet(nirsKeyPath + ".0.name").String()).To(Equal(nirs12Name))
			Expect(f.ValuesGet(nirsKeyPath + ".0.nodeName").String()).To(Equal("kube-worker-2"))
			Expect(f.ValuesGet(nirsKeyPath + ".0.ownerIRSName").String()).To(Equal("testirs1"))
			Expect(f.ValuesGet(nirsKeyPath + ".0.rules").String()).To(MatchYAML(`
- actions:
    lookup:
      ipRoutingTableID: 500
      routingTableName: testrt1
  selectors:
    not: true
    from:
    - 192.168.111.0/24
    - 192.168.222.0/24
    to:
    - 3.0.0.0/8
    - 4.0.0.0/8
    ipproto: 6
    dport: 300-400
    sport: 100-200
    iif: eth1
    oif: cilium_net
    fwmark: 0x42/0xff
    tos: "0x10"
    uidrange: 1001-2000
`))
			nirs21Name := "testirs2" + "-" + lib.GenerateShortHash("testirs2"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nirsKeyPath + ".3.name").String()).To(Equal(nirs21Name))
			Expect(f.ValuesGet(nirsKeyPath + ".3.nodeName").String()).To(Equal("kube-worker-1"))
			Expect(f.ValuesGet(nirsKeyPath + ".3.ownerIRSName").String()).To(Equal("testirs2"))
			Expect(f.ValuesGet(nirsKeyPath + ".3.rules").String()).To(MatchYAML(`
- actions:
    lookup:
      ipRoutingTableID: 300
  selectors:
    not: true
    from:
    - 192.168.111.0/24
    - 192.168.222.0/24
    to:
    - 3.0.0.0/8
    - 4.0.0.0/8
    ipproto: 6
    dport: 300-400
    sport: 100-200
    iif: eth1
    oif: cilium_net
    fwmark: 0x42/0xff
    tos: "0x10"
    uidrange: 1001-2000
`))
			nirs22Name := "testirs2" + "-" + lib.GenerateShortHash("testirs2"+"#"+"kube-worker-2")
			Expect(f.ValuesGet(nirsKeyPath + ".2.name").String()).To(Equal(nirs22Name))
			Expect(f.ValuesGet(nirsKeyPath + ".2.nodeName").String()).To(Equal("kube-worker-2"))
			Expect(f.ValuesGet(nirsKeyPath + ".2.ownerIRSName").String()).To(Equal("testirs2"))
			Expect(f.ValuesGet(nirsKeyPath + ".2.rules").String()).To(MatchYAML(`
- actions:
    lookup:
      ipRoutingTableID: 300
  selectors:
    not: true
    from:
    - 192.168.111.0/24
    - 192.168.222.0/24
    to:
    - 3.0.0.0/8
    - 4.0.0.0/8
    ipproto: 6
    dport: 300-400
    sport: 100-200
    iif: eth1
    oif: cilium_net
    fwmark: 0x42/0xff
    tos: "0x10"
    uidrange: 1001-2000
`))

		})
	})

	Context("Checking the creation operation of a CR IPRuleSet with not exists RoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(irs1YAML + node1YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			nirs11Name := "testirs1" + "-" + lib.GenerateShortHash("testirs1"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nirsKeyPath).String()).To(Equal("[]"))
			Expect(f.ValuesGet(nirsKeyPath + ".0.name").String()).NotTo(Equal(nirs11Name))
		})
	})

	Context("Checking the deletion operation of a CR NodeRoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + rt1YAML + nirs11YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			nirs11Name := "testirs1" + "-" + lib.GenerateShortHash("testirs1"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nirsKeyPath).String()).To(Equal("[]"))
			Expect(f.ValuesGet(nirsKeyPath + ".0.name").String()).NotTo(Equal(nirs11Name))
		})
	})

	Context("Checking case when node was deleted", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt1YAML + irs1YAML + nirs11YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			nirs11Name := "testirs1" + "-" + lib.GenerateShortHash("testirs1"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nirsKeyPath).String()).To(Equal("[]"))
			Expect(f.ValuesGet(nirsKeyPath + ".0.name").String()).NotTo(Equal(nirs11Name))
		})
	})

	Context("Checking the updating operation of a CR NodeRoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + rt1YAML + irs1upYAML + nirs11YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			nirs11Name := "testirs1" + "-" + lib.GenerateShortHash("testirs1"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nirsKeyPath + ".0.name").String()).To(Equal(nirs11Name))
			Expect(f.ValuesGet(nirsKeyPath + ".0.nodeName").String()).To(Equal("kube-worker-1"))
			Expect(f.ValuesGet(nirsKeyPath + ".0.ownerIRSName").String()).To(Equal("testirs1"))
			Expect(f.ValuesGet(nirsKeyPath + ".0.ipRoutingTableID").String()).To(Equal("500"))
			Expect(f.ValuesGet(nirsKeyPath + ".0.rules").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 1.2.3.4
- destination: 192.168.1.0/24
  gateway: 192.168.2.1
`))
		})
	})

	Context("Checking condition update", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + rt1YAML + irs666YAML + nirs666YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))
			Expect(f.KubernetesGlobalResource("RoutingTable", "testirs666").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("RoutingTable", "testirs666").Field("status").Exists()).To(BeTrue())
			rtstatusraw := f.KubernetesGlobalResource("RoutingTable", "testirs666").Field("status").String()
			Expect(rtstatusraw).NotTo(Equal(""))
			var rtstatus *v1alpha1.RoutingTableStatus
			_ = json.Unmarshal([]byte(rtstatusraw), &rtstatus)
			Expect(rtstatus.AffectedNodeRoutingTables).To(Equal(1))
			Expect(rtstatus.ReadyNodeRoutingTables).To(Equal(1))
			Expect(rtstatus.IPRoutingTableID).To(Equal(500))
			Expect(rtstatus.ObservedGeneration).To(Equal(int64(1)))
			Expect(rtstatus.Conditions[0].Type).To(Equal(v1alpha1.ReconciliationSucceedType))
			Expect(rtstatus.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(rtstatus.Conditions[0].Reason).To(Equal(v1alpha1.ReconciliationReasonSucceed))
			Expect(rtstatus.Conditions[0].Message).To(Equal(""))
			Expect(rtstatus.Conditions[0].LastHeartbeatTime).NotTo(Equal(nil))
			Expect(rtstatus.Conditions[0].LastTransitionTime).NotTo(Equal(nil))

		})
	})

})
