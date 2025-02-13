/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vsphere :: hooks :: change host ip ::", func() {
	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("With CCM pod", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Pod
metadata:
  name: ccm-test
  namespace: d8-cloud-provider-vsphere
  labels:
    app: cloud-controller-manager
status:
  hostIP: 1.2.3.4
`, 1))
			f.RunHook()
		})

		It("Should run", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-cloud-provider-vsphere", "ccm-test")
			Expect(pod.Exists()).To(BeTrue())
			Expect(pod.Field(`metadata.annotations.node\.deckhouse\.io\/initial-host-ip`).String()).To(Equal("1.2.3.4"))
		})

		Context("Changing host ip", func() {
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Pod
metadata:
  name: ccm-test
  namespace: d8-cloud-provider-vsphere
  labels:
    app: cloud-controller-manager
  annotations:
    node.deckhouse.io/initial-host-ip: "1.2.3.4"
status:
  hostIP: 4.5.6.7
`, 2))
				f.RunHook()
			})

			It("Should delete the pod", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("Pod", "d8-cloud-provider-vsphere", "ccm-test").Exists()).To(BeFalse())
			})
		})
	})

	Context("With same initial ip and host ip", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Pod
metadata:
  name: ccm-test
  namespace: d8-cloud-provider-vsphere
  labels:
    app: cloud-controller-manager
  annotations:
    node.deckhouse.io/initial-host-ip: "1.2.3.4"
status:
  hostIP: 1.2.3.4
`, 1))
			f.RunHook()
		})

		It("Should leave the pod as it is", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-cloud-provider-vsphere", "ccm-test")
			Expect(pod.Exists()).To(BeTrue())
			Expect(pod.Field(`metadata.annotations.node\.deckhouse\.io\/initial-host-ip`).String()).To(Equal("1.2.3.4"))
		})
	})

	Context("With empty host ip", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(`
---
apiVersion: v1
kind: Pod
metadata:
  name: ccm-test
  namespace: d8-cloud-provider-vsphere
  labels:
    app: deckhouse
  annotations:
    node.deckhouse.io/initial-host-ip: "1.2.3.4"
status: {}
`, 1))
			f.RunHook()
		})

		It("Should leave the pod as it is", func() {
			Expect(f).To(ExecuteSuccessfully())
			pod := f.KubernetesResource("Pod", "d8-cloud-provider-vsphere", "ccm-test")
			Expect(pod.Exists()).To(BeTrue())
			Expect(pod.Field(`metadata.annotations.node\.deckhouse\.io\/initial-host-ip`).String()).To(Equal("1.2.3.4"))
		})
	})
})
