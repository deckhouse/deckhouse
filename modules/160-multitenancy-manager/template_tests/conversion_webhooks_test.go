/*
Copyright 2026 Flant JSC

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

package template_tests

import (
	"cmp"
	"maps"
	"path/filepath"
	"runtime"
	"slices"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/webhooks/conversionwebhook"
)

// The ConversionWebhooks of the module, named after their CRDs: the file of webhooks/conversion that
// holds their methods and the from/to pairs of their conversions, in the order the chart lists them.
var multitenancyConversionWebhooks = map[string]struct {
	file        string
	conversions [][2]string
}{
	"clusterresourcegrantpolicies.multitenancy.deckhouse.io": {file: "webhooks/conversion/clusterresourcegrantpolicies.py", conversions: [][2]string{{"v1alpha1", "v1alpha2"}, {"v1alpha2", "v1alpha1"}}},
	"projects.deckhouse.io":                                  {file: "webhooks/conversion/projects.py", conversions: [][2]string{{"v1alpha1", "v1alpha2"}, {"v1alpha2", "v1alpha1"}, {"v1alpha2", "v1alpha3"}, {"v1alpha3", "v1alpha2"}}},
	"projecttemplates.deckhouse.io":                          {file: "webhooks/conversion/projecttemplates.py", conversions: [][2]string{{"v1alpha1", "v1alpha2"}, {"v1alpha2", "v1alpha1"}}},
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: conversion webhooks", func() {
	f := SetupHelmConfig(``)

	_, thisFile, _, _ := runtime.Caller(0)
	moduleDir := filepath.Join(filepath.Dir(thisFile), "..")

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
		f.ValuesSet("multitenancyManager.internal.admissionWebhookCert.ca", "test")
		f.ValuesSet("multitenancyManager.internal.admissionWebhookCert.crt", "test")
		f.ValuesSet("multitenancyManager.internal.admissionWebhookCert.key", "test")
		f.HelmRender()
	})

	It("Should render the conversions and the handler from webhooks/conversion", func() {
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		for name, expected := range multitenancyConversionWebhooks {
			object := f.KubernetesGlobalResource("ConversionWebhook", name)
			Expect(object.Exists()).To(BeTrue(), name)

			var pairs [][2]string
			var conversions []conversionwebhook.Conversion
			for _, item := range object.Field("conversions").Array() {
				pairs = append(pairs, [2]string{item.Get("from").String(), item.Get("to").String()})
				conversions = append(conversions, conversionwebhook.Conversion{
					From:   item.Get("from").String(),
					To:     item.Get("to").String(),
					Python: item.Get("handler.python").String(),
				})
			}
			Expect(pairs).To(Equal(expected.conversions), name)
			Expect(conversionwebhook.HandlerMatchesFile(moduleDir, expected.file, conversions)).To(Succeed(), name)
		}
	})

})

// The values a converge renders the chart with ahead of the release: global.modules.https, the module
// config, and the internal values the hooks left on their previous run, if any.
type valuesAheadOfRelease struct {
	https    string
	config   string
	internal string
}

// A converge applies the ConversionWebhooks of the module ahead of its release (the ModuleEnsureHooks
// task of addon-operator). It renders the whole chart with the values the module has before its hooks
// run, and does not check them against openapi/values.yaml: on the first converge after a Deckhouse
// start, the openapi defaults and the config with no internal values; on a later one, the config with
// the internal values the hooks derived from the previous config. A failed render stops the main
// queue, and the converge with it.
var _ = Describe("Module :: multitenancy-manager :: helm template :: conversion webhooks :: before the hooks fill the internal values", func() {
	f := SetupHelmConfig(``)

	_, thisFile, _, _ := runtime.Caller(0)
	moduleDir := filepath.Join(filepath.Dir(thisFile), "..")

	DescribeTable("Should render the chart and its ConversionWebhooks",
		func(values valuesAheadOfRelease) {
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("global.enabledModules", `["cert-manager", "vertical-pod-autoscaler"]`)
			f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
			f.ValuesSet("global.internal.modules.kubeRBACProxyCA.cert", "test")
			f.ValuesSet("global.internal.modules.kubeRBACProxyCA.key", "test")
			f.ValuesSetFromYaml("global.modules.https", values.https)
			f.ValuesSetFromYaml("multitenancyManager", cmp.Or(values.config, `{}`))
			if values.internal != "" {
				f.ValuesSetFromYaml("multitenancyManager.internal", values.internal)
			}
			f.ApplyOpenAPIDefaults()

			names, err := conversionwebhook.RenderAheadOfRelease(f, moduleDir, "multitenancyManager")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(names).To(ConsistOf(slices.Collect(maps.Keys(multitenancyConversionWebhooks))))
		},
		Entry("with the openapi defaults", valuesAheadOfRelease{
			https: `{mode: Disabled}`,
		}),
		Entry("with https CertManager in the global settings", valuesAheadOfRelease{
			https: `{mode: CertManager, certManager: {clusterIssuerName: letsencrypt}}`,
		}),
		Entry("with https CustomCertificate in the global settings", valuesAheadOfRelease{
			https: `{mode: CustomCertificate, customCertificate: {secretName: custom-certificate}}`,
		}),
		Entry("with allowNamespacesWithoutProjects disabled", valuesAheadOfRelease{
			https:  `{mode: CertManager, certManager: {clusterIssuerName: letsencrypt}}`,
			config: `{allowNamespacesWithoutProjects: false}`,
		}),
		Entry("with highAvailability", valuesAheadOfRelease{
			https:  `{mode: CertManager, certManager: {clusterIssuerName: letsencrypt}}`,
			config: `{highAvailability: true}`,
		}),
		Entry("with the internal values of the previous run of the hooks", valuesAheadOfRelease{
			https: `{mode: CertManager, certManager: {clusterIssuerName: letsencrypt}}`,
			internal: `
admissionWebhookCert: {ca: plainstring, crt: plainstring, key: plainstring}
grantWebhookRules: [{apiGroup: rbac.authorization.k8s.io, resource: rolebindings}]
`,
		}),
	)

})
