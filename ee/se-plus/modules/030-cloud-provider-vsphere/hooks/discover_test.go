/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/
//nolint:unused // TODO: fix unused linter
package hooks

import (
	"encoding/base64"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-vsphere :: hooks :: discover_zones_and_datastores ::", func() {
	const (
		initValuesStringA = `
cloudProviderVsphere:
  internal:
    providerClusterConfiguration:
      provider:
        server: test.test.com
        username: test
        password: test
        insecure: true
      region: Test
      regionTagCategory: test-region
      zoneTagCategory: test-zone
      sshPublicKey: test
      vmFolderPath: test
`
		initValuesStringB = `
cloudProviderVsphere:
  internal:
    providerClusterConfiguration:
      provider:
        server: test.test.com
        username: test
        password: test
        insecure: true
      region: Test
      regionTagCategory: test-region
      zoneTagCategory: test-zone
      sshPublicKey: test
      vmFolderPath: test
  storageClass:
    exclude:
    - .*lun.*
    - bar
    default: other-bar
`
		initValuesStringC = `
global:
  defaultClusterStorageClass: default-cluster-sc
cloudProviderVsphere:
  internal:
    providerClusterConfiguration:
      provider:
        server: test.test.com
        username: test
        password: test
        insecure: true
      region: Test
      regionTagCategory: test-region
      zoneTagCategory: test-zone
      sshPublicKey: test
      vmFolderPath: test
  storageClass:
    exclude:
    - .*lun.*
    - bar
    default: other-bar
`
		initValuesStringD = `
global:
  defaultClusterStorageClass: ""
cloudProviderVsphere:
  internal:
    providerClusterConfiguration:
      provider:
        server: test.test.com
        username: test
        password: test
        insecure: true
      region: Test
      regionTagCategory: test-region
      zoneTagCategory: test-zone
      sshPublicKey: test
      vmFolderPath: test
  storageClass:
    exclude:
    - .*lun.*
    - bar
    default: other-bar
`

		initValuesStringE = `
cloudProviderVsphere:
  internal:
    providerClusterConfiguration:
      provider:
        server: test.test.com
        username: test
        password: test
        insecure: true
      region: Test
      regionTagCategory: test-region
      zoneTagCategory: test-zone
      sshPublicKey: test
      vmFolderPath: test
  storageClass:
    exclude:
    - .*
    default: other-bar
`
	)

	//nolint:misspell
	discoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "VsphereCloudDiscoveryData",
  "vmFolderPath": "test",
  "datacenter": "DCTEST",
  "zones": ["ZONE-TEST"],
  "datastores": [
	  {
		"datastoreType": "DatastoreCluster",
		"datastoreURL": "",
		"name": "TeSt-1-k8s-3cf5ce84",
		"path": "/DCTEST/datastore/test_1_k8s",
		"zones": [
		  "ZONE-TEST"
		]
	  },
	  {
		"datastoreType": "Datastore",
	    "datastoreURL":"ds:///vmfs/volumes/503a9af1-291d17b0-52e0-1d01842f428c/",
		"name": "test-1-LUN101-b39d82fa",
		"path": "/DCTEST/datastore/test_1_Lun101",
		"zones": [
		  "ZONE-TEST"
		]
	  },
	  {
		"datastoreType": "Datastore",
	    "datastoreURL":"ds:///vmfs/volumes/55832249-30a68048-496f-33f77fed3c5c/",
		"name": "test-1-lun102-0403073a",
		"path": "/DCTEST/datastore/test_1_Lun102",
		"zones": [
		  "ZONE-TEST"
		]
	  }
  ],
   "storagePolicies":[
      {
         "name":"Management Storage Policy - Large",
         "id":"cd8f7c94-3e11-67fc-17f5-4e96d91a5beb"
      },
      {
         "name":"VVol No Requirements Policy",
         "id":"f4e5bade-15a2-4805-bf8e-52318c4ce443"
      }
   ]
}
`

	state := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-provider-discovery-data
  namespace: kube-system
data:
  "discovery-data.json": %s
`, base64.StdEncoding.EncodeToString([]byte(discoveryData)))

	f := HookExecutionConfigInit(initValuesStringA, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(state))
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should discover all volumeTypes and no default", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData.datacenter").String()).To(Equal(`DCTEST`))
			Expect(f.ValuesGet("cloudProviderVsphere.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["ZONE-TEST"]`))
			Expect(f.ValuesGet("cloudProviderVsphere.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "test-1-k8s-3cf5ce84",
            "path": "/DCTEST/datastore/test_1_k8s",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "DatastoreCluster",
            "datastoreURL": "",
            "storagePolicyName": ""
          },
          {
            "name": "test-1-k8s-3cf5ce84-management-storage-policy---large",
            "path": "/DCTEST/datastore/test_1_k8s",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "DatastoreCluster",
            "datastoreURL": "",
            "storagePolicyName": "Management Storage Policy - Large"
          },
          {
            "name": "test-1-k8s-3cf5ce84-vvol-no-requirements-policy",
            "path": "/DCTEST/datastore/test_1_k8s",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "DatastoreCluster",
            "datastoreURL": "",
            "storagePolicyName": "VVol No Requirements Policy"
          },
          {
            "name": "test-1-lun101-b39d82fa",
            "path": "/DCTEST/datastore/test_1_Lun101",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "Datastore",
            "datastoreURL": "ds:///vmfs/volumes/503a9af1-291d17b0-52e0-1d01842f428c/",
            "storagePolicyName": ""
          },
          {
            "name": "test-1-lun101-b39d82fa-management-storage-policy---large",
            "path": "/DCTEST/datastore/test_1_Lun101",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "Datastore",
            "datastoreURL": "ds:///vmfs/volumes/503a9af1-291d17b0-52e0-1d01842f428c/",
            "storagePolicyName": "Management Storage Policy - Large"
          },
          {
            "name": "test-1-lun101-b39d82fa-vvol-no-requirements-policy",
            "path": "/DCTEST/datastore/test_1_Lun101",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "Datastore",
            "datastoreURL": "ds:///vmfs/volumes/503a9af1-291d17b0-52e0-1d01842f428c/",
            "storagePolicyName": "VVol No Requirements Policy"
          },
          {
            "name": "test-1-lun102-0403073a",
            "path": "/DCTEST/datastore/test_1_Lun102",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "Datastore",
            "datastoreURL": "ds:///vmfs/volumes/55832249-30a68048-496f-33f77fed3c5c/",
            "storagePolicyName": ""
          },
          {
            "name": "test-1-lun102-0403073a-management-storage-policy---large",
            "path": "/DCTEST/datastore/test_1_Lun102",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "Datastore",
            "datastoreURL": "ds:///vmfs/volumes/55832249-30a68048-496f-33f77fed3c5c/",
            "storagePolicyName": "Management Storage Policy - Large"
          },
          {
            "name": "test-1-lun102-0403073a-vvol-no-requirements-policy",
            "path": "/DCTEST/datastore/test_1_Lun102",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "Datastore",
            "datastoreURL": "ds:///vmfs/volumes/55832249-30a68048-496f-33f77fed3c5c/",
            "storagePolicyName": "VVol No Requirements Policy"
          }
        ]
`))
		})

	})

	b := HookExecutionConfigInit(initValuesStringB, `{}`)

	Context("Cluster has minimal cloudProviderVsphere configuration with excluded storage classes", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(state))
			b.BindingContexts.Set(b.GenerateBeforeHelmContext())
			b.RunHook()
		})

		It("Should discover volumeTypes without excluded", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderVsphere.internal.storageClasses").String()).To(MatchJSON(`
[
          {
            "name": "test-1-k8s-3cf5ce84",
            "path": "/DCTEST/datastore/test_1_k8s",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "DatastoreCluster",
            "datastoreURL": "",
            "storagePolicyName": ""
          },
          {
            "name": "test-1-k8s-3cf5ce84-management-storage-policy---large",
            "path": "/DCTEST/datastore/test_1_k8s",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "DatastoreCluster",
            "datastoreURL": "",
            "storagePolicyName": "Management Storage Policy - Large"
          },
          {
            "name": "test-1-k8s-3cf5ce84-vvol-no-requirements-policy",
            "path": "/DCTEST/datastore/test_1_k8s",
            "zones": [
              "ZONE-TEST"
            ],
            "datastoreType": "DatastoreCluster",
            "datastoreURL": "",
            "storagePolicyName": "VVol No Requirements Policy"
          }
]
`))
		})
	})

	e := HookExecutionConfigInit(initValuesStringE, `{}`)

	Context("When all discovered storage classes are excluded", func() {
		BeforeEach(func() {
			e.BindingContexts.Set(e.KubeStateSet(state))
			e.BindingContexts.Set(e.GenerateBeforeHelmContext())
			e.RunHook()
		})

		It("Should result empty storageClasses list", func() {
			Expect(e).To(ExecuteSuccessfully())
			Expect(e.ValuesGet("cloudProviderVsphere.internal.storageClasses").String()).To(MatchJSON(`[]`))
		})
	})
})
