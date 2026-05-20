/*
Copyright 2024 Flant JSC

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

var _ = Describe("Modules :: node-manager :: hooks :: set_api_version_on_machine_set ::", func() {
	const (
		machineSets = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineSet
metadata:
  name: empty
  namespace: d8-cloud-instance-manager
spec:
  template:
    spec:
      infrastructureRef:
        kind: HuaweiCloudMachineTemplate
        name: template-empty
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineSet
metadata:
  name: ready
  namespace: d8-cloud-instance-manager
spec:
  template:
    spec:
      infrastructureRef:
        apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
        kind: HuaweiCloudMachineTemplate
        name: template-ready
`
	)

	f := HookExecutionConfigInit(`{"nodeManager": {"internal": {}}}`, `{}`)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "MachineSet", true)

	Context("Cluster with MachineSets", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machineSets))
			f.RunHook()
		})

		It("fills missing infrastructureRef apiVersion", func() {
			Expect(f).To(ExecuteSuccessfully())
			msEmpty := f.KubernetesResource("MachineSet", "d8-cloud-instance-manager", "empty")
			msReady := f.KubernetesResource("MachineSet", "d8-cloud-instance-manager", "ready")

			Expect(msEmpty.Field("spec.template.spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
			Expect(msReady.Field("spec.template.spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
		})
	})
})
