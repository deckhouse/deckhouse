/*
Copyright 2026 Flant JSC

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
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: discover ::", func() {
	const initValues = `
cloudProviderDvp:
  internal: {}
`

	const initValuesWithExclude = `
cloudProviderDvp:
  storageClass:
    exclude:
    - excluded-.*
  internal:
    defaultStorageClass: stale-default
`

	storageClassesOnly := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: replicated
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: csi.dvp.deckhouse.io
parameters:
  dvpStorageClass: replicated
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: Immediate
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: secondary
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
provisioner: csi.dvp.deckhouse.io
parameters:
  dvpStorageClass: secondary
reclaimPolicy: Retain
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`

	discoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DVPCloudDiscoveryData",
  "zones": ["default"],
  "storageClasses": [
    {
      "name": "replicated",
      "volumeBindingMode": "Immediate",
      "reclaimPolicy": "Delete",
      "allowVolumeExpansion": true,
      "isEnabled": true,
      "isDefault": true
    },
    {
      "name": "Excluded Fast",
      "volumeBindingMode": "Immediate",
      "reclaimPolicy": "Retain",
      "allowVolumeExpansion": false,
      "isEnabled": true,
      "isDefault": false
    },
    {
      "name": "Disabled",
      "volumeBindingMode": "Immediate",
      "reclaimPolicy": "Delete",
      "allowVolumeExpansion": false,
      "isEnabled": false,
      "isDefault": false
    }
  ]
}
`

	discoverySecret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  "discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(discoveryData)))

	existingStorageClass := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: replicated
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: csi.dvp.deckhouse.io
parameters:
  dvpStorageClass: replicated
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: Immediate
`

	existingRetainedStorageClass := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: retained
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
provisioner: csi.dvp.deckhouse.io
parameters:
  dvpStorageClass: retained
reclaimPolicy: Retain
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`

	Context("When cluster state is empty", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should succeed and leave internal values empty", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.storageClasses").String()).To(BeEmpty())
			Expect(f.ValuesGet("cloudProviderDvp.internal.defaultStorageClass").Exists()).To(BeFalse())
		})
	})

	Context("When StorageClasses exist on OperatorStartup but no ModuleConfig (no nodes/provider)", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext(), f.KubeStateSet(storageClassesOnly))
			f.RunHook()
		})

		It("Should succeed without requiring nodes/provider in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.storageClasses").Exists()).To(BeFalse())
		})
	})

	Context("When only managed StorageClass snapshots are present", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(storageClassesOnly))
			f.RunHook()
		})

		It("Should discover storage classes from snapshots and normalize volumeBindingMode", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.storageClasses").String()).To(MatchJSON(`
[
  {
    "name": "replicated",
    "dvpStorageClass": "replicated",
    "volumeBindingMode": "WaitForFirstConsumer",
    "reclaimPolicy": "Delete",
    "allowVolumeExpansion": true,
    "isDefault": true
  },
  {
    "name": "secondary",
    "dvpStorageClass": "secondary",
    "volumeBindingMode": "WaitForFirstConsumer",
    "reclaimPolicy": "Retain",
    "allowVolumeExpansion": false,
    "isDefault": false
  }
]
`))
			Expect(f.ValuesGet("cloudProviderDvp.internal.defaultStorageClass").String()).To(Equal("replicated"))
			Expect(f.KubernetesGlobalResource("StorageClass", "replicated").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("StorageClass", "secondary").Exists()).To(BeTrue())
		})
	})

	Context("When discovery data and managed StorageClasses are present", func() {
		f := HookExecutionConfigInit(initValuesWithExclude, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
				f.KubeStateSet(discoverySecret+existingStorageClass+existingRetainedStorageClass),
			)
			f.RunHook()
		})

		It("Should merge discovery data with snapshots, apply excludes and keep only enabled classes", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.kind").String()).To(Equal("DVPCloudDiscoveryData"))
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["default"]`))
			Expect(f.ValuesGet("cloudProviderDvp.internal.storageClasses").String()).To(MatchJSON(`
[
  {
    "name": "replicated",
    "dvpStorageClass": "replicated",
    "volumeBindingMode": "WaitForFirstConsumer",
    "reclaimPolicy": "Delete",
    "allowVolumeExpansion": true,
    "isDefault": true
  },
  {
    "name": "retained",
    "dvpStorageClass": "retained",
    "volumeBindingMode": "WaitForFirstConsumer",
    "reclaimPolicy": "Retain",
    "allowVolumeExpansion": false,
    "isDefault": false
  }
]
`))
			Expect(f.ValuesGet("cloudProviderDvp.internal.defaultStorageClass").String()).To(Equal("replicated"))
			Expect(f.KubernetesGlobalResource("StorageClass", "replicated").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("StorageClass", "retained").Exists()).To(BeTrue())
		})
	})

	Context("When no discovered class is marked as default", func() {
		nonDefaultDiscoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DVPCloudDiscoveryData",
  "zones": ["default"],
  "storageClasses": [
    {
      "name": "replicated",
      "volumeBindingMode": "Immediate",
      "reclaimPolicy": "Delete",
      "allowVolumeExpansion": true,
      "isEnabled": true,
      "isDefault": false
    }
  ]
}
`
		nonDefaultDiscoverySecret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  "discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(nonDefaultDiscoveryData)))

		f := HookExecutionConfigInit(initValuesWithExclude, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
				f.KubeStateSet(nonDefaultDiscoverySecret),
			)
			f.RunHook()
		})

		It("Should remove stale defaultStorageClass", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.defaultStorageClass").Exists()).To(BeFalse())
		})
	})

	Context("When discovery data secret contains invalid payload", func() {
		invalidDiscoverySecret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  "discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(`{"apiVersion":"deckhouse.io/v1","kind":"DVPCloudDiscoveryData","storageClasses":"broken"}`)))

		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
				f.KubeStateSet(invalidDiscoverySecret),
			)
			f.RunHook()
		})

		It("Should fail validation", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	DescribeTable("getStorageClassName",
		func(input, expected string) {
			Expect(getStorageClassName(input)).To(Equal(expected))
		},
		Entry("keeps valid name", "replicated", "replicated"),
		Entry("normalizes spaces and case", "Excluded Fast", "excluded-fast"),
		Entry("removes invalid symbols and trims ends", "-Xx__$()? -foo-", "xx--foo"),
		Entry("trims dots and dashes", ".. YY fast SSD-foo.-", "yy-fast-ssd-foo"),
	)

	DescribeTable("storageClassToStorageClassValue",
		func(input *storagev1.StorageClass, expected storageClass) {
			Expect(storageClassToStorageClassValue(input)).To(Equal(expected))
		},
		Entry("uses defaults for nil optional fields",
			&storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "replicated",
				},
				Parameters: map[string]string{
					"dvpStorageClass": "replicated",
				},
			},
			storageClass{
				Name:                 "replicated",
				DVPStorageClass:      "replicated",
				VolumeBindingMode:    string(defaultVolumeBindingMode),
				ReclaimPolicy:        string(corev1.PersistentVolumeReclaimDelete),
				AllowVolumeExpansion: false,
				IsDefault:            false,
			},
		),
		Entry("reads stable default annotation and optional fields",
			&storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "stable-default",
					Annotations: map[string]string{
						stableDefaultAnnotation: "TrUe",
					},
				},
				Parameters: map[string]string{
					"dvpStorageClass": "stable-default",
				},
				ReclaimPolicy:        ptrTo(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: ptrTo(true),
			},
			storageClass{
				Name:                 "stable-default",
				DVPStorageClass:      "stable-default",
				VolumeBindingMode:    string(defaultVolumeBindingMode),
				ReclaimPolicy:        string(corev1.PersistentVolumeReclaimRetain),
				AllowVolumeExpansion: true,
				IsDefault:            true,
			},
		),
		Entry("reads beta default annotation",
			&storagev1.StorageClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: "beta-default",
					Annotations: map[string]string{
						betaDefaultAnnotation: "true",
					},
				},
				Parameters: map[string]string{
					"dvpStorageClass": "beta-default",
				},
			},
			storageClass{
				Name:                 "beta-default",
				DVPStorageClass:      "beta-default",
				VolumeBindingMode:    string(defaultVolumeBindingMode),
				ReclaimPolicy:        string(corev1.PersistentVolumeReclaimDelete),
				AllowVolumeExpansion: false,
				IsDefault:            true,
			},
		),
	)
})

func ptrTo[T any](v T) *T {
	return &v
}
