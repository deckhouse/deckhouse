/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
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
		testMadisonAuthKey = "abc"
		testLicenseKey     = "license"
	)

	f := HookExecutionConfigInit(
		`{ "global": {}, "flantIntegration": {"internal": {}} }`,
		`{"flantIntegration": {}}`)

	Context("project is archived", func() {
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

			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
		})

		Context("with license and madison key", func() {
			BeforeEach(func() {
				f.ValuesSet(internalLicenseKeyPath, testLicenseKey)
				f.ValuesSet(internalMadisonKeyPath, testMadisonAuthKey)
				f.RunHook()
			})

			It("should remove internal values and create ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.ValuesGet(internalLicenseKeyPath).String()).To(BeEmpty())
				Expect(f.ValuesGet(internalMadisonKeyPath).String()).To(BeEmpty())

				Expect(f.KubernetesResource("ConfigMap", revokedCMNamespace, revokedCMName).Exists()).To(BeTrue())
			})
		})

		Context("without license", func() {
			BeforeEach(func() {
				f.RunHook()
			})

			It("should not remove internal values or create ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("ConfigMap", revokedCMNamespace, revokedCMName).Exists()).To(BeFalse())
			})
		})
	})

	Context("project is active", func() {
		BeforeEach(func() {
			dependency.TestDC.HTTPClient.DoMock.
				Expect(&http.Request{}).
				Return(&http.Response{
					Header:     map[string][]string{"Content-Type": {"application/json"}},
					StatusCode: http.StatusOK,
					Body:       ioutil.NopCloser(bytes.NewBuffer(nil)),
				}, nil)

			f.KubeStateSet("")
			f.BindingContexts.Set(f.GenerateScheduleContext("*/5 * * * *"))
		})

		Context("no license key", func() {
			BeforeEach(func() {
				f.RunHook()
			})

			It("should not create ConfigMap", func() {
				Expect(f).To(ExecuteSuccessfully())
				Expect(f.KubernetesResource("ConfigMap", revokedCMNamespace, revokedCMName).Exists()).To(BeFalse())
			})
		})

		Context("with license key", func() {
			BeforeEach(func() {
				f.ValuesSet(internalLicenseKeyPath, "license")
			})

			When("madison is disabled", func() {
				BeforeEach(func() {
					f.ConfigValuesSet(madisonKeyPath, "false")
					f.RunHook()
				})

				It("should not create ConfigMap", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.KubernetesResource("ConfigMap", revokedCMNamespace, revokedCMName).Exists()).To(BeFalse())
					Expect(f.ConfigValuesGet(madisonKeyPath).Exists()).To(BeTrue())
				})
			})

			When("madison is disabled with boolean", func() {
				BeforeEach(func() {
					f.ConfigValuesSet(madisonKeyPath, false)
					f.RunHook()
				})

				It("should not create ConfigMap", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.KubernetesResource("ConfigMap", revokedCMNamespace, revokedCMName).Exists()).To(BeFalse())
					Expect(f.ConfigValuesGet(madisonKeyPath).Exists()).To(BeTrue())
				})
			})

			When("madison is disabled in internal values", func() {
				BeforeEach(func() {
					f.ValuesSet(internalMadisonKeyPath, "false")
					f.RunHook()
				})

				It("should not create ConfigMap", func() {
					Expect(f).To(ExecuteSuccessfully())
					Expect(f.KubernetesResource("ConfigMap", revokedCMNamespace, revokedCMName).Exists()).To(BeFalse())
					Expect(f.ValuesGet(internalMadisonKeyPath).Exists()).To(BeTrue())
				})
			})
		})
	})
})
