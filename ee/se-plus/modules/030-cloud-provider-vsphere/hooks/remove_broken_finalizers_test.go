/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vsphere :: hooks :: vsphere_broken_finalizers ::", func() {

	const (
		properVolumeAttachment = `
---
apiVersion: storage.k8s.io/v1
kind: VolumeAttachment
metadata:
  annotations:
    csi.alpha.kubernetes.io/node-id: node1
  finalizers:
  - external-attacher/cinder-csi-openstack-org
  managedFields:
  - apiVersion: storage.k8s.io/v1
    manager: kube-controller-manager
    operation: Update
    time: "2021-02-02T10:59:28Z"
  - apiVersion: storage.k8s.io/v1
    manager: csi-attacher
    operation: Update
    time: "2021-02-02T10:59:55Z"
  name: csi-1
spec:
  attacher: cinder.csi.openstack.org
  nodeName: node1
  source:
    persistentVolumeName: pvc-1
status:
  attached: true
  attachmentMetadata:
    DevicePath: /dev/vdb
`
		brokenVolumeAttachment = `
---
apiVersion: storage.k8s.io/v1
kind: VolumeAttachment
metadata:
  annotations:
    csi.alpha.kubernetes.io/node-id: node2
  finalizers:
  - external-attacher/cinder-csi-openstack-org
  managedFields:
  - apiVersion: storage.k8s.io/v1
    manager: kube-controller-manager
    operation: Update
    time: "2021-02-02T10:59:28Z"
  - apiVersion: storage.k8s.io/v1
    manager: csi-attacher
    operation: Update
    time: "2021-02-02T10:59:55Z"
  name: csi-2
spec:
  attacher: cinder.csi.openstack.org
  nodeName: node2
  source:
    persistentVolumeName: pvc-2
status:
  attached: true
  attachmentMetadata:
    DevicePath: /dev/vdb
  detachError:
    message: "rpc error: code = Unknown desc = No VM found"
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("Cluster with volumeattachment objects", func() {
		BeforeEach(func() {
			f.KubeStateSet(properVolumeAttachment + brokenVolumeAttachment)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must run successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("VolumeAttachment csi-1 should not be changed, VolumeAttachment csi-2 should be changed", func() {
			volumeAttachmentCsi1 := f.KubernetesGlobalResource("VolumeAttachment", "csi-1")
			volumeAttachmentCsi2 := f.KubernetesGlobalResource("VolumeAttachment", "csi-2")
			Expect(volumeAttachmentCsi1.ToYaml()).To(MatchYAML(properVolumeAttachment))
			Expect(volumeAttachmentCsi2.Field("metadata.finalizers").String()).Should(BeEmpty())
		})
	})
})
