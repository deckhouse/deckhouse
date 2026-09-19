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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: storage_classes ::", func() {
	// The hook reads the discovery data from values, where discover.go has already put it, so the
	// fixtures carry it as values rather than as a Secret.
	const provider = `
cloudProviderDvp:
  provider:
    parameters:
      namespace: test-ns
`

	const discoveryDataValues = provider + `
  internal:
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      kind: DVPCloudDiscoveryData
      zones: ["default"]
      storageClasses:
      - name: replicated
        volumeBindingMode: Immediate
        reclaimPolicy: Delete
        allowVolumeExpansion: true
        isEnabled: true
        isDefault: true
      - name: Excluded Fast
        volumeBindingMode: Immediate
        reclaimPolicy: Retain
        allowVolumeExpansion: false
        isEnabled: true
        isDefault: false
      - name: Disabled
        volumeBindingMode: Immediate
        reclaimPolicy: Delete
        allowVolumeExpansion: false
        isEnabled: false
        isDefault: false
`

	const excludes = `
  storage:
    parameters:
      excludedStorageClasses:
      - excluded-.*
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
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: excluded-legacy
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
provisioner: csi.dvp.deckhouse.io
parameters:
  dvpStorageClass: Excluded Legacy
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`

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

	// A StorageClass created by the module before it was added to excludedStorageClasses.
	// It is absent from the discovery data, so it only reaches the values through the snapshots.
	existingExcludedStorageClass := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: excluded-legacy
  labels:
    heritage: deckhouse
    module: cloud-provider-dvp
provisioner: csi.dvp.deckhouse.io
parameters:
  dvpStorageClass: Excluded Legacy
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`

	Context("When cluster state is empty", func() {
		f := HookExecutionConfigInit(provider+"\n  internal: {}\n", `{}`)
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

	Context("When only managed StorageClass snapshots are present", func() {
		f := HookExecutionConfigInit(provider+"\n  internal: {}\n", `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(storageClassesOnly))
			f.RunHook()
		})

		It("Should discover storage classes from snapshots and normalize volumeBindingMode", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.storageClasses").String()).To(MatchJSON(`
[
  {
    "name": "excluded-legacy",
    "dvpStorageClass": "Excluded Legacy",
    "volumeBindingMode": "WaitForFirstConsumer",
    "reclaimPolicy": "Delete",
    "allowVolumeExpansion": false,
    "isDefault": false
  },
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

	Context("When only managed StorageClass snapshots are present and excludes are set", func() {
		f := HookExecutionConfigInit(provider+excludes+"\n  internal:\n    defaultStorageClass: stale-default\n", `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(storageClassesOnly))
			f.RunHook()
		})

		It("Should apply excludes to the snapshot fallback as well", func() {
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
		})
	})

	Context("When discovery data and managed StorageClasses are present", func() {
		f := HookExecutionConfigInit(discoveryDataValues+excludes, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(
				f.GenerateBeforeHelmContext(),
				f.KubeStateSet(existingStorageClass+existingRetainedStorageClass+existingExcludedStorageClass),
			)
			f.RunHook()
		})

		It("Should merge discovery data with snapshots, apply excludes and keep only enabled classes", func() {
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

	Context("When excludedStorageClasses contains a plain StorageClass name", func() {
		const similarlyNamedValues = provider + `
  storage:
    parameters:
      excludedStorageClasses:
      - fast
  internal:
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      kind: DVPCloudDiscoveryData
      zones: ["default"]
      storageClasses:
      - name: fast
        volumeBindingMode: Immediate
        reclaimPolicy: Delete
        allowVolumeExpansion: false
        isEnabled: true
        isDefault: false
      - name: ultra-fast-ssd
        volumeBindingMode: Immediate
        reclaimPolicy: Delete
        allowVolumeExpansion: false
        isEnabled: true
        isDefault: false
`

		f := HookExecutionConfigInit(similarlyNamedValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should match the whole name and keep the classes it is only a part of", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.storageClasses").String()).To(MatchJSON(`
[
  {
    "name": "ultra-fast-ssd",
    "dvpStorageClass": "ultra-fast-ssd",
    "volumeBindingMode": "WaitForFirstConsumer",
    "reclaimPolicy": "Delete",
    "allowVolumeExpansion": false,
    "isDefault": false
  }
]
`))
		})
	})

	Context("When excludedStorageClasses contains the default StorageClass", func() {
		const defaultExcludedValues = discoveryDataValues + `
  storage:
    parameters:
      excludedStorageClasses:
      - replicated
`

		f := HookExecutionConfigInit(defaultExcludedValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(existingStorageClass))
			f.RunHook()
		})

		It("Should exclude the default class and drop defaultStorageClass", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.storageClasses").String()).To(MatchJSON(`
[
  {
    "name": "excluded-fast",
    "dvpStorageClass": "Excluded Fast",
    "volumeBindingMode": "WaitForFirstConsumer",
    "reclaimPolicy": "Retain",
    "allowVolumeExpansion": false,
    "isDefault": false
  }
]
`))
			Expect(f.ValuesGet("cloudProviderDvp.internal.defaultStorageClass").Exists()).To(BeFalse())
		})
	})

	Context("When excludedStorageClasses contains an invalid regular expression", func() {
		const brokenExcludeValues = provider + `
  storage:
    parameters:
      excludedStorageClasses:
      - "excluded-([a-z"
  internal: {}
`

		f := HookExecutionConfigInit(brokenExcludeValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(storageClassesOnly))
			f.RunHook()
		})

		It("Should fail instead of panicking", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
		})
	})

	Context("When no discovered class is marked as default", func() {
		const nonDefaultValues = provider + `
  internal:
    defaultStorageClass: stale-default
    providerDiscoveryData:
      apiVersion: deckhouse.io/v1
      kind: DVPCloudDiscoveryData
      zones: ["default"]
      storageClasses:
      - name: replicated
        volumeBindingMode: Immediate
        reclaimPolicy: Delete
        allowVolumeExpansion: true
        isEnabled: true
        isDefault: false
`

		f := HookExecutionConfigInit(nonDefaultValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(``))
			f.RunHook()
		})

		It("Should remove stale defaultStorageClass", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.defaultStorageClass").Exists()).To(BeFalse())
		})
	})

	Context("When the module configuration is not ready yet", func() {
		f := HookExecutionConfigInit(`{"cloudProviderDvp":{"internal":{}}}`, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(storageClassesOnly))
			f.RunHook()
		})

		// Any patch would be rejected by schema validation while the required settings are
		// missing, so the hook waits for the run in which they are there.
		It("Should skip without touching values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.storageClasses").Exists()).To(BeFalse())
		})
	})
})
