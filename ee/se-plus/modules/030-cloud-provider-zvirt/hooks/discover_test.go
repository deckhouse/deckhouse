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

var _ = Describe("Modules :: cloud-provider-zvirt :: hooks :: cloud_provider_discovery_data ::", func() {
	initValues := `
cloudProviderZvirt:
  internal: {}
`

	//nolint:misspell
	discoveryData := `
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "ZvirtCloudProviderDiscoveryData",
  "storageDomains": [
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

		// The discovery data secret is written by the cloud-data-discoverer, which may not have
		// run yet. That is not an error — the hook has nothing to publish.
		It("Hook should not fail with errors", func() {
			Expect(a).To(ExecuteSuccessfully())
			Expect(a.GoHookError).Should(BeNil())
			Expect(a.ValuesGet("cloudProviderZvirt.internal.providerDiscoveryData").Exists()).To(BeFalse())
		})
	})

	b := HookExecutionConfigInit(initValues, `{}`)
	Context("Provider data is successfully discovered", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.KubeStateSet(state))
			b.RunHook()
		})

		// The hook validates the payload against the module's OpenAPI schema and puts it into
		// values as it is. Turning it into StorageClasses is storage_classes.go's job.
		It("Should publish the discovery data as-is, with defaults applied", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderZvirt.internal.providerDiscoveryData").String()).To(MatchJSON(`
{
  "apiVersion": "deckhouse.io/v1",
  "kind": "ZvirtCloudProviderDiscoveryData",
  "zones": ["default"],
  "storageDomains": [
    {"name": "D1", "isEnabled": true},
    {"name": "D2"},
    {"name": "D3", "isEnabled": true}
  ]
}
`))
		})
	})
})
