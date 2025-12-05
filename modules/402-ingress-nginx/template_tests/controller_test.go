/*
Copyright 2021 Flant JSC

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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// Set to true to update golden files with: `make FOCUS=ingress-nginx CGO_ENABLED=1 GOLDEN=true tests-modules`
var (
	golden             bool
	manifestsDelimiter = regexp.MustCompile("(?m)^---$")
)

func init() {
	if env := os.Getenv("GOLDEN"); env != "" {
		golden, _ = strconv.ParseBool(env)
	}
	format.TruncatedDiff = false
	format.MaxLength = 0
}

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: ingress-nginx :: helm template :: controllers", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.30.14")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.deckhouse.io/deckhouse/fe")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus", "control-plane-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("ingressNginx.defaultControllerVersion", "1.9")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.ca", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.cert", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.key", "test")
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.namespaces", json.RawMessage("[]"))
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.ingresses", json.RawMessage("[]"))
		hec.ValuesSet("ingressNginx.internal.geoproxyReady", true)
	})

	table.DescribeTable("Render IngressNginx controllers",
		func(fileName string) {
			var ctrl ingressNginxController

			// Load YAML definition
			data, err := os.ReadFile(filepath.Join("testdata", fileName))
			Expect(err).ShouldNot(HaveOccurred())

			if strings.HasSuffix(fileName, "with-istio.yaml") {
				hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus", "control-plane-manager", "istio"})
			}

			err = yaml.Unmarshal(data, &ctrl)
			Expect(err).ShouldNot(HaveOccurred())

			controllerSpecYAML, _ := yaml.Marshal(ctrl)

			// Set TLS certs
			cert := fmt.Sprintf(`
- controllerName: %s
  ingressClass: nginx
  data:
    cert: teststring
    key: teststring
`, ctrl.Name)
			hec.ValuesSetFromYaml("ingressNginx.internal.nginxAuthTLS", cert)
			hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllers.0", string(controllerSpecYAML))

			// Render templates
			rendered := make(map[string]string)
			hec.HelmRender(WithFilteredRenderOutput(rendered, []string{
				"ingress-nginx/templates/controller/",
				"ingress-nginx/templates/failover/",
			}))
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			// Assert DaemonSet exists
			daemonSet := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-"+ctrl.Name)
			Expect(daemonSet.Exists()).To(BeTrue())

			// Compare with golden files
			goldenDir := filepath.Join("testdata", "golden", strings.TrimSuffix(fileName, filepath.Ext(fileName)))
			for path, content := range rendered {
				var renderedFile string

				switch {
				case strings.HasPrefix(path, "ingress-nginx/templates/failover/"):
					if strings.HasSuffix(path, "podmonitor.yaml") || len(content) == 0 {
						continue
					}
					renderedFile = filepath.Join("failover", filepath.Base(path))

				case strings.HasPrefix(path, "ingress-nginx/templates/controller/"):
					if strings.HasSuffix(path, "fake-ingress.yaml") {
						continue
					}
					renderedFile = filepath.Join("controller", filepath.Base(path))

				default:
					continue
				}

				filePath := filepath.Join(goldenDir, renderedFile)

				if golden {
					Expect(os.MkdirAll(filepath.Dir(filePath), os.ModePerm)).To(Succeed())
					By("Writing golden file: " + filePath)
					Expect(os.WriteFile(filePath, []byte(content), 0o644)).To(Succeed())
				} else {
					By("Reading golden file: " + filePath)
					expectedContent, err := os.ReadFile(filePath)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(content).Should(MatchYAML(expectedContent))

					exp := splitManifests(expectedContent)
					got := splitManifests([]byte(content))
					Expect(got).To(HaveLen(len(exp)))

					for i := range got {
						Expect(got[i]).Should(MatchYAML(exp[i]))
					}
				}
			}
		},

		// Test cases
		table.Entry("HostPortWithProxyProtocol inlet", "host-port-with-pp.yaml"),
		table.Entry("HostWithFailover inlet with custom resources and filter IP with acceptRequestsFrom", "host-with-failover.yaml"),
		table.Entry("LoadBalancer inlet", "lb.yaml"),
		table.Entry("LoadBalancerWithProxyProtocol inlet", "lb-with-pp.yaml"),
		table.Entry("LoadBalancer inlet with custom terminating time", "lb-with-terminating.yaml"),
		table.Entry("LoadBalancer without hpa deployment", "lb-without-hpa.yaml"),
		table.Entry("LoadBalancer inlet with istio", "lb-with-istio.yaml"),
		table.Entry("LoadBalancer inlet with hide-headers", "lb-with-hide-headers.yaml"),
		table.Entry("LoadBalancer inlet with hide-headers and istio", "lb-with-hide-headers-and-with-istio.yaml"),
		table.Entry("LoadBalancer inlet with hide-headers and envoy header added", "lb-with-hide-headers-and-envoy-header-added.yaml"),
		table.Entry("LoadBalancer inlet with hide-headers and envoy header added and istio", "lb-with-hide-headers-and-envoy-header-added-and-with-istio.yaml"),
	)
})

// ingressNginxController holds simplified structure to extract controller spec
type ingressNginxController struct {
	Name string          `json:"name"`
	Spec json.RawMessage `json:"spec"`
}

func (ing *ingressNginxController) UnmarshalJSON(data []byte) error {
	aux := struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec json.RawMessage `json:"spec"`
	}{}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	ing.Name = aux.Metadata.Name
	ing.Spec = aux.Spec
	return nil
}

func splitManifests(doc []byte) []string {
	splits := manifestsDelimiter.Split(string(doc), -1)

	result := make([]string, 0, len(splits))
	for i := range splits {
		if splits[i] != "" {
			result = append(result, splits[i])
		}
	}

	return result
}
