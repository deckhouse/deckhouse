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

var _ = Describe("Modules :: node-manager :: hooks :: deployment_required ::", func() {
	const (
		nodeGroupCloudPermanent = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: master
spec:
  nodeType: CloudPermanent
status: {}
`
		nodeGroupCloudEphemeral = `
---
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  nodeType: CloudEphemeral
status: {}
`
		machineDeployment = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: aaa
  labels:
    heritage: deckhouse
  name: machine-deployment-name
  namespace: d8-cloud-instance-manager
`
		machineSet = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineSet
metadata:
  annotations:
    zone: aaa
  name: machine-set-name
  namespace: d8-cloud-instance-manager
`
		machine = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: Machine
metadata:
  name: machine-name
  namespace: d8-cloud-instance-manager
`
	)

	f := HookExecutionConfigInit(`{"global":{"discovery":{"kubernetesVersion": "1.16.15", "kubernetesVersions":["1.16.15"], "clusterUUID":"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}},"nodeManager":{"internal": {"cloudProvider": {"machineClassKind":"some"}}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1", "NodeGroup", false)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineSet", true)
	f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "Machine", true)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail; flag must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with CloudPermanent NG only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(nodeGroupCloudPermanent, 1))
			f.RunHook()
		})

		It("Hook must not fail; flag must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with CloudEphemeral NG only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(nodeGroupCloudEphemeral, 1))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").String()).To(Equal("true"))
		})
	})

	Context("Cluster with CloudEphemeral NG only and without machine class", func() {
		BeforeEach(func() {
			f.ValuesSet("nodeManager.internal.cloudProvider.machineClassKind", "")
			f.BindingContexts.Set(f.KubeStateSetAndWaitForBindingContexts(nodeGroupCloudEphemeral, 1))
			f.RunHook()
		})

		It("Hook must not fail; flag must not be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").Exists()).To(BeFalse())
		})
	})

	Context("Cluster with MDs, MSs and Ms", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machineDeployment + machineSet + machine))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").String()).To(Equal("true"))
		})
	})

	Context("Cluster with MDs only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machineDeployment))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").String()).To(Equal("true"))
		})
	})

	Context("Cluster with MDs and MSs only", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(machineDeployment + machineSet))
			f.RunHook()
		})

		It("Hook must not fail; flag must be set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("nodeManager.internal.machineControllerManagerEnabled").String()).To(Equal("true"))
		})
	})

})
