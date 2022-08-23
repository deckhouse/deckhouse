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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.etcd.io/etcd/api/v3/etcdserverpb"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: controler-plane-manager :: hooks :: reconcile-etcd-members ::", func() {
	var (
		initValuesString = `{"controlPlaneManager":{"internal": {}, "apiserver": {"authn": {}, "authz": {}}}}`
	)
	const (
		initConfigValuesString = ``
	)

	var (
		reconcileEtcdMembers = []*etcdserverpb.Member{
			{
				ID:       111,
				PeerURLs: []string{"https://192.168.1.1:2379"},
				Name:     "main-master-0",
			},
			{
				ID:       222,
				PeerURLs: []string{"https://192.168.1.2:2379"},
				Name:     "main-master-1",
			},
			{
				ID:       333,
				PeerURLs: []string{"https://192.168.1.3:2379"},
				Name:     "main-master-2",
			},
		}

		reconcileStartState = `
---
apiVersion: v1
kind: Node
metadata:
  name: main-master-0
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  addresses:
    - address: 192.168.1.1
      type: InternalIP
---
apiVersion: v1
kind: Node
metadata:
  name: main-master-1
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  addresses:
    - address: 192.168.1.2
      type: InternalIP
---
apiVersion: v1
kind: Node
metadata:
  name: main-master-2
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  addresses:
    - address: 192.168.1.3
      type: InternalIP
`

		reconcileChangedState = strings.Join(strings.Split(reconcileStartState, "---")[:3], "---")
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)
	testHelperRegisterEtcdMemberUpdate()
	setEtcdMembers := func() {
		testHelperSetETCDMembers(reconcileEtcdMembers)
	}

	Context("Multimaster cluster set", func() {
		BeforeEach(func() {
			setEtcdMembers()
			f.BindingContexts.Set(f.KubeStateSet(testETCDSecret + reconcileStartState))
			f.RunHook()
		})

		It("Hook is running successfully", func() {
			Expect(f).Should(ExecuteSuccessfully())
		})

		Context("main-master-2 was removed", func() {
			BeforeEach(func() {
				setEtcdMembers()
				f.BindingContexts.Set(f.KubeStateSet(testETCDSecret + reconcileChangedState))
				f.RunHook()
			})

			It("Expects main-master-2 etcd member was removed", func() {
				Expect(f).Should(ExecuteSuccessfully())
				resp, _ := dependency.TestDC.EtcdClient.MemberList(context.TODO())
				Expect(resp.Members).To(HaveLen(2))
			})
		})

		Context("etcd external member defined", func() {
			BeforeEach(func() {
				setEtcdMembers()
				f.ValuesSet("controlPlaneManager.etcd.externalMembersNames", []string{"main-master-2"})
				f.BindingContexts.Set(f.KubeStateSet(testETCDSecret + reconcileChangedState))
				f.RunHook()
			})

			It("External etcd member should left", func() {
				Expect(f).Should(ExecuteSuccessfully())
				resp, _ := dependency.TestDC.EtcdClient.MemberList(context.TODO())
				Expect(resp.Members).To(HaveLen(3))
			})
		})

		Context("All old masters were removed", func() {
			BeforeEach(func() {
				setEtcdMembers()
				f.BindingContexts.Set(f.KubeStateSet(testETCDSecret + `
---
apiVersion: v1
kind: Node
metadata:
  name: main-master-3
  labels:
    node-role.kubernetes.io/control-plane: ""
status:
  addresses:
    - address: 192.168.1.4
      type: InternalIP
`))
				f.RunHook()
			})

			It("Should exit with error: remove all members", func() {
				Expect(f).ShouldNot(ExecuteSuccessfully())
				Expect(f.GoHookError).Should(MatchError("attempting do delete every single member from etcd cluster. Exiting"))
			})
		})
	})
})
