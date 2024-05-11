/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: system-registry :: hooks :: discover_etcd_addresses ::", func() {
	const (
		stateEtcdPod1 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd-1
  namespace: kube-system
  labels:
    component: "etcd"
    tier: "control-plane"
status:
  hostIP: 192.168.1.1
  conditions:
  - status: "True"
    type: Ready
`
		stateEtcdPod2 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd-2
  namespace: kube-system
  labels:
    component: "etcd"
    tier: "control-plane"
status:
  hostIP: 192.168.1.2
  conditions:
  - status: "True"
    type: Ready
`
		stateEtcdPod3 = `
---
apiVersion: v1
kind: Pod
metadata:
  name: etcd-3
  namespace: kube-system
  labels:
    component: "etcd"
    tier: "control-plane"
status:
  hostIP: 192.168.1.3
  conditions:
  - status: "False"
    type: Ready
`
	)

	f := HookExecutionConfigInit(`{"systemRegistry":{"internal":{"etcd":{}}}}`, `{}`)

	Context("Etcd pods are not found", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(""))
			f.RunHook()
		})

		It("Hook should execute successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("One etcd pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateEtcdPod1))
			f.RunHook()
		})

		It("`systemRegistry.internal.etcd.addresses` must be ['192.168.1.1:2379']", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("systemRegistry.internal.etcd.addresses").String()).To(MatchJSON(`["192.168.1.1:2379"]`))
		})

		Context("Add second etcd pod", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(stateEtcdPod1 + stateEtcdPod2))
				f.RunHook()
			})

			It("`systemRegistry.internal.etcd.addresses` must be ['192.168.1.1:2379','192.168.1.2:2379']", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet("systemRegistry.internal.etcd.addresses").String()).To(MatchJSON(`["192.168.1.1:2379","192.168.1.2:2379"]`))
			})

			Context("Add third etcd not ready pod", func() {
				BeforeEach(func() {
					f.BindingContexts.Set(f.KubeStateSet(stateEtcdPod1 + stateEtcdPod2 + stateEtcdPod3))
					f.RunHook()
				})

				It("`systemRegistry.internal.etcd.addresses` must be ['192.168.1.1:2379','192.168.1.2:2379'], third pod is not ready", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.ValuesGet("systemRegistry.internal.etcd.addresses").String()).To(MatchJSON(`["192.168.1.1:2379","192.168.1.2:2379"]`))
				})
			})
		})
	})
})
