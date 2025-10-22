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

var _ = Describe("Modules :: node-manager :: hooks :: MachineClass checksum collect ::", func() {
	const valuesRoot = "nodeManager.internal.machineDeployments"

	Context("Empty cluster", func() {
		f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"cloudProvider":{"type":"openstack"}}}}`, `{}`)
		f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Hook creates empty values object", func() {
			Expect(f.ValuesGet(valuesRoot).String()).To(MatchJSON(`{}`))
		})
	})

	Context("Single MachineDeployment with checksum", func() {
		f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"cloudProvider":{"type":"openstack"}}}}`, `{}`)
		f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)

		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager.internal.nodeGroups",
				[]byte(`[{
                          "cloudInstances": {
                              "classReference": { "kind": "OpenStackInstanceClass", "name": "worker-small" },
                              "maxPerZone": 3,
                              "minPerZone": 3,
                              "zones": [ "nova" ]
                          },
                          "cri": { "type": "Containerd" },
                          "disruptions": { "approvalMode": "Automatic" },
                          "instanceClass": {
                              "flavorName": "m1.small",
                              "imageName": "ubuntu-18-04-cloud-amd64",
                              "mainNetwork": "dev2"
                          },
                          "kubernetesVersion": "1.29",
                          "manualRolloutID": "",
                          "name": "worker",
                          "nodeType": "CloudEphemeral",
                          "updateEpoch": "112714"
			}]`))
			f.KubeStateSet(`
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: nova
  labels:
    node-group: worker
  name: dev2-worker-391f2ede
  namespace: d8-cloud-instance-manager
spec:
  minReadySeconds: 300
  replicas: 3
  selector:
    matchLabels:
      instance-group: worker-nova
  template:
    metadata:
      annotations:
        checksum/machine-class: XXX_XXX_XXX
      creationTimestamp: null
      labels:
        instance-group: worker-nova
    spec:
      class:
        kind: OpenStackMachineClass
        name: worker-391f2ede
      nodeTemplate:
        metadata:
          creationTimestamp: null
          labels:
            node-role.kubernetes.io/worker: ""
            node.deckhouse.io/group: worker
            node.deckhouse.io/type: CloudEphemeral
        spec: {}
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Values contain checksum of MachineClass from MachineDeployment spec", func() {
			key := valuesRoot + ".dev2-worker-391f2ede.checksum"
			Expect(f.ValuesGet(key).String()).To(Equal("XXX_XXX_XXX"))
		})

		It("Values contain node group name from MachineDeployment", func() {
			key := valuesRoot + ".dev2-worker-391f2ede.nodeGroup"
			Expect(f.ValuesGet(key).String()).To(Equal("worker"))
		})
	})

	Context("Single MachineDeployment but no node groups", func() {
		f := HookExecutionConfigInit(`{"nodeManager":{"internal":{"cloudProvider":{"type":"openstack"}}}}`, `{}`)
		f.RegisterCRD("machine.sapcloud.io", "v1alpha1", "MachineDeployment", true)

		BeforeEach(func() {
			f.ValuesSetFromYaml("nodeManager.internal.nodeGroups", []byte(`[]`))
			f.KubeStateSet(`
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  annotations:
    zone: nova
  labels:
    node-group: worker
  name: dev2-worker-391f2ede
  namespace: d8-cloud-instance-manager
spec:
  minReadySeconds: 300
  replicas: 3
  selector:
    matchLabels:
      instance-group: worker-nova
  template:
    metadata:
      creationTimestamp: null
      labels:
        instance-group: worker-nova
      annotations:
        checksum/machine-class: abracadabra
    spec:
      class:
        kind: OpenStackMachineClass
        name: worker-391f2ede
      nodeTemplate:
        metadata:
          creationTimestamp: null
          labels:
            node-role.kubernetes.io/worker: ""
            node.deckhouse.io/group: worker
            node.deckhouse.io/type: CloudEphemeral
        spec: {}
`)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Does not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Cleans machineDeployment value", func() {
			Expect(f.ValuesGet(valuesRoot).String()).To(Equal("{}"))
		})
	})
})
