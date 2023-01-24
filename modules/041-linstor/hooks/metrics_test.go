/*
Copyright 2022 Flant JSC

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
	"k8s.io/utils/pointer"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: linstor :: hooks :: metrics ", func() {
	f := HookExecutionConfigInit(`{"linstor":{}}`, "")
	f.RegisterCRD("internal.linstor.linbit.com", "v1-19-1", "Nodes", false)
	f.RegisterCRD("internal.linstor.linbit.com", "v1-19-1", "NodeStorPool", false)
	f.RegisterCRD("internal.linstor.linbit.com", "v1-19-1", "ResourceDefinitions", false)
	f.RegisterCRD("internal.linstor.linbit.com", "v1-19-1", "Resources", false)

	assertMetric := func(f *HookExecutionConfig, name string, value float64) {
		metrics := f.MetricsCollector.CollectedMetrics()
		metricIndex := -1
		for i, m := range metrics {
			if m.Name == name {
				Expect(m.Value).To(Equal(pointer.Float64Ptr(value)))
				metricIndex = i
				break
			}
		}

		Expect(metricIndex >= 0).To(BeTrue())
	}

	assertStoragePoolMetric := func(f *HookExecutionConfig, driver string, value float64) {
		metrics := f.MetricsCollector.CollectedMetrics()
		metricIndex := -1
		for i, m := range metrics {
			if m.Name == "d8_telemetry_linstor_storage_pools" && m.Labels["driver"] == driver {
				Expect(m.Value).To(Equal(pointer.Float64Ptr(value)))
				metricIndex = i
				break
			}
		}

		Expect(metricIndex >= 0).To(BeTrue())
	}

	Context("Empty cluster linstor module enabled", func() {
		BeforeEach(func() {
			f.RunHook()
		})

		It("Executes hook successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Sets metric lisntor_enabled", func() {
			assertMetric(f, "d8_telemetry_linstor_enabled", 1)
		})
	})

	Context("Linstor installed, collect metrics", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.KubeStateSet(`
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nodestorpool.internal.linstor.linbit.com
spec:
  group: internal.linstor.linbit.com
  names:
    kind: NodeStorPool
    listKind: NodeStorPoolList
    plural: nodestorpool
    singular: nodestorpool
  scope: Cluster
  versions:
  - name: v1-19-1
    served: true
    storage: true
  - name: v1-18-2
    served: true
    storage: false
  - name: v1-17-0
    served: true
    storage: false
  - name: v1-19-1
    served: true
    storage: false
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: nodes.internal.linstor.linbit.com
spec:
  group: internal.linstor.linbit.com
  names:
    kind: Nodes
    listKind: NodesList
    plural: nodes
    singular: nodes
  scope: Cluster
  versions:
  - name: v1-19-1
    served: true
    storage: true
  - name: v1-18-2
    served: true
    storage: false
  - name: v1-17-0
    served: true
    storage: false
  - name: v1-19-1
    served: true
    storage: false
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: resourcedefinitions.internal.linstor.linbit.com
spec:
  group: internal.linstor.linbit.com
  names:
    kind: ResourceDefinitions
    listKind: ResourceDefinitionsList
    plural: resourcedefinitions
    singular: resourcedefinitions
  scope: Cluster
  versions:
  - name: v1-19-1
    served: true
    storage: true
  - name: v1-18-2
    served: true
    storage: false
  - name: v1-17-0
    served: true
    storage: false
  - name: v1-19-1
    served: true
    storage: false
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: resources.internal.linstor.linbit.com
spec:
  group: internal.linstor.linbit.com
  names:
    kind: Resources
    listKind: ResourcesList
    plural: resources
    singular: resources
  scope: Cluster
  versions:
  - name: v1-19-1
    served: true
    storage: true
  - name: v1-18-2
    served: true
    storage: false
  - name: v1-17-0
    served: true
    storage: false
  - name: v1-19-1
    served: true
    storage: false
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Nodes
metadata:
  name: 531fa09e2545cc9ef5b6b2199c8ef4a60de8db5227610dd5e40bd78d0cb6ad28
spec:
  node_dsp_name: linstor-controller-6858f65c69-vjdtr
  node_flags: 0
  node_name: LINSTOR-CONTROLLER-6858F65C69-VJDTR
  node_type: 1
  uuid: ca8c047d-3243-4d88-b917-4537676636ae
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Nodes
metadata:
  name: 55a2fd2fc05eaa5b1e6a2b572cb2d81a387d1bfe9181340a7dd7c87de5506923
spec:
  node_dsp_name: hf-virt-02
  node_flags: 0
  node_name: HF-VIRT-02
  node_type: 2
  uuid: 367f9e94-a942-425e-b69c-f4b3a8d2d982
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Nodes
metadata:
  name: 70ddbc237fe582721714ddc3856d978f98b0247885ebdc0ed33f31bb39ee2d02
spec:
  node_dsp_name: hf-virt-01
  node_flags: 0
  node_name: HF-VIRT-01
  node_type: 2
  uuid: 884bc58b-2dd1-485a-81c1-b7e9f712f370
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: NodeStorPool
metadata:
  name: 38c67058dff7edfdf359cbf5c1892079f2f7721ed7e6b80232aa522461a1968d
spec:
  driver_name: LVM_THIN
  external_locking: false
  free_space_mgr_dsp_name: hf-virt-03:thindata
  free_space_mgr_name: HF-VIRT-03:THINDATA
  node_name: HF-VIRT-03
  pool_name: THINDATA
  uuid: 44819636-91ec-4851-8d24-7b860e847657
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: NodeStorPool
metadata:
  name: 60b3065c80b9b35f3d149cc8219653b41449a43f81fc0671834a830280ee7774
spec:
  driver_name: DISKLESS
  external_locking: false
  free_space_mgr_dsp_name: hf-virt-01:DfltDisklessStorPool
  free_space_mgr_name: HF-VIRT-01:DFLTDISKLESSSTORPOOL
  node_name: HF-VIRT-01
  pool_name: DFLTDISKLESSSTORPOOL
  uuid: f88f39e9-fb69-4831-9b8f-195bf0a9674e
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: NodeStorPool
metadata:
  name: 9f065cb41340702a1416a6a0579fcefe0ed5854458b9024535246c038a243e7f
spec:
  driver_name: DISKLESS
  external_locking: false
  free_space_mgr_dsp_name: hf-virt-02:DfltDisklessStorPool
  free_space_mgr_name: HF-VIRT-02:DFLTDISKLESSSTORPOOL
  node_name: HF-VIRT-02
  pool_name: DFLTDISKLESSSTORPOOL
  uuid: e9c5dc15-011d-4935-9bec-b662624eaf1f
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: NodeStorPool
metadata:
  name: 9fe4604f7057cbc66337151f14477630106f5e9e288d7578d715e247e7ca092c
spec:
  driver_name: LVM_THIN
  external_locking: false
  free_space_mgr_dsp_name: hf-virt-02:thindata
  free_space_mgr_name: HF-VIRT-02:THINDATA
  node_name: HF-VIRT-02
  pool_name: THINDATA
  uuid: 12ea214f-da62-4213-bdd7-c37af94c8be2
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: NodeStorPool
metadata:
  name: a961f699fcd6b1a70dce624d120af16116b96983386b376d8ff537a5b1e06dba
spec:
  driver_name: DISKLESS
  external_locking: false
  free_space_mgr_dsp_name: hf-virt-03:DfltDisklessStorPool
  free_space_mgr_name: HF-VIRT-03:DFLTDISKLESSSTORPOOL
  node_name: HF-VIRT-03
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: ResourceDefinitions
metadata:
  name: 1564c1b78164837b396da496c9749c4d4b30c34a9a62ed687ac4f8171b009cd8
spec:
  layer_stack: '[]'
  parent_uuid: 1dab5087-96be-4657-9078-9c97a1e9220f
  resource_flags: 641
  resource_group_name: SC-6123FE2A-DD95-575C-ADF8-C54A4F636D6B
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_dsp_name: back_20230124_110601
  snapshot_name: BACK_20230124_110601
  uuid: 0df9b4d3-9621-4d87-8036-2d98f54dc6b3
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: ResourceDefinitions
metadata:
  name: c58c4ceb29d213360ffd07456fd31772775a4fb68a2d145cefbe0856bc959f0d
spec:
  layer_stack: '["DRBD","STORAGE"]'
  resource_dsp_name: pvc-31d4a5db-f498-40ca-9641-7a4dd2c906f6
  resource_flags: 0
  resource_group_name: SC-6123FE2A-DD95-575C-ADF8-C54A4F636D6B
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_dsp_name: ""
  snapshot_name: ""
  uuid: 1dab5087-96be-4657-9078-9c97a1e9220f
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: ResourceDefinitions
metadata:
  name: fcd75deb6c2fa8d9af9ba7655f53e725f43ef643c2ab87ae444ba41e2719249f
spec:
  layer_stack: '[]'
  parent_uuid: 1dab5087-96be-4657-9078-9c97a1e9220f
  resource_flags: 641
  resource_group_name: SC-6123FE2A-DD95-575C-ADF8-C54A4F636D6B
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_dsp_name: back_20230124_111601
  snapshot_name: BACK_20230124_111601
  uuid: 85dffaa3-638d-4f89-bddd-275dde5cb448
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Resources
metadata:
  name: 12eda5d45a722d55085bb5e2b44525a19c3a5674436271031d69002c2440e5fb
spec:
  create_timestamp: 1674558447641
  node_name: HF-VIRT-01
  resource_flags: 0
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_name: BACK_20230124_110601
  uuid: 46ef9765-7906-4087-a447-c00e03699e72
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Resources
metadata:
  name: 60bf19e91047e4821c75ac12ba6e3156321c691e445fbd7ef5a6389e65317e64
spec:
  create_timestamp: 1674558447641
  node_name: HF-VIRT-02
  resource_flags: 0
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_name: BACK_20230124_110601
  uuid: 226b6d5c-2674-43f0-b83f-8a03bc86534f
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Resources
metadata:
  name: 76b2c363a95c8aa1fd199abe205e7670ddb02f4b20c13b10b077caeeb5830f68
spec:
  create_timestamp: 1673963605796
  node_name: HF-VIRT-01
  resource_flags: 0
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_name: ""
  uuid: 6e22191d-78e9-4f4f-ac3f-120ef0616a8b
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Resources
metadata:
  name: 82a81cffa69bde6ece5d1499c0db6063b252b8da93c730c314922ad3249f8293
spec:
  create_timestamp: 1673963607093
  node_name: HF-VIRT-03
  resource_flags: 388
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_name: ""
  uuid: 94e6ab9b-56c9-4913-ba69-71a1acd623a3
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Resources
metadata:
  name: 891f22bd43a044f588774c2a51ea07b10b6594e6c6fd906568cf8b8f5b70ee3a
spec:
  create_timestamp: 1674559047907
  node_name: HF-VIRT-01
  resource_flags: 0
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_name: BACK_20230124_111601
  uuid: 5bacb49b-048b-4760-87e3-83822ad300c8
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Resources
metadata:
  name: a2591d9e4a91c867301bb015195888702bd9569195835da968125f34cbdd9af9
spec:
  create_timestamp: 1674559047907
  node_name: HF-VIRT-02
  resource_flags: 0
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_name: BACK_20230124_111601
  uuid: 5242397e-4c3f-4a3d-a7a6-9cf3f9ff1cdd
---
apiVersion: internal.linstor.linbit.com/v1-19-1
kind: Resources
metadata:
  name: e605fdfd97bf10ea0dec2830edb13e8c84484f0f9ea37c2c12fbc9b74284c230
spec:
  create_timestamp: 1673963607680
  node_name: HF-VIRT-02
  resource_flags: 0
  resource_name: PVC-31D4A5DB-F498-40CA-9641-7A4DD2C906F6
  snapshot_name: ""
  uuid: d843296e-7e9e-4b4e-a32b-b2e2718e9c58
			`))
			f.RunHook()
		})

		It("Executes hook successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Sets metric for objects", func() {
			assertMetric(f, "d8_telemetry_linstor_enabled", 1)
			assertMetric(f, "d8_telemetry_linstor_satellites", 2)
			assertStoragePoolMetric(f, "LVM_THIN", 2)
			assertStoragePoolMetric(f, "DISKLESS", 3)
			assertMetric(f, "d8_telemetry_linstor_resource_definitions", 1)
			assertMetric(f, "d8_telemetry_linstor_snapshot_definitions", 2)
			assertMetric(f, "d8_telemetry_linstor_resources", 3)
		})
	})

})
