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
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

const (
	customClusterRolesFlat = `---
admin:
  - cert-manager:user-authz:user
editor:
- cert-manager:user-authz:editor
`

	testCLusterRoleCRDsWithLimitNamespaces = `---
- name: testenev
  spec:
    accessLevel: Admin
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

	testCLusterRoleCRDsWithAllowAccessToSystemNamespaces = `---
- name: testenev
  spec:
    accessLevel: Admin
    allowScale: true
    allowAccessToSystemNamespaces: true
    subjects:
    - kind: User
      name: Efrem Testenev
    additionalRoles:
    - apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-write-all
`

	testCLusterRoleCRDsWithCRDsKey = `---
crds:
  - name: testenev
    spec:
      accessLevel: Admin
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

	testRoleCRDs = `---
- name: testenev-namespaced
  namespace: testenv
  spec:
    accessLevel: Editor
    allowScale: true
    subjects:
      - kind: User
        name: Namespace Testenev
`
)

var testCRDsWithCRDsKeyJSON, _ = ConvertYAMLToJSON([]byte(testCLusterRoleCRDsWithCRDsKey))

var _ = Describe("Module :: user-authz :: helm template ::", func() {
	f := SetupHelmConfig(``)

	BeforeSuite(func() {
		err := os.Symlink("/deckhouse/ee/be/modules/140-user-authz/templates/webhook", "/deckhouse/modules/140-user-authz/templates/webhook")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Symlink("/deckhouse/ee/be/modules/140-user-authz/templates/permission-browser-apiserver", "/deckhouse/modules/140-user-authz/templates/permission-browser-apiserver")
		Expect(err).ShouldNot(HaveOccurred())
	})

	AfterSuite(func() {
		err := os.Remove("/deckhouse/modules/140-user-authz/templates/webhook")
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Remove("/deckhouse/modules/140-user-authz/templates/permission-browser-apiserver")
		Expect(err).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)

		// Ensure the root userAuthz object exists (some EE templates access .Values.userAuthz.* directly).
		f.ValuesSet("userAuthz.enableMultiTenancy", false)

		// Minimal defaults to avoid nil-pointer panics in EE templates when rendering without explicitly
		// setting all userAuthz.internal.* values in a particular test context.
		// - webhook/configmap.yaml iterates over .Values.userAuthz.internal.clusterAuthRuleCrds even when enableMultiTenancy=false
		// - webhook/secret.yaml requires webhookCertificate when enableMultiTenancy=true
		f.ValuesSetFromYaml("userAuthz.internal.clusterAuthRuleCrds", `[]`)
		f.ValuesSetFromYaml("userAuthz.internal.authRuleCrds", `[]`)
		f.ValuesSetFromYaml("userAuthz.internal.customClusterRoles", `{}`)

		f.ValuesSet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.ca", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.crt", "test")
		f.ValuesSet("userAuthz.internal.webhookCertificate.key", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.ca", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.crt", "test")
		f.ValuesSet("userAuthz.internal.apiserverCertificate.key", "test")

		// Some EE templates access this field unconditionally and will panic if the object is absent.
		f.ValuesSet("userAuthz.controlPlaneConfigurator.enabled", true)
	})

	Context("With custom resources (incl. limitNamespaces), enabledMultiTenancy and controlPlaneConfigurator", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("userAuthz.internal.clusterAuthRuleCrds", testCLusterRoleCRDsWithLimitNamespaces)
			f.ValuesSetFromYaml("userAuthz.internal.authRuleCrds", testRoleCRDs)
			f.ValuesSetFromYaml("userAuthz.internal.customClusterRoles", customClusterRolesFlat)

			f.ValuesSet("userAuthz.enableMultiTenancy", true)
			f.ValuesSet("userAuthz.controlPlaneConfigurator.enabled", true)
			// Make SecurityPolicyException available for template rendering.
			// In CI template-tests, `.Capabilities.APIVersions` is typically empty, so we emulate discovery.
			f.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
			f.ValuesSet("global.discovery.extensionAPIServerAuthenticationRequestheaderClientCA", "test")
			f.ValuesSet("userAuthz.internal.webhookCertificate.ca", "test")
			f.ValuesSet("userAuthz.internal.webhookCertificate.crt", "test")
			f.ValuesSet("userAuthz.internal.webhookCertificate.key", "test")
			f.ValuesSet("userAuthz.internal.apiserverCertificate.ca", "test")
			f.ValuesSet("userAuthz.internal.apiserverCertificate.crt", "test")
			f.ValuesSet("userAuthz.internal.apiserverCertificate.key", "test")

			f.HelmRender()
		})

		It("Should create a ClusterRoleBinding for each additionalRole", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:additional-role:cluster-write-all")
			Expect(crb.Exists()).To(BeTrue())

			Expect(crb.Field("roleRef.name").String()).To(Equal("cluster-write-all"))
			Expect(crb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
			Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
		})

		It("Should create a ClusterRoleBinding to an appropriate ClusterRole", func() {
			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:admin")
			Expect(crb.Exists()).To(BeTrue())

			Expect(crb.Field("roleRef.name").String()).To(Equal("user-authz:admin"))
			Expect(crb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
			Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
		})

		It("Should create a RoleBinding to an appropriate Role", func() {
			rb := f.KubernetesResource("RoleBinding", "testenv", "user-authz:testenev-namespaced:editor")
			Expect(rb.Exists()).To(BeTrue())

			Expect(rb.Field("roleRef.name").String()).To(Equal("user-authz:editor"))
			Expect(rb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
			Expect(rb.Field("subjects.0.name").String()).To(Equal("Namespace Testenev"))
		})

		It("Should create additional ClusterRoleBinding for each ClusterRole with the \"user-authz.deckhouse.io/access-level\" annotation", func() {
			crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:admin:custom-cluster-role:cert-manager:user-authz:user")
			Expect(crb.Exists()).To(BeTrue())

			Expect(crb.Field("roleRef.name").String()).To(Equal("cert-manager:user-authz:user"))
			Expect(crb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
			Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
		})

		It("Should create additional RoleBinding for each ClusterRole with the \"user-authz.deckhouse.io/access-level\" annotation", func() {
			rb := f.KubernetesResource("RoleBinding", "testenv", "user-authz:testenev-namespaced:editor:custom-cluster-role:cert-manager:user-authz:editor")
			Expect(rb.Exists()).To(BeTrue())

			Expect(rb.Field("roleRef.name").String()).To(Equal("cert-manager:user-authz:editor"))
			Expect(rb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
			Expect(rb.Field("subjects.0.name").String()).To(Equal("Namespace Testenev"))
		})

		Context("portForwarding option is set in a CAR", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthz.internal.clusterAuthRuleCrds.0.spec.portForwarding", true)
				f.HelmRender()
			})

			It("Should create a port-forward ClusterRoleBinding", func() {
				crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:port-forward")
				Expect(crb.Exists()).To(BeTrue())

				Expect(crb.Field("roleRef.name").String()).To(Equal("user-authz:port-forward"))
				Expect(crb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
				Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
			})
		})

		Context("portForwarding option is set in a AR", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthz.internal.authRuleCrds.0.spec.portForwarding", true)
				f.HelmRender()
			})

			It("Should create a port-forward RoleBinding", func() {
				rb := f.KubernetesResource("RoleBinding", "testenv", "user-authz:testenev-namespaced:port-forward")
				Expect(rb.Exists()).To(BeTrue())

				Expect(rb.Field("roleRef.name").String()).To(Equal("user-authz:port-forward"))
				Expect(rb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
				Expect(rb.Field("subjects.0.name").String()).To(Equal("Namespace Testenev"))
			})
		})

		Context("allowScale option is set to true in a CAR", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthz.internal.clusterAuthRuleCrds.0.spec.allowScale", true)
				f.HelmRender()
			})

			It("Should create a scale RoleBinding", func() {
				crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:scale")
				Expect(crb.Exists()).To(BeTrue())

				Expect(crb.Field("roleRef.name").String()).To(Equal("user-authz:scale"))
				Expect(crb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
				Expect(crb.Field("subjects.0.name").String()).To(Equal("Efrem Testenev"))
			})
		})

		Context("allowScale option is set to true in a AR", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthz.internal.authRuleCrds.0.spec.allowScale", true)
				f.HelmRender()
			})

			It("Should create a scale RoleBinding", func() {
				rb := f.KubernetesResource("RoleBinding", "testenv", "user-authz:testenev-namespaced:scale")
				Expect(rb.Exists()).To(BeTrue())

				Expect(rb.Field("roleRef.name").String()).To(Equal("user-authz:scale"))
				Expect(rb.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
				Expect(rb.Field("subjects.0.name").String()).To(Equal("Namespace Testenev"))
			})
		})

		Context("allowScale option is set to false in a CAR", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthz.internal.clusterAuthRuleCrds.0.spec.allowScale", false)
				f.HelmRender()
			})

			It("Should not create a scale RoleBinding", func() {
				crb := f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:scale")
				Expect(crb.Exists()).To(BeFalse())
			})
		})

		Context("allowScale option is set to false in a AR", func() {
			BeforeEach(func() {
				f.ValuesSet("userAuthz.internal.clusterAuthRuleCrds.0.spec.allowScale", false)
				f.HelmRender()
			})

			It("Should not create a scale RoleBinding", func() {
				rb := f.KubernetesResource("RoleBinding", "testenv", "user-authz:testenev:scale")
				Expect(rb.Exists()).To(BeFalse())
			})
		})

		It("Should deploy authorization webhook and supporting objects", func() {
			Expect(f.KubernetesResource("DaemonSet", "d8-user-authz", "user-authz-webhook").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "control-plane-configurator").Field("data.ca").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "apiserver-authentication-requestheader-client-ca").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-user-authz", "user-authz-webhook").Exists()).To(BeTrue())

			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "user-authz-webhook").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "user-authz-webhook").Field("data.config\\.json").String()).To(MatchJSON(testCRDsWithCRDsKeyJSON))
		})

		It("Should configure user-authz-webhook to use local kube-apiserver endpoint", func() {
			ds := f.KubernetesResource("DaemonSet", "d8-user-authz", "user-authz-webhook")
			Expect(ds.Exists()).To(BeTrue())
			Expect(ds.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())

			// Webhook listens on a node-local port, kube-apiserver calls it via https://127.0.0.1:40443.
			Expect(ds.Field("spec.template.spec.containers.0.ports.0.containerPort").Int()).To(Equal(int64(40443)))
			Expect(ds.Field("spec.template.spec.containers.0.ports.0.protocol").String()).To(Equal("TCP"))

			Expect(ds.Field("spec.template.spec.containers.0.env.0.name").String()).To(Equal("KUBERNETES_SERVICE_HOST"))
			Expect(ds.Field("spec.template.spec.containers.0.env.0.valueFrom.fieldRef.fieldPath").String()).To(Equal("status.hostIP"))
			Expect(ds.Field("spec.template.spec.containers.0.env.1.name").String()).To(Equal("KUBERNETES_SERVICE_PORT"))
			Expect(ds.Field("spec.template.spec.containers.0.env.1.value").String()).To(Equal("6443"))
		})

		It("Should allow node-local webhook port in SecurityPolicyException", func() {
			rendered := map[string]string{}
			f.HelmRender(WithFilteredRenderOutput(rendered, []string{"webhook/daemonset.yaml"}))
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			manifest := ""
			for k, v := range rendered {
				if strings.Contains(k, "webhook/daemonset.yaml") {
					manifest = v
					break
				}
			}
			if manifest == "" {
				if len(rendered) == 1 {
					for _, v := range rendered {
						manifest = v
						break
					}
				}
			}
			Expect(manifest).ToNot(BeEmpty())
			Expect(manifest).To(ContainSubstring("kind: SecurityPolicyException"))
			Expect(manifest).To(ContainSubstring("name: user-authz-webhook"))
			Expect(manifest).To(ContainSubstring("allowedValue: true"))
			Expect(manifest).To(ContainSubstring("hostPorts:"))
			Expect(manifest).To(ContainSubstring("port: 40443"))
			Expect(manifest).To(ContainSubstring("protocol: TCP"))
		})

		It("Should deploy permission-browser-apiserver and supporting objects", func() {
			Expect(f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Service", "d8-user-authz", "permission-browser-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-user-authz", "permission-browser-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "permission-browser-apiserver-kubeconfig").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", "d8-user-authz", "permission-browser-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("PodDisruptionBudget", "d8-user-authz", "permission-browser-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("APIService", "v1alpha1.authorization.deckhouse.io").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:permission-browser-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:bulk-sar-creator").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:self-bulk-sar-creator").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:permission-browser-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:permission-browser-apiserver:auth-delegator").Exists()).To(BeTrue())
		})

		It("Should configure permission-browser-apiserver deployment correctly", func() {
			deploy := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
			Expect(deploy.Field("spec.template.spec.containers.0.args").String()).To(ContainSubstring("--secure-port=8443"))
			Expect(deploy.Field("spec.template.spec.containers.0.ports.0.containerPort").Int()).To(Equal(int64(8443)))
			Expect(deploy.Field("spec.template.spec.containers.0.ports.0.name").String()).To(Equal("https"))
		})

		It("Should configure APIService correctly", func() {
			apiService := f.KubernetesGlobalResource("APIService", "v1alpha1.authorization.deckhouse.io")
			Expect(apiService.Field("spec.group").String()).To(Equal("authorization.deckhouse.io"))
			Expect(apiService.Field("spec.version").String()).To(Equal("v1alpha1"))
			Expect(apiService.Field("spec.service.name").String()).To(Equal("permission-browser-apiserver"))
			Expect(apiService.Field("spec.service.namespace").String()).To(Equal("d8-user-authz"))
		})
	})

	Context("With CAR (incl. limitNamespaces) and not enabledMultiTenancy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("userAuthz.internal.clusterAuthRuleCrds", testCLusterRoleCRDsWithLimitNamespaces)
			f.HelmRender()
		})

		It("Helm should fail", func() {
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).Should(ContainSubstring("You must turn on userAuthz.enableMultiTenancy to use limitNamespaces option in your ClusterAuthorizationRule resources."))
		})
	})

	Context("Namespace access permissions based on edition", func() {
		Context("EE edition (non-CE)", func() {
			BeforeEach(func() {
				f.ValuesSet("global.deckhouseEdition", "EE")
				f.ValuesSet("userAuthz.enableMultiTenancy", true)
				f.HelmRender()
			})

			It("Should render without errors", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("user-authz:user should have accessiblenamespaces instead of namespaces", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:user")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring("accessiblenamespaces"))
				Expect(rules).To(ContainSubstring("authorization.deckhouse.io"))
				Expect(rules).NotTo(MatchRegexp(`"resources":\s*\[\s*"namespaces"\s*\]`))
			})

			It("user-authz:editor should have accessiblenamespaces instead of namespaces", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:editor")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring("accessiblenamespaces"))
			})

			It("user-authz:cluster-admin should still have full namespaces access", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:cluster-admin")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring(`"namespaces"`))
				Expect(rules).To(ContainSubstring(`"create"`))
				Expect(rules).To(ContainSubstring(`"delete"`))
			})

			It("system:authenticated should be able to discover accessiblenamespaces", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:accessible-namespaces-reader")
				Expect(cr.Exists()).To(BeTrue())
				Expect(cr.Field("rules").String()).To(ContainSubstring("accessiblenamespaces"))

				crb := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:accessible-namespaces-reader:system-authenticated")
				Expect(crb.Exists()).To(BeTrue())
				Expect(crb.Field("roleRef.name").String()).To(Equal("d8:user-authz:accessible-namespaces-reader"))
				Expect(crb.Field("subjects.0.kind").String()).To(Equal("Group"))
				Expect(crb.Field("subjects.0.name").String()).To(Equal("system:authenticated"))
			})
		})

		Context("EE edition (non-CE) with MultiTenancy disabled", func() {
			BeforeEach(func() {
				f.ValuesSet("global.deckhouseEdition", "EE")
				f.ValuesSet("userAuthz.enableMultiTenancy", false)
				f.HelmRender()
			})

			It("Should render without errors", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("user-authz:user should keep namespaces access (fallback)", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:user")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring(`"namespaces"`))
				Expect(rules).NotTo(ContainSubstring("accessiblenamespaces"))
			})

			It("system:authenticated should not get accessiblenamespaces discovery", func() {
				Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:accessible-namespaces-reader:system-authenticated").Exists()).To(BeFalse())
			})
		})

		Context("CE edition", func() {
			BeforeEach(func() {
				f.ValuesSet("global.deckhouseEdition", "CE")
				f.HelmRender()
			})

			It("Should render without errors", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("user-authz:user should have namespaces access (not accessiblenamespaces)", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:user")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring(`"namespaces"`))
				Expect(rules).NotTo(ContainSubstring("accessiblenamespaces"))
			})

			It("user-authz:editor should have namespaces access", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:editor")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring(`"namespaces"`))
				Expect(rules).NotTo(ContainSubstring("accessiblenamespaces"))
			})

			It("system:authenticated should not get accessiblenamespaces discovery", func() {
				Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:accessible-namespaces-reader:system-authenticated").Exists()).To(BeFalse())
			})
		})

		Context("EE edition with MultiTenancy enabled and consoleLegacyCompat enabled", func() {
			BeforeEach(func() {
				f.ValuesSet("global.deckhouseEdition", "EE")
				f.ValuesSet("userAuthz.enableMultiTenancy", true)
				f.ValuesSet("userAuthz.internal.consoleLegacyCompat", true)
				f.HelmRender()
			})

			It("Should render without errors", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("user-authz:user should have namespaces access (legacy compat)", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:user")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring(`"namespaces"`))
				Expect(rules).NotTo(ContainSubstring("accessiblenamespaces"))
			})

			It("user-authz:editor should have namespaces access (legacy compat)", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:editor")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring(`"namespaces"`))
				Expect(rules).NotTo(ContainSubstring("accessiblenamespaces"))
			})

			It("permission-browser-apiserver should still be deployed", func() {
				Expect(f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver").Exists()).To(BeTrue())
			})
		})

		Context("EE edition with MultiTenancy enabled and consoleLegacyCompat disabled", func() {
			BeforeEach(func() {
				f.ValuesSet("global.deckhouseEdition", "EE")
				f.ValuesSet("userAuthz.enableMultiTenancy", true)
				f.ValuesSet("userAuthz.internal.consoleLegacyCompat", false)
				f.HelmRender()
			})

			It("Should render without errors", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
			})

			It("user-authz:user should have accessiblenamespaces instead of namespaces", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:user")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring("accessiblenamespaces"))
				Expect(rules).To(ContainSubstring("authorization.deckhouse.io"))
			})

			It("user-authz:editor should have accessiblenamespaces", func() {
				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:editor")
				Expect(cr.Exists()).To(BeTrue())

				rules := cr.Field("rules").String()
				Expect(rules).To(ContainSubstring("accessiblenamespaces"))
			})
		})
	})

	Context("With CAR (incl. limitNamespaces) and not enabledMultiTenancy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("userAuthz.internal.clusterAuthRuleCrds", testCLusterRoleCRDsWithAllowAccessToSystemNamespaces)
			f.HelmRender()
		})

		It("Helm should fail", func() {
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).Should(ContainSubstring("You must turn on userAuthz.enableMultiTenancy to use allowAccessToSystemNamespaces flag in your ClusterAuthorizationRule resources."))
		})
	})

})
