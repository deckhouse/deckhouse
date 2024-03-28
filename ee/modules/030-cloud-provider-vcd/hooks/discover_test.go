/*
Copyright 2023 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vcd :: hooks :: cloud_provider_discovery_data ::", func() {
	initValues := `
cloudProviderVcd:
  internal: {}
`

	storageClasses := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: default
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cloud-provider-vcd
  annotations:
    meta.helm.sh/release-name: cloud-provider-Vcd
    meta.helm.sh/release-namespace: d8-system
provisioner: named-disk.csi.cloud-director.vmware.com
parameters:
  storageProfile: "SAS"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    meta.helm.sh/release-name: local-path-provisioner
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2022-11-24T16:33:07Z"
  labels:
    app: local-path-provisioner
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: local-path-provisioner
  name: localpath-system
provisioner: deckhouse.io/localpath-system
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: hdd
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cloud-provider-vcd
  annotations:
    meta.helm.sh/release-name: cloud-provider-Vcd
    meta.helm.sh/release-namespace: d8-system
provisioner: named-disk.csi.cloud-director.vmware.com
parameters:
  storageProfile: "HDD"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`

	storageClassesWithDefault := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: default
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cloud-provider-vcd
  annotations:
    meta.helm.sh/release-name: cloud-provider-Vcd
    meta.helm.sh/release-namespace: d8-system
    storageclass.kubernetes.io/is-default-class: 'true'
provisioner: named-disk.csi.cloud-director.vmware.com
parameters:
  storageProfile: "SAS"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    meta.helm.sh/release-name: local-path-provisioner
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2022-11-24T16:33:07Z"
  labels:
    app: local-path-provisioner
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: local-path-provisioner
  name: localpath-system
provisioner: deckhouse.io/localpath-system
reclaimPolicy: Retain
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: hdd
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: cloud-provider-vcd
  annotations:
    meta.helm.sh/release-name: cloud-provider-Vcd
    meta.helm.sh/release-namespace: d8-system
provisioner: named-disk.csi.cloud-director.vmware.com
parameters:
  storageProfile: "HDD"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`

	manualStorageClasses := `---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: manual-default
provisioner: named-disk.csi.cloud-director.vmware.com
parameters:
  storageProfile: "MANUAL-DEFAULT"
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: manual-SAS
  annotations:
    storageclass.kubernetes.io/is-default-class: 'true'
provisioner: named-disk.csi.cloud-director.vmware.com
parameters:
  storageProfile: "MANUAL-SAS"
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
`

	//nolint:misspell
	discoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "VCDCloudProviderDiscoveryData",
  "storageProfiles": [
    {
      "name": "D1",
      "isEnabled": true
 	},
    {
      "name": "D2",
      "isEnabled": false
 	},
    {
      "name": "D3",
      "isEnabled": true,
 	},
  ]
}`

	state := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  "discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(discoveryData)))

	a := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has empty state", func() {
		BeforeEach(func() {
			a.BindingContexts.Set(a.KubeStateSet(``))
			a.RunHook()
		})

		It("Hook should not fail with errors", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.GoHookError).Should(BeNil())
		})
	})

	b := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has only storage classes", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(storageClasses))
			b.RunHook()
		})

		It("Should discover all volumeTypes only for storage classes where deployed by cloud-provider-Vcd module and no default", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderVcd.internal.storageClasses").String()).To(MatchJSON(`
[
         {
            "name": "default",
            "storageProfile": "SAS"
          },
          {
            "name": "hdd",
            "storageProfile": "HDD"
          }
]
`))
			Expect(b.ValuesGet("cloudProviderVcd.internal.defaultStorageClass").Exists()).To(BeFalse())
		})
	})

	Context("Cluster has only storage classes wit default", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(storageClassesWithDefault))
			b.RunHook()
		})

		It("Should discover all volumeTypes only for storage classes where deployed by cloud-provider-Vcd module and no default", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderVcd.internal.storageClasses").String()).To(MatchJSON(`
[
         {
            "name": "default",
            "storageProfile": "SAS"
          },
          {
            "name": "hdd",
            "storageProfile": "HDD"
          }
]
`))
			Expect(b.ValuesGet("cloudProviderVcd.internal.defaultStorageClass").String()).Should(Equal("default"))
		})
	})

	c := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has only manual storage classes", func() {
		BeforeEach(func() {
			c.BindingContexts.Set(c.KubeStateSet(manualStorageClasses))
			c.RunHook()
		})

		It("Should not discover manual volumeTypes", func() {
			Expect(c).To(ExecuteSuccessfully())
			Expect(c.ValuesGet("cloudProviderVcd.internal.storageClasses").String()).To(BeEmpty())
		})
	})

	d := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster has deckhouse managed storage classes and manual storage classes", func() {
		BeforeEach(func() {
			d.BindingContexts.Set(d.KubeStateSet(storageClasses + manualStorageClasses))
			d.RunHook()
		})

		It("Should discover all deckhouse managed volumeTypes and no default", func() {
			Expect(d).To(ExecuteSuccessfully())
			Expect(d.ValuesGet("cloudProviderVcd.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "default",
            "storageProfile": "SAS"
          },
          {
            "name": "hdd",
            "storageProfile": "HDD"
          }
]
`))
			Expect(d.ValuesGet("cloudProviderVcd.internal.defaultStorageClass").Exists()).To(BeFalse())
		})
	})

	e := HookExecutionConfigInit(initValues, `{}`)
	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.KubeStateSet(state))
			e.RunHook()
		})

		It("Should discover all enabled volumeTypes and no default", func() {
			Expect(e).To(ExecuteSuccessfully())
			Expect(e.ValuesGet("cloudProviderVcd.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "d1",
            "storageProfile": "D1"
          },
          {
            "name": "d3",
            "storageProfile": "D3"
          }
]
`))
			Expect(e.ValuesGet("cloudProviderVcd.internal.defaultStorageClass").Exists()).To(BeFalse())
		})
	})

	initValues = `
cloudProviderVcd:
  internal: {}
  storageClass:
    exclude:
    - d3*
    - bar
    default: d1
`

	f := HookExecutionConfigInit(initValues, `{}`)
	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.RunHook()
		})

		It("All values should be gathered from discovered data", func() {
			Expect(f).To(ExecuteSuccessfully())
		})

		It("Should discover volumeTypes without excluded and default set", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderVcd.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "d1",
            "storageProfile": "D1"
          }
]
`))
			Expect(f.ValuesGet("cloudProviderVcd.internal.defaultStorageClass").String()).To(Equal(`d1`))
		})
	})
})
