/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package template_tests

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const testCRDsWithCRDsKey = `---
  - name: testenev
    spec:
      accessLevel: Admin
      allowScale: true
      limitNamespaces:
      - default
      subjects:
      - kind: User
        name: Efrem Testenev
      additionalRoles:
      - apiGroup: rbac.authorization.k8s.io
        kind: ClusterRole
        name: cluster-write-all
`

var testCRDsWithCRDsKeyJSON, _ = ConvertYAMLToJSON([]byte(testCRDsWithCRDsKey))

var _ = FDescribe("Module :: user-authz :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		// TODO: move to some common function???
		f.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		f.ValuesSet("global.modulesImages.registry", "registryAddr")
		f.ValuesSet("userAuthz.enableMultiTenancy", true)
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
	})

	Context("With custom resources (incl. limitNamespaces), enabledMultiTenancy and controlPlaneConfigurator", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("userAuthz.internal.multitenancyCRDs", testCRDsWithCRDsKey)

			f.HelmRender()
		})

		It("Should create a ClusterRoleBinding for each additionalRole", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:additional-role:cluster-write-all")
			Expect(crb.Exists()).To(BeTrue())

			Expect(crb.Field("roleRef.name").String()).To(Equal("cluster-write-all"))
			Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
		})
	})
})
