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
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/set"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: node-manager :: hooks :: MachineClass checksum calculation and assignment ::", func() {
	RequireCloudProvider := newCloudProviderAvailabilityChecker()

	const (
		mdGroup      = "machine.sapcloud.io"
		mdVersion    = "v1alpha1"
		mdKind       = "MachineDeployment"
		mdNamespaced = true
	)
	registerCrd := func(f *HookExecutionConfig) {
		f.RegisterCRD(mdGroup, mdVersion, mdKind, mdNamespaced)
	}

	const mdValuesPath = "nodeManager.internal.machineDeployments"
	const nodeGroupsPath = "nodeManager.internal.nodeGroups"
	const cloudProviderTypePath = "nodeManager.internal.cloudProvider.type"

	When("Execute in empty cluster", func() {
		const cloudProviderType = "aws"

		f := HookExecutionConfigInit(`{}`, `{}`)
		registerCrd(f)

		BeforeEach(func() {
			RequireCloudProvider(cloudProviderType)

			// Ensure cloudProvider.type.
			f.ValuesSet(cloudProviderTypePath, cloudProviderType)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	When("Execute with unknown and missing MachineDeployment objects", func() {
		// There is item in machineDeployments and corresponding item in nodeGroups
		// but no MachineDeployment object in cluster.
		// Hook should not fail.
		const (
			cloudProviderType = "aws"
			nodeGroupsValues  = `
- name: worker
  nodeType: CloudEphemeral
  cri:
    type: Containerd
  kubernetesVersion: "1.29"
  manualRolloutID: ""
  updateEpoch: "112714"
  disruptions:
    approvalMode: Automatic
  instanceClass:
    ami: myami
    diskSizeGb: 50
    diskType: gp2
    iops: 42
    instanceType: t2.medium
  cloudInstances:
    classReference:
      kind: AWSInstanceClass
      name: worker-small
    maxPerZone: 3
    minPerZone: 3
    zones:
    - zonea
`
			mdValues = `
aaa:
  checksum: SOME_CHECKSUM
  nodeGroup: worker
  name: aaa
ccc:
  checksum: SOME_CHECKSUM
  nodeGroup: worker
  name: ccc
`
			expectedChecksum = "21b7f37222f1cbad6c644c0aa4eef85aa309b874ec725dc0cdc087ca06fc6c19"
			mdObject         = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: aaa
  namespace: d8-cloud-instance-manager
  labels:
    node-group: worker
spec: {}
`
			mdUnknown = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: bbb
  namespace: d8-cloud-instance-manager
  labels:
    node-group: worker
spec: {}
`
		)

		f := HookExecutionConfigInit(`{}`, `{}`)
		registerCrd(f)

		BeforeEach(func() {
			RequireCloudProvider(cloudProviderType)

			// Ensure cloudProvider.type value.
			f.ValuesSet(cloudProviderTypePath, cloudProviderType)
			// Set machineDeployments values.
			f.ValuesSetFromYaml(mdValuesPath, []byte(mdValues))
			f.ValuesSetFromYaml(nodeGroupsPath, []byte(nodeGroupsValues))

			f.KubeStateSet(mdObject + mdUnknown)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("should assign the checksum to existing MachineDeployment 'aaa'", func() {
			machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "aaa")
			Expect(machineDeployment.Exists()).To(BeTrue())
			checksum := machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class")
			Expect(checksum.String()).To(Equal(expectedChecksum))
		})

		It("should not assign the checksum to unknown MachineDeployment", func() {
			machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "bbb")
			Expect(machineDeployment.Exists()).To(BeTrue())
			checksum := machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class")
			Expect(checksum.Exists()).To(BeFalse())
		})
	})

	When("Execute with a single checksum", func() {
		const (
			cloudProviderType = "openstack"
			nodeGroupsValues  = `[{
            "name": "worker",
            "nodeType": "CloudEphemeral",
            "cloudInstances": {
                "classReference": { "kind": "OpenStackInstanceClass", "name": "worker-small"},
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
            "updateEpoch": "112714"
        }]`
			expectedChecksum = "b94a18d06cc6cb58ac397ae6b671dbb666744ee06da8ce42a56e58db56ecd4a0"
			mdInValues       = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: aaa
  namespace: d8-cloud-instance-manager
  labels:
    node-group: worker
spec: {}
`
			mdNotInValues = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: bbb
  namespace: d8-cloud-instance-manager
  labels:
    node-group: worker
spec: {}
`
		)

		f := HookExecutionConfigInit(`{}`, `{}`)
		registerCrd(f)

		BeforeEach(func() {
			RequireCloudProvider(cloudProviderType)

			// Ensure cloudProvider.type value.
			f.ValuesSet("nodeManager.internal.cloudProvider.type", cloudProviderType)
			// Set machineDeployments values.
			f.ValuesSetFromYaml(mdValuesPath, []byte(`{
				"aaa": {
					"checksum": "NONSENSE",
					"nodeGroup": "worker",
					"name": "aaa"
				}
			}`))
			f.ValuesSetFromYaml("nodeManager.internal.nodeGroups", []byte(nodeGroupsValues))

			f.KubeStateSet(mdInValues + mdNotInValues)
			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("should assign the checksum to MachineDeployments from values", func() {
			machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "aaa")
			Expect(machineDeployment.Exists()).To(BeTrue())
			checksum := machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class")
			Expect(checksum.String()).To(Equal(expectedChecksum))
		})

		It("should not assign the checksum to MachineDeployments that are not in values", func() {
			machineDeployment := f.KubernetesResource("MachineDeployment", "d8-cloud-instance-manager", "bbb")
			Expect(machineDeployment.Exists()).To(BeTrue())
			checksum := machineDeployment.Field("spec.template.metadata.annotations.checksum/machine-class")
			Expect(checksum.Exists()).To(BeFalse())
		})
	})

	When("Execute with no matching nodeGroup", func() {
		const cloudProviderType = "aws"
		const mdValues = `
aaa:
  checksum: NO-CHECKSUM
  nodeGroup: worker
  name: aaa
`
		const nodeGroupsValues = "[]"

		f := HookExecutionConfigInit(`{}`, `{}`)
		registerCrd(f)

		BeforeEach(func() {
			RequireCloudProvider(cloudProviderType)

			// Ensure cloudProvider.type.
			f.ValuesSet(cloudProviderTypePath, cloudProviderType)
			// TODO what is this comment about?
			// No MachineDeployment state here. We should not touch a MachineDeployment if there is
			// no nodegroup for it. The hook should fail in this test if we do.
			f.ValuesSetFromYaml(mdValuesPath, []byte(mdValues))
			f.ValuesSetFromYaml(nodeGroupsPath, []byte(nodeGroupsValues))

			f.BindingContexts.Set(f.GenerateAfterHelmContext())
			f.RunHook()
		})

		It("should not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("should remove value for existing object MachineDeployment/aaa", func() {
			Expect(f.ValuesGet(mdValuesPath + ".aaa").Exists()).To(BeFalse())
			Expect(f.ValuesGet(mdValuesPath).String()).To(Equal("{}"))
		})
	})

	Context("Checksums", func() {
		const nodeManagerAWS = `
internal:
  instancePrefix: myprefix
  cloudProvider:
    type: aws
  nodeGroups:
  - name: worker
    instanceClass:
      ami: myami
      diskSizeGb: 50
      diskType: gp2
      iops: 42
      instanceType: t2.medium
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: AWSInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineDeployments:
    myprefix-worker-02320933:
      name: myprefix-worker-02320933
      nodeGroup: worker
    myprefix-worker-6bdb5b0d:
      name: myprefix-worker-6bdb5b0d
      nodeGroup: worker
`

		const nodeManagerAzure = `
internal:
  instancePrefix: myprefix
  cloudProvider:
    type: azure
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      machineType: mymachinetype
      preemptible: true #optional
      diskType: superdisk #optional
      diskSizeGb: 42 #optional
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: GCPInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineDeployments:
    myprefix-worker-02320933:
      name: myprefix-worker-02320933
      nodeGroup: worker
    myprefix-worker-6bdb5b0d:
      name: myprefix-worker-6bdb5b0d
      nodeGroup: worker
`

		const nodeManagerGCP = `
internal:
  instancePrefix: myprefix
  cloudProvider:
    type: gcp
  nodeGroups:
  - name: worker
    instanceClass: # maximum filled
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      machineType: mymachinetype
      preemptible: true #optional
      diskType: superdisk #optional
      diskSizeGb: 42 #optional
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: GCPInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineDeployments:
    myprefix-worker-02320933:
      name: myprefix-worker-02320933
      nodeGroup: worker
    myprefix-worker-6bdb5b0d:
      name: myprefix-worker-6bdb5b0d
      nodeGroup: worker
`

		const nodeManagerOpenstack = `
internal:
  instancePrefix: myprefix
  cloudProvider:
    type: openstack
    openstack:
      tags:
        yyy: zzz
        aaa: xxx
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      mainNetwork: shared
      additionalNetworks:
      - mynetwork
      - mynetwork2
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: OpenStackInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  - name: simple
    instanceClass:
      flavorName: m1.xlarge
      additionalSecurityGroups:
      - ic-groupa
      - ic-groupb
      additionalTags:
        aaa: bbb
        ccc: ddd
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: OpenStackInstanceClass
        name: simple
      maxPerZone: 1
      minPerZone: 1
      zones:
      - zonea
  machineDeployments:
    myprefix-worker-02320933:
      name: myprefix-worker-02320933
      nodeGroup: worker
    myprefix-worker-6bdb5b0d:
      name: myprefix-worker-6bdb5b0d
      nodeGroup: worker
    myprefix-simple-02320933:
      name: myprefix-simple-02320933
      nodeGroup: simple
`

		const nodeManagerVsphere = `
internal:
  instancePrefix: myprefix
  cloudProvider:
    type: vsphere
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      numCPUs: 3
      memory: 3
      rootDiskSize: 42
      template: dev/test
      mainNetwork: mymainnetwork
      additionalNetworks: [aaa, bbb]
      datastore: lun-111
      runtimeOptions: # optional
        nestedHardwareVirtualization: true
        memoryReservation: 42
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: VsphereInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineDeployments:
    myprefix-worker-02320933:
      name: myprefix-worker-02320933
      nodeGroup: worker
    myprefix-worker-6bdb5b0d:
      name: myprefix-worker-6bdb5b0d
      nodeGroup: worker
`

		const nodeManagerYandex = `
internal:
  instancePrefix: myprefix
  cloudProvider:
    type: yandex
  nodeGroups:
  - name: worker
    instanceClass:
      flavorName: m1.large
      imageName: ubuntu-18-04-cloud-amd64
      platformID: myplaid
      cores: 42
      coreFraction: 50 #optional
      memory: 42
      gpus: 2
      imageID: myimageid
      preemptible: true #optional
      diskType: ssd #optional
      diskSizeGB: 42 #optional
      assignPublicIPAddress: true #optional
      mainSubnet: mymainsubnet
      additionalSubnets: [aaa, bbb]
      additionalLabels: # optional
        my: label
    nodeType: CloudEphemeral
    kubernetesVersion: "1.29"
    cri:
      type: "Containerd"
    cloudInstances:
      classReference:
        kind: YandexInstanceClass
        name: worker
      maxPerZone: 5
      minPerZone: 2
      zones:
      - zonea
      - zoneb
  machineDeployments:
    myprefix-worker-02320933:
      name: myprefix-worker-02320933
      nodeGroup: worker
    myprefix-worker-6bdb5b0d:
      name: myprefix-worker-6bdb5b0d
      nodeGroup: worker
`

		const workerMachineDeployments = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: myprefix-worker-02320933
  namespace: d8-cloud-instance-manager
  annotations:
    zone: zonea
  labels:
    node-group: worker
spec: {}
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: myprefix-worker-6bdb5b0d
  namespace: d8-cloud-instance-manager
  annotations:
    zone: zoneb
  labels:
    node-group: worker
spec: {}
`

		const simpleOpenstackMachineDeployment = `
---
apiVersion: machine.sapcloud.io/v1alpha1
kind: MachineDeployment
metadata:
  name: myprefix-simple-02320933
  namespace: d8-cloud-instance-manager
  annotations:
    zone: zonea
  labels:
    node-group: simple
spec: {}
`

		type nameSum struct{ name, checksum string }
		type entryData struct {
			moduleValues string
			k8sState     string

			// varies
			manualRolloutID string

			// these two vary
			providerTagsPath  string
			nodeGroupTagsPath string

			assertions []nameSum
		}

		f := HookExecutionConfigInit(`{}`, `{}`)
		registerCrd(f)

		// Important! If checksum changes, the MachineDeployments will re-deploy!
		// All nodes in MD will reboot! If you're not sure, don't change it.
		table.DescribeTable("Checksums",
			func(data entryData) {
				// Get cloud provider type from values fixture to skip tests for unavailable providers.
				f.ValuesSetFromYaml("nodeManager", []byte(data.moduleValues))
				cloudProviderType := f.ValuesGet("nodeManager.internal.cloudProvider.type").String()
				RequireCloudProvider(cloudProviderType)

				f.KubeStateSet(data.k8sState)
				f.BindingContexts.Set(f.GenerateBeforeHelmContext())

				if data.manualRolloutID != "" {
					// Set manualRolloutID in all nodegroups
					for i := 0; i < len(f.ValuesGet("nodeManager.internal.nodeGroups").Array()); i++ {
						key := fmt.Sprintf("nodeManager.internal.nodeGroups.%d.manualRolloutID", i)
						f.ValuesSet(key, data.manualRolloutID)
					}
				}

				if data.providerTagsPath != "" {
					// Set tags/lables in cloud provider and in all nodegroups
					providerValues := ` { "o":"provider",  "z":"provider"  }`
					nodeGroupValues := `{ "o":"nodegroup", "a":"nodegroup" }`

					f.ValuesSetFromYaml("nodeManager.internal.cloudProvider."+data.providerTagsPath, []byte(providerValues))

					for i := 0; i < len(f.ValuesGet("nodeManager.internal.nodeGroups").Array()); i++ {
						key := fmt.Sprintf("nodeManager.internal.nodeGroups.%d.instanceClass.%s", i, data.nodeGroupTagsPath)
						f.ValuesSetFromYaml(key, []byte(nodeGroupValues))
					}
				}

				f.RunHook()

				Expect(f).To(ExecuteSuccessfully())

				for _, md := range data.assertions {
					// MachineDeployment must be filled in values by the hook
					mdKey := fmt.Sprintf("%s.%s", mdValuesPath, md.name)
					Expect(f.ValuesGet(mdKey).Exists()).To(BeTrue(), mdKey+" should be present in values")

					// MachineClass checksum is calculated in the hook and saved to the values.
					// It must have fixed expected value.
					checksumKey := fmt.Sprintf("%s.%s.%s", mdValuesPath, md.name, "checksum")
					checksum := f.ValuesGet(checksumKey).String()
					Expect(checksum).To(Equal(md.checksum), checksumKey+" should be of expected value "+checksum)
				}
			},

			// AWS

			table.Entry("AWS", entryData{
				moduleValues: nodeManagerAWS,
				k8sState:     workerMachineDeployments,
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "21b7f37222f1cbad6c644c0aa4eef85aa309b874ec725dc0cdc087ca06fc6c19",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "21b7f37222f1cbad6c644c0aa4eef85aa309b874ec725dc0cdc087ca06fc6c19",
					},
				},
			}),
			table.Entry("AWS with manual rollout ID", entryData{
				moduleValues:    nodeManagerAWS,
				k8sState:        workerMachineDeployments,
				manualRolloutID: "test",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "7b787b33650a0f9166b6eacfdaff5d7c1e0cc508d2831d392ca938e47b7460f6",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "7b787b33650a0f9166b6eacfdaff5d7c1e0cc508d2831d392ca938e47b7460f6",
					},
				},
			}),
			table.Entry("AWS with additional tags", entryData{
				moduleValues:      nodeManagerAWS,
				k8sState:          workerMachineDeployments,
				providerTagsPath:  "aws.tags",
				nodeGroupTagsPath: "additionalTags",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "32ed026c31873a9b40c14182924c1d5d6766f025581f4562652f8ccb784898f2",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "32ed026c31873a9b40c14182924c1d5d6766f025581f4562652f8ccb784898f2",
					},
				},
			}),

			// GCP

			table.Entry("GCP", entryData{
				moduleValues: nodeManagerGCP,
				k8sState:     workerMachineDeployments,
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "a9e6ed184c6eab25aa7e47d3d4c7e5647fee9fa5bc2d35eb0232eab45749d3ae",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "a9e6ed184c6eab25aa7e47d3d4c7e5647fee9fa5bc2d35eb0232eab45749d3ae",
					},
				},
			}),
			table.Entry("GCP with manual rollout ID", entryData{
				moduleValues:    nodeManagerGCP,
				k8sState:        workerMachineDeployments,
				manualRolloutID: "test",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "48aa95710a1ea40e5dc26d36a8a0b2d461a85e4fc47953e94a84cef64a4060ca",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "48aa95710a1ea40e5dc26d36a8a0b2d461a85e4fc47953e94a84cef64a4060ca",
					},
				},
			}),
			table.Entry("GCP with additional labels", entryData{
				moduleValues:      nodeManagerGCP,
				k8sState:          workerMachineDeployments,
				providerTagsPath:  "gcp.labels",
				nodeGroupTagsPath: "additionalLabels",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "c87109f7fbd4b885f754a0f3d913bbc4340e5a585449ed29e36930b6b6503ac6",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "c87109f7fbd4b885f754a0f3d913bbc4340e5a585449ed29e36930b6b6503ac6",
					},
				},
			}),

			// Openstack

			table.Entry("Openstack", entryData{
				moduleValues: nodeManagerOpenstack,
				k8sState:     workerMachineDeployments + simpleOpenstackMachineDeployment,
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "d4829faf5ac0babecf268f0c74a512d3d00f48533af62f337e41bd7ccd12ce23",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "d4829faf5ac0babecf268f0c74a512d3d00f48533af62f337e41bd7ccd12ce23",
					}, {
						name:     "myprefix-simple-02320933",
						checksum: "06fc1339c280004581ec19e19e6eef8f3ee919931dbc450b60db608cd074feca",
					},
				},
			}),
			table.Entry("Openstack with manual rollout ID", entryData{
				moduleValues:    nodeManagerOpenstack,
				k8sState:        workerMachineDeployments + simpleOpenstackMachineDeployment,
				manualRolloutID: "test",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "9b3c57c4b09792ff626866698884907c89dc3f8d6571b81a5c226e1cae35057d",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "9b3c57c4b09792ff626866698884907c89dc3f8d6571b81a5c226e1cae35057d",
					},
				},
			}),
			table.Entry("Openstack with additional tags", entryData{
				moduleValues:      nodeManagerOpenstack,
				k8sState:          workerMachineDeployments + simpleOpenstackMachineDeployment,
				providerTagsPath:  "openstack.tags",
				nodeGroupTagsPath: "additionalTags",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "453963d10ea1bfa125d4186fe8a3cf9ec01cc769c694b0c0a74ed781364cb71e",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "453963d10ea1bfa125d4186fe8a3cf9ec01cc769c694b0c0a74ed781364cb71e",
					}, {
						name:     "myprefix-simple-02320933",
						checksum: "93aeec59514f8c4711efbd138a497bd1da322466fc2bcd308882e33b833cabb3",
					},
				},
			}),

			// Vsphere

			table.Entry("Vsphere", entryData{
				moduleValues: nodeManagerVsphere,
				k8sState:     workerMachineDeployments,
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "e54154626facdf7ba3937af03fb11ac3e626cf1ebab8e36fb17c8320ed4ae906",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "e54154626facdf7ba3937af03fb11ac3e626cf1ebab8e36fb17c8320ed4ae906",
					},
				},
			}),
			table.Entry("Vsphere with manual rollout ID", entryData{
				moduleValues:    nodeManagerVsphere,
				k8sState:        workerMachineDeployments,
				manualRolloutID: "test",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "72d791f90322f67ea0f42d80fbae93c5e5dacf3b26e2dc0cf03c8bad0a0bb072",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "72d791f90322f67ea0f42d80fbae93c5e5dacf3b26e2dc0cf03c8bad0a0bb072",
					},
				},
			}),

			// Yandex

			table.Entry("Yandex", entryData{
				moduleValues: nodeManagerYandex,
				k8sState:     workerMachineDeployments,
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "e8f505559b08cf2de57171d574feae2b258c66d9adf83808fc173e70cb006c47",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "e8f505559b08cf2de57171d574feae2b258c66d9adf83808fc173e70cb006c47",
					},
				},
			}),
			table.Entry("Yandex with manual rollout ID", entryData{
				moduleValues:    nodeManagerYandex,
				k8sState:        workerMachineDeployments,
				manualRolloutID: "test",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "d0de381052e706a0e28a9b2cfde60ed2e29854900549ef253d1283d1673a6625",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "d0de381052e706a0e28a9b2cfde60ed2e29854900549ef253d1283d1673a6625",
					},
				},
			}),
			table.Entry("Yandex with additional labels", entryData{
				moduleValues:      nodeManagerYandex,
				k8sState:          workerMachineDeployments,
				providerTagsPath:  "yandex.labels",
				nodeGroupTagsPath: "additionalLabels",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "55b0c5ac9c7e72252f509bc825f5046e198eab25ebd80efa3258cfb38e881359",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "55b0c5ac9c7e72252f509bc825f5046e198eab25ebd80efa3258cfb38e881359",
					},
				},
			}),

			// Azure

			table.Entry("Azure", entryData{
				moduleValues: nodeManagerAzure,
				k8sState:     workerMachineDeployments,
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "22501f2cc926a805859128046cf1b739f224eda731be0a7f93e0715c0b5ff1d3",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "22501f2cc926a805859128046cf1b739f224eda731be0a7f93e0715c0b5ff1d3",
					},
				},
			}),
			table.Entry("Azure with manual rollout ID", entryData{
				moduleValues:    nodeManagerAzure,
				k8sState:        workerMachineDeployments,
				manualRolloutID: "test",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "2feeeb7e50aa8656ebd2b26b8f9f6ba81d1b740e3a681b17ee3ad29f52a69497",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "2feeeb7e50aa8656ebd2b26b8f9f6ba81d1b740e3a681b17ee3ad29f52a69497",
					},
				},
			}),
			table.Entry("Azure with additional labels", entryData{
				moduleValues:      nodeManagerAzure,
				k8sState:          workerMachineDeployments,
				providerTagsPath:  "azure.additionalTags",
				nodeGroupTagsPath: "additionalTags",
				assertions: []nameSum{
					{
						name:     "myprefix-worker-02320933",
						checksum: "891a23e39148fe1457b88ad65898164c65df2e4cd34b013e4289127091089d95",
					}, {
						name:     "myprefix-worker-6bdb5b0d",
						checksum: "891a23e39148fe1457b88ad65898164c65df2e4cd34b013e4289127091089d95",
					},
				},
			}),
		)
	})
})

// Get available cloud providers to check if test can run on CE codebase.
func newCloudProviderAvailabilityChecker() func(tYpE string) {
	availTypes := getAvailableCloudProviderTypes()
	return func(tYpE string) {
		if availTypes.Has(tYpE) {
			return
		}
		Skip(fmt.Sprintf("'%s' cloud provider templates are not available. It is OK for CE codebase.", tYpE))
	}
}

// getAvailableCloudProviderTypes returns all cloud providers
// containing corresponding checksum template in cloud-providers directory.
func getAvailableCloudProviderTypes() set.Set {
	ptypes := set.New()
	for _, modulesInEditionDir := range []string{"/deckhouse/modules", "/deckhouse/ee/modules", "/deckhouse/ee/fe/modules"} {
		ptypes.AddSet(getAvailableCloudProviderTypesInDir(modulesInEditionDir))
	}

	return ptypes
}

func getAvailableCloudProviderTypesInDir(modulesDir string) set.Set {
	ptypes := set.New()

	dir := filepath.Join(modulesDir, "040-node-manager", "cloud-providers")

	files, err := os.ReadDir(dir)
	if err != nil {
		return ptypes
	}

	for _, f := range files {
		tmplBytes, err := readChecksumTemplate(f.Name())
		if err != nil {
			continue
		}
		if len(tmplBytes) > 0 {
			ptypes.Add(f.Name())
		}
	}

	return ptypes
}
