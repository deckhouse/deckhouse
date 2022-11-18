/*
Copyright 2022 Flant JSC

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

var _ = Describe("Module :: deckhouse-web :: config values conversions :: version 1", func() {
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
			Expect(f.FinalValues.Get("auth.password").Exists()).Should(BeFalse(), "should delete auth.password field")
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
			Expect(f.FinalValues.Get("auth.password").Exists()).Should(BeFalse(), "should delete auth.password field")
		})
	})
})

// Test older values conversion to latest version.
var _ = Describe("Module :: deckhouse-web :: config values conversions :: to latest", func() {
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
			Expect(f.FinalValues.Get("auth.password").Exists()).Should(BeFalse(), "should delete auth.password field")
		})
	})
})
