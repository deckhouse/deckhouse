// Copyright 2025 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

/*

User-stories:
1. There is DVP cloud provider with discovery data containing storage classes. Hook must find SC with isDefault=true and store it to `global.discovery.cloudProviderDefaultStorageClass`.
2. If actual default SC in cluster differs from cloud provider default, hook must set drift metric.
3. If there's no default SC from cloud provider, hook must unset the discovery value.

*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Global hooks :: discovery :: cloud_provider_default_storage_class ::", func() {
	const (
		initValuesString       = `{"global": {"discovery": {}}}`
		initConfigValuesString = `{}`
	)

	const (
		// DVP discovery data with default SC
		dvpDiscoveryWithDefault = `
{
  "global": {
    "discovery": {}
  },
  "cloudProviderDvp": {
    "internal": {
      "providerDiscoveryData": {
        "storageClasses": [
          {
            "name": "fast-ssd",
            "isDefault": true,
            "allowVolumeExpansion": true
          },
          {
            "name": "standard",
            "isDefault": false,
            "allowVolumeExpansion": true
          }
        ]
      }
    }
  }
}
`

		// DVP discovery data without default SC
		dvpDiscoveryWithoutDefault = `
{
  "global": {
    "discovery": {}
  },
  "cloudProviderDvp": {
    "internal": {
      "providerDiscoveryData": {
        "storageClasses": [
          {
            "name": "fast-ssd",
            "isDefault": false,
            "allowVolumeExpansion": true
          },
          {
            "name": "standard",
            "isDefault": false,
            "allowVolumeExpansion": true
          }
        ]
      }
    }
  }
}
`

		// No DVP provider at all
		noDVPProvider = `
{
  "global": {
    "discovery": {}
  }
}
`

		// DVP discovery with default + actual default SC matches
		dvpWithMatchingActualSC = `
{
  "global": {
    "discovery": {
      "defaultStorageClass": "fast-ssd"
    }
  },
  "cloudProviderDvp": {
    "internal": {
      "providerDiscoveryData": {
        "storageClasses": [
          {
            "name": "fast-ssd",
            "isDefault": true,
            "allowVolumeExpansion": true
          }
        ]
      }
    }
  }
}
`

		// DVP discovery with default + actual default SC differs (drift)
		dvpWithDriftedActualSC = `
{
  "global": {
    "discovery": {
      "defaultStorageClass": "standard"
    }
  },
  "cloudProviderDvp": {
    "internal": {
      "providerDiscoveryData": {
        "storageClasses": [
          {
            "name": "fast-ssd",
            "isDefault": true,
            "allowVolumeExpansion": true
          },
          {
            "name": "standard",
            "isDefault": false,
            "allowVolumeExpansion": true
          }
        ]
      }
    }
  }
}
`
	)

	f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

	Context("No DVP cloud provider configured", func() {
		BeforeEach(func() {
			f.ValuesSet("global", map[string]interface{}{"discovery": map[string]interface{}{}})
			f.RunHook()
		})

		It("Should not set cloudProviderDefaultStorageClass", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").Exists()).To(BeFalse())
		})

		It("Should not set drift metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			// Expire operation is expected when no drift
			m := f.MetricsCollector.CollectedMetrics()
			if len(m) > 0 {
				Expect(int(m[0].Action)).To(Equal(4)) // Action 4 = Expire
			}
		})
	})

	Context("DVP provider with default SC", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", []byte(`{"discovery": {}}`))
			f.ValuesSetFromYaml("cloudProviderDvp", []byte(`{
				"internal": {
					"providerDiscoveryData": {
						"storageClasses": [
							{"name": "fast-ssd", "isDefault": true, "allowVolumeExpansion": true},
							{"name": "standard", "isDefault": false, "allowVolumeExpansion": true}
						]
					}
				}
			}`))
			f.RunHook()
		})

		It("Should set cloudProviderDefaultStorageClass to 'fast-ssd'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").String()).To(Equal("fast-ssd"))
		})

		It("Should not set drift metric when no actual SC exists yet", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			if len(m) > 0 {
				Expect(m[0].Action).To(Equal(4))
			}
		})
	})

	Context("DVP provider without default SC", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", []byte(`{"discovery": {}}`))
			f.ValuesSetFromYaml("cloudProviderDvp", []byte(`{
				"internal": {
					"providerDiscoveryData": {
						"storageClasses": [
							{"name": "fast-ssd", "isDefault": false, "allowVolumeExpansion": true},
							{"name": "standard", "isDefault": false, "allowVolumeExpansion": true}
						]
					}
				}
			}`))
			f.RunHook()
		})

		It("Should not set cloudProviderDefaultStorageClass", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").Exists()).To(BeFalse())
		})

		It("Should not set drift metric", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			if len(m) > 0 {
				Expect(m[0].Action).To(Equal(4))
			}
		})
	})

	Context("DVP provider with default SC matching actual cluster default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", []byte(`{"discovery": {"defaultStorageClass": "fast-ssd"}}`))
			f.ValuesSetFromYaml("cloudProviderDvp", []byte(`{
				"internal": {
					"providerDiscoveryData": {
						"storageClasses": [
							{"name": "fast-ssd", "isDefault": true, "allowVolumeExpansion": true}
						]
					}
				}
			}`))
			f.RunHook()
		})

		It("Should set cloudProviderDefaultStorageClass to 'fast-ssd'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").String()).To(Equal("fast-ssd"))
		})

		It("Should not set drift metric (no drift)", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			if len(m) > 0 {
				Expect(m[0].Action).To(Equal(4))
			}
		})
	})

	Context("DVP provider with default SC NOT matching actual cluster default (drift)", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", []byte(`{"discovery": {"defaultStorageClass": "standard"}}`))
			f.ValuesSetFromYaml("cloudProviderDvp", []byte(`{
				"internal": {
					"providerDiscoveryData": {
						"storageClasses": [
							{"name": "fast-ssd", "isDefault": true, "allowVolumeExpansion": true},
							{"name": "standard", "isDefault": false, "allowVolumeExpansion": true}
						]
					}
				}
			}`))
			f.RunHook()
		})

		It("Should set cloudProviderDefaultStorageClass to 'fast-ssd'", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("global.discovery.cloudProviderDefaultStorageClass").String()).To(Equal("fast-ssd"))
		})

		It("Should set drift metric with expected and actual labels", func() {
			Expect(f).To(ExecuteSuccessfully())
			m := f.MetricsCollector.CollectedMetrics()
			Expect(m).To(HaveLen(1))
			Expect(m[0].Name).To(Equal("d8_cloud_provider_dvp_default_storage_class_drifted"))
			Expect(*m[0].Value).To(Equal(1.0))
			Expect(m[0].Labels).To(HaveKeyWithValue("expected", "fast-ssd"))
			Expect(m[0].Labels).To(HaveKeyWithValue("actual", "standard"))
		})
	})
})
