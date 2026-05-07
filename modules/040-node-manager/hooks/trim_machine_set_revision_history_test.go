/*
Copyright 2026 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: trim_machine_set_revision_history ::", func() {
	const machineSets = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineSet
metadata:
  name: long-revision-history
  namespace: d8-cloud-instance-manager
  annotations:
    deployment.kubernetes.io/revision-history: "1,2,3,4,5,6,7,8,9"
    other-annotation: value
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineSet
metadata:
  name: short-revision-history
  namespace: d8-cloud-instance-manager
  annotations:
    deployment.kubernetes.io/revision-history: "1,2,3"
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineSet
metadata:
  name: boundary-revision-history
  namespace: d8-cloud-instance-manager
  annotations:
    deployment.kubernetes.io/revision-history: "1,2,3,4,5,6,7,8"
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineSet
metadata:
  name: long-revision-history-without-comma
  namespace: d8-cloud-instance-manager
  annotations:
    deployment.kubernetes.io/revision-history: "12345678901234567"
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineSet
metadata:
  name: empty-revision-history
  namespace: d8-cloud-instance-manager
  annotations:
    deployment.kubernetes.io/revision-history: ""
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineSet
metadata:
  name: absent-revision-history
  namespace: d8-cloud-instance-manager
`

	f := HookExecutionConfigInit(`{"nodeManager":{"internal":{}}}`, `{}`)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineSet", true)

	Context("Cluster with MachineSets", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machineSets))
			f.RunHook()
		})

		It("trims long revision history and keeps short or absent values unchanged", func() {
			Expect(f).To(ExecuteSuccessfully())

			longMachineSet := f.KubernetesResource("MachineSet", "d8-cloud-instance-manager", "long-revision-history")
			Expect(longMachineSet.Field(`metadata.annotations.deployment\.kubernetes\.io\/revision-history`).String()).To(Equal("1"))
			Expect(longMachineSet.Field(`metadata.annotations.other-annotation`).String()).To(Equal("value"))

			shortMachineSet := f.KubernetesResource("MachineSet", "d8-cloud-instance-manager", "short-revision-history")
			Expect(shortMachineSet.Field(`metadata.annotations.deployment\.kubernetes\.io\/revision-history`).String()).To(Equal("1,2,3"))

			boundaryMachineSet := f.KubernetesResource("MachineSet", "d8-cloud-instance-manager", "boundary-revision-history")
			Expect(boundaryMachineSet.Field(`metadata.annotations.deployment\.kubernetes\.io\/revision-history`).String()).To(Equal("1,2,3,4,5,6,7,8"))

			longWithoutCommaMachineSet := f.KubernetesResource("MachineSet", "d8-cloud-instance-manager", "long-revision-history-without-comma")
			Expect(longWithoutCommaMachineSet.Field(`metadata.annotations.deployment\.kubernetes\.io\/revision-history`).String()).To(Equal("12345678901234567"))

			emptyMachineSet := f.KubernetesResource("MachineSet", "d8-cloud-instance-manager", "empty-revision-history")
			Expect(emptyMachineSet.Field(`metadata.annotations.deployment\.kubernetes\.io\/revision-history`).String()).To(Equal(""))

			absentMachineSet := f.KubernetesResource("MachineSet", "d8-cloud-instance-manager", "absent-revision-history")
			Expect(absentMachineSet.Field(`metadata.annotations.deployment\.kubernetes\.io\/revision-history`).Exists()).To(BeFalse())
		})
	})
})
