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
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-dvp :: hooks :: discover ::", func() {
	const initValues = `
cloudProviderDvp:
  internal: {}
`

	const initValuesWithProvider = `
cloudProviderDvp:
  provider:
    parameters:
      namespace: test-ns
  internal: {}
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

	Context("When cluster state is empty", func() {
		f := HookExecutionConfigInit(initValuesWithProvider, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(``))
			f.RunHook()
		})

		// The secret is written by the cloud-data-discoverer, which may not have run yet.
		It("Should succeed and publish nothing", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData").Exists()).To(BeFalse())
		})
	})

	Context("When the discovery data secret is present", func() {
		f := HookExecutionConfigInit(initValuesWithProvider, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(discoverySecret))
			f.RunHook()
		})

		// The hook validates the payload against the module's OpenAPI schema and puts it into
		// values as it is. Turning it into StorageClasses is storage_classes.go's job.
		It("Should publish the discovery data as-is", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.apiVersion").String()).To(Equal("deckhouse.io/v1"))
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.kind").String()).To(Equal("DVPCloudDiscoveryData"))
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.zones").String()).To(MatchJSON(`["default"]`))
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData.storageClasses.0.name").String()).To(Equal("replicated"))
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

		f := HookExecutionConfigInit(initValuesWithProvider, `{}`)
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

	Context("When the module configuration is not ready yet", func() {
		f := HookExecutionConfigInit(initValues, `{}`)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext(), f.KubeStateSet(discoverySecret))
			f.RunHook()
		})

		// Any patch would be rejected by schema validation while the required settings are
		// missing, so the hook waits for the run in which they are there.
		It("Should skip without touching values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderDvp.internal.providerDiscoveryData").Exists()).To(BeFalse())
		})
	})
})
