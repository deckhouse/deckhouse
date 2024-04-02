/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const ingressNginxHelmlibSymlink = "/deckhouse/ee/modules/402-ingress-nginx/charts"
const ingressNginxHelmlibPath = "/deckhouse/modules/402-ingress-nginx/charts"
const ingressNginxChartSymlink = "/deckhouse/ee/modules/402-ingress-nginx/Chart.yaml"
const ingressNginxChartPath = "/deckhouse/modules/402-ingress-nginx/Chart.yaml"

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: ingress-nginx :: helm template :: controllers ", func() {
	hec := SetupHelmConfig("")

	BeforeSuite(func() {
		err := os.Symlink(ingressNginxHelmlibPath, ingressNginxHelmlibSymlink)
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink(ingressNginxChartPath, ingressNginxChartSymlink)
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove(ingressNginxHelmlibSymlink)
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Remove(ingressNginxChartSymlink)
		Expect(err).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.21.0")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.deckhouse.io/deckhouse/fe")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd", "operator-prometheus-crd"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("ingressNginx.internal.admissionCertificate.ca", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.cert", "test")
		hec.ValuesSet("ingressNginx.internal.admissionCertificate.key", "test")
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.namespaces", json.RawMessage("[]"))
		hec.ValuesSet("ingressNginx.internal.discardMetricResources.ingresses", json.RawMessage("[]"))
	})
	Context("With ingress nginx controller in values", func() {
		BeforeEach(func() {
			var certificates string
			for _, ingressName := range []string{"test", "test-lbwpp", "test-next", "solid"} {
				certificates += fmt.Sprintf(`
- controllerName: %s
  ingressClass: nginx
  data:
    cert: teststring
    key: teststring
`, ingressName)
			}
			hec.ValuesSetFromYaml("ingressNginx.internal.nginxAuthTLS", certificates)

			hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", `
- name: test
  spec:
    inlet: L2LoadBalancer
    ingressClass: nginx
    controllerVersion: "1.1"
    l2LoadBalancer:
      addressPool: "mypool"
      nodeSelector:
        role: worker
      sourceRanges:
      - 1.1.1.1
`)
			hec.HelmRender()
		})
		It("Should add desired objects", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			testD := hec.KubernetesResource("L2LoadBalancer", "d8-ingress-nginx", "test")
			Expect(testD.Exists()).To(BeTrue())
			Expect(testD.Field("spec.service").String()).To(MatchYAML(`
externalTrafficPolicy: Local
labelSelector:
  app: controller
  name: test
ports:
- name: http
  port: 80
  targetPort: 80
  protocol: TCP
- name: https
  port: 443
  targetPort: 443
  protocol: TCP
sourceRanges:
- 1.1.1.1
`))
		})
	})
})
