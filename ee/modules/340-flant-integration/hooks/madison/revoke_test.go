/*
Copyright 2021 Flant CJSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE
*/

package madison

import (
	"bytes"
	"io/ioutil"
	"net/http"

	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Flant integration :: hooks :: madison revoke ::", func() {
	const (
		initValuesString = `
{
  "global": {
    "project": "test-me"
  },
  "flantIntegration": {
    "madisonAuthKey": "abc"
  }
}`

		initConfigValuesString = `
{
  "flantIntegration": {
    "madisonAuthKey": "abc",
    "licenseKey": "xxx"
  }
}`
	)

	Context("Project is archived", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			buf := bytes.NewBufferString(`{"error": "Archived setup"}`)
			rc := ioutil.NopCloser(buf)
			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					Header:     map[string][]string{"Content-Type": {"application/json"}},
					StatusCode: http.StatusUnauthorized,
					Body:       rc,
				}, nil)

			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		It("values must be absent", func() {
			Expect(f.ConfigValuesGet("flantIntegration.licenseKey").Exists()).To(BeFalse())
			Expect(f.ConfigValuesGet("flantIntegration.madisonAuthKey").Exists()).To(BeFalse())
		})
	})

	Context("Project is active", func() {
		f := HookExecutionConfigInit(initValuesString, initConfigValuesString)

		BeforeEach(func() {
			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					Header:     map[string][]string{"Content-Type": {"application/json"}},
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBuffer(nil)),
				}, nil)

			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
			f.RunHook()
		})

		It("values must be present", func() {
			Expect(f.ConfigValuesGet("flantIntegration.licenseKey").Exists()).To(BeTrue())
			Expect(f.ConfigValuesGet("flantIntegration.madisonAuthKey").Exists()).To(BeTrue())
		})
	})
})
