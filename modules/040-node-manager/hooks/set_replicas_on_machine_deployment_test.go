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

var _ = Describe("Modules :: node-manager :: hooks :: set_replicas_on_machine_deployment ::", func() {
	const (
		staticNGs = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng-static-1
spec:
  nodeType: Static
`
		stateNGs = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  cloudInstances:
    maxPerZone: 2
    minPerZone: 5 # $ng_min_instances -ge $ng_max_instances
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng20
spec:
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3 # "$replicas" == "null"
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng21
spec:
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3 # $replicas -eq 0
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng3
spec:
  cloudInstances:
    maxPerZone: 10
    minPerZone: 6 # $replicas -le $ng_min_instances
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng4
spec:
  cloudInstances:
    maxPerZone: 4
    minPerZone: 3 # $replicas -gt $ng_max_instances
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng5
spec:
  cloudInstances:
    maxPerZone: 10
    minPerZone: 1 # $ng_min_instances <= $replicas <= $ng_max_instances
`
		stateMDs = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng1
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng1
spec:
  replicas: 1 # $ng_min_instances -ge $ng_max_instances
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng20
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng20
spec: {} # "$replicas" == "null"
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng21
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng21
spec:
  replicas: 0 # $replicas -eq 0
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng3
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng3
spec:
  replicas: 2 # $replicas -le $ng_min_instances
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng4
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng4
spec:
  replicas: 7 # $replicas -gt $ng_max_instances
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng5
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng5
spec:
  replicas: 5 # $ng_min_instances <= $replicas <= $ng_max_instances
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: md-ng6
  namespace: d8-cloud-instance-manager
  labels:
    node-group: ng6 #ng6 is missing
spec:
  replicas: 5
`
	)

	f := HookExecutionConfigInit(`{"nodeManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("cluster.x-k8s.io", "v1beta1", "MachineDeployment", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with static nodes", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(staticNGs))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with set of different pairs of MDs and NGs", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateNGs + stateMDs))
			f.RunHook()
		})

		It("", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-ng1").Field("spec.replicas").String()).To(Equal("2"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-ng20").Field("spec.replicas").String()).To(Equal("3"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-ng21").Field("spec.replicas").String()).To(Equal("3"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-ng3").Field("spec.replicas").String()).To(Equal("6"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-ng4").Field("spec.replicas").String()).To(Equal("4"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-ng5").Field("spec.replicas").String()).To(Equal("5"))
			Expect(f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "md-ng6").Field("spec.replicas").String()).To(Equal("5"))
		})
	})
	// TODO add tests for CAPI machineDeployments
})
