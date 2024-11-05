// Copyright 2024 Flant JSC
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
	initValuesNoDefaultClusterStorageClass = `
{"global": {"discovery": {}}}
`

	initValuesEmptyDefaultClusterStorageClass = `
{"global": {"discovery": {}, "defaultClusterStorageClass": ""}}
`

	initValuesWithDefinedDefaultClusterStorageClass = `
{"global": {"discovery": {}, "defaultClusterStorageClass": "%s"}}
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
	a := HookExecutionConfigInit(initValuesNoDefaultClusterStorageClass, `{}`)

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
	b := HookExecutionConfigInit(fmt.Sprintf(initValuesWithDefinedDefaultClusterStorageClass, `non-default`), `{}`)

	Context("User set global.defaultClusterStorageClass to `non-default`", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(scDefault + scNonDefault))

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

	// cluster C: global.defaultClusterStorageClass set to empty string
	c := HookExecutionConfigInit(initValuesEmptyDefaultClusterStorageClass, `{}`)

	Context("User set global.defaultClusterStorageClass to empty string", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(scDefault + scNonDefault))
			c.RunHook()
		})

		Context("`default` and `non-default` storage classes", func() {
			It("Should exist one default storage class", func() {
				Expect(c).To(ExecuteSuccessfully())

				sc := c.KubernetesGlobalResource("StorageClass", "default")
				Expect(sc.Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).String()).To(Equal("true"))
			})

			It("Should exist one NON-default storage class", func() {
				Expect(c).To(ExecuteSuccessfully())

				sc := c.KubernetesGlobalResource("StorageClass", "non-default")
				Expect(sc.Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).Exists()).To(BeFalse())
			})
		})
	})

	// cluster D: global.defaultClusterStorageClass = "non-existent"
	d := HookExecutionConfigInit(fmt.Sprintf(initValuesWithDefinedDefaultClusterStorageClass, `non-existent`), `{}`)

	Context("User set global.defaultClusterStorageClass to non-existent/misspelling `non-existent`", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(scDefault + scNonDefault))

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

			d.RunHook()
		})

		Context("`default` and `non-default` storage classes", func() {
			It("StorageClass `default` should stay AS IS", func() {
				Expect(d).To(ExecuteSuccessfully())

				sc := d.KubernetesGlobalResource("StorageClass", "default")
				Expect(sc.Exists()).To(BeTrue())

				Expect(sc.Field(`metadata.annotations`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).String()).To(Equal("true"))
			})

			It("StorageClass `non-default` should stay AS IS", func() {
				Expect(d).To(ExecuteSuccessfully())

				sc := d.KubernetesGlobalResource("StorageClass", "non-default")
				Expect(sc.Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations`).Exists()).To(BeTrue())
				Expect(sc.Field(`metadata.annotations.storageclass\.kubernetes\.io\/is-default-class`).Exists()).To(BeFalse())
			})
		})
	})

})
