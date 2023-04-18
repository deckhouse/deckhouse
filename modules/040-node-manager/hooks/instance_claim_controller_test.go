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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = FDescribe("Modules :: node-manager :: hooks :: instance_claim_controller ::", func() {
	f := HookExecutionConfigInit(`{"global": {"discovery": {"kubernetesVersion": "1.23.1"}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "InstanceClaim", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	const ng = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
  uid: 87233806-25b3-41b4-8c15-46b7212326b4
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
`

	assertFinalizersExists := func(f *HookExecutionConfig, claimName string) {
		finalizers := f.KubernetesGlobalResource("InstanceClaim", claimName).Field("metadata.finalizers")
		Expect(finalizers.AsStringSlice()).To(Equal([]string{"hooks.deckhouse.io/node-manager/instance_claim_controller"}))
	}

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Removing instance claims", func() {
		const (
			ic1 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceClaim
metadata:
  name: worker-ac32h
  finalizers:
  - hooks.deckhouse.io/node-manager/instance_claim_controller
status: {}
`
			machine = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: worker-ac32h
  namespace: d8-cloud-instance-manager
  labels:
    instance-group: ng1-nova
spec:
  nodeTemplate:
    metadata:
      labels:
        node-role.kubernetes.io/ng1: ""
        node.deckhouse.io/group: ng1
        node.deckhouse.io/type: CloudEphemeral
`
		)

		Context("does not start deletion instance claim (without deletion timestamp", func() {
			const ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceClaim
metadata:
  name: worker-bg11u
  finalizers:
  - hooks.deckhouse.io/node-manager/instance_claim_controller
status: {}
`
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + ic2 + machine))
				f.RunHook()
			})

			It("Should keep instance claim with machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("InstanceClaim", "worker-ac32h").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h"))
				assertFinalizersExists(f, "worker-ac32h")
			})

			It("Should delete instance claim without machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("InstanceClaim", "worker-bg11u").Exists()).To(BeFalse())
			})
		})

		Context("start deletion instance claim (with deletion timestamp)", func() {
			const ic2 = `
---
apiVersion: deckhouse.io/v1alpha1
kind: InstanceClaim
metadata:
  name: worker-bg11u
  finalizers:
  - hooks.deckhouse.io/node-manager/instance_claim_controller
  deletionTimestamp: "1970-01-01T00:00:00Z"
status: {}
`
			BeforeEach(func() {
				f.BindingContexts.Set(f.KubeStateSet(ng + ic1 + ic2 + machine))
				f.RunHook()
			})

			It("Should keep instance claim with machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				Expect(f.KubernetesGlobalResource("NodeGroup", "ng1").Exists()).To(BeTrue())
				Expect(f.KubernetesGlobalResource("InstanceClaim", "worker-ac32h").Exists()).To(BeTrue())
				Expect(f.KubernetesResource("Machine", "d8-cloud-instance-manager", "worker-ac32h"))
				assertFinalizersExists(f, "worker-ac32h")
			})

			It("Should remove finalizers from instance claim without machine", func() {
				Expect(f).To(ExecuteSuccessfully())

				ic := f.KubernetesGlobalResource("InstanceClaim", "worker-bg11u")
				Expect(ic.Exists()).To(BeTrue())
				Expect(ic.Field("metadata.finalizers").Array()).To(BeEmpty())
			})
		})
	})
})
