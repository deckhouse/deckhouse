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
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/deckhouse/deckhouse/go_lib/encoding"
	"github.com/deckhouse/deckhouse/modules/150-user-authn/hooks"
	. "github.com/deckhouse/deckhouse/testing/helm"
)

// consoleClientID is the client id of the console DexAuthenticator. The template, the hook and
// user-authn-controller each build the id; this test and TestDexAuthenticatorClientIDOfTheConsole
// of the controller pin the same value, so the three copies cannot drift apart unnoticed.
const consoleClientID = "dex-authenticator-console@d8-console"

// dexAuthenticatorClientIDCase is one DexAuthenticator whose name and namespace together contain a
// dash: the old id joined them by a dash and gave both one client.
type dexAuthenticatorClientIDCase struct{ name, namespace string }

var dexAuthenticatorClientIDCases = []dexAuthenticatorClientIDCase{{"a", "b-c"}, {"a-b", "c"}}

// Dex finds a client by the name derived from its id, so the name the hook computes from its id
// has to be the name derived from the id the template renders: otherwise Dex stops finding the
// clients of every DexAuthenticator.
var _ = Describe("Module :: user-authn :: helm template :: DexAuthenticator client id", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.29.1")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")

		crds, names := "", ""
		for i, c := range dexAuthenticatorClientIDCases {
			// The values the hook would have written: the encoded name comes from its id.
			crds += fmt.Sprintf(`
- name: %[1]s
  encodedName: %[3]s
  namespace: %[2]s
  credentials:
    appDexSecret: dh-test-secret-0123456789abcdef
    cookieSecret: dh-test-secret-0123456789abcdef
  spec:
    applications:
    - domain: app%[4]d.example.com
      ingressClassName: nginx
`, c.name, c.namespace, encoding.ToFnvLikeDex(hooks.DexAuthenticatorClientID(c.name, c.namespace)), i)
			names += fmt.Sprintf(`
"%[1]s@%[2]s":
  name: "%[1]s-dex-authenticator"
  truncated: false
  hash: ""
  secretName: "dex-authenticator-%[1]s"
  secretTruncated: false
  secretHash: ""
  ingressNames:
    "0":
      name: "%[1]s-dex-authenticator"
      truncated: false
      hash: ""
  signOutIngressNames:
    "0":
      name: "%[1]s-dex-authenticator-sign-out"
      truncated: false
      hash: ""
`, c.name, c.namespace)
		}
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorCRDs", crds)
		hec.ValuesSetFromYaml("userAuthn.internal.dexAuthenticatorNames", names)
		hec.HelmRender()
	})

	It("Should name every OAuth2Client after the id it renders, and give each authenticator its own", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		ids := map[string]bool{}
		for _, c := range dexAuthenticatorClientIDCases {
			id := hooks.DexAuthenticatorClientID(c.name, c.namespace)
			client := hec.KubernetesResource("OAuth2Client", "d8-user-authn", encoding.ToFnvLikeDex(id))
			Expect(client.Exists()).To(BeTrue(), id)
			Expect(client.Field("id").String()).To(Equal(id))
			Expect(encoding.ToFnvLikeDex(client.Field("id").String())).To(Equal(client.Field("metadata.name").String()))

			args := hec.KubernetesResource("Deployment", c.namespace, c.name+"-dex-authenticator").
				Field(`spec.template.spec.containers.#(name=="dex-authenticator").args`).AsStringSlice()
			Expect(args).To(ContainElement("--client-id=" + id))
			ids[id] = true
		}
		Expect(ids).To(HaveLen(len(dexAuthenticatorClientIDCases)))
	})

	It("Should give user-api the id of the console the hook builds", func() {
		Expect(hooks.DexAuthenticatorClientID("console", "d8-console")).To(Equal(consoleClientID))
		args := hec.KubernetesResource("Deployment", "d8-user-authn", "user-api").
			Field(`spec.template.spec.containers.#(name=="user-api").args`).AsStringSlice()
		Expect(args).To(ContainElement("--console-client-id=" + consoleClientID))
	})
})
