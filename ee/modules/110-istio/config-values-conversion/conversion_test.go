/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package config_values_conversion

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/conversion"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: istio :: config values conversions :: version 1", func() {
	f := SetupConverter(``)

	const migratedValues = `
auth:
  allowedUserGroups:
  - admin
`
	Context("giving already migrated values in ConfigMap", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml(".", migratedValues)
			f.Convert(1)
		})

		It("should convert", func() {
			Expect(f.Error).ShouldNot(HaveOccurred())
			Expect(f.FinalVersion).Should(Equal(2))
			Expect(f.FinalValues.Get("auth.password").String()).Should(BeEmpty())
		})
	})

	const nonMigratedValues = `
auth:
  password: Long-password-value
  allowedUserGroups:
  - admin
`
	Context("giving non-migrated values in ConfigMap", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml(".", nonMigratedValues)
			f.Convert(1)
		})

		It("should convert to latest version", func() {
			Expect(f.Error).ShouldNot(HaveOccurred())
			Expect(f.FinalVersion).Should(Equal(2))
			Expect(f.FinalValues.Get("auth.password").String()).Should(BeEmpty())
		})
	})
})

// Test older values conversion to latest version.
var _ = Describe("Module :: istio :: config values conversions :: to latest", func() {
	f := SetupConverter(``)

	Context("giving values of version 1", func() {
		const v1Values = `
auth:
  password: Long-password-value
  allowedUserGroups:
  - admin
`

		BeforeEach(func() {
			f.ValuesSetFromYaml(".", v1Values)
			f.ConvertToLatest(1)
		})

		It("should convert", func() {
			Expect(f.Error).ShouldNot(HaveOccurred())
			Expect(f.FinalVersion).Should(Equal(2))
			Expect(f.FinalValues.Get("auth.password").String()).Should(BeEmpty())
		})
	})
})
