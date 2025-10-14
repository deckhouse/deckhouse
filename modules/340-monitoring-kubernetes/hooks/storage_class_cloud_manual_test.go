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
	"k8s.io/utils/ptr"

	"github.com/deckhouse/deckhouse/pkg/metrics-storage/operation"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: monitoring-kubernetes :: hooks :: storage_class_cloud_manual ::", func() {
	const (
		properStorageClass = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: vsphere-main
provisioner: vsphere.csi.vmware.com
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
			Expect(f.KubernetesGlobalResource("StorageClass", "vsphere-main").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "aws-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "azure-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "openstack-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "vsphere-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "yandex-proper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "local-storage").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "rook-rbd").Exists()).To(BeTrue())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(1))

			// first is expiration
			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionExpireMetrics,
			}))
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
			Expect(f.KubernetesGlobalResource("StorageClass", "gcp-improper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "openstack-improper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "vsphere-improper").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("StorageClass", "yandex-improper").Exists()).To(BeTrue())
			ops := f.MetricsCollector.CollectedMetrics()
			Expect(len(ops)).To(BeEquivalentTo(7))

			// first is expiration
			Expect(ops[0]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionExpireMetrics,
			}))
			Expect(ops[1]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionGaugeSet,
				Name:   "storage_class_cloud_manual",
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"name": "aws-improper",
				},
			}))
			Expect(ops[2]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionGaugeSet,
				Name:   "storage_class_cloud_manual",
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"name": "azure-improper",
				},
			}))
			Expect(ops[3]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionGaugeSet,
				Name:   "storage_class_cloud_manual",
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"name": "gcp-improper",
				},
			}))
			Expect(ops[4]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionGaugeSet,
				Name:   "storage_class_cloud_manual",
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"name": "openstack-improper",
				},
			}))
			Expect(ops[5]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionGaugeSet,
				Name:   "storage_class_cloud_manual",
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"name": "vsphere-improper",
				},
			}))
			Expect(ops[6]).To(BeEquivalentTo(operation.MetricOperation{
				Action: operation.ActionGaugeSet,
				Name:   "storage_class_cloud_manual",
				Value:  ptr.To(1.0),
				Labels: map[string]string{
					"name": "yandex-improper",
				},
			}))
		})
	})

})
