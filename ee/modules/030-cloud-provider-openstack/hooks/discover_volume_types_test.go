/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-provider-openstack :: hooks :: discover_volume_types ::", func() {
	const (
		initValuesStringA = `
cloudProviderOpenstack:
  internal:
    connection:
      authURL: https://test.tests.com:5000/v3/
      domainName: default
      username: jamie
      password: nein
      region: HetznerFinland
`
		initValuesStringB = `
cloudProviderOpenstack:
  internal:
    connection:
      authURL: https://test.tests.com:5000/v3/
      domainName: default
      username: jamie
      password: nein
      region: HetznerFinland
  storageClass:
    exclude:
    - .*-foo
    - bar
    default: other-bar
`
	)

	f := HookExecutionConfigInit(initValuesStringA, `{}`)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should discover all volumeTypes and no default", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet("cloudProviderOpenstack.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "bar",
	"type": "bar"
  },
  {
	"name": "default",
	"type": "__DEFAULT__"
  },
  {
	"name": "other-bar",
	"type": "other-bar"
  },
  {
	"name": "some-foo",
	"type": "some-foo"
  },
  {
	"name": "ssd-r1",
	"type": "SSD R1"
  }
]
`))
			Expect(f.ValuesGet("cloudProviderOpenstack.internal.defaultStorageClass").Exists()).To(BeFalse())
		})

	})

	b := HookExecutionConfigInit(initValuesStringB, `{}`)

	Context("Cluster has minimal cloudProviderOpenstack configuration with excluded storage classes and default specified", func() {
		BeforeEach(func() {
			b.BindingContexts.Set(b.GenerateBeforeHelmContext())
			b.RunHook()
		})

		It("Should discover volumeTypes without excluded and default set", func() {
			Expect(b).To(ExecuteSuccessfully())
			Expect(b.ValuesGet("cloudProviderOpenstack.internal.storageClasses").String()).To(MatchJSON(`
[
  {
	"name": "default",
	"type": "__DEFAULT__"
  },
  {
	"name": "other-bar",
	"type": "other-bar"
  },
  {
	"name": "ssd-r1",
	"type": "SSD R1"
  }
]
`))
			Expect(b.ValuesGet("cloudProviderOpenstack.internal.defaultStorageClass").String()).To(Equal(`other-bar`))
		})
	})
})
