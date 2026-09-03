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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// externalAuthentication URLs come from ModuleConfig and are rendered into nginx ingress
// annotations. They must be quoted, so a value carrying a newline and a "---" separator
// stays inside its scalar instead of adding documents to the module release.
var _ = Describe("Module :: upmeter :: helm template :: yaml injection", func() {
	f := SetupHelmConfig(``)

	Context("Payloads in externalAuthentication URLs", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("upmeter", `
auth:
  status:
    externalAuthentication:
      authURL: |-
        https://api.example.com/auth
        ---
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: pwned-status-auth-url
          namespace: d8-upmeter
      authSignInURL: |-
        https://www.example.com/login
        ---
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: pwned-status-signin-url
          namespace: d8-upmeter
  webui:
    externalAuthentication:
      authURL: |-
        https://api.example.com/auth
        ---
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: pwned-webui-auth-url
          namespace: d8-upmeter
      authSignInURL: |-
        https://www.example.com/login
        ---
        apiVersion: v1
        kind: ConfigMap
        metadata:
          name: pwned-webui-signin-url
          namespace: d8-upmeter
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
`)
			f.HelmRender()
		})

		It("Must render and must not produce injected documents", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("Ingress", "d8-upmeter", "webui").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Ingress", "d8-upmeter", "status").Exists()).To(BeTrue())

			for _, name := range []string{
				"pwned-status-auth-url",
				"pwned-status-signin-url",
				"pwned-webui-auth-url",
				"pwned-webui-signin-url",
			} {
				Expect(f.KubernetesResource("ConfigMap", "d8-upmeter", name).Exists()).To(BeFalse(), name)
			}
		})
	})
})
