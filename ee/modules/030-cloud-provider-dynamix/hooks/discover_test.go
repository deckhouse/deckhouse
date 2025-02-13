/*
Copyright 2024 Flant JSC
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

var _ = Describe("Modules :: cloud-provider-dynamix :: hooks :: cloud_provider_discovery_data ::", func() {
	initValues := `
cloudProviderDynamix:
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
    module: cloud-provider-dynamix
  annotations:
    meta.helm.sh/release-name: cloud-provider-Dynamix
    meta.helm.sh/release-namespace: d8-system
provisioner: dynamix.deckhouse.io
parameters:
  storageEndpoint: defaultEndpoint
  pool: defaultPool
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
    module: cloud-provider-dynamix
  annotations:
    meta.helm.sh/release-name: cloud-provider-Dynamix
    meta.helm.sh/release-namespace: d8-system
provisioner: dynamix.deckhouse.io
parameters:
  storageEndpoint: hddEndpoint
  pool: hddPool
reclaimPolicy: Delete
allowVolumeExpansion: false
volumeBindingMode: WaitForFirstConsumer
`

	manualStorageClasses := `---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: manual-default
provisioner: dynamix.deckhouse.io
parameters:
  storageEndpoint: "MANUAL-DEFAULT"
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
provisioner: dynamix.deckhouse.io
parameters:
  storageEndpoint: "MANUAL-SAS"
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
`

	//nolint:misspell
	discoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DynamixCloudProviderDiscoveryData",
  "storageEndpoints": [
    {
      "name": "D1",
      "pools": ["poolD1"],
      "isEnabled": true
 	},
    {
      "name": "D2",
      "pools": ["poolD2"],
      "isEnabled": false
 	},
    {
      "name": "D3",
      "pools": ["poolD3"],
      "isEnabled": true
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

		It("Should discover all volumeTypes only for storage classes where deployed by cloudProviderDynamix module", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
         {
            "name": "default",
            "storageEndpoint": "defaultEndpoint",
			"pool": "defaultPool",
			"allowVolumeExpansion": false
          },
          {
            "name": "hdd",
            "storageEndpoint": "hddEndpoint",
			"pool": "hddPool",
			"allowVolumeExpansion": false
          }
]
`))
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
			Expect(c.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(BeEmpty())
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
			Expect(d.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
         {
            "name": "default",
            "storageEndpoint": "defaultEndpoint",
			"pool": "defaultPool",
			"allowVolumeExpansion": false
          },
          {
            "name": "hdd",
            "storageEndpoint": "hddEndpoint",
			"pool": "hddPool",
			"allowVolumeExpansion": false
          }
]
`))
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
			Expect(e.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "d1",
            "storageEndpoint": "D1",
            "pool": "poolD1",
            "allowVolumeExpansion": true
          },
          {
            "name": "d3",
            "storageEndpoint": "D3",
            "pool": "poolD3",
            "allowVolumeExpansion": true
          }
]
`))
		})
	})
})
