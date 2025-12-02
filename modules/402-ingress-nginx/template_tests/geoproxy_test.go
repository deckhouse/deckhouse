/*
Copyright 2025 Flant JSC

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
	"encoding/base64"
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

type (
	mirrorInfo struct {
		URL                string `json:"url"`
		InsecureSkipVerify bool   `json:"insecureSkipVerify"`
	}

	licenseInfo struct {
		AccountID int        `json:"maxmindAccountID"`
		Editions  []string   `json:"editions"`
		Mirror    mirrorInfo `json:"maxmindMirror"`
	}
)

const (
	controllersWithLicenseData = `
- name: public
  spec:
    geoIP2:
      maxmindLicenseKey: shared-license
      maxmindMirror:
        url: https://mirror.shared
        insecureSkipVerify: true
      maxmindEditionIDs:
        - GeoLite2-City
        - GeoLite2-ASN
- name: fallback
  spec:
    geoIP2:
      maxmindLicenseKey: shared-license
      maxmindAccountID: 777
      maxmindEditionIDs:
        - GeoLite2-ASN
        - GeoLite2-Country
- name: second
  spec:
    geoIP2:
      maxmindLicenseKey: another-license
      maxmindEditionIDs:
        - GeoLite2-Country
- name: second-with-account
  spec:
    geoIP2:
      maxmindLicenseKey: another-license
      maxmindMirror:
        url: https://mirror.com
        insecureSkipVerify: false
      maxmindAccountID: 888
      maxmindEditionIDs:
        - GeoLite2-ISP
`

	controllersWithoutLicenseData = `
- name: public
  spec:
    geoIP2:
      maxmindLicenseKey: ""
      maxmindEditionIDs:
        - GeoLite2-City
- name: no-geo
  spec:
    geoIP2: {}
`

	controllersWithMirrorOnly = `
- name: mirror-only
  spec:
    geoIP2:
      maxmindMirror:
        url: https://mirror-only.local
      maxmindEditionIDs:
        - GeoLite2-City
`
)

var _ = Describe("Module :: ingress-nginx :: helm template :: geoproxy helper", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.14")
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
	})

	It("aggregates license data across controllers", func() {
		hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", controllersWithLicenseData)

		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		secret := hec.KubernetesResource("Secret", "d8-ingress-nginx", "geoip-license-editions")
		Expect(secret.Exists()).To(BeTrue(), "geoproxy secret should be rendered when licenses exist")

		licenseMap := decodeLicenseMap(secret)
		Expect(licenseMap).To(HaveLen(2))
		shared := licenseMap["shared-license"]
		Expect(shared.AccountID).To(Equal(777))
		Expect(shared.Editions).To(ConsistOf("GeoLite2-City", "GeoLite2-ASN", "GeoLite2-Country"))
		Expect(shared.Mirror).To(Equal(mirrorInfo{
			URL:                "https://mirror.shared",
			InsecureSkipVerify: true,
		}))

		another := licenseMap["another-license"]
		Expect(another.AccountID).To(Equal(888))
		Expect(another.Editions).To(ConsistOf("GeoLite2-Country", "GeoLite2-ISP"))
		Expect(another.Mirror).To(Equal(mirrorInfo{
			URL:                "https://mirror.com",
			InsecureSkipVerify: false,
		}))
	})

	It("skips geoproxy resources when license data absent", func() {
		hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", controllersWithoutLicenseData)

		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		secret := hec.KubernetesResource("Secret", "d8-ingress-nginx", "geoip-license-editions")
		Expect(secret.Exists()).To(BeFalse())

		statefulSet := hec.KubernetesResource("StatefulSet", "d8-ingress-nginx", "geoproxy")
		Expect(statefulSet.Exists()).To(BeFalse())

		service := hec.KubernetesResource("Service", "d8-ingress-nginx", "geoproxy")
		Expect(service.Exists()).To(BeFalse())
	})

	It("renders geoproxy when only mirror is configured", func() {
		hec.ValuesSetFromYaml("ingressNginx.internal.ingressControllers", controllersWithMirrorOnly)

		hec.HelmRender()
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		statefulSet := hec.KubernetesResource("StatefulSet", "d8-ingress-nginx", "geoproxy")
		Expect(statefulSet.Exists()).To(BeTrue())

		secret := hec.KubernetesResource("Secret", "d8-ingress-nginx", "geoip-license-editions")
		Expect(secret.Exists()).To(BeTrue())
		licenseMap := decodeLicenseMap(secret)
		Expect(licenseMap).To(HaveKey("mirror:e17d2092"))
		mirrorEntry := licenseMap["mirror:e17d2092"]
		Expect(mirrorEntry.Mirror.URL).To(Equal("https://mirror-only.local"))
		Expect(mirrorEntry.AccountID).To(Equal(0))
		Expect(mirrorEntry.Editions).To(ConsistOf("GeoLite2-City"))
	})
})

func decodeLicenseMap(secret object_store.KubeObject) map[string]licenseInfo {
	encoded := secret.Field(`data.license_editions\.json`).String()
	Expect(encoded).NotTo(BeEmpty())

	raw, err := base64.StdEncoding.DecodeString(encoded)
	Expect(err).NotTo(HaveOccurred())

	var result map[string]licenseInfo
	Expect(json.Unmarshal(raw, &result)).NotTo(HaveOccurred())

	return result
}
