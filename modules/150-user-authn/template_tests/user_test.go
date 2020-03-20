package template_tests

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

var _ = Describe("Module :: user-authn :: helm template :: user", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.clusterVersion", "1.15.6")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler-crd"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.kubernetesCA", "plainstring")
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
    password: userPassword
    userID: user
- encodedName: encodedAdmin
  name: adminName
  spec:
    email: adminTest@example.com
    groups:
    - Everyone
    - Admins
    password: adminPassword
    userID: admin
`)
			hec.HelmRender()
		})
		It("Should create Password objects", func() {
			userPassword := hec.KubernetesResource("Password", "d8-user-authn", "encodedUser")
			Expect(userPassword.Exists()).To(BeTrue())
			Expect(userPassword.Field("email").String()).To(Equal("user@example.com"))
			Expect(userPassword.Field("username").String()).To(Equal("userName"))
			Expect(userPassword.Field("userID").String()).To(Equal("user"))
			Expect(userPassword.Field("hash").String()).To(Equal("dXNlclBhc3N3b3Jk"))
			Expect(userPassword.Field("groups").String()).To(MatchJSON(`["Everyone"]`))

			adminPassword := hec.KubernetesResource("Password", "d8-user-authn", "encodedAdmin")
			Expect(adminPassword.Exists()).To(BeTrue())
			Expect(adminPassword.Field("email").String()).To(Equal("admintest@example.com"))
			Expect(adminPassword.Field("username").String()).To(Equal("adminName"))
			Expect(adminPassword.Field("userID").String()).To(Equal("admin"))
			Expect(adminPassword.Field("hash").String()).To(Equal("YWRtaW5QYXNzd29yZA=="))
			Expect(adminPassword.Field("groups").String()).To(MatchJSON(`["Everyone","Admins"]`))
		})
	})
})
