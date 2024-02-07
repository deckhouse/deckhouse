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

var _ = Describe("Module :: upmeter :: config values conversions :: version 1", func() {
	ct := SetupConversionTester()

	Context("giving settings in version 1", func() {
		table.DescribeTable("should convert from 1 to 2",
			ct.TestConversionToNextVersion(1, 2),
			table.Entry("giving empty settings", ``, ``),
			table.Entry("giving empty conversion result", `
auth:
  webui:
    password: Long-password-value
`, ``),
			table.Entry("giving empty conversion result", `
auth:
  webui:
    password: Long-password-value
  status:
    password: Long-password-value
`, ``),
			table.Entry("giving non-migrated settings with auth.password",
				`
auth:
  webui:
    password: Long-password-value
    allowedUserGroups:
    - admin
  status:
    password: Long-password-value
    allowedUserGroups:
    - admin
`,
				`
auth:
  webui:
    allowedUserGroups:
    - admin
  status:
    allowedUserGroups:
    - admin
`,
			),
			table.Entry("giving migrated settings without auth.password",
				`
auth:
  webui:
    allowedUserGroups:
    - admin
`,
				`
auth:
  webui:
    allowedUserGroups:
    - admin
`,
			))

		table.DescribeTable("should convert from 1 to valid latest",
			ct.TestConversionToValidLatest(1),
			table.Entry("giving empty conversion result", `
auth:
  webui:
    password: Long-password-value
`),
			table.Entry("giving empty conversion result", `
auth:
  webui:
    password: Long-password-value
  status:
    password: Long-password-value
`),
			table.Entry("giving non-migrated settings with auth.password",
				`
auth:
  webui:
    password: Long-password-value
    allowedUserGroups:
    - admin
  status:
    password: Long-password-value
    allowedUserGroups:
    - admin
`),
			table.Entry("giving migrated settings without auth.password",
				`
auth:
  webui:
    allowedUserGroups:
    - admin
`))
	})
})
