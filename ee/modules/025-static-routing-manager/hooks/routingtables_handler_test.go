/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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

	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/hooks/lib"
	"github.com/deckhouse/deckhouse/ee/modules/025-static-routing-manager/hooks/lib/v1alpha1"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("StatisRouteMgr hooks :: routingtables_handler ::", func() {

	const (
		initValuesString       = `{"staticRoutingManager":{"internal": {}}}`
		initConfigValuesString = `{"staticRoutingManager":{"routingTableIDMin": "20000", "routingTableIDMax": "21000"}}`
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
  - destination: 192.168.100.0/24
    dev: eth0
  nodeSelector:
    node-role: testrole1
status:
  ipRoutingTableID: 500
`
		rt1upYAML = `
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
  - destination: 192.168.1.0/24
    gateway: 192.168.2.1
  - destination: 192.168.200.0/24
    dev: eth3
  nodeSelector:
    node-role: testrole1
status:
  ipRoutingTableID: 500
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
  ipRoutingTableID: 300
`
		rt3YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: testrt3
spec:
  ipRoutingTableID: 300
  routes:
  - destination: 0.0.0.0/0
    gateway: 2.2.2.1
  nodeSelector:
    node-role: testrole1
`
		rt4YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: testrt4
spec:
  routes:
  - destination: 0.0.0.0/0
    gateway: 2.2.2.1
  nodeSelector:
    node-role: testrole1
`
		rt5YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: testrt5
spec:
  ipRoutingTableID: 300
  routes:
  - destination: 0.0.0.0/0
    gateway: 2.2.2.1
  nodeSelector:
    node-role: testrole1
status:
  ipRoutingTableID: 500
`
		nrt11YAML = `
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalNodeRoutingTable
metadata:
  name: testrt1-29c8b10d14
spec:
  nodeName: kube-worker-1
  ipRoutingTableID: 500
  routes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.0.0/24
    gateway: 192.168.0.1
  - destination: 192.168.100.0/24
    dev: eth0
`
		orphannrt11YAML = `
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalNodeRoutingTable
metadata:
  finalizers:
  - routing-tables-manager.network.deckhouse.io
  - yet-another-finalizer
  generation: 1
  deletionGracePeriodSeconds: 0
  deletionTimestamp: "2024-05-30T08:29:20Z"
  labels:
    routing-manager.network.deckhouse.io/node-name: kube-worker-1
  name: testrt1-29c8b10d14
  ownerReferences:
  - apiVersion: SDNInternalNodeRoutingTable
    blockOwnerDeletion: true
    controller: true
    kind: RoutingTable
    name: testrt1
spec:
  nodeName: kube-worker-1
  ipRoutingTableID: 500
  routes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.0.0/24
    gateway: 192.168.0.1
  - destination: 192.168.100.0/24
    dev: eth0
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
		rt666YAML = `
---
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  generation: 1
  name: testrt666
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
  affectedNodeRoutingTables: 1
  conditions:
  - lastHeartbeatTime: "2024-05-16T16:28:56Z"
    lastTransitionTime: "2024-05-16T16:28:56Z"
    message: ""
    reason: Pending
    status: "False"
    type: Ready
  ipRoutingTableID: 500
  observedGeneration: 1
`
		nrt666YAML = `
---
apiVersion: internal.network.deckhouse.io/v1alpha1
kind: SDNInternalNodeRoutingTable
metadata:
  finalizers:
  - routing-tables-manager.network.deckhouse.io
  generation: 1
  labels:
    routing-manager.network.deckhouse.io/node-name: kube-worker-1
  name: testrt666-4795340ecf
  ownerReferences:
  - apiVersion: SDNInternalNodeRoutingTable
    blockOwnerDeletion: true
    controller: true
    kind: RoutingTable
    name: testrt666
spec:
  nodeName: kube-worker-1
  ipRoutingTableID: 500
  routes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.0.0/24
    gateway: 192.168.0.1
status:
  appliedRoutes:
  - destination: 0.0.0.0/0
    gateway: 1.2.3.4
  - destination: 192.168.0.0/24
    gateway: 192.168.0.1
  conditions:
  - lastHeartbeatTime: "2024-05-16T15:28:50Z"
    lastTransitionTime: "2024-05-16T15:28:50Z"
    message: ""
    reason: ReconciliationSucceed
    status: "True"
    type: Ready
  observedGeneration: 1
`
	)

	var (
		rtGVK = schema.GroupVersionKind{
			Group:   v1alpha1.Group,
			Version: v1alpha1.Version,
			Kind:    v1alpha1.RTKind,
		}
		nrtGVK = schema.GroupVersionKind{
			Group:   v1alpha1.InternalGroup,
			Version: v1alpha1.Version,
			Kind:    v1alpha1.NRTKind,
		}
		rt1u  *unstructured.Unstructured
		rt2u  *unstructured.Unstructured
		rt3u  *unstructured.Unstructured
		rt4u  *unstructured.Unstructured
		rt5u  *unstructured.Unstructured
		nrt1u *unstructured.Unstructured
		node1 *v1.Node
		node2 *v1.Node
	)
	BeforeEach(func() {
		_ = yaml.Unmarshal([]byte(rt1YAML), &rt1u)
		_ = yaml.Unmarshal([]byte(rt2YAML), &rt2u)
		_ = yaml.Unmarshal([]byte(rt3YAML), &rt3u)
		_ = yaml.Unmarshal([]byte(rt4YAML), &rt4u)
		_ = yaml.Unmarshal([]byte(rt5YAML), &rt5u)
		_ = yaml.Unmarshal([]byte(nrt11YAML), &nrt1u)
		_ = yaml.Unmarshal([]byte(node1YAML), &node1)
		_ = yaml.Unmarshal([]byte(node2YAML), &node2)
	})

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	f.RegisterCRD(rtGVK.Group, rtGVK.Version, rtGVK.Kind, false)
	f.RegisterCRD(nrtGVK.Group, nrtGVK.Version, nrtGVK.Kind, false)

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

	Context("Checking the creation operation of a CR SDNInternalNodeRoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt1YAML + rt2YAML + rt3YAML + rt4YAML + rt5YAML + node1YAML + node2YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())

			nrt11Name := "testrt1" + "-" + lib.GenerateShortHash("testrt1"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nrtKeyPath + ".0.name").String()).To(Equal(nrt11Name))
			Expect(f.ValuesGet(nrtKeyPath + ".0.nodeName").String()).To(Equal("kube-worker-1"))
			Expect(f.ValuesGet(nrtKeyPath + ".0.ownerRTName").String()).To(Equal("testrt1"))
			Expect(f.ValuesGet(nrtKeyPath + ".0.ipRoutingTableID").String()).To(Equal("500"))
			Expect(f.ValuesGet(nrtKeyPath + ".0.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 1.2.3.4
- destination: 192.168.0.0/24
  gateway: 192.168.0.1
- destination: 192.168.100.0/24
  dev: eth0
`))
			nrt12Name := "testrt1" + "-" + lib.GenerateShortHash("testrt1"+"#"+"kube-worker-2")
			Expect(f.ValuesGet(nrtKeyPath + ".1.name").String()).To(Equal(nrt12Name))
			Expect(f.ValuesGet(nrtKeyPath + ".1.nodeName").String()).To(Equal("kube-worker-2"))
			Expect(f.ValuesGet(nrtKeyPath + ".1.ownerRTName").String()).To(Equal("testrt1"))
			Expect(f.ValuesGet(nrtKeyPath + ".1.ipRoutingTableID").String()).To(Equal("500"))
			Expect(f.ValuesGet(nrtKeyPath + ".1.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 1.2.3.4
- destination: 192.168.0.0/24
  gateway: 192.168.0.1
- destination: 192.168.100.0/24
  dev: eth0
`))
			nrt21Name := "testrt2" + "-" + lib.GenerateShortHash("testrt2"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nrtKeyPath + ".3.name").String()).To(Equal(nrt21Name))
			Expect(f.ValuesGet(nrtKeyPath + ".3.nodeName").String()).To(Equal("kube-worker-1"))
			Expect(f.ValuesGet(nrtKeyPath + ".3.ownerRTName").String()).To(Equal("testrt2"))
			Expect(f.ValuesGet(nrtKeyPath + ".3.ipRoutingTableID").String()).To(Equal("300"))
			Expect(f.ValuesGet(nrtKeyPath + ".3.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 2.2.2.1
`))
			nrt22Name := "testrt2" + "-" + lib.GenerateShortHash("testrt2"+"#"+"kube-worker-2")
			Expect(f.ValuesGet(nrtKeyPath + ".2.name").String()).To(Equal(nrt22Name))
			Expect(f.ValuesGet(nrtKeyPath + ".2.nodeName").String()).To(Equal("kube-worker-2"))
			Expect(f.ValuesGet(nrtKeyPath + ".2.ownerRTName").String()).To(Equal("testrt2"))
			Expect(f.ValuesGet(nrtKeyPath + ".2.ipRoutingTableID").String()).To(Equal("300"))
			Expect(f.ValuesGet(nrtKeyPath + ".2.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 2.2.2.1
`))
			nrt31Name := "testrt3" + "-" + lib.GenerateShortHash("testrt3"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nrtKeyPath + ".5.name").String()).To(Equal(nrt31Name))
			Expect(f.ValuesGet(nrtKeyPath + ".5.nodeName").String()).To(Equal("kube-worker-1"))
			Expect(f.ValuesGet(nrtKeyPath + ".5.ownerRTName").String()).To(Equal("testrt3"))
			Expect(f.ValuesGet(nrtKeyPath + ".5.ipRoutingTableID").String()).To(Equal("300"))
			Expect(f.ValuesGet(nrtKeyPath + ".5.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 2.2.2.1
`))
			nrt32Name := "testrt3" + "-" + lib.GenerateShortHash("testrt3"+"#"+"kube-worker-2")
			Expect(f.ValuesGet(nrtKeyPath + ".4.name").String()).To(Equal(nrt32Name))
			Expect(f.ValuesGet(nrtKeyPath + ".4.nodeName").String()).To(Equal("kube-worker-2"))
			Expect(f.ValuesGet(nrtKeyPath + ".4.ownerRTName").String()).To(Equal("testrt3"))
			Expect(f.ValuesGet(nrtKeyPath + ".4.ipRoutingTableID").String()).To(Equal("300"))
			Expect(f.ValuesGet(nrtKeyPath + ".4.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 2.2.2.1
`))
			nrt41Name := "testrt4" + "-" + lib.GenerateShortHash("testrt4"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nrtKeyPath + ".6.name").String()).To(Equal(nrt41Name))
			Expect(f.ValuesGet(nrtKeyPath + ".6.nodeName").String()).To(Equal("kube-worker-1"))
			Expect(f.ValuesGet(nrtKeyPath + ".6.ownerRTName").String()).To(Equal("testrt4"))
			Expect(f.ValuesGet(nrtKeyPath + ".6.ipRoutingTableID").String()).To(Equal("20000"))
			Expect(f.ValuesGet(nrtKeyPath + ".6.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 2.2.2.1
`))
			nrt42Name := "testrt4" + "-" + lib.GenerateShortHash("testrt4"+"#"+"kube-worker-2")
			Expect(f.ValuesGet(nrtKeyPath + ".7.name").String()).To(Equal(nrt42Name))
			Expect(f.ValuesGet(nrtKeyPath + ".7.nodeName").String()).To(Equal("kube-worker-2"))
			Expect(f.ValuesGet(nrtKeyPath + ".7.ownerRTName").String()).To(Equal("testrt4"))
			Expect(f.ValuesGet(nrtKeyPath + ".7.ipRoutingTableID").String()).To(Equal("20000"))
			Expect(f.ValuesGet(nrtKeyPath + ".7.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 2.2.2.1
`))
			nrt51Name := "testrt5" + "-" + lib.GenerateShortHash("testrt5"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nrtKeyPath + ".9.name").String()).To(Equal(nrt51Name))
			Expect(f.ValuesGet(nrtKeyPath + ".9.nodeName").String()).To(Equal("kube-worker-1"))
			Expect(f.ValuesGet(nrtKeyPath + ".9.ownerRTName").String()).To(Equal("testrt5"))
			Expect(f.ValuesGet(nrtKeyPath + ".9.ipRoutingTableID").String()).To(Equal("300"))
			Expect(f.ValuesGet(nrtKeyPath + ".9.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 2.2.2.1
`))
			nrt52Name := "testrt5" + "-" + lib.GenerateShortHash("testrt5"+"#"+"kube-worker-2")
			Expect(f.ValuesGet(nrtKeyPath + ".8.name").String()).To(Equal(nrt52Name))
			Expect(f.ValuesGet(nrtKeyPath + ".8.nodeName").String()).To(Equal("kube-worker-2"))
			Expect(f.ValuesGet(nrtKeyPath + ".8.ownerRTName").String()).To(Equal("testrt5"))
			Expect(f.ValuesGet(nrtKeyPath + ".8.ipRoutingTableID").String()).To(Equal("300"))
			Expect(f.ValuesGet(nrtKeyPath + ".8.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 2.2.2.1
`))
		})
	})

	Context("Checking the deletion operation of a CR SDNInternalNodeRoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + nrt11YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			nrt11Name := "testrt1" + "-" + lib.GenerateShortHash("testrt1"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nrtKeyPath).String()).To(Equal("[]"))
			Expect(f.ValuesGet(nrtKeyPath + ".0.name").String()).NotTo(Equal(nrt11Name))
		})
	})

	Context("Checking case when node was deleted", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt1YAML + orphannrt11YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			nrt11Name := "testrt1" + "-" + lib.GenerateShortHash("testrt1"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nrtKeyPath).String()).To(Equal("[]"))
			Expect(f.ValuesGet(nrtKeyPath + ".0.name").String()).NotTo(Equal(nrt11Name))
			Expect(f.KubernetesGlobalResource(v1alpha1.NRTKind, nrt11Name).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource(v1alpha1.NRTKind, nrt11Name).Field("metadata.finalizers").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource(v1alpha1.NRTKind, nrt11Name).Field("metadata.finalizers").String()).To(MatchYAML(`
- yet-another-finalizer
`))
			Expect(f.KubernetesGlobalResource(v1alpha1.NRTKind, nrt11Name).Field("metadata.deletionTimestamp").Exists()).To(BeTrue())

		})
	})

	Context("Checking the updating operation of a CR SDNInternalNodeRoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + rt1upYAML + nrt11YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			nrt11Name := "testrt1" + "-" + lib.GenerateShortHash("testrt1"+"#"+"kube-worker-1")
			Expect(f.ValuesGet(nrtKeyPath + ".0.name").String()).To(Equal(nrt11Name))
			Expect(f.ValuesGet(nrtKeyPath + ".0.nodeName").String()).To(Equal("kube-worker-1"))
			Expect(f.ValuesGet(nrtKeyPath + ".0.ownerRTName").String()).To(Equal("testrt1"))
			Expect(f.ValuesGet(nrtKeyPath + ".0.ipRoutingTableID").String()).To(Equal("500"))
			Expect(f.ValuesGet(nrtKeyPath + ".0.routes").String()).To(MatchYAML(`
- destination: 0.0.0.0/0
  gateway: 1.2.3.4
- destination: 192.168.1.0/24
  gateway: 192.168.2.1
- destination: 192.168.200.0/24
  dev: eth3
`))
		})
	})

	Context("Checking setting id in status(from spec) of a CR RoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt3YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt3").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt3").Field("status.ipRoutingTableID").Exists()).To(BeTrue())
			rtstatus := f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt3").Field("status").String()
			Expect(rtstatus).NotTo(Equal(""))
			rtstatusid := f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt3").Field("status.ipRoutingTableID").String()
			Expect(rtstatusid).To(MatchYAML(`300`))
		})
	})

	Context("Checking generating and setting id in status of a CR RoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt3YAML + rt4YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt4").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt4").Field("status.ipRoutingTableID").Exists()).To(BeTrue())
			rtstatus := f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt4").Field("status").String()
			Expect(rtstatus).NotTo(Equal(""))
			rtstatusid := f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt4").Field("status.ipRoutingTableID").String()
			Expect(rtstatusid).To(MatchYAML(`20000`))
		})
	})

	Context("Checking setting id in status(from spec) (overwrite) of a CR RoutingTable", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(rt5YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt5").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt5").Field("status.ipRoutingTableID").Exists()).To(BeTrue())
			rtstatus := f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt5").Field("status").String()
			Expect(rtstatus).NotTo(Equal(""))
			rtstatusid := f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt5").Field("status.ipRoutingTableID").String()
			Expect(rtstatusid).To(MatchYAML(`300`))
		})
	})

	Context("Checking condition update", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(node1YAML + rt666YAML + nrt666YAML))
			f.RunHook()
		})

		It("Hook must execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(string(f.LogrusOutput.Contents())).To(HaveLen(0))
			Expect(f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt666").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt666").Field("status").Exists()).To(BeTrue())
			rtstatusraw := f.KubernetesGlobalResource(v1alpha1.RTKind, "testrt666").Field("status").String()
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
