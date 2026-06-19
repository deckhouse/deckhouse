// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*

User-stories:
1. Hook must discover nodes with label node.deckhouse.io/etcd-arbiter and set global.discovery.hasEtcdArbiterNode to true if exactly one such node exists, else — to false.
2. If more than one node has label node.deckhouse.io/etcd-arbiter — hook must fail.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: cluster_etcd_arbiter ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		stateFirstEtcdArbiterNode = `
apiVersion: v1
kind: Node
metadata:
  name: arbiter-0
  labels:
    node.deckhouse.io/etcd-arbiter: ""`

		stateSecondEtcdArbiterNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: arbiter-1
  labels:
    node.deckhouse.io/etcd-arbiter: ""`

		stateThirdEtcdArbiterNode = `
---
apiVersion: v1
kind: Node
metadata:
  name: arbiter-2
  labels:
    node.deckhouse.io/etcd-arbiter: ""`
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

		It("`global.discovery.hasEtcdArbiterNode` must be false", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.hasEtcdArbiterNode").Bool()).To(BeFalse())
		})
	})

	Context("One etcd arbiter node in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstEtcdArbiterNode))
			f.RunHook()
		})

		It("`global.discovery.hasEtcdArbiterNode` must be true", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.BindingContexts.Array()).ShouldNot(BeEmpty())

			Expect(f.ValuesGet("global.discovery.hasEtcdArbiterNode").Bool()).To(BeTrue())
		})
	})

	Context("More than one etcd arbiter node in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstEtcdArbiterNode + stateSecondEtcdArbiterNode))
			f.RunHook()
		})

		It("Must fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})

	Context("More than two etcd arbiter nodes in cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateFirstEtcdArbiterNode + stateSecondEtcdArbiterNode + stateThirdEtcdArbiterNode))
			f.RunHook()
		})

		It("Must fail", func() {
			Expect(f).NotTo(ExecuteSuccessfully())
		})
	})
})
