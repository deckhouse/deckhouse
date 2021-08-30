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
	"github.com/onsi/ginkgo/extensions/table"
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

	Context("createMadisonPayload", func() {
		schemaMock := func(schema string) func() (string, error) {
			return func() (string, error) { return schema, nil }
		}

		table.DescribeTable("createMadisonPayload",
			func(domainTemplate string, schemaMock func() (string, error), want madisonRequestData) {
				p, err := createMadisonPayload(domainTemplate, schemaMock)

				Expect(err).NotTo(HaveOccurred())
				Expect(p).To(Equal(want))
			},
			table.Entry(
				"empty input, schema ignored",
				"",
				schemaMock("http"),
				madisonRequestData{GrafanaURL: "-", PrometheusURL: "-"},
			),
			table.Entry(
				"template available and http",
				"%s.one.two",
				schemaMock("http"),
				madisonRequestData{
					GrafanaURL:    "http://grafana.one.two",
					PrometheusURL: "http://grafana.one.two/prometheus",
				},
			),
			table.Entry(
				"template available and https",
				"%s.one.two",
				schemaMock("https"),
				madisonRequestData{
					GrafanaURL:    "https://grafana.one.two",
					PrometheusURL: "https://grafana.one.two/prometheus",
				},
			),
		)
	})

	Context("calculatePromentheusURLSchema", func() {
		table.DescribeTable("calculation of promentheus URL schema from values",
			func(globalMode, promMode, want string) {
				schema := calculatePromentheusURLSchema(globalMode, promMode)
				Expect(schema).To(Equal(want))
			},
			table.Entry("empty inputs", "", "", "https"),
			table.Entry("globally disabled", "Disabled", "", "http"),
			table.Entry("disabled for prom", "", "Disabled", "http"),
			table.Entry("both disabled", "Disabled", "Disabled", "http"),
		)
	})
})
