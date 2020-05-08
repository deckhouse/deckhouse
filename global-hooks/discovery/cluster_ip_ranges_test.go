/*

User-stories:
1. There are Nodes in cluster. They have information about InternalIP in .status.addresses list. Hook must parse all InternalIPs and store them to `global.discovery.nodeInternalIPs` as array.
2. There is kube-controller-manager in cluster. It has --cluster-cidr=<subnet> in his args. Hook must parse subnet and store it to `global.discovery.podSubnet`.
3. There is kube-apiserver in cluster. It has --service-cluster-ip-range=<subnet> in his args. Hook must parse subnet and store it to `global.discovery.serviceSubnet`.
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery/cluster_ip_ranges ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateMaster = `
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
status:
  addresses:
  - address: 10.0.3.192
    type: ExternalIP
  - address: 192.168.199.10
    type: InternalIP
`
		stateMasterAndHisFriends = `
---
apiVersion: v1
kind: Node
metadata:
  name: sandbox-21-master
status:
  addresses:
  - address: 10.0.3.192
    type: ExternalIP
  - address: 192.168.199.10
    type: InternalIP
---
apiVersion: v1
kind: Node
metadata:
  name: nif-nif
status:
  addresses:
  - address: 10.0.3.192
    type: ExternalIP
  - address: 192.168.199.20
    type: InternalIP
---
apiVersion: v1
kind: Node
metadata:
  name: naf-naf
status:
  addresses:
  - address: 10.0.3.192
    type: ExternalIP
  - address: 192.168.199.30
    type: InternalIP
  - address: 192.168.199.31
    type: InternalIP
---
apiVersion: v1
kind: Node
metadata:
  name: nuf-nuf
status:
  addresses:
  - address: 10.0.3.192
    type: ExternalIP
`
		stateControllerManagerK8SApp = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kube-controller-manager
    tier: control-plane
  name: kube-controller-manager-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    command:
    - kube-controller-manager
    args:
    - --cluster-cidr=192.168.10.0/24
    - zzz
`

		stateControllerManagerComponent = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kube-controller-manager
    tier: control-plane
  name: kube-controller-manager-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-controller-manager
    command:
    - kube-controller-manager
    - --cluster-cidr=192.168.20.0/24
    args:
    - qqq
`

		stateApiserverK8SApp = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    k8s-app: kube-apiserver
  name: kube-apiserver-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    command:
    - kube-apiserver
    args:
    - --service-cluster-ip-range=192.168.30.0/24
    - zzz
`

		stateApiserverComponent = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: kube-apiserver
    tier: control-plane
  name: kube-apiserver-sandbox-21-master
  namespace: kube-system
spec:
  containers:
  - name: kube-apiserver
    command:
    - kube-apiserver
    - --service-cluster-ip-range=192.168.40.0/24
    args:
    - qqq
`
		stateClusterConfiguration = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cluster-configuration
  namespace: d8-system
data:
  cluster-configuration.yaml: test
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("Single master; controller-manager by k8s-app; apiserver by k8s-app", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMaster + stateControllerManagerK8SApp + stateApiserverK8SApp))
			f.RunHook()
		})

		It("Expect: nodeInternalIPs — array of 1 ips; podSubnet = '192.168.10.0/24'; serviceSubnet = '192.168.30.0/24'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.nodeInternalIPs").String()).To(MatchJSON(`["192.168.199.10"]`))
			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("192.168.10.0/24"))
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("192.168.30.0/24"))
		})

		Context("master + additional nodes; controller-manager by k8s-app; apiserver by k8s-app", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMasterAndHisFriends + stateControllerManagerK8SApp + stateApiserverK8SApp))
				f.RunHook()
			})

			It("Expect: nodeInternalIPs — array of 4 ips; podSubnet = '192.168.10.0/24'; serviceSubnet = '192.168.30.0/24'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.nodeInternalIPs").String()).To(MatchJSON(`["192.168.199.10","192.168.199.20","192.168.199.30","192.168.199.31"]`))
				Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("192.168.10.0/24"))
				Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("192.168.30.0/24"))
			})
		})

	})

	Context("With d8-cluster-configuration secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMaster + stateControllerManagerK8SApp + stateApiserverK8SApp + stateClusterConfiguration))
			f.RunHook()
		})

		It("Should not have \"global.discovery.podSubnet\" and \"global.discovery.serviceSubnet\" values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(BeEmpty())
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(BeEmpty())
		})
	})

	Context("multiple nodes; controller-manager by component; apiserver by component", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateMasterAndHisFriends + stateControllerManagerComponent + stateApiserverComponent))
			f.RunHook()
		})

		It("Expect: nodeInternalIPs — array of 4 ips; podSubnet = '192.168.20.0/24'; serviceSubnet = '192.168.40.0/24'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.nodeInternalIPs").String()).To(MatchJSON(`["192.168.199.10","192.168.199.20","192.168.199.30","192.168.199.31"]`))
			Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("192.168.20.0/24"))
			Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("192.168.40.0/24"))
		})

		Context("single master; controller-manager by component; apiserver by component", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateMaster + stateControllerManagerComponent + stateApiserverComponent))
				f.RunHook()
			})

			It("Expect: nodeInternalIPs — array of 1 ips; podSubnet = '192.168.20.0/24'; serviceSubnet = '192.168.40.0/24'", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("global.discovery.nodeInternalIPs").String()).To(MatchJSON(`["192.168.199.10"]`))
				Expect(f.ValuesGet("global.discovery.podSubnet").String()).To(Equal("192.168.20.0/24"))
				Expect(f.ValuesGet("global.discovery.serviceSubnet").String()).To(Equal("192.168.40.0/24"))
			})
		})
	})
})
