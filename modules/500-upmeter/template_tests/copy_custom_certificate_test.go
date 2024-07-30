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
	"encoding/base64"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const globalValues = `
clusterConfiguration:
  apiVersion: deckhouse.io/v1
  cloud:
    prefix: myprefix
    provider: OpenStack
  clusterDomain: cluster.local
  clusterType: "Cloud"
  defaultCRI: Containerd
  kind: ClusterConfiguration
  kubernetesVersion: "1.29"
  podSubnetCIDR: 10.111.0.0/16
  podSubnetNodeCIDRPrefix: "24"
  serviceSubnetCIDR: 10.222.0.0/16
enabledModules: ["vertical-pod-autoscaler-crd", "upmeter"]
modules:
  https:
    mode: CustomCertificate
  publicDomainTemplate: "%s.example.com"
  placement: {}
discovery:
  d8SpecificNodeCountByRole:
    system: 1
    master: 1
  kubernetesVersion: 1.16.15
`

const customCertificatePresent = `
auth:
  webui: {}
  status: {}
disabledProbes: []
https:
  mode: CustomCertificate
internal:
  customCertificateData:
    tls.crt: CRTCRTCRT
    tls.key: KEYKEYKEY
  disabledProbes: []
  smokeMini:
    sts:
      a: {}
      b: {}
      c: {}
      d: {}
      e: {}
  auth:
    status:
      password: testP4ssw0rd
    webui:
      password: testP4ssw0rd
smokeMini: { auth: {} }
smokeMiniDisabled: false
statusPageAuthDisabled: false
`

var _ = Describe("Module :: upmeter :: helm template :: custom-certificate", func() {
	f := SetupHelmConfig(``)

	Context("Default", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("upmeter", customCertificatePresent)
			f.HelmRender()
		})

		It("Everything must render properly for default cluster", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			createdSecret := f.KubernetesResource("Secret", "d8-upmeter", "ingress-tls-smoke-mini-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
			createdSecret = f.KubernetesResource("Secret", "d8-upmeter", "ingress-tls-status-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
			createdSecret = f.KubernetesResource("Secret", "d8-upmeter", "ingress-tls-webui-customcertificate")
			Expect(createdSecret.Exists()).To(BeTrue())
			Expect(createdSecret.Field("data").String()).To(Equal(`{"tls.crt":"CRTCRTCRT","tls.key":"KEYKEYKEY"}`))
			createdSecret = f.KubernetesResource("Secret", "d8-upmeter", "basic-auth-status")
			Expect(createdSecret.Exists()).To(BeTrue())
			ensureBasicAuthPassword(createdSecret, "testP4ssw0rd")
			createdSecret = f.KubernetesResource("Secret", "d8-upmeter", "basic-auth-webui")
			Expect(createdSecret.Exists()).To(BeTrue())
			ensureBasicAuthPassword(createdSecret, "testP4ssw0rd")
		})
	})
})

func ensureBasicAuthPassword(secret object_store.KubeObject, pass string) {
	Expect(secret.Field("data").Map()).To(HaveKey("auth"))
	decodedBytes, err := base64.StdEncoding.DecodeString(secret.Field("data.auth").String())
	Expect(err).ShouldNot(HaveOccurred())
	Expect(string(decodedBytes)).To(ContainSubstring(pass))
}
