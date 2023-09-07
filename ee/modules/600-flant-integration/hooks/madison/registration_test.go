/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package madison

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"

	"github.com/flant/addon-operator/pkg/module_manager/go_hook"
	_ "github.com/flant/addon-operator/sdk"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Flant integration :: hooks :: madison registration ::", func() {
	const (
		madisonTestAuthKey    = "abc"
		valuesWithLicenseOnly = `
{
  "global": {
  },
  "flantIntegration": {
    "internal": {
      "licenseKey": "xxx"
    }
  }
}`
		valuesWithAuthKey = `
{
  "global": {
  },
  "flantIntegration": {
    "internal": {
      "madisonAuthKey": "` + madisonTestAuthKey + `",
      "licenseKey": "xxx"
    }
  }
}`
	)

	var (
		madisonNS = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: ` + madisonSecretNS + `
`
		madisonAuthKeyB64 = base64.StdEncoding.EncodeToString([]byte(madisonTestAuthKey))
		madisonSecret     = `
---
apiVersion: v1
kind: Secret
metadata:
  name: ` + madisonSecretName + `
  namespace: ` + madisonSecretNS + `
data:
  ` + madisonSecretField + `: |
    ` + madisonAuthKeyB64
	)

	Context("No license key in internal values", func() {
		f := HookExecutionConfigInit(`{"global": {}, "flantIntegration": {"internal": {}} }`, `{"flantIntegration":{}}`)

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.ConfigValuesSet(madisonKeyPath, madisonTestAuthKey)
			f.ValuesSet(internalMadisonKeyPath, madisonTestAuthKey)
			f.RunHook()
		})

		It("should remove auth key internal values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(internalMadisonKeyPath).String()).To(BeEmpty())
		})
	})

	Context("Madison Auth key in config values", func() {
		f := HookExecutionConfigInit(valuesWithLicenseOnly, fmt.Sprintf(`{"flantIntegration":{"madisonAuthKey":"%s"}}`, madisonTestAuthKey))

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should set auth key from config values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(internalMadisonKeyPath).String()).To(Equal(madisonTestAuthKey))
		})
	})

	Context("Madison Auth key stored in Secret", func() {
		f := HookExecutionConfigInit(valuesWithLicenseOnly, ``)

		BeforeEach(func() {
			f.KubeStateSet(madisonNS + madisonSecret)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should set auth key from Secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(internalMadisonKeyPath).String()).To(Equal(madisonTestAuthKey))
		})
	})

	Context("Madison Auth key already in internal values", func() {
		f := HookExecutionConfigInit(valuesWithAuthKey, ``)

		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should keep auth key in values", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(internalMadisonKeyPath).String()).To(Equal(madisonTestAuthKey))
		})
	})

	Context("Madison Auth key doesn't exist", func() {
		f := HookExecutionConfigInit(valuesWithLicenseOnly, ``)

		BeforeEach(func() {
			// Mock HTTP client to emulate registration.
			buf := bytes.NewBufferString(fmt.Sprintf(`{"error": "", "auth_key":"%s"}`, madisonTestAuthKey))
			rc := io.NopCloser(buf)
			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					Header:     map[string][]string{"Content-Type": {"application/json"}},
					StatusCode: http.StatusOK,
					Body:       rc,
				}, nil)

			f.KubeStateSet(madisonNS)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("should register new auth key", func() {
			Expect(f).To(ExecuteSuccessfully())
			Expect(f.ValuesGet(internalMadisonKeyPath).String()).To(Equal(madisonTestAuthKey))
		})
	})

	Context("Connect request failed", func() {
		f := HookExecutionConfigInit(valuesWithLicenseOnly, ``)

		BeforeEach(func() {
			buf := bytes.NewBufferString(fmt.Sprintf(`{"error": "foobar", "auth_key":"%s"}`, madisonTestAuthKey))
			rc := io.NopCloser(buf)
			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					Header:     map[string][]string{"Content-Type": {"application/json"}},
					Status:     "Internal Server Error",
					StatusCode: http.StatusInternalServerError,
					Body:       rc,
				}, nil)

			f.KubeStateSet(madisonNS)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should fail with error message", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("500 Internal Server Error: foobar"))
		})
	})

	Context("Connect request failed without error message", func() {
		f := HookExecutionConfigInit(valuesWithLicenseOnly, ``)

		BeforeEach(func() {
			buf := bytes.NewBufferString(`{"message":"Unauthorized"}`)
			rc := io.NopCloser(buf)
			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					Header:     map[string][]string{"Content-Type": {"application/json"}},
					Status:     "Unauthorized",
					StatusCode: http.StatusUnauthorized,
					Body:       rc,
				}, nil)

			f.KubeStateSet(madisonNS)
			f.BindingContexts.Set(f.GenerateBeforeHelmContext())
			f.RunHook()
		})

		It("Should fail with error message", func() {
			Expect(f).ToNot(ExecuteSuccessfully())
			Expect(f.GoHookError.Error()).To(ContainSubstring("401 Unauthorized"))
		})
	})

	Context("createMadisonPayload", func() {
		table.DescribeTable("createMadisonPayload",
			func(domainTemplate string, schema string, want madisonRequestData) {
				p := createMadisonPayload(domainTemplate, schema)
				Expect(p).To(Equal(want))
			},
			table.Entry(
				"empty input, schema ignored",
				"",
				"http",
				madisonRequestData{GrafanaURL: "-", PrometheusURL: "-"},
			),
			table.Entry(
				"template available and http",
				"%s.one.two",
				"http",
				madisonRequestData{
					Type:          "prometheus",
					GrafanaURL:    "http://grafana.one.two",
					PrometheusURL: "http://grafana.one.two/prometheus",
				},
			),
			table.Entry(
				"template available and https",
				"%s.one.two",
				"https",
				madisonRequestData{
					Type:          "prometheus",
					GrafanaURL:    "https://grafana.one.two",
					PrometheusURL: "https://grafana.one.two/prometheus",
				},
			),
		)
	})

	Context("getPrometheusURLSchema", func() {

		table.DescribeTable("getPrometheusURLSchema",
			func(input *go_hook.HookInput, want string) {
				p := getPrometheusURLSchema(input)
				Expect(p).To(Equal(want))
			},
			table.Entry(
				"an empty snapshot",
				&go_hook.HookInput{Snapshots: go_hook.Snapshots{prometheusSecretBinding: []go_hook.FilterResult{}}},
				"http",
			),
			table.Entry(
				"a snapshot with http",
				&go_hook.HookInput{Snapshots: go_hook.Snapshots{prometheusSecretBinding: []go_hook.FilterResult{"http"}}},
				"http",
			),
			table.Entry(
				"a snapshot with https",
				&go_hook.HookInput{Snapshots: go_hook.Snapshots{prometheusSecretBinding: []go_hook.FilterResult{"https"}}},
				"https",
			),
		)
	})
})
