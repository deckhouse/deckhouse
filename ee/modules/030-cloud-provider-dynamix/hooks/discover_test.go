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

	initValuesWithExclude := `
cloudProviderDynamix:
  internal: {}
  storageClass:
    exclude:
    - .*-hdd
    - storagepolicy02
`

	initValuesWithDefault := `
cloudProviderDynamix:
  internal: {}
  storageClass:
    default: storagepolicy02
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
  account: acc_user
  storagePolicy: Default Policy
reclaimPolicy: Delete
allowVolumeExpansion: true
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
  account: acc_user
  storagePolicy: HDD Policy
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
`

	manualStorageClasses := `---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: manual-default
provisioner: dynamix.deckhouse.io
parameters:
  storagePolicy: "MANUAL-DEFAULT"
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
  storagePolicy: "MANUAL-SAS"
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
`

	discoveryDataSecret := func(discoveryData string) string {
		return fmt.Sprintf(`
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  "discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(discoveryData)))
	}

	singlePolicyDiscoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DynamixCloudProviderDiscoveryData",
  "storagePolicies": [
    {
      "name": "Storage Policy 01",
      "limitIOPS": 2000
    }
  ]
}`

	severalPoliciesDiscoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DynamixCloudProviderDiscoveryData",
  "storagePolicies": [
    {
      "name": "storage_policy02",
      "limitIOPS": 0
    },
    {
      "name": "Storage Policy 01",
      "limitIOPS": 2000
    },
    {
      "name": "fast-hdd",
      "limitIOPS": 500
    }
  ]
}`

	emptyDiscoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DynamixCloudProviderDiscoveryData"
}`

	collidingPoliciesDiscoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DynamixCloudProviderDiscoveryData",
  "storagePolicies": [
    {
      "name": "Storage Policy 01",
      "limitIOPS": 2000
    },
    {
      "name": "storage-policy-01",
      "limitIOPS": 500
    }
  ]
}`

	// StorageClasses of a cluster provisioned before 4.6: named after a storage endpoint,
	// parameterized with the endpoint and the pool. "storage-policy-01" collides with the
	// name the "Storage Policy 01" policy normalizes to, "legacy-sep" does not.
	legacyStorageClasses := `
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: storage-policy-01
  labels:
    heritage: deckhouse
    module: cloud-provider-dynamix
provisioner: dynamix.deckhouse.io
parameters:
  account: acc_user
  location: dynamix
  storageEndpoint: SharedTatlin_G1_SEP
  pool: pool_a
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: legacy-sep
  labels:
    heritage: deckhouse
    module: cloud-provider-dynamix
provisioner: dynamix.deckhouse.io
parameters:
  account: acc_user
  location: dynamix
  storageEndpoint: SharedTatlin_G2_SEP
  pool: pool_b
