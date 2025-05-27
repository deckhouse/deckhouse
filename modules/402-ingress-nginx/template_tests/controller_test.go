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
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// generate golden files with: `make FOCUS=ingress-nginx GOLDEN=true tests-modules`
var golden bool

func init() {
	if os.Getenv("GOLDEN") == "" {
		return
	}
	golden, _ = strconv.ParseBool(os.Getenv("GOLDEN"))
}

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: ingress-nginx :: helm template :: controllers ", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.14")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.deckhouse.io/deckhouse/fe")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("ingressNginx.defaultControllerVersion", "1.9")

		hec.ValuesSet("ingressNginx.internal.admissionCertificate.ca", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.cert", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.key", "test")
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.namespaces", json.RawMessage("[]"))
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.ingresses", json.RawMessage("[]"))
	})

	table.DescribeTable("Render IngressNginx controllers",
		func(fileName string) {
			var ctrl ingressNginxController

			data, err := os.ReadFile("testdata/" + fileName)
			Expect(err).ShouldNot(HaveOccurred())
			// read yaml from fileName
			err = yaml.Unmarshal(data, &ctrl)
			Expect(err).ShouldNot(HaveOccurred())

			controllerSpec, _ := yaml.Marshal(ctrl)

			cert := fmt.Sprintf(`
- controllerName: %s
  ingressClass: nginx
  data:
    cert: teststring
    key: teststring
`, ctrl.Name)

			hec.ValuesSetFromYaml("ingressNginx.internal.nginxAuthTLS", cert)
			hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllers.0", string(controllerSpec))
			out := make(map[string]string)
			hec.HelmRender(WithFilteredRenderOutput(out, []string{"ingress-nginx/templates/controller/", "ingress-nginx/templates/failover/"}))
			testD := hec.KubernetesResource("DaemonSet", "d8-ingress-nginx", "controller-"+ctrl.Name)
			Expect(testD.Exists()).To(BeTrue())
			goldenDir := filepath.Join("testdata", "golden", strings.TrimSuffix(fileName, filepath.Ext(fileName)))

			for fn, content := range out {
				renderedFile := filepath.Base(fn)
				if strings.HasPrefix(fn, "ingress-nginx/templates/failover/") {
					// skip pod monitors
					if strings.HasSuffix(fn, "podmonitor.yaml") {
						continue
					}
					if len(content) == 0 {
						continue
					}

					renderedFile = filepath.Join("failover", renderedFile)
				} else if strings.HasPrefix(fn, "ingress-nginx/templates/controller/") {
					if strings.HasSuffix(fn, "fake-ingress.yaml") {
						continue
					}
					renderedFile = filepath.Join("controller", renderedFile)
				} else {
					continue
				}
				filePath := filepath.Join(goldenDir, renderedFile)

				if golden {
					Expect(os.MkdirAll(filepath.Dir(filePath), os.ModePerm)).To(Succeed())
					By("writing golden file " + filePath)
					Expect(os.WriteFile(filePath, []byte(content), 0o644)).To(Succeed())
				} else {
					By("reading golden file " + filePath)
					goldenContent, err := os.ReadFile(filePath)
					Expect(err).ShouldNot(HaveOccurred())
					Expect(content).Should(MatchYAML(string(goldenContent)))
				}
			}
		},

		table.Entry("HostPortWithProxyProtocol inlet", "host-port-with-pp.yaml"),
		table.Entry("HostWithFailover inlet with custom resources and filter IP with acceptRequestsFrom", "host-with-failover.yaml"),
		table.Entry("LoadBalancer inlet", "lb.yaml"),
		table.Entry("LoadBalancerWithProxyProtocol inlet", "lb-with-pp.yaml"),
		table.Entry("LoadBalancer inlet with custom terminating time", "lb-with-terminating.yaml"),
		table.Entry("LoadBalancer without hpa deployment", "lb-without-hpa.yaml"),
	)
})

type ingressNginxController struct {
	Name string          `json:"name"`
	Spec json.RawMessage `json:"spec"`
}

// need to adopt IngressNginxController object to the internal values structure
func (ing *ingressNginxController) UnmarshalJSON(data []byte) error {
	s := struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Spec json.RawMessage `json:"spec"`
	}{}

	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	ing.Name = s.Metadata.Name
	ing.Spec = s.Spec
	return nil
}
