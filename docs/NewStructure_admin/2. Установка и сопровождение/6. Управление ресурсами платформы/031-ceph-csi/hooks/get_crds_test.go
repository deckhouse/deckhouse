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

var _ = Describe("Modules :: cephCsi :: hooks :: get_crds ::", func() {
	f := HookExecutionConfigInit(`{"cephCsi":{"internal":{ "crs":[],"csiConfig":{} } } }`, ``)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "CephCSIDriver", false)

	cr := `
---
kind: CephCSIDriver
apiVersion: deckhouse.io/v1alpha1
metadata:
  name: test
spec:
  clusterID: "42"
  monitors:
  - 1.2.3.4:6789
  userID: admin
  userKey: test
  rbd:
    storageClasses:
    - namePostfix: rbd
      pool: kubernetes
      defaultFSType: ext4
      reclaimPolicy: Delete
      allowVolumeExpansion: true
      mountOptions:
      - discard
  cephfs:
    storageClasses:
    - namePostfix: cephfs
      fsName: cephfs
`
	sc := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    app: ceph-csi
  annotations:
    migration-secret-name-changed: ""
  name: test-rbd
reclaimPolicy: Delete
`
	scChanged := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    app: ceph-csi
  name: test-rbd
reclaimPolicy: Delete
`

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})
		It("Value should not change", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with cr", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cr + sc))
			f.RunHook()
		})
		It("Value should not change", func() {
			Expect(f).To(ExecuteSuccessfully())

			Expect(f.ValuesGet("cephCsi.internal.crs.0.name").String()).To(Equal("test"))
			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.clusterID").String()).To(Equal("42"))

			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.userID").String()).To(Equal("admin"))
			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.userKey").String()).To(Equal("test"))
			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.monitors.0").String()).To(Equal("1.2.3.4:6789"))

			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.cephfs.storageClasses.0.namePostfix").String()).To(Equal("cephfs"))
			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.cephfs.storageClasses.0.fsName").String()).To(Equal("cephfs"))

			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.rbd.storageClasses.0.namePostfix").String()).To(Equal("rbd"))
			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.rbd.storageClasses.0.pool").String()).To(Equal("kubernetes"))
			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.rbd.storageClasses.0.defaultFSType").String()).To(Equal("ext4"))
			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.rbd.storageClasses.0.reclaimPolicy").String()).To(Equal("Delete"))
			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.rbd.storageClasses.0.allowVolumeExpansion").String()).To(Equal("true"))
			Expect(f.ValuesGet("cephCsi.internal.crs.0.spec.rbd.storageClasses.0.mountOptions.0").String()).To(Equal("discard"))
		})
	})

	Context("Cluster with cr", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(cr + scChanged))
			f.RunHook()
		})
		It("StorageClass must be deleted", func() {
			Expect(f).To(ExecuteSuccessfully())
			sc := f.KubernetesGlobalResource("StorageClass", "test-rbd")
			Expect(sc.Exists()).To(BeFalse())
		})
	})
})
