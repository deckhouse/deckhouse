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

var _ = Describe("Flant integration :: hooks :: madison registration ::", func() {
	const (
		initValuesString = `
{
  "global": {
  },
  "flantIntegration": {
    "internal": {
      "licenseKey": "xxx"
    }
  }
}`
		initValuesStringMadisonKeyExists = `
{
  "global": {
  },
  "flantIntegration": {
    "madisonAuthKey": "abc",
    "internal": {
      "licenseKey": "xxx"
    }
  }
}`
	)

	Context("Madison Auth key exists", func() {
		f := HookExecutionConfigInit(initValuesStringMadisonKeyExists, ``)
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Madison Auth key doesn't exist", func() {
		f := HookExecutionConfigInit(initValuesString, ``)

		BeforeEach(func() {
			buf := bytes.NewBufferString(`{"error": "", "auth_key":"cde"}`)
			rc := ioutil.NopCloser(buf)
			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					Header:     map[string][]string{"Content-Type": {"application/json"}},
					StatusCode: http.StatusOK,
					Body:       rc,
				}, nil)

			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("values must be present", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ConfigValuesGet("flantIntegration.madisonAuthKey").String()).To(Equal("cde"))
		})
	})
})
