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

var _ = Describe("Modules :: node-manager :: hooks :: set_api_version_on_capi_resources ::", func() {
	f := HookExecutionConfigInit(`{"nodeManager": {"internal": {}}}`, `{}`)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "MachineDeployment", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "MachineSet", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "Machine", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "MachinePool", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "Cluster", true)

	Context("MachineDeployment", func() {
		const machineDeployments = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachineDeployment
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
kind: MachineDeployment
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
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machineDeployments))
			f.RunHook()
		})

		It("fills missing infrastructureRef apiVersion", func() {
			Expect(f).To(ExecuteSuccessfully())
			mdEmpty := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "empty")
			mdReady := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "ready")

			Expect(mdEmpty.Field("spec.template.spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
			Expect(mdReady.Field("spec.template.spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
		})
	})

	Context("MachineSet", func() {
		const machineSets = `
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

	Context("Machine", func() {
		const machines = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: empty
  namespace: d8-cloud-instance-manager
spec:
  infrastructureRef:
    kind: HuaweiCloudMachine
    name: template-empty
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Machine
metadata:
  name: ready
  namespace: d8-cloud-instance-manager
spec:
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: HuaweiCloudMachine
    name: template-ready
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machines))
			f.RunHook()
		})

		It("fills missing infrastructureRef apiVersion", func() {
			Expect(f).To(ExecuteSuccessfully())
			machineEmpty := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "empty")
			machineReady := f.KubernetesResource("Machine", "d8-cloud-instance-manager", "ready")

			Expect(machineEmpty.Field("spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
			Expect(machineReady.Field("spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
		})
	})

	Context("MachinePool", func() {
		const machinePools = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: MachinePool
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
kind: MachinePool
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
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machinePools))
			f.RunHook()
		})

		It("fills missing infrastructureRef apiVersion", func() {
			Expect(f).To(ExecuteSuccessfully())
			mpEmpty := f.KubernetesResource("MachinePool", "d8-cloud-instance-manager", "empty")
			mpReady := f.KubernetesResource("MachinePool", "d8-cloud-instance-manager", "ready")

			Expect(mpEmpty.Field("spec.template.spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
			Expect(mpReady.Field("spec.template.spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
		})
	})

	Context("Cluster", func() {
		const clusters = `
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: empty-refs
  namespace: d8-cloud-instance-manager
spec:
  infrastructureRef:
    kind: DeckhouseCluster
    name: my-cluster
  controlPlaneRef:
    kind: DeckhouseControlPlane
    name: my-control-plane
---
apiVersion: cluster.x-k8s.io/v1beta1
kind: Cluster
metadata:
  name: ready-refs
  namespace: d8-cloud-instance-manager
spec:
  infrastructureRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: DeckhouseCluster
    name: my-cluster
  controlPlaneRef:
    apiVersion: infrastructure.cluster.x-k8s.io/v1alpha1
    kind: DeckhouseControlPlane
    name: my-control-plane
`
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(clusters))
			f.RunHook()
		})

		It("fills missing infrastructureRef and controlPlaneRef apiVersion", func() {
			Expect(f).To(ExecuteSuccessfully())
			clusterEmpty := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "empty-refs")
			clusterReady := f.KubernetesResource("Cluster", "d8-cloud-instance-manager", "ready-refs")

			Expect(clusterEmpty.Field("spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
			Expect(clusterEmpty.Field("spec.controlPlaneRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
			Expect(clusterReady.Field("spec.infrastructureRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
			Expect(clusterReady.Field("spec.controlPlaneRef.apiVersion").String()).To(Equal("infrastructure.cluster.x-k8s.io/v1alpha1"))
		})
	})
})
