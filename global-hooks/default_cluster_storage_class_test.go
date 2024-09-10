// Copyright 2021 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hooks

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	storage "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

// TODO: add tests with global.modules.storageClass variants

const (
	cmWithDefinedDefaultStorageClassName = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-default-cluster-storage-class
  namespace: d8-system
data:
  default-cluster-storage-class: "%s"
`
	cmWithEmptyDefaultStorageClassName = `
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: d8-default-cluster-storage-class
  namespace: d8-system
data:
  default-cluster-storage-class: ""
`

	scDefault = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: default
  uid: '0c09c147-d4c8-4d48-b014-cb34d508eac5'
  resourceVersion: '45632997'
  creationTimestamp: '2023-06-01T06:09:25Z'
  labels:
    app.kubernetes.io/managed-by: Helm
  annotations:
    meta.helm.sh/release-name: cloud-provider-openstack
    meta.helm.sh/release-namespace: d8-system
    storageclass.kubernetes.io/is-default-class: "true"
  selfLink: /apis/storage.k8s.io/v1/storageclasses/default
provisioner: cinder.csi.openstack.org
parameters:
  type: __DEFAULT__
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
`

	scNonDefault = `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: non-default
  uid: '1c09c147-d4c8-4d48-b014-cb34d508eac5'
  resourceVersion: '45632997'
  creationTimestamp: '2023-06-01T06:10:25Z'
  labels:
    app.kubernetes.io/managed-by: Helm
  annotations:
    meta.helm.sh/release-name: cloud-provider-openstack
    meta.helm.sh/release-namespace: d8-system
  selfLink: /apis/storage.k8s.io/v1/storageclasses/non-default
provisioner: cinder.csi.openstack.org
parameters:
  type: __NONDEFAULT__
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
`
)

var _ = Describe("Global hooks :: default_storage_class_name_test ::", func() {
	// cluster A: global.defaultClusterStorageClass NOT defined
	a := HookExecutionConfigInit(`{"global": {"discovery": {}}}`, `{}`)

	Context("Cluster A is empty", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		It("no configmap `d8-default-cluster-storage-class`", func() {
			Expect(a).To(ExecuteSuccessfully())

			cm := a.KubernetesResource("ConfigMap", d8Namespace, "d8-default-cluster-storage-class")
			Expect(cm.Exists()).To(BeFalse())
		})
	})

	Context("User NOT set global.defaultClusterStorageClass (default behaviour)", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(scDefault + scNonDefault))
			a.RunHook()
		})

		Context("`default` and `non-default` storage classes", func() {
			It("Should exist one default storage class", func() {
				Expect(a).To(ExecuteSuccessfully())

				sc := a.KubernetesGlobalResource("StorageClass", "default")
				Expect(sc.Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).String()).To(Equal("true"))
			})

			It("Should exist one NON-default storage class", func() {
				Expect(a).To(ExecuteSuccessfully())

				sc := a.KubernetesGlobalResource("StorageClass", "non-default")
				Expect(sc.Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).Exists()).To(BeFalse())
			})
		})
	})

	// cluster B: global.defaultClusterStorageClass = "non-default"
	b := HookExecutionConfigInit(`{"global": {"defaultClusterStorageClass": "non-default"}}`, `{}`)

	Context("User set global.defaultClusterStorageClass to `non-default`", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(fmt.Sprintf(cmWithDefinedDefaultStorageClassName, `non-default`) + scDefault + scNonDefault))

			// create required storage classes in fake k8s cluster
			for _, scYaml := range []string{scDefault, scNonDefault} {
				var sc storage.StorageClass
				_ = yaml.Unmarshal([]byte(scYaml), &sc)
				_, err := dependency.TestDC.MustGetK8sClient().
					StorageV1().
					StorageClasses().
					Create(context.TODO(), &sc, metav1.CreateOptions{})

				Expect(err).To(BeNil())
			}

			b.RunHook()
		})

		Context("has required k8s resources", func() {
			It("configmap `d8-default-cluster-storage-class` with proper value", func() {
				Expect(b).To(ExecuteSuccessfully())

				cm := b.KubernetesResource("ConfigMap", d8Namespace, "d8-default-cluster-storage-class")
				Expect(cm.Exists()).To(BeTrue())
				Expect(cm.Field(`data`).String()).To(MatchJSON(`{"default-cluster-storage-class": "non-default"}`))
			})
		})

		Context("`default` and `non-default` storage classes", func() {
			It("StorageClass `default` became non-default", func() {
				Expect(b).To(ExecuteSuccessfully())

				sc := b.KubernetesGlobalResource("StorageClass", "default")
				Expect(sc.Exists()).To(BeTrue())

				Expect(sc.Field(`metadata.annotations`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).Exists()).To(BeFalse())
			})

			It("StorageClass `non-default` must be new default", func() {
				Expect(b).To(ExecuteSuccessfully())

				sc := b.KubernetesGlobalResource("StorageClass", "non-default")
				Expect(sc.Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).String()).To(Equal("true"))
			})
		})
	})
})
