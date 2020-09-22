package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controler-plane-manager :: hooks :: check_etcd_peer_urls ::", func() {
	const (
		initValuesString       = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
		initConfigValuesString = ``
		etcdPod                = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: etcd
    tier: control-plane
  name: etcd-main-master-0
  namespace: kube-system
spec:
  containers:
  - args:
    - --listen-peer-urls=https://192.168.199.182:2380,https://192.168.199.182:2381
    command:
    - etcd
    - --listen-peer-urls=https://192.168.199.182:2380
    name: etcd
status:
  phase: Running
`
		etcdPodLocalhostPeer = `
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: etcd
    tier: control-plane
  name: etcd-main-master-0
  namespace: kube-system
spec:
  containers:
  - args:
    - --listen-peer-urls=https://localhost:2380,https://localhost:2381
    command:
    - etcd
    - --listen-peer-urls=https://localhost:2380
    name: etcd
status:
  phase: Running
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster started with etcd Pod and single master", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(etcdPod))
			f.ValuesSet("global.discovery.clusterMasterCount", 1)
			f.RunHook()
		})

		It("Test etcd member should be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.testEtcdMemberUpdated").Exists()).To(BeTrue())
		})

	})

	Context("Cluster started with etcd Pod and multiple masters", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(etcdPod))
			f.ValuesSet("global.discovery.clusterMasterCount", 2)
			f.RunHook()
		})

		It("Test etcd member should not be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.testEtcdMemberUpdated").Exists()).To(BeFalse())
		})

	})

	Context("Cluster started with etcd Pod with proper peer-urls and single master", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(etcdPodLocalhostPeer))
			f.ValuesSet("global.discovery.clusterMasterCount", 1)
			f.RunHook()
		})

		It("Test etcd member should not be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("controlPlaneManager.internal.testEtcdMemberUpdated").Exists()).To(BeFalse())
		})

	})

})
