/*
Copyright 2022 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package settings_conversion

import (
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/conversion"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: istio :: config values conversions :: version 1", func() {
	ct := SetupConversionTester()

	Context("giving settings in version 1", func() {
		table.DescribeTable("should convert from 1 to 2",
			ct.TestConversionToNextVersion(1, 2),
			table.Entry("giving empty settings", ``, ``),
			table.Entry("giving empty conversion result", `
auth:
  password: Long-password-value
`, ``),
			table.Entry("giving settings with auth.password",
				`
auth:
  password: Long-password-value
  allowedUserGroups:
  - admin
`,
				`
auth:
  allowedUserGroups:
  - admin
`,
			),
			table.Entry("giving settings without auth.password",
				`auth:
  allowedUserGroups:
  - admin
`,
				`auth:
  allowedUserGroups:
  - admin
`,
			))

		table.DescribeTable("should convert from 1 to valid latest",
			ct.TestConversionToValidLatest(1),
			table.Entry("giving empty conversion result", `
auth:
  password: Long-password-value
`),
			table.Entry("giving settings with auth.password",
				`
auth:
  password: Long-password-value
  allowedUserGroups:
  - admin
`,
			),
			table.Entry("giving settings without auth.password",
				`
auth:
  allowedUserGroups:
  - admin
`,
			))
	})
})