reclaimPolicy: Delete
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
`

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

		It("Should restore storage classes deployed by the cloudProviderDynamix module from their storagePolicy parameter", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "default",
            "storagePolicy": "Default Policy"
          },
          {
            "name": "hdd",
            "storagePolicy": "HDD Policy"
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

		It("Should not discover manual storage classes", func() {
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

		It("Should discover only deckhouse managed storage classes", func() {
			Expect(d).To(ExecuteSuccessfully())
			Expect(d.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "default",
            "storagePolicy": "Default Policy"
          },
          {
            "name": "hdd",
            "storagePolicy": "HDD Policy"
          }
]
`))
		})
	})

	e := HookExecutionConfigInit(initValues, `{}`)
	Context("Discovery data has a single storage policy", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.KubeStateSet(discoveryDataSecret(singlePolicyDiscoveryData)))
			e.RunHook()
		})

		It("Should create a single storage class named after the normalized policy name", func() {
			Expect(e).To(ExecuteSuccessfully())
			Expect(e.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "storage-policy-01",
            "storagePolicy": "Storage Policy 01"
          }
]
`))
		})

		It("Should store the discovery data as is", func() {
			Expect(e).To(ExecuteSuccessfully())
			Expect(e.ValuesGet("cloudProviderDynamix.internal.providerDiscoveryData").String()).To(MatchJSON(`
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "DynamixCloudProviderDiscoveryData",
  "storagePolicies": [
    {
      "name": "Storage Policy 01",
      "limitIOPS": 2000
    }
  ]
}
`))
		})
	})

	f := HookExecutionConfigInit(initValues, `{}`)
	Context("Discovery data has several storage policies", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(discoveryDataSecret(severalPoliciesDiscoveryData)))
			f.RunHook()
		})

		It("Should create one storage class per policy, ordered by name", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "fast-hdd",
            "storagePolicy": "fast-hdd"
          },
          {
            "name": "storage-policy-01",
            "storagePolicy": "Storage Policy 01"
          },
          {
            "name": "storagepolicy02",
            "storagePolicy": "storage_policy02"
          }
]
`))
		})
	})

	g := HookExecutionConfigInit(initValuesWithExclude, `{}`)
	Context("Discovery data has several storage policies and storageClass.exclude is set", func() {
		BeforeEach(func() {
			g.BindingContexts.Set(g.KubeStateSet(discoveryDataSecret(severalPoliciesDiscoveryData)))
			g.RunHook()
		})

		It("Should skip excluded storage classes", func() {
			Expect(g).To(ExecuteSuccessfully())
			Expect(g.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "storage-policy-01",
            "storagePolicy": "Storage Policy 01"
          }
]
`))
		})
	})

	h := HookExecutionConfigInit(initValuesWithDefault, `{}`)
	Context("Discovery data has several storage policies and storageClass.default is set", func() {
		BeforeEach(func() {
			h.BindingContexts.Set(h.KubeStateSet(discoveryDataSecret(severalPoliciesDiscoveryData)))
			h.RunHook()
		})

		It("Should put the default storage class first, the rest stays ordered by name", func() {
			Expect(h).To(ExecuteSuccessfully())
			Expect(h.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "storagepolicy02",
            "storagePolicy": "storage_policy02"
          },
          {
            "name": "fast-hdd",
            "storagePolicy": "fast-hdd"
          },
          {
            "name": "storage-policy-01",
            "storagePolicy": "Storage Policy 01"
          }
]
`))
		})
	})

	i := HookExecutionConfigInit(initValues, `{}`)
	Context("Discovery data has no storage policies", func() {
		BeforeEach(func() {
			i.BindingContexts.Set(i.KubeStateSet(discoveryDataSecret(emptyDiscoveryData)))
			i.RunHook()
		})

		It("Should not create any storage class", func() {
			Expect(i).To(ExecuteSuccessfully())
			Expect(i.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`[]`))
		})
	})

	j := HookExecutionConfigInit(initValues, `{}`)
	Context("Two storage policies normalize to the same storage class name", func() {
		BeforeEach(func() {
			j.BindingContexts.Set(j.KubeStateSet(discoveryDataSecret(collidingPoliciesDiscoveryData)))
			j.RunHook()
		})

		It("Should keep one of them, picked independently of the discovery data order", func() {
			Expect(j).To(ExecuteSuccessfully())
			Expect(j.ValuesGet("cloudProviderDynamix.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "storage-policy-01",
            "storagePolicy": "Storage Policy 01"
          }
]
`))
		})
	})

	k := HookExecutionConfigInit(initValues, `{}`)
	Context("Cluster still has pre-4.6 storage classes", func() {
		BeforeEach(func() {
			k.BindingContexts.Set(k.KubeStateSet(legacyStorageClasses + discoveryDataSecret(singlePolicyDiscoveryData)))
			k.RunHook()
		})

		It("Should delete the one the chart re-renders with different parameters", func() {
			Expect(k).To(ExecuteSuccessfully())
			// StorageClass.parameters are immutable: Helm cannot patch storageEndpoint/pool
			// into account/storagePolicy, so the hook deletes it before Helm runs.
			Expect(k.KubernetesGlobalResource("StorageClass", "storage-policy-01").Exists()).To(BeFalse())
		})

		It("Should leave alone the one the chart no longer renders", func() {
			Expect(k).To(ExecuteSuccessfully())
			// Not part of the release any more, so Helm removes it on its own.
			Expect(k.KubernetesGlobalResource("StorageClass", "legacy-sep").Exists()).To(BeTrue())
		})
	})
})
