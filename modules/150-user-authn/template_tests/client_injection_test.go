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
	"sort"

	"github.com/flant/kube-client/manifest/releaseutil"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

// injectedClusterRoleBinding is a YAML document appended to a value taken from a DexClient or a
// DexAuthenticator. A line break returns the rendered output to column 0, so unless the value is
// emitted as a quoted scalar the remaining lines become an extra manifest of the release.
const injectedClusterRoleBinding = "\n" +
	"---\n" +
	"apiVersion: rbac.authorization.k8s.io/v1\n" +
	"kind: ClusterRoleBinding\n" +
	"metadata:\n" +
	"  name: injected-by-a-tenant\n" +
	"roleRef:\n" +
	"  apiGroup: rbac.authorization.k8s.io\n" +
	"  kind: ClusterRole\n" +
	"  name: injected-role\n" +
	"subjects: []"

const injectedClusterRoleBindingName = "injected-by-a-tenant"

// hostileValue carries a legitimate-looking value followed by the injected document.
const hostileValue = "tenant@example.test" + injectedClusterRoleBinding

var _ = Describe("Module :: user-authn :: helm template :: DexClient value injection", func() {
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
	})

	// Every list field of the DexClient spec is rendered twice: once for the current OAuth2Client
	// and once for the legacy one below it, at differing indentation.
	fields := []string{"trustedPeers", "redirectURIs", "allowedEmails", "allowedGroups"}

	for _, field := range fields {
		Context("With a DexClient carrying a YAML document in "+field, func() {
			var rendered map[string]string

			BeforeEach(func() {
				hec.ValuesSetFromYaml("userAuthn.internal.dexClientCRDs", `
- id: dex-client-tenant@tenant-ns
  encodedID: "m5zgcztbnzq4x4u44scceizf"
  name: tenant
  namespace: tenant-ns
  spec: {}
  legacyID: dex-client-tenant:tenant-ns
  legacyEncodedID: "m5zgcztbnzq4x4u44scceizfxxx"
  clientSecret: clientSecretValue
`)
				hec.ValuesSet("userAuthn.internal.dexClientCRDs.0.spec."+field, []string{hostileValue})

				rendered = map[string]string{}
				hec.HelmRender(WithFilteredRenderOutput(rendered, []string{"dex-client/oauth2client.yaml"}))
			})

			It("Should not add an object to the release", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				Expect(hec.KubernetesGlobalResource("ClusterRoleBinding", injectedClusterRoleBindingName).Exists()).
					To(BeFalse(), "the value must not become a manifest of its own")

				for path, manifests := range rendered {
					Expect(renderedKinds(manifests)).To(Equal([]string{"OAuth2Client", "OAuth2Client"}),
						"%s must render exactly the two OAuth2Client objects", path)
				}
			})

			It("Should keep the value a single scalar of both OAuth2Client objects", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				for _, name := range []string{"m5zgcztbnzq4x4u44scceizf", "m5zgcztbnzq4x4u44scceizfxxx"} {
					client := hec.KubernetesResource("OAuth2Client", "d8-user-authn", name)
					Expect(client.Exists()).To(BeTrue())
					Expect(client.Field(field).AsStringSlice()).To(Equal([]string{hostileValue}))
				}
			})

			It("Should keep the rest of both OAuth2Client objects intact", func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				client := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "m5zgcztbnzq4x4u44scceizf")
				Expect(client.Field("id").String()).To(Equal("dex-client-tenant@tenant-ns"))
				Expect(client.Field("name").String()).To(Equal("dex-client-tenant@tenant-ns"))
				Expect(client.Field("secret").String()).To(Equal("clientSecretValue"))

				legacy := hec.KubernetesResource("OAuth2Client", "d8-user-authn", "m5zgcztbnzq4x4u44scceizfxxx")
				Expect(legacy.Field("id").String()).To(Equal("dex-client-tenant:tenant-ns"))
				Expect(legacy.Field("name").String()).To(Equal("dex-client-tenant:tenant-ns"))
				Expect(legacy.Field("secret").String()).To(Equal("clientSecretValue"))
			})
		})
	}
})

// renderedKinds lists the kind of every manifest of a rendered template file, in file order. It
// splits the stream the way the release does, so a value that stays a single scalar cannot be
// mistaken for a manifest of its own.
func renderedKinds(manifests string) []string {
	split := releaseutil.SplitManifests(manifests)

	names := make([]string, 0, len(split))
	for name := range split {
		names = append(names, name)
	}
	sort.Sort(releaseutil.BySplitManifestsOrder(names))

	kinds := make([]string, 0, len(names))
	for _, name := range names {
		object := ManifestStringToUnstructed(split[name])
		if object == nil {
			continue
		}

		kinds = append(kinds, object.GetKind())
	}

	return kinds
}
