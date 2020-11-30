package hooks

import (
	. "github.com/deckhouse/deckhouse/testing/hooks"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Modules :: cloud-provider-vsphere :: hooks :: discover_zones_and_datastores ::", func() {
	const (
		initValuesStringA = `
cloudProviderVsphere:
  internal:
    server: test.test.com
    username: test
    password: test
    insecure: true
    region: Test
    regionTagCategory: test-region
    zoneTagCategory: test-zone
`
		initValuesStringB = `
cloudProviderVsphere:
  internal:
    server: test.test.com
    username: test
    password: test
    insecure: true
    region: Test
    regionTagCategory: test-region
    zoneTagCategory: test-zone
  storageClass:
    exclude:
    - .*lun.*
    - bar
    default: other-bar
`
	)

	f := HookExecutionConfigInit(initValuesStringA, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(BeforeHelmContext)
			f.RunHook()
		})

		It("Should discover all volumeTypes and no default", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderVsphere.internal.datacenter").String()).To(Equal(`DCTEST`))
			Expect(f.ValuesGet("cloudProviderVsphere.internal.zones").String()).To(MatchJSON(`["ZONE-TEST"]`))
			Expect(f.ValuesGet("cloudProviderVsphere.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"datastoreType": "DatastoreCluster",
	"name": "test-1-k8s-3cf5ce84",
	"path": "/DCTEST/datastore/test_1_k8s",
	"zones": [
	  "ZONE-TEST"
	]
  },
  {
	"datastoreType": "Datastore",
	"name": "test-1-lun101-b39d82fa",
	"path": "/DCTEST/datastore/test_1_Lun101",
	"zones": [
	  "ZONE-TEST"
	]
  },
  {
	"datastoreType": "Datastore",
	"name": "test-1-lun102-0403073a",
	"path": "/DCTEST/datastore/test_1_Lun102",
	"zones": [
	  "ZONE-TEST"
	]
  }
]
`))
			Expect(f.ValuesGet("cloudProviderVsphere.internal.defaultStorageClass").Exists()).To(BeFalse())
		})

	})

	b := HookExecutionConfigInit(initValuesStringB, `{}`)

	Context("Cluster has minimal cloudProviderVsphere configuration with excluded storage classes and default specified", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(BeforeHelmContext)
			b.RunHook()
		})

		It("Should discover volumeTypes without excluded and default set", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderVsphere.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"datastoreType": "DatastoreCluster",
	"name": "test-1-k8s-3cf5ce84",
	"path": "/DCTEST/datastore/test_1_k8s",
	"zones": [
	  "ZONE-TEST"
	]
  }
]
`))
			Expect(b.ValuesGet("cloudProviderVsphere.internal.defaultStorageClass").String()).To(Equal(`other-bar`))
		})
	})
})
