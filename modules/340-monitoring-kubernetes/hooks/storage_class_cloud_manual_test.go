package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-kubernetes :: hooks :: storage_class_cloud_manual ::", func() {
	const (
		properStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    heritage: deckhouse
  name: aws-proper
provisioner: ebs.csi.aws.com
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    heritage: deckhouse
  name: azure-proper
provisioner: disk.csi.azure.com
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    heritage: deckhouse
  name: gcp-proper
provisioner: pd.csi.storage.gke.io
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    heritage: deckhouse
  name: openstack-proper
provisioner: cinder.csi.openstack.org
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    heritage: deckhouse
  name: vsphere-proper
provisioner: vsphere.csi.vmware.com
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  labels:
    heritage: deckhouse
  name: yandex-proper
provisioner: yandex.csi.flant.com
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: local-storage
provisioner: kubernetes.io/no-provisioner
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: rook-rbd
provisioner: rook-external-ceph.rbd.csi.ceph.com
`
		improperStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: aws-improper
provisioner: ebs.csi.aws.com
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: azure-improper
provisioner: disk.csi.azure.com
---
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: gcp-improper
provisioner: pd.csi.storage.gke.io
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: openstack-improper
provisioner: cinder.csi.openstack.org
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: vsphere-improper
provisioner: vsphere.csi.vmware.com
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: yandex-improper
provisioner: yandex.csi.flant.com

`
	)
	f := HookExecutionConfigInit(
		`{"monitoringKubernetes":{"internal":{}},"global":{"enabledModules":[]}}`,
		`{}`,
	)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster containing proper StorageClasses", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(properStorageClass))
			f.RunHook()
		})

		It("Hook must not fail, StorageClasses must be in cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("StorageClass", "aws-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "azure-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "openstack-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "vsphere-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "yandex-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "local-storage").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "rook-rbd").Exists()).To(BeTrue())
		})
	})

	Context("Cluster containing improper StorageClasses", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(improperStorageClass))
			f.RunHook()
		})

		It("Hook must not fail, StorageClasses must be in cluster", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.KubernetesGlobalResource("StorageClass", "aws-improper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "azure-improper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "openstack-improper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "vsphere-improper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "yandex-improper").Exists()).To(BeTrue())
		})
	})

})
