package template_tests

import (
	"github.com/onsi/gomega/gbytes"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const customClusterRolesFlat = `---
master:
  - cert-manager:user-authz:user
`

const testCRDsWithLimitNamespaces = `---
- name: testenev
  spec:
    accessLevel: Master
    allowScale: true
    limitNamespaces:
    - default
    - .*
    subjects:
    - kind: User
      name: Efrem Testenev
    additionalRoles:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-write-all
`

const testCRDsWithAllowAccessToSystemNamespaces = `---
- name: testenev
  spec:
    accessLevel: Master
    allowScale: true
    allowAccessToSystemNamespaces: trus
    subjects:
    - kind: User
      name: Efrem Testenev
    additionalRoles:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-write-all
`


const testCRDsWithCrdsKey = `---
crds:
  - name: testenev
    spec:
      accessLevel: Master
      allowScale: true
      limitNamespaces:
      - default
      - .*
      subjects:
      - kind: User
        name: Efrem Testenev
      additionalRoles:
      - apiGroup: rbac.authorization.k8s.io
        kind: ClusterRole
        name: cluster-write-all
`

var testCRDsWithCrdsKeyJson, _ = ConvertYamlToJson([]byte(testCRDsWithCrdsKey))

var _ = Describe("Module :: user-authz :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		// TODO: move to some common function???
		f.ValuesSet("global.discovery.kubernetesVersion", "1.15.6")
		f.ValuesSet("global.modulesImages.registry", "registryAddr")
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
	})

	Context("With custom resources (incl. limitNamespaces), enabledMultiTenancy and controlPlaneConfigurator", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("userAuthz.internal.crds", testCRDsWithLimitNamespaces)
			f.ValuesSetFromYaml("userAuthz.internal.customClusterRoles", customClusterRolesFlat)

			f.ValuesSet("userAuthz.enableMultiTenancy", true)
			f.ValuesSet("userAuthz.controlPlaneConfigurator.enabled", true)
			f.ValuesSet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA", "test")
			f.ValuesSet("userAuthz.internal.webhookCA", "test")
			f.ValuesSet("userAuthz.internal.webhookServerCrt", "test")
			f.ValuesSet("userAuthz.internal.webhookServerKey", "test")

			f.HelmRender()
		})

		It("Should create a ClusterRoleBinding for each additionalRole", func() {
			Expect(string(f.Session.Err.Contents())).To(HaveLen(0))
			Expect(f.Session.ExitCode()).To(BeZero())

			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:additional-role:cluster-write-all")
			Expect(crb.Exists()).To(BeTrue())

			Expect(crb.Field("roleRef.name").String()).To(Equal("cluster-write-all"))
			Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
		})

		It("Should create a ClusterRoleBinding to an appropriate ClusterRole", func() {
			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:master")
			Expect(crb.Exists()).To(BeTrue())

			Expect(crb.Field("roleRef.name").String()).To(Equal("user-authz:master"))
			Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
		})

		It("Should create additional ClusterBinding for each ClusterRole with the \"user-authz.deckhouse.io/access-level\" annotation", func() {
			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:master:custom-cluster-role:cert-manager:user-authz:user")
			Expect(crb.Exists()).To(BeTrue())

			Expect(crb.Field("roleRef.name").String()).To(Equal("cert-manager:user-authz:user"))
			Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
		})

		Context("portForwarding option is set in a CAR", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthz.internal.crds.0.spec.portForwarding", true)
				f.HelmRender()
			})

			It("Should create a port-forward RoleBinding", func() {
				crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:port-forward")
				Expect(crb.Exists()).To(BeTrue())

				Expect(crb.Field("roleRef.name").String()).To(Equal("user-authz:port-forward"))
				Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
			})
		})

		Context("allowScale option is set in a CAR", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthz.internal.crds.0.spec.allowScale", true)
				f.HelmRender()
			})

			It("Should create a port-forward RoleBinding", func() {
				crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:scale")
				Expect(crb.Exists()).To(BeTrue())

				Expect(crb.Field("roleRef.name").String()).To(Equal("user-authz:scale"))
				Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
			})
		})

		It("Should deploy authorization webhook and supporting objects", func() {
			Expect(f.KubernetesResource("DaemonSet", "d8-user-authz", "user-authz-webhook").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "control-plane-configurator").Field("data.ca").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "apiserver-authentication-requestheader-client-ca").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-user-authz", "user-authz-webhook").Exists()).To(BeTrue())

			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "user-authz-webhook").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "user-authz-webhook").Field("data.config\\.json").String()).To(MatchJSON(testCRDsWithCrdsKeyJson))
		})
	})


	Context("With custom resources (incl. limitNamespaces) and not enabledMultiTenancy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("userAuthz.internal.crds", testCRDsWithLimitNamespaces)
			f.HelmRender()
		})

		It("Helm should fail", func() {
			Expect(f.Session.ExitCode()).To(Not(BeZero()))
			Expect(f.Session.Err).Should(gbytes.Say("You must turn on userAuthz.enableMultiTenancy to use limitNamespaces option in your ClusterAuthorizationRule resources."))
		})
	})

	Context("With custom resources (incl. limitNamespaces) and not enabledMultiTenancy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("userAuthz.internal.crds", testCRDsWithAllowAccessToSystemNamespaces)
			f.HelmRender()
		})

		It("Helm should fail", func() {
			Expect(f.Session.ExitCode()).To(Not(BeZero()))
			Expect(f.Session.Err).Should(gbytes.Say("You must turn on userAuthz.enableMultiTenancy to use allowAccessToSystemNamespaces flag in your ClusterAuthorizationRule resources."))
		})
	})

})

