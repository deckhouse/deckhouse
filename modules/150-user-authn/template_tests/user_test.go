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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: user", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
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
	})

	Context("With Users in values", func() {
		BeforeEach(func() {
			hec.ValuesSetFromYaml("userAuthn.internal.dexUsersCRDs", `
- encodedName: encodedUser
  name: userName
  spec:
    email: user@example.com
    groups:
    - Everyone
    password: $2a$10$7rxcwh8r2Rcnwc3jDysqhOrbskLBjtx1zvzWaQVPFO78DDAMZHhLC
- encodedName: base64EncodedUser
  name: base64UserName
  spec:
    email: base64@example.com
    groups:
    - Everyone
    password: JDJhJDEwJDdyeGN3aDhyMlJjbndjM2pEeXNxaE9yYnNrTEJqdHgxenZ6V2FRVlBGTzc4RERBTVpIaExD
- encodedName: encodedAdmin
  name: adminName
  spec:
    email: adminTest@example.com
    groups:
    - Everyone
    - Admins
    password: $2a$10$E/MjyzFi6GZkta9GHd8zCeuYigbLenXv18jkxOZ6vhoWsKnaxNJou
`)
			hec.HelmRender()
		})
		It("Should create Password objects", func() {
			userPassword := hec.KubernetesResource("Password", "d8-user-authn", "encodedUser")
			Expect(userPassword.Exists()).To(BeTrue())
			Expect(userPassword.Field("email").String()).To(Equal("user@example.com"))
			Expect(userPassword.Field("username").String()).To(Equal("userName"))
			Expect(userPassword.Field("userID").String()).To(Equal("userName"))
			Expect(userPassword.Field("hash").String()).To(Equal("JDJhJDEwJDdyeGN3aDhyMlJjbndjM2pEeXNxaE9yYnNrTEJqdHgxenZ6V2FRVlBGTzc4RERBTVpIaExD"))
			Expect(userPassword.Field("groups").String()).To(MatchJSON(`["Everyone"]`))

			base64Password := hec.KubernetesResource("Password", "d8-user-authn", "base64EncodedUser")
			Expect(base64Password.Exists()).To(BeTrue())
			Expect(base64Password.Field("email").String()).To(Equal("base64@example.com"))
			Expect(base64Password.Field("username").String()).To(Equal("base64UserName"))
			Expect(base64Password.Field("userID").String()).To(Equal("base64UserName"))
			Expect(base64Password.Field("hash").String()).To(Equal("JDJhJDEwJDdyeGN3aDhyMlJjbndjM2pEeXNxaE9yYnNrTEJqdHgxenZ6V2FRVlBGTzc4RERBTVpIaExD"))
			Expect(base64Password.Field("groups").String()).To(MatchJSON(`["Everyone"]`))

			adminPassword := hec.KubernetesResource("Password", "d8-user-authn", "encodedAdmin")
			Expect(adminPassword.Exists()).To(BeTrue())
			Expect(adminPassword.Field("email").String()).To(Equal("admintest@example.com"))
			Expect(adminPassword.Field("username").String()).To(Equal("adminName"))
			Expect(adminPassword.Field("userID").String()).To(Equal("adminName"))
			Expect(adminPassword.Field("hash").String()).To(Equal("JDJhJDEwJEUvTWp5ekZpNkdaa3RhOUdIZDh6Q2V1WWlnYkxlblh2MThqa3hPWjZ2aG9Xc0tuYXhOSm91"))
			Expect(adminPassword.Field("groups").String()).To(MatchJSON(`["Everyone","Admins"]`))
		})
	})
})
