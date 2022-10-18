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
			f.BindingContexts.Set(f.KubeStateSet(etcdPod + testETCDSecret))
			f.ValuesSet("global.discovery.clusterMasterCount", 1)
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
			f.BindingContexts.Set(f.KubeStateSet(etcdPod + testETCDSecret))
			f.ValuesSet("global.discovery.clusterMasterCount", 2)
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
			f.BindingContexts.Set(f.KubeStateSet(etcdPodLocalhostPeer + testETCDSecret))
			f.ValuesSet("global.discovery.clusterMasterCount", 1)
			f.RunHook()
		})

		It("Test etcd member should not be updated", func() {
			Expect(f).To(ExecuteSuccessfully())
			resp, _ := dependency.TestDC.EtcdClient.MemberList(context.Background())
			Expect(resp.Members).To(HaveLen(1))
			Expect(resp.Members[0].PeerURLs[0]).To(BeEquivalentTo("https://localhost:2380"))
		})

	})
})
