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

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.etcd.io/etcd/api/v3/etcdserverpb"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: control-plane-manager :: hooks :: check_etcd_peer_urls ::", func() {
	var (
		initValuesString = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
	)
	const (
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
  nodeName: main-master-0
status:
  phase: Running
  conditions:
  - type: Ready
    status: "True"
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
  nodeName: main-master-0
status:
  phase: Running
  conditions:
  - type: Ready
    status: "True"
`
		masterNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: main-master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
`
		secondMasterNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: main-master-1
  labels:
    node-role.kubernetes.io/control-plane: ""
`
		// etcd pod left on a node whose control-plane role was removed,
		// sorted before the master pod by name
		staleEtcdPodOnFormerMaster = `
---
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: etcd
    tier: control-plane
  name: etcd-main-former-master
  namespace: kube-system
spec:
  containers:
  - command:
    - etcd
    - --listen-peer-urls=https://192.168.199.100:2380
    name: etcd
  nodeName: main-former-master
status:
  phase: Running
  conditions:
  - type: Ready
    status: "True"
---
apiVersion: v1
kind: Node
metadata:
  name: main-former-master
`
		notReadyEtcdPod = `
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
  - command:
    - etcd
    - --listen-peer-urls=https://192.168.199.182:2380
    name: etcd
  nodeName: main-master-0
status:
  phase: Running
  conditions:
  - type: Ready
    status: "False"
`
	)
	var (
		checkEtcdPeerMembers = []*etcdserverpb.Member{
			{
				ID:       123456,
				PeerURLs: []string{"https://localhost:2380"},
				Name:     "main-master-0",
			},
		}
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	testHelperRegisterEtcdMemberUpdate()
	setEtcdMembers := func() {
		testHelperSetETCDMembers(checkEtcdPeerMembers)
	}

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
			setEtcdMembers()
			f.BindingContexts.Set(f.KubeStateSet(etcdPod + masterNode + testETCDSecret))
			f.RunHook()
		})

		It("Test etcd member should be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			resp, _ := dependency.TestDC.EtcdClient.MemberList(context.Background())
			Expect(resp.Members).To(HaveLen(1))
			Expect(resp.Members[0].PeerURLs[0]).To(BeEquivalentTo("https://192.168.199.182:2380"))
		})

	})

	Context("Cluster started with etcd Pod and multiple masters", func() {
		BeforeEach(func() {
			setEtcdMembers()
			f.BindingContexts.Set(f.KubeStateSet(etcdPod + masterNode + secondMasterNode + testETCDSecret))
			f.RunHook()
		})

		It("Test etcd member should not be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			resp, _ := dependency.TestDC.EtcdClient.MemberList(context.Background())
			Expect(resp.Members).To(HaveLen(1))
			Expect(resp.Members[0].PeerURLs[0]).To(BeEquivalentTo("https://localhost:2380"))
		})

	})

	Context("Cluster started with etcd Pod with proper peer-urls and single master", func() {
		BeforeEach(func() {
			setEtcdMembers()
			f.BindingContexts.Set(f.KubeStateSet(etcdPodLocalhostPeer + masterNode + testETCDSecret))
			f.RunHook()
		})

		It("Test etcd member should not be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			resp, _ := dependency.TestDC.EtcdClient.MemberList(context.Background())
			Expect(resp.Members).To(HaveLen(1))
			Expect(resp.Members[0].PeerURLs[0]).To(BeEquivalentTo("https://localhost:2380"))
		})

	})

	Context("Single master and stale etcd Pod on a node without control-plane role", func() {
		BeforeEach(func() {
			setEtcdMembers()
			f.BindingContexts.Set(f.KubeStateSet(etcdPod + masterNode + staleEtcdPodOnFormerMaster + testETCDSecret))
			f.RunHook()
		})

		It("Etcd member should be updated with peer URL of the master etcd Pod", func() {
			Expect(f).To(ExecuteSuccessfully())
			resp, _ := dependency.TestDC.EtcdClient.MemberList(context.Background())
			Expect(resp.Members).To(HaveLen(1))
			Expect(resp.Members[0].PeerURLs[0]).To(BeEquivalentTo("https://192.168.199.182:2380"))
		})
	})

	Context("Single master with not ready etcd Pod", func() {
		BeforeEach(func() {
			setEtcdMembers()
			f.BindingContexts.Set(f.KubeStateSet(notReadyEtcdPod + masterNode + testETCDSecret))
			f.RunHook()
		})

		It("Etcd member should not be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			resp, _ := dependency.TestDC.EtcdClient.MemberList(context.Background())
			Expect(resp.Members).To(HaveLen(1))
			Expect(resp.Members[0].PeerURLs[0]).To(BeEquivalentTo("https://localhost:2380"))
		})
	})

	Context("Single master and etcd Pod only on a node without control-plane role", func() {
		BeforeEach(func() {
			setEtcdMembers()
			f.BindingContexts.Set(f.KubeStateSet(masterNode + staleEtcdPodOnFormerMaster + testETCDSecret))
			f.RunHook()
		})

		It("Etcd member should not be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			resp, _ := dependency.TestDC.EtcdClient.MemberList(context.Background())
			Expect(resp.Members).To(HaveLen(1))
			Expect(resp.Members[0].PeerURLs[0]).To(BeEquivalentTo("https://localhost:2380"))
		})
	})
})
