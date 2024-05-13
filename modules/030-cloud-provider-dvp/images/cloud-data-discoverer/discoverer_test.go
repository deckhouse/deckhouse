/*
Copyright 2024 Flant JSC

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

package main

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/deckhouse/deckhouse/go_lib/cloud-data/app"
)

var storageProfileGVRToListKind = map[schema.GroupVersionResource]string{
	storageProfileGVR: "dvpinternalstorageprofilesList",
}

const iAmDefault = `
{
	"apiVersion": "internal.virtualization.deckhouse.io/v1beta1",
	"kind": "DVPInternalStorageProfile",
	"metadata": {
			"name": "iAmDefault"
	},
	"status": {
			"claimPropertySets": [
					{
							"accessModes": [
									"ReadWriteMany"
							],
							"volumeMode": "Block"
					},
					{
							"accessModes": [
									"ReadOnlyMany"
							],
							"volumeMode": "Block"
					},
					{
							"accessModes": [
									"ReadOnlyMany"
							],
							"volumeMode": "Filesystem"
					}
			],
			"storageClass": "iAmDefault"
	}
}
`

const blockRWX0 = `
{
	"apiVersion": "internal.virtualization.deckhouse.io/v1beta1",
	"kind": "DVPInternalStorageProfile",
	"metadata": {
			"name": "blockRWX0"
	},
	"status": {
			"claimPropertySets": [
					{
							"accessModes": [
									"ReadWriteMany"
							],
							"volumeMode": "Block"
					},
					{
							"accessModes": [
									"ReadOnlyMany"
							],
							"volumeMode": "Block"
					},
					{
							"accessModes": [
									"ReadOnlyMany"
							],
							"volumeMode": "Filesystem"
					}
			],
			"storageClass": "blockRWX0"
	}
}
`

const blockRWX1 = `
{
	"apiVersion": "internal.virtualization.deckhouse.io/v1beta1",
	"kind": "DVPInternalStorageProfile",
	"metadata": {
			"name": "blockRWX1"
	},
	"status": {
			"claimPropertySets": [
					{
							"accessModes": [
									"ReadWriteMany"
							],
							"volumeMode": "Block"
					}
			],
			"storageClass": "blockRWX1"
	}
}
`

const blockRO = `
{
	"apiVersion": "internal.virtualization.deckhouse.io/v1beta1",
	"kind": "DVPInternalStorageProfile",
	"metadata": {
			"name": "blockRO"
	},
	"status": {
			"claimPropertySets": [
					{
							"accessModes": [
									"ReadWriteOnce"
							],
							"volumeMode": "Block"
					}
			],
			"storageClass": "blockRO"
	}
}
`

const fsRO = `
{
	"apiVersion": "internal.virtualization.deckhouse.io/v1beta1",
	"kind": "DVPInternalStorageProfile",
	"metadata": {
			"name": "fsRO"
	},
	"status": {
			"claimPropertySets": [
					{
							"accessModes": [
									"ReadWriteOnce"
							],
							"volumeMode": "Filesystem"
					}
			],
			"storageClass": "fsRO"
	}
}
`

var testStorageProfiles = map[string]string{
	// supported storage profiles
	"iAmDefault": iAmDefault, // default storage class
	"blockRWX0":  blockRWX0,
	"blockRWX1":  blockRWX1,
	// unsupported storage profiles
	"blockRO": blockRO,
	"fsRO":    fsRO,
}

var _ = Describe("DVP Cloud discovery data tests", func() {

	var (
		fakeKubeClient    *fake.Clientset
		fakeDynamicClient *dynamicfake.FakeDynamicClient
		d                 Discoverer
	)

	BeforeEach(func() {
		logger := app.InitLogger()
		fakeKubeClient = fake.NewSimpleClientset()
		fakeDynamicClient = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), storageProfileGVRToListKind)

		d = Discoverer{
			logger:        logger,
			client:        fakeKubeClient,
			dynamicClient: fakeDynamicClient,
		}
	})

	Describe("Run", func() {

		Context("No storage classes in the cluster", func() {
			It("should return no error", func() {
				data, err := d.DiscoveryData(context.TODO(), []byte{})
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(MatchJSON(`{}`))
			})
		})

		Context("Two storage classes exist in the cluster", func() {
			It("should return no error", func() {

				var err error
				for storageClassName, storageProfileJson := range testStorageProfiles {
					spUnstructured := &unstructured.Unstructured{}
					err = spUnstructured.UnmarshalJSON([]byte(storageProfileJson))
					Expect(err).NotTo(HaveOccurred())

					err = fakeDynamicClient.Tracker().Add(spUnstructured)
					Expect(err).NotTo(HaveOccurred())

					annotations := make(map[string]string)
					if storageClassName == "iAmDefault" {
						annotations["storageclass.kubernetes.io/is-default-class"] = "true"
					}
					_, err = fakeKubeClient.StorageV1().StorageClasses().Create(context.TODO(), &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: storageClassName, Annotations: annotations}}, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
				}

				data, err := d.DiscoveryData(context.TODO(), []byte{})
				Expect(err).NotTo(HaveOccurred())
				Expect(data).To(MatchJSON(`{"storageClasses":[{"name":"blockRWX0"},{"name":"blockRWX1"},{"name": "iAmDefault","isDefault": true}]}`))
			})
		})

	})
})

func TestDiscoverer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DVP Discoverer Test Suite")
}
