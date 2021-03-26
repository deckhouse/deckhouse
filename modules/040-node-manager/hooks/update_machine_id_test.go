package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-yandex :: hooks :: update_machine_id ::", func() {
	const (
		nodes = `
---
apiVersion: v1
kind: Node
metadata:
  name: worker-1
spec:
  providerID: yandex://1
---
apiVersion: v1
kind: Node
metadata:
  name: worker-2
spec:
  providerID: yandex://2
---
apiVersion: v1
kind: Node
metadata:
  name: worker-3
---
apiVersion: v1
kind: Node
metadata:
  name: worker-4
spec:
  providerID: yandex://4
---
apiVersion: v1
kind: Node
metadata:
  name: master
spec:
  providerID: yandex://master
`
		machines = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-1
  namespace: d8-cloud-instance-manager
spec:
  providerID: yandex://1/zone-a/worker-1
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-2
  namespace: d8-cloud-instance-manager
spec:
  providerID: yandex://1/zone-a/worker-2
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-3
  namespace: d8-cloud-instance-manager
spec:
  providerID: yandex://unchangeable
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-4
  namespace: d8-cloud-instance-manager
spec:
`
	)

	f := HookExecutionConfigInit(`{"global":{"enabledModules":["cloud-provider-yandex"]}}`, `{}`)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with node and machine", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(nodes + machines))
			f.RunHook()
		})

		It("Must be executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-1").Field("spec.providerID").String()).To(Equal("yandex://1"))
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-2").Field("spec.providerID").String()).To(Equal("yandex://2"))
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-3").Field("spec.providerID").String()).To(Equal("yandex://unchangeable"))
			Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-4").Field("spec.providerID").String()).To(Equal("yandex://4"))
		})
	})
})
