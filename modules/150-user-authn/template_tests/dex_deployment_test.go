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
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: dex deployment", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", `
discovery:
  kubernetesVersion: 1.33.1
  d8SpecificNodeCountByRole:
    system: 2
  kubernetesCA: plainstring
modules:
  publicDomainTemplate: "%s.example.com"
  https:
    mode: Disabled
`)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		f.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		f.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		f.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
	})

	// startupSeconds is how long the startup probe of the container lets Dex start before liveness takes over.
	startupSeconds := func(container string) int64 {
		dex := f.KubernetesResource("Deployment", "d8-user-authn", "dex")
		Expect(dex.Exists()).To(BeTrue())

		probe := dex.Field(`spec.template.spec.containers.#(name=="` + container + `").startupProbe`)
		Expect(probe.Get("httpGet.path").String()).To(Equal("/healthz/live"))
		Expect(probe.Get("httpGet.port").Int()).To(BeEquivalentTo(5559))
		return probe.Get("periodSeconds").Int() * probe.Get("failureThreshold").Int()
	}

	// Dex opens every connector before it starts serving /healthz/live, and an OIDC issuer that does not answer
	// costs the 10s discovery deadline of Dex patch 400. The dex container needs a startup budget that grows with
	// the OIDC providers, or the liveness probe restarts the pod in a loop. The budget is pinned: a larger one only
	// delays the restart of a Dex that is really stuck. kube-rbac-proxy checks its own liveness, so a slow or hung
	// Dex does not restart it, and needs no startup budget.
	DescribeTable("gives Dex time to open unreachable OIDC connectors",
		func(providers string, oidcProviders int64) {
			if providers != "" {
				f.ValuesSetFromYaml("userAuthn.internal.providers", providers)
			}
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(startupSeconds("dex")).To(Equal(60 + 15*oidcProviders))

			proxy := f.KubernetesResource("Deployment", "d8-user-authn", "dex").Field(`spec.template.spec.containers.#(name=="kube-rbac-proxy")`)
			Expect(proxy.Get("startupProbe").Exists()).To(BeFalse())
			Expect(proxy.Get("livenessProbe.httpGet.path").String()).To(Equal("/livez"))
			Expect(proxy.Get("args").String()).To(ContainSubstring("--livez-path=/livez"))
			Expect(proxy.Get("readinessProbe.httpGet.path").String()).To(Equal("/healthz/ready"))
		},
		Entry("without providers", "", int64(0)),
		Entry("with two OIDC providers and an LDAP one", `
- id: first
  displayName: first
  type: OIDC
  oidc:
    issuer: https://first.example.com
    clientID: clientID
    clientSecret: secret
- id: second
  displayName: second
  type: OIDC
  oidc:
    issuer: https://second.example.com
    clientID: clientID
    clientSecret: secret
- id: ldap
  displayName: ldap
  type: LDAP
  ldap:
    host: ldap.example.com:636
    userSearch:
      baseDN: cn=users,dc=example,dc=com
      username: uid
      idAttr: uid
      emailAttr: mail
`, int64(2)),
	)
})
