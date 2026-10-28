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
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

var _ = Describe("Module :: user-authz :: helm template ::", func() {
	f := SetupHelmConfig(``)

	// The BE-edition templates live under ee/be; link them into the module so the render covers
	// them. Paths are derived from this file rather than from the hardcoded /deckhouse checkout
	// path. Note that the suite still needs the CI layout: charts/helm_lib is a symlink to
	// /deckhouse/helm_lib, which does not exist in a plain local checkout.
	_, thisFile, _, _ := runtime.Caller(0)
	moduleDir := filepath.Join(filepath.Dir(thisFile), "..")
	repoRoot := filepath.Join(moduleDir, "..", "..")

	// relink replaces whatever is at link (a symlink left by an interrupted run) with a symlink to
	// target, so the suite does not fail on its own leftovers.
	relink := func(target, link string) {
		if err := os.Remove(link); err != nil && !os.IsNotExist(err) {
			Expect(err).ShouldNot(HaveOccurred())
		}
		Expect(os.Symlink(target, link)).ShouldNot(HaveOccurred())
	}

	BeforeSuite(func() {
		relink(filepath.Join(repoRoot, "ee/be/modules/140-user-authz/templates/webhook"), filepath.Join(moduleDir, "templates/webhook"))
		relink(filepath.Join(repoRoot, "ee/be/modules/140-user-authz/templates/permission-browser-apiserver"), filepath.Join(moduleDir, "templates/permission-browser-apiserver"))
	})

	AfterSuite(func() {
		err := os.Remove(filepath.Join(moduleDir, "templates/webhook"))
		Expect(err).ShouldNot(HaveOccurred())
		err = os.Remove(filepath.Join(moduleDir, "templates/permission-browser-apiserver"))
		Expect(err).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)

		// Ensure the root userAuthz object exists (some EE templates access .Values.userAuthz.* directly).
		f.ValuesSet("userAuthz.enableMultiTenancy", false)

		// webhook/secret.yaml requires webhookCertificate when enableMultiTenancy=true.

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

	Context("With enabledMultiTenancy and highAvailability", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd", "prometheus"]`)
			f.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
			f.ValuesSet("global.highAvailability", true)
			f.ValuesSet("userAuthz.enableMultiTenancy", true)
			f.HelmRender()
		})

		It("Should keep permission-browser serving for 5 seconds after SIGTERM", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			deployment := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
			Expect(deployment.Field(`spec.template.spec.containers.#(name=="apiserver").lifecycle.preStop.sleep.seconds`).Int()).To(BeEquivalentTo(5))
		})

		// In HA the aggregated API server keeps two replicas and rolls them one at a time with no
		// surge, as before; the existing budget (maxUnavailable 1) keeps one of them through
		// voluntary disruptions.
		It("Should keep two permission-browser replicas and one always available", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
			Expect(deployment.Exists()).To(BeTrue())
			Expect(deployment.Field("spec.replicas").Int()).To(BeEquivalentTo(2))
			Expect(deployment.Field("spec.strategy.rollingUpdate.maxSurge").Int()).To(BeEquivalentTo(0))
			Expect(deployment.Field("spec.strategy.rollingUpdate.maxUnavailable").Int()).To(BeEquivalentTo(1))

			pdb := f.KubernetesResource("PodDisruptionBudget", "d8-user-authz", "permission-browser-apiserver")
			Expect(pdb.Exists()).To(BeTrue())
			Expect(pdb.Field("spec.maxUnavailable").Int()).To(BeEquivalentTo(1))
		})
	})

	Context("With custom resources (incl. limitNamespaces), enabledMultiTenancy and controlPlaneConfigurator", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd", "prometheus"]`)

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

		It("Should render the RBACv2 role and capability framework", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			systemRole := f.KubernetesGlobalResource("ClusterRole", "d8:system:manager")
			Expect(systemRole.Exists()).To(BeTrue())
			Expect(systemRole.Field(`metadata.labels.rbac\.deckhouse\.io/kind`).String()).To(Equal("role"))
			Expect(systemRole.Field(`metadata.labels.rbac\.deckhouse\.io/scope`).String()).To(Equal("system"))
			Expect(systemRole.Field(`metadata.labels.rbac\.deckhouse\.io/use-role`).String()).To(Equal("viewer"))
			Expect(systemRole.Field("aggregationRule.clusterRoleSelectors").Array()).ToNot(BeEmpty())

			subsystemRole := f.KubernetesGlobalResource("ClusterRole", "d8:subsystem:network:viewer")
			Expect(subsystemRole.Exists()).To(BeTrue())
			Expect(subsystemRole.Field(`metadata.labels.rbac\.deckhouse\.io/scope`).String()).To(Equal("subsystem"))
			Expect(subsystemRole.Field(`metadata.labels.rbac\.deckhouse\.io/subsystem`).String()).To(Equal("network"))

			// One subsystem role per subsystem of the list, and none of the subsystems it replaced.
			for _, subsystem := range []string{"iam", "security", "cluster", "delivery", "network", "storage", "observability", "managed-services"} {
				for _, level := range []string{"viewer", "manager", "superadmin"} {
					name := "d8:subsystem:" + subsystem + ":" + level
					role := f.KubernetesGlobalResource("ClusterRole", name)
					Expect(role.Exists()).To(BeTrue(), "expected ClusterRole %s to be rendered", name)
					Expect(role.Field(`metadata.labels.rbac\.deckhouse\.io/subsystem`).String()).To(Equal(subsystem), name)
					lineage := `aggregationRule.clusterRoleSelectors.0.matchLabels.rbac\.deckhouse\.io/aggregate-to-` + subsystem + `-as`
					Expect(role.Field(lineage).String()).To(Equal(level), name)
				}
			}
			for _, subsystem := range []string{"deckhouse", "infrastructure", "kubernetes", "networking"} {
				Expect(f.KubernetesGlobalResource("ClusterRole", "d8:subsystem:"+subsystem+":manager").Exists()).To(BeFalse(), subsystem)
			}

			for _, name := range []string{
				"d8:namespace:viewer", "d8:namespace:user", "d8:namespace:manager", "d8:namespace:admin", "d8:namespace:superadmin",
			} {
				namespaceRole := f.KubernetesGlobalResource("ClusterRole", name)
				Expect(namespaceRole.Exists()).To(BeTrue(), "expected ClusterRole %s to be rendered", name)
				Expect(namespaceRole.Field(`metadata.labels.rbac\.deckhouse\.io/kind`).String()).To(Equal("role"))
				Expect(namespaceRole.Field(`metadata.labels.rbac\.deckhouse\.io/scope`).String()).To(Equal("namespace"))
				Expect(namespaceRole.Field(`metadata.labels.rbac\.deckhouse\.io/delegatable`).String()).To(Equal("true"))
				Expect(namespaceRole.Field("rules").Array()).To(BeEmpty())
			}

			capability := f.KubernetesGlobalResource("ClusterRole", "d8:namespace-capability:kubernetes:view_resources")
			Expect(capability.Exists()).To(BeTrue())
			Expect(capability.Field(`metadata.labels.rbac\.deckhouse\.io/kind`).String()).To(Equal("capability"))
			Expect(capability.Field(`metadata.labels.rbac\.deckhouse\.io/aggregate-to-namespace-as`).String()).To(Equal("viewer"))
			Expect(capability.Field("rules").Array()).ToNot(BeEmpty())
			Expect(capability.Field(`metadata.annotations.en\.meta\.deckhouse\.io/title`).String()).ToNot(BeEmpty())
			Expect(capability.Field(`metadata.annotations.ru\.meta\.deckhouse\.io/title`).String()).ToNot(BeEmpty())

			dict := f.KubernetesGlobalResource("ClusterRole", "d8:dict")
			Expect(dict.Exists()).To(BeTrue())

			// The legacy role names are kept for one release as deprecated compatibility aliases so
			// existing bindings to them keep working; they must carry the deprecation marker.
			legacyManage := f.KubernetesGlobalResource("ClusterRole", "d8:manage:all:manager")
			Expect(legacyManage.Exists()).To(BeTrue())
			Expect(legacyManage.Field(`metadata.labels.rbac\.deckhouse\.io/deprecated`).String()).To(Equal("true"))
			// an alias carries the can-assign range of the old role, so holders of the old name keep
			// the right to assign roles for the compatibility release
			Expect(legacyManage.Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-scope`).String()).To(Equal("system"))
			// a manager assigns up to the manager level and ClusterEditor, the old names as the new ones
			Expect(legacyManage.Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-max-level`).String()).To(Equal("manager"))
			Expect(legacyManage.Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-basic-max`).String()).To(Equal("ClusterEditor"))
			legacySecurity := f.KubernetesGlobalResource("ClusterRole", "d8:manage:security:manager")
			Expect(legacySecurity.Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-subsystem`).String()).To(Equal("security"))
			Expect(legacySecurity.Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-max-level`).String()).To(Equal("manager"))
			Expect(legacySecurity.Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-basic-max`).String()).To(Equal("ClusterEditor"))
			for _, manager := range []string{"d8:system:manager", "d8:subsystem:iam:manager"} {
				role := f.KubernetesGlobalResource("ClusterRole", manager)
				Expect(role.Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-max-level`).String()).To(Equal("manager"), manager)
				Expect(role.Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-basic-max`).String()).To(Equal("ClusterEditor"), manager)
			}
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:manage:all:viewer").Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-basic-max`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:system:superadmin").Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-max-level`).String()).To(Equal("superadmin"))
			// the range of assigning roles belongs to the IAM subsystem, the security roles carry none
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:subsystem:iam:manager").Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-subsystem`).String()).To(Equal("iam"))
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:subsystem:iam:superadmin").Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-subsystem`).String()).To(Equal("iam"))
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:subsystem:security:manager").Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-subsystem`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:subsystem:security:superadmin").Field(`metadata.labels.user-authz\.deckhouse\.io/can-assign-subsystem`).Exists()).To(BeFalse())
			legacyUse := f.KubernetesGlobalResource("ClusterRole", "d8:use:role:admin")
			Expect(legacyUse.Exists()).To(BeTrue())
			Expect(legacyUse.Field(`metadata.labels.rbac\.deckhouse\.io/deprecated`).String()).To(Equal("true"))
			// a use-role alias stays delegatable (existing RoleBindings keep working) but may not be
			// the target of a NEW project role binding: the webhooks read this annotation
			Expect(legacyUse.Field(`metadata.labels.rbac\.deckhouse\.io/delegatable`).String()).To(Equal("true"))
			Expect(legacyUse.Field(`metadata.annotations.rbac\.deckhouse\.io/disabled-for-direct-use-in-projects`).String()).To(Equal("true"))
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:use:role:admin:kubernetes").Field(`metadata.annotations.rbac\.deckhouse\.io/disabled-for-direct-use-in-projects`).String()).To(Equal("true"))
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:namespace:admin").Field(`metadata.annotations.rbac\.deckhouse\.io/disabled-for-direct-use-in-projects`).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:manage:all:manager").Field(`metadata.annotations.rbac\.deckhouse\.io/disabled-for-direct-use-in-projects`).Exists()).To(BeFalse())

			// d8:use:dict has no alias: its bindings are migrated to d8:dict by the dict-bindings hook.
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:use:dict").Exists()).To(BeFalse())
		})

		It("Should keep the rights of the old subsystems in their aliases", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// selectors renders the clusterRoleSelectors of a role as "key=value,key notin (value)"
			// strings, the requirements sorted.
			selectors := func(name string) []string {
				var out []string
				for _, selector := range f.KubernetesGlobalResource("ClusterRole", name).Field("aggregationRule.clusterRoleSelectors").Array() {
					var pairs []string
					for key, value := range selector.Get("matchLabels").Map() {
						pairs = append(pairs, key+"="+value.String())
					}
					for _, expression := range selector.Get("matchExpressions").Array() {
						var values []string
						for _, value := range expression.Get("values").Array() {
							values = append(values, value.String())
						}
						pairs = append(pairs, expression.Get("key").String()+" "+strings.ToLower(expression.Get("operator").String())+" ("+strings.Join(values, ",")+")")
					}
					sort.Strings(pairs)
					out = append(out, strings.Join(pairs, ","))
				}
				return out
			}
			// retiredSelector selects the old lineage of a subsystem, without the use capabilities of
			// the old scheme.
			retiredSelector := func(lineage, level string) string {
				return "rbac.deckhouse.io/aggregate-to-" + lineage + "-as=" + level + ",rbac.deckhouse.io/kind notin (use)"
			}
			// narrowSelector selects the capabilities of a lineage without the modules that joined it.
			narrowSelector := func(lineage, level, joined string) string {
				return "module notin (" + joined + "),rbac.deckhouse.io/aggregate-to-" + lineage + "-as=" + level + ",rbac.deckhouse.io/kind notin (role,use)"
			}
			// lineageSelector selects a lineage of the new model, without the use capabilities of the old
			// scheme.
			lineageSelector := func(lineage, level string) string {
				return "rbac.deckhouse.io/aggregate-to-" + lineage + "-as=" + level + ",rbac.deckhouse.io/kind notin (use)"
			}
			moduleSelector := func(module, lineage, level string) string {
				return "module=" + module + ",rbac.deckhouse.io/aggregate-to-" + lineage + "-as=" + level +
					",rbac.deckhouse.io/kind=capability,rbac.deckhouse.io/scope=system"
			}
			capabilitySelector := func(capability string) string {
				return "module=user-authz,rbac.deckhouse.io/capability=" + capability + ",rbac.deckhouse.io/kind=capability"
			}

			// deckhouse, infrastructure and kubernetes were merged into cluster: their aliases select
			// the capabilities of their former modules, wherever the modules are now, and never the
			// whole cluster lineage, so a binding to an old name does not widen on upgrade.
			for _, name := range []string{
				"d8:manage:deckhouse:viewer", "d8:manage:deckhouse:manager",
				"d8:manage:infrastructure:viewer", "d8:manage:infrastructure:manager",
				"d8:manage:kubernetes:viewer", "d8:manage:kubernetes:manager",
			} {
				level := name[strings.LastIndex(name, ":")+1:]
				alias := f.KubernetesGlobalResource("ClusterRole", name)
				Expect(alias.Field(`metadata.annotations.rbac\.deckhouse\.io/deprecated-replaced-by`).String()).To(Equal("d8:subsystem:cluster:"+level), name)
				Expect(alias.Field(`metadata.labels.rbac\.deckhouse\.io/delegatable`).Exists()).To(BeFalse(), name)
				Expect(selectors(name)).NotTo(ContainElement("rbac.deckhouse.io/aggregate-to-cluster-as="+level), name)
			}
			Expect(selectors("d8:manage:deckhouse:viewer")).To(ConsistOf(
				moduleSelector("deckhouse", "cluster", "viewer"),
				moduleSelector("multitenancy-manager", "iam", "viewer"),
				retiredSelector("deckhouse", "viewer"),
			))
			Expect(selectors("d8:manage:deckhouse:manager")).To(ConsistOf(
				moduleSelector("deckhouse", "cluster", "viewer"),
				moduleSelector("deckhouse", "cluster", "manager"),
				moduleSelector("multitenancy-manager", "iam", "viewer"),
				moduleSelector("multitenancy-manager", "iam", "manager"),
				retiredSelector("deckhouse", "viewer"),
				retiredSelector("deckhouse", "manager"),
			))
			infrastructure := selectors("d8:manage:infrastructure:manager")
			Expect(infrastructure).To(HaveLen(44))
			Expect(infrastructure).To(ContainElements(
				retiredSelector("infrastructure", "viewer"),
				retiredSelector("infrastructure", "manager"),
				moduleSelector("node-manager", "cluster", "viewer"),
				moduleSelector("node-manager", "cluster", "manager"),
				moduleSelector("cloud-provider-zvirt", "cluster", "manager"),
				moduleSelector("istio", "network", "manager"),
				moduleSelector("keepalived", "network", "manager"),
				moduleSelector("local-path-provisioner", "storage", "manager"),
				moduleSelector("csi-vsphere", "cluster", "manager"),
				capabilitySelector("subsystem-capability.cluster.view_resources"),
				capabilitySelector("subsystem-capability.cluster.manage_resources"),
			))
			Expect(infrastructure).NotTo(ContainElement(capabilitySelector("subsystem-capability.network.manage_resources")))
			Expect(selectors("d8:manage:kubernetes:viewer")).To(ConsistOf(
				moduleSelector("control-plane-manager", "cluster", "viewer"),
				moduleSelector("kube-dns", "network", "viewer"),
				moduleSelector("kube-proxy", "network", "viewer"),
				capabilitySelector("subsystem-capability.cluster.view_resources"),
				capabilitySelector("subsystem-capability.network.view_resources"),
				capabilitySelector("subsystem-capability.security.view_resources"),
				capabilitySelector("subsystem-capability.storage.view_resources"),
				retiredSelector("kubernetes", "viewer"),
			))

			// security keeps the user-authz and user-authn capabilities that moved to iam
			Expect(selectors("d8:manage:security:manager")).To(ConsistOf(
				lineageSelector("security", "manager"),
				moduleSelector("user-authn", "iam", "viewer"),
				moduleSelector("user-authn", "iam", "manager"),
				moduleSelector("user-authz", "iam", "viewer"),
				moduleSelector("user-authz", "iam", "manager"),
			))
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:manage:security:manager").
				Field(`metadata.annotations.rbac\.deckhouse\.io/deprecated-replaced-by`).String()).To(Equal("d8:subsystem:security:manager"))

			// a description that sends the holder to the role of another subsystem says that the role
			// grants more and names the narrow way
			for name, hint := range map[string]string{
				"d8:manage:security:viewer":    "Rights for user-authn, user-authz come with d8:subsystem:iam:viewer, which grants more.",
				"d8:manage:deckhouse:viewer":   "Rights for multitenancy-manager come with d8:subsystem:iam:viewer, which grants more as well.",
				"d8:manage:kubernetes:manager": "Rights for kube-dns, kube-proxy come with d8:subsystem:network:manager, which grants more as well.",
			} {
				for _, annotation := range []string{`en\.meta\.deckhouse\.io/description`, `ru\.meta\.deckhouse\.io/description`} {
					Expect(f.KubernetesGlobalResource("ClusterRole", name).Field("metadata.annotations."+annotation).String()).To(ContainSubstring("rbac.deckhouse.io/capability"), name)
				}
				Expect(f.KubernetesGlobalResource("ClusterRole", name).Field(`metadata.annotations.en\.meta\.deckhouse\.io/description`).String()).To(ContainSubstring(hint), name)
			}

			// networking was renamed to network, and its alias selects the old lineage as well; the
			// other subsystems kept their names
			// keepalived joined network from infrastructure, and local-path-provisioner joined storage,
			// so the aliases of the old roles leave them out
			Expect(selectors("d8:manage:networking:manager")).To(ConsistOf(
				narrowSelector("network", "viewer", "keepalived"),
				narrowSelector("network", "manager", "keepalived"),
				retiredSelector("networking", "viewer"),
				retiredSelector("networking", "manager"),
			))
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:manage:networking:manager").
				Field(`metadata.annotations.rbac\.deckhouse\.io/deprecated-replaced-by`).String()).To(Equal("d8:subsystem:network:manager"))
			Expect(selectors("d8:manage:storage:viewer")).To(ConsistOf(narrowSelector("storage", "viewer", "local-path-provisioner")))
			Expect(selectors("d8:manage:storage:manager")).To(ConsistOf(
				narrowSelector("storage", "viewer", "local-path-provisioner"),
				narrowSelector("storage", "manager", "local-path-provisioner"),
			))
			Expect(selectors("d8:manage:observability:manager")).To(ConsistOf(lineageSelector("observability", "manager")))
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:manage:storage:viewer").
				Field(`metadata.annotations.en\.meta\.deckhouse\.io/description`).String()).To(ContainSubstring("also grants rights for local-path-provisioner"))
			// all also keeps the subsystem roles of the modules with a subsystem of their own, which
			// carry the manage kind and aggregate-to-all-as, as the previous all role selected them
			Expect(selectors("d8:manage:all:manager")).To(ConsistOf(
				lineageSelector("system", "manager"),
				"rbac.deckhouse.io/aggregate-to-all-as=manager,rbac.deckhouse.io/kind=manage",
				retiredSelector("deckhouse", "viewer"),
				retiredSelector("deckhouse", "manager"),
				retiredSelector("infrastructure", "viewer"),
				retiredSelector("infrastructure", "manager"),
				retiredSelector("kubernetes", "viewer"),
				retiredSelector("kubernetes", "manager"),
				retiredSelector("networking", "viewer"),
				retiredSelector("networking", "manager"),
			))
		})

		It("Should keep in the aliases the capabilities that still carry the lineage of an old subsystem", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// selects reports whether a selector of the role matches an object with the given labels,
			// the way the aggregation controller matches it.
			selects := func(name string, objectLabels map[string]string) bool {
				for _, raw := range f.KubernetesGlobalResource("ClusterRole", name).Field("aggregationRule.clusterRoleSelectors").Array() {
					var selector metav1.LabelSelector
					Expect(json.Unmarshal([]byte(raw.Raw), &selector)).To(Succeed(), name)
					parsed, err := metav1.LabelSelectorAsSelector(&selector)
					Expect(err).ShouldNot(HaveOccurred(), name)
					if parsed.Matches(labels.Set(objectLabels)) {
						return true
					}
				}
				return false
			}
			// oldCapability has the labels of a capability of the previous release, the way a module
			// released on its own still ships it.
			oldCapability := func(module, lineage, level string) map[string]string {
				return map[string]string{
					"heritage":                "deckhouse",
					"module":                  module,
					"rbac.deckhouse.io/kind":  "manage",
					"rbac.deckhouse.io/level": "module",
					"rbac.deckhouse.io/aggregate-to-" + lineage + "-as": level,
				}
			}

			// A module of the network table still on the networking lineage, such as ingress-nginx.
			ingressView := oldCapability("ingress-nginx", "networking", "viewer")
			ingressEdit := oldCapability("ingress-nginx", "networking", "manager")
			for _, name := range []string{"d8:manage:networking:viewer", "d8:manage:all:viewer"} {
				Expect(selects(name, ingressView)).To(BeTrue(), name)
				Expect(selects(name, ingressEdit)).To(BeFalse(), name)
			}
			for _, name := range []string{"d8:manage:networking:manager", "d8:manage:all:manager"} {
				Expect(selects(name, ingressView)).To(BeTrue(), name)
				Expect(selects(name, ingressEdit)).To(BeTrue(), name)
			}

			// A module still on the infrastructure lineage, such as loki or storage-foundation.
			lokiEdit := oldCapability("loki", "infrastructure", "manager")
			Expect(selects("d8:manage:infrastructure:manager", lokiEdit)).To(BeTrue())
			Expect(selects("d8:manage:infrastructure:viewer", lokiEdit)).To(BeFalse())
			Expect(selects("d8:manage:all:manager", lokiEdit)).To(BeTrue())
			Expect(selects("d8:manage:deckhouse:manager", lokiEdit)).To(BeFalse())

			// A capability of the new scheme that kept the old lineage.
			snapshotView := map[string]string{
				"heritage":                "deckhouse",
				"module":                  "snapshot-controller",
				"rbac.deckhouse.io/kind":  "capability",
				"rbac.deckhouse.io/scope": "system",
				"rbac.deckhouse.io/aggregate-to-infrastructure-as": "viewer",
			}
			Expect(selects("d8:manage:infrastructure:viewer", snapshotView)).To(BeTrue())

			// A customer extension of d8:manage:deckhouse:manager the way the previous release
			// documented it.
			extension := map[string]string{
				"rbac.deckhouse.io/kind":                      "manage",
				"rbac.deckhouse.io/aggregate-to-deckhouse-as": "manager",
			}
			Expect(selects("d8:manage:deckhouse:manager", extension)).To(BeTrue())
			Expect(selects("d8:manage:deckhouse:viewer", extension)).To(BeFalse())
			Expect(selects("d8:manage:all:manager", extension)).To(BeTrue())

			// The subsystem role of a module with a subsystem of its own, the way virtualization
			// ships it on the labels of the previous scheme. The previous all role of its level
			// selected it, and so does the alias; no alias of a subsystem role does.
			virtualizationManager := map[string]string{
				"heritage":                    "deckhouse",
				"module":                      "virtualization",
				"rbac.deckhouse.io/kind":      "manage",
				"rbac.deckhouse.io/level":     "subsystem",
				"rbac.deckhouse.io/subsystem": "virtualization",
				"rbac.deckhouse.io/use-role":  "user",
				"rbac.deckhouse.io/aggregate-to-virtualization-as": "superadmin",
				"rbac.deckhouse.io/aggregate-to-all-as":            "manager",
			}
			Expect(selects("d8:manage:all:manager", virtualizationManager)).To(BeTrue())
			for _, name := range []string{"d8:manage:all:viewer", "d8:manage:networking:manager", "d8:manage:kubernetes:manager", "d8:system:manager"} {
				Expect(selects(name, virtualizationManager)).To(BeFalse(), name)
			}
			Expect(selects("d8:manage:all:manager", map[string]string{
				"rbac.deckhouse.io/kind":                "use",
				"rbac.deckhouse.io/aggregate-to-all-as": "manager",
			})).To(BeFalse(), "a use role of the previous scheme")

			// A module that joined network or storage from infrastructure is left out of the alias of
			// the old role, the other modules of the lineage and the custom capabilities stay.
			newCapability := func(module, lineage, level string) map[string]string {
				return map[string]string{
					"heritage":                "deckhouse",
					"module":                  module,
					"rbac.deckhouse.io/kind":  "capability",
					"rbac.deckhouse.io/scope": "system",
					"rbac.deckhouse.io/aggregate-to-" + lineage + "-as": level,
				}
			}
			Expect(selects("d8:manage:networking:manager", newCapability("keepalived", "network", "manager"))).To(BeFalse())
			Expect(selects("d8:manage:networking:manager", newCapability("keepalived", "network", "viewer"))).To(BeFalse())
			Expect(selects("d8:manage:networking:manager", newCapability("cni-cilium", "network", "manager"))).To(BeTrue())
			Expect(selects("d8:manage:networking:manager", newCapability("cni-cilium", "network", "viewer"))).To(BeTrue())
			Expect(selects("d8:manage:storage:viewer", newCapability("local-path-provisioner", "storage", "viewer"))).To(BeFalse())
			Expect(selects("d8:manage:storage:viewer", map[string]string{
				"rbac.deckhouse.io/kind":                    "custom-capability",
				"rbac.deckhouse.io/aggregate-to-storage-as": "viewer",
			})).To(BeTrue())
			Expect(selects("d8:subsystem:network:manager", newCapability("keepalived", "network", "manager"))).To(BeTrue())
			// The subsystem viewer role aggregates keepalived, so the alias does not take it in.
			Expect(selects("d8:manage:networking:manager", map[string]string{
				"heritage":               "deckhouse",
				"module":                 "user-authz",
				"rbac.deckhouse.io/kind": "role",
				"rbac.deckhouse.io/aggregate-to-network-as": "manager",
			})).To(BeFalse())

			// The use capabilities of the old scheme granted namespace-level rights through
			// aggregate-to-kubernetes-as, and no manage role took them.
			useCapability := map[string]string{
				"heritage":               "deckhouse",
				"module":                 "cert-manager",
				"rbac.deckhouse.io/kind": "use",
				"rbac.deckhouse.io/aggregate-to-kubernetes-as": "manager",
			}
			for _, name := range []string{"d8:manage:kubernetes:manager", "d8:manage:all:manager"} {
				Expect(selects(name, useCapability)).To(BeFalse(), name)
			}

			// The roles of the new model collect none of the old lineages.
			for _, name := range []string{
				"d8:system:viewer", "d8:system:manager",
				"d8:subsystem:cluster:viewer", "d8:subsystem:cluster:manager",
				"d8:subsystem:network:viewer", "d8:subsystem:network:manager",
			} {
				for _, object := range []map[string]string{ingressView, ingressEdit, lokiEdit, snapshotView, extension} {
					Expect(selects(name, object)).To(BeFalse(), name)
				}
			}
		})

		It("Should project the bindings to the old manage roles into the namespaces the previous release did", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// projects reports whether a selector of the use-binding-selectors annotation of the role
			// matches an object with the given labels, the way user-authz-controller matches it. Every
			// selector names the kind of what it selects, so none can select every ClusterRole.
			projects := func(name string, objectLabels map[string]string) bool {
				raw := f.KubernetesGlobalResource("ClusterRole", name).Field(`metadata.annotations.user-authz\.deckhouse\.io/use-binding-selectors`)
				Expect(raw.Exists()).To(BeTrue(), name)
				var selectors []metav1.LabelSelector
				Expect(json.Unmarshal([]byte(raw.String()), &selectors)).To(Succeed(), name)
				Expect(selectors).NotTo(BeEmpty(), name)
				for i := range selectors {
					Expect(selectors[i].MatchLabels).To(HaveKey("rbac.deckhouse.io/kind"), name)
					parsed, err := metav1.LabelSelectorAsSelector(&selectors[i])
					Expect(err).ShouldNot(HaveOccurred(), name)
					if parsed.Matches(labels.Set(objectLabels)) {
						return true
					}
				}
				return false
			}
			newCapability := func(module, lineage, level string) map[string]string {
				return map[string]string{
					"heritage":                "deckhouse",
					"module":                  module,
					"rbac.deckhouse.io/kind":  "capability",
					"rbac.deckhouse.io/scope": "system",
					"rbac.deckhouse.io/aggregate-to-" + lineage + "-as": level,
				}
			}
			oldCapability := func(module, lineage, level string) map[string]string {
				return map[string]string{
					"heritage":                "deckhouse",
					"module":                  module,
					"rbac.deckhouse.io/kind":  "manage",
					"rbac.deckhouse.io/level": "module",
					"rbac.deckhouse.io/aggregate-to-" + lineage + "-as": level,
				}
			}

			// An alias carries the labels the previous release put on the role: the manage kind and
			// the use role. The projection of a manager is viewer, as on the current roles, so an
			// old and a migrated binding of one subject give the same access.
			for _, subsystem := range []string{"all", "deckhouse", "infrastructure", "kubernetes", "networking", "observability", "security", "storage"} {
				for level, useRole := range map[string]string{"viewer": "viewer", "manager": "viewer"} {
					name := "d8:manage:" + subsystem + ":" + level
					alias := f.KubernetesGlobalResource("ClusterRole", name)
					Expect(alias.Field(`metadata.labels.rbac\.deckhouse\.io/kind`).String()).To(Equal("manage"), name)
					Expect(alias.Field(`metadata.labels.rbac\.deckhouse\.io/use-role`).String()).To(Equal(useRole), name)
					Expect(alias.Field(`metadata.labels.rbac\.deckhouse\.io/scope`).Exists()).To(BeFalse(), name)
				}
			}
			// The other roles are projected the current way, or not at all.
			for _, name := range []string{"d8:use:role:viewer", "d8:use:role:admin:kubernetes", "d8:subsystem:network:manager", "d8:system:viewer"} {
				role := f.KubernetesGlobalResource("ClusterRole", name)
				Expect(role.Field(`metadata.annotations.user-authz\.deckhouse\.io/use-binding-selectors`).Exists()).To(BeFalse(), name)
				Expect(role.Field(`metadata.labels.rbac\.deckhouse\.io/kind`).String()).NotTo(Equal("manage"), name)
			}
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:use:role:viewer").Field(`metadata.labels.rbac\.deckhouse\.io/use-role`).Exists()).To(BeFalse())

			// networking: the modules of this repository that were in it, wherever they are now, and
			// the old lineage of the modules released on their own. keepalived joined network from
			// infrastructure, and a module already relabelled to network was not in the previous
			// projection, which took the manage kind only.
			Expect(projects("d8:manage:networking:viewer", newCapability("cni-cilium", "network", "viewer"))).To(BeTrue())
			Expect(projects("d8:manage:networking:viewer", newCapability("cni-cilium", "network", "manager"))).To(BeFalse())
			Expect(projects("d8:manage:networking:manager", newCapability("cni-cilium", "network", "viewer"))).To(BeTrue())
			Expect(projects("d8:manage:networking:manager", newCapability("cni-cilium", "network", "manager"))).To(BeTrue())
			Expect(projects("d8:manage:networking:manager", newCapability("istio", "network", "manager"))).To(BeTrue())
			Expect(projects("d8:manage:networking:manager", newCapability("keepalived", "network", "manager"))).To(BeFalse())
			Expect(projects("d8:manage:networking:manager", newCapability("sdn", "network", "manager"))).To(BeFalse())
			Expect(projects("d8:manage:networking:viewer", oldCapability("ingress-nginx", "networking", "viewer"))).To(BeTrue())
			Expect(projects("d8:manage:networking:viewer", oldCapability("ingress-nginx", "networking", "manager"))).To(BeFalse())
			Expect(projects("d8:manage:networking:manager", oldCapability("ingress-nginx", "networking", "manager"))).To(BeTrue())

			// infrastructure keeps the modules that moved to cluster, network and storage, and the
			// old lineage; a capability of the current scheme that kept the old lineage was not in
			// the previous projection either.
			for _, object := range []map[string]string{
				newCapability("node-manager", "cluster", "viewer"),
				newCapability("istio", "network", "viewer"),
				newCapability("keepalived", "network", "viewer"),
				newCapability("local-path-provisioner", "storage", "viewer"),
				oldCapability("loki", "infrastructure", "viewer"),
			} {
				Expect(projects("d8:manage:infrastructure:viewer", object)).To(BeTrue(), object["module"])
			}
			for _, object := range []map[string]string{
				newCapability("control-plane-manager", "cluster", "viewer"),
				newCapability("csi-nfs", "storage", "viewer"),
				newCapability("snapshot-controller", "infrastructure", "viewer"),
				oldCapability("ingress-nginx", "networking", "viewer"),
			} {
				Expect(projects("d8:manage:infrastructure:viewer", object)).To(BeFalse(), object["module"])
			}
			Expect(projects("d8:manage:kubernetes:manager", newCapability("control-plane-manager", "cluster", "manager"))).To(BeTrue())
			Expect(projects("d8:manage:kubernetes:manager", newCapability("kube-proxy", "network", "manager"))).To(BeTrue())
			Expect(projects("d8:manage:kubernetes:manager", newCapability("node-manager", "cluster", "manager"))).To(BeFalse())
			Expect(projects("d8:manage:deckhouse:manager", newCapability("multitenancy-manager", "iam", "manager"))).To(BeTrue())
			Expect(projects("d8:manage:deckhouse:manager", map[string]string{
				"rbac.deckhouse.io/kind":                      "manage",
				"rbac.deckhouse.io/aggregate-to-deckhouse-as": "manager",
				"rbac.deckhouse.io/namespace":                 "custom-ns",
			})).To(BeTrue(), "a capability of the previous scheme that extended the role into a namespace")

			// security keeps user-authz and user-authn, which moved to iam, and not the rest of iam.
			Expect(projects("d8:manage:security:manager", newCapability("cert-manager", "security", "manager"))).To(BeTrue())
			Expect(projects("d8:manage:security:manager", newCapability("user-authn", "iam", "manager"))).To(BeTrue())
			Expect(projects("d8:manage:security:manager", newCapability("multitenancy-manager", "iam", "manager"))).To(BeFalse())
			Expect(projects("d8:manage:security:manager", newCapability("operator-trivy", "security", "manager"))).To(BeFalse())
			Expect(projects("d8:manage:security:manager", oldCapability("operator-trivy", "security", "manager"))).To(BeTrue())

			// observability and storage kept their names: the old lineage with the manage kind.
			Expect(projects("d8:manage:observability:viewer", newCapability("okmeter", "observability", "viewer"))).To(BeTrue())
			Expect(projects("d8:manage:observability:viewer", oldCapability("prometheus", "observability", "viewer"))).To(BeTrue())
			Expect(projects("d8:manage:observability:viewer", newCapability("prometheus", "observability", "viewer"))).To(BeFalse())
			Expect(projects("d8:manage:storage:viewer", oldCapability("sds-node-configurator", "storage", "viewer"))).To(BeTrue())
			Expect(projects("d8:manage:storage:viewer", newCapability("csi-nfs", "storage", "viewer"))).To(BeFalse())
			Expect(projects("d8:manage:storage:viewer", newCapability("local-path-provisioner", "storage", "viewer"))).To(BeFalse())

			// all: every module of this repository and every old lineage, at the level of the role
			// only, as the previous release reached the capabilities through the subsystem roles of
			// that level.
			for _, object := range []map[string]string{
				newCapability("deckhouse", "cluster", "viewer"),
				newCapability("node-manager", "cluster", "viewer"),
				newCapability("cni-cilium", "network", "viewer"),
				newCapability("keepalived", "network", "viewer"),
				newCapability("local-path-provisioner", "storage", "viewer"),
				newCapability("user-authn", "iam", "viewer"),
				newCapability("multitenancy-manager", "iam", "viewer"),
				newCapability("cert-manager", "security", "viewer"),
				newCapability("okmeter", "observability", "viewer"),
				oldCapability("loki", "infrastructure", "viewer"),
				oldCapability("ingress-nginx", "networking", "viewer"),
				oldCapability("prometheus", "observability", "viewer"),
				oldCapability("sds-node-configurator", "storage", "viewer"),
			} {
				Expect(projects("d8:manage:all:viewer", object)).To(BeTrue(), object["module"])
			}
			for _, object := range []map[string]string{
				newCapability("node-manager", "cluster", "manager"),
				newCapability("csi-nfs", "storage", "viewer"),
				newCapability("virtualization", "cluster", "viewer"),
				oldCapability("ingress-nginx", "networking", "manager"),
			} {
				Expect(projects("d8:manage:all:viewer", object)).To(BeFalse(), object["module"])
			}
			Expect(projects("d8:manage:all:manager", newCapability("node-manager", "cluster", "manager"))).To(BeTrue())
			Expect(projects("d8:manage:all:manager", oldCapability("ingress-nginx", "networking", "manager"))).To(BeTrue())

			// all also reaches the namespaces of the modules with a subsystem of their own through
			// their subsystem roles of its level, as the previous release did, and the controller
			// follows those roles to their capabilities.
			ownSubsystemRole := func(level string) map[string]string {
				return map[string]string{
					"heritage":                              "deckhouse",
					"module":                                "virtualization",
					"rbac.deckhouse.io/kind":                "manage",
					"rbac.deckhouse.io/level":               "subsystem",
					"rbac.deckhouse.io/subsystem":           "virtualization",
					"rbac.deckhouse.io/aggregate-to-all-as": level,
				}
			}
			Expect(projects("d8:manage:all:viewer", ownSubsystemRole("viewer"))).To(BeTrue())
			Expect(projects("d8:manage:all:viewer", ownSubsystemRole("manager"))).To(BeFalse())
			Expect(projects("d8:manage:all:manager", ownSubsystemRole("manager"))).To(BeTrue())
			Expect(projects("d8:manage:networking:manager", ownSubsystemRole("manager"))).To(BeFalse())

			// The use capabilities of the previous scheme carried no namespace and were never part
			// of the projection.
			Expect(projects("d8:manage:kubernetes:viewer", map[string]string{
				"heritage":               "deckhouse",
				"module":                 "log-shipper",
				"rbac.deckhouse.io/kind": "use",
				"rbac.deckhouse.io/aggregate-to-kubernetes-as": "viewer",
			})).To(BeFalse())
		})

		It("Should keep in the use aliases the use capabilities of the previous scheme", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// selectors renders the clusterRoleSelectors of a role as sorted "key=value" strings.
			selectors := func(name string) []string {
				var out []string
				for _, selector := range f.KubernetesGlobalResource("ClusterRole", name).Field("aggregationRule.clusterRoleSelectors").Array() {
					Expect(selector.Get("matchExpressions").Exists()).To(BeFalse(), name)
					var pairs []string
					for key, value := range selector.Get("matchLabels").Map() {
						pairs = append(pairs, key+"="+value.String())
					}
					sort.Strings(pairs)
					out = append(out, strings.Join(pairs, ","))
				}
				return out
			}
			// selects reports whether a selector of the role matches an object with the given labels.
			selects := func(name string, objectLabels map[string]string) bool {
				for _, raw := range f.KubernetesGlobalResource("ClusterRole", name).Field("aggregationRule.clusterRoleSelectors").Array() {
					var selector metav1.LabelSelector
					Expect(json.Unmarshal([]byte(raw.Raw), &selector)).To(Succeed(), name)
					parsed, err := metav1.LabelSelectorAsSelector(&selector)
					Expect(err).ShouldNot(HaveOccurred(), name)
					if parsed.Matches(labels.Set(objectLabels)) {
						return true
					}
				}
				return false
			}
			oldUse := func(lineage, level string) string {
				return "rbac.deckhouse.io/aggregate-to-" + lineage + "-as=" + level + ",rbac.deckhouse.io/kind=use"
			}

			// The previous release collected the use capabilities of a level and of the levels below
			// it through the d8:use:role:<level>:kubernetes chain, by kind use and the kubernetes
			// lineage. Both aliases of a level keep them next to the namespace lineage.
			// d8:use:role:<level> also selected kind use with aggregate-to-all-as at its level, which
			// the use roles of the modules with a subsystem of their own carry, and keeps that too;
			// the kubernetes variant did not.
			levels := []string{"viewer", "user", "manager", "admin"}
			for i, level := range levels {
				want := []string{"rbac.deckhouse.io/aggregate-to-namespace-as=" + level}
				for _, lower := range levels[:i+1] {
					want = append(want, oldUse("kubernetes", lower))
				}
				Expect(selectors("d8:use:role:"+level+":kubernetes")).To(ConsistOf(want), level)
				Expect(selectors("d8:use:role:"+level)).To(ConsistOf(append(want, oldUse("all", level))), level)
			}

			// The use role of virtualization for its user level, as the module ships it on the labels of
			// the previous scheme.
			virtualizationUser := map[string]string{
				"heritage":               "deckhouse",
				"module":                 "virtualization",
				"rbac.deckhouse.io/kind": "use",
				"rbac.deckhouse.io/aggregate-to-virtualization-as": "manager",
				"rbac.deckhouse.io/aggregate-to-all-as":            "user",
			}
			Expect(selects("d8:use:role:user", virtualizationUser)).To(BeTrue())
			for _, name := range []string{"d8:use:role:user:kubernetes", "d8:use:role:viewer", "d8:use:role:manager", "d8:use:role:admin", "d8:namespace:user"} {
				Expect(selects(name, virtualizationUser)).To(BeFalse(), name)
			}

			// A use capability of a module released on its own, such as log-shipper, and a custom
			// one of the previous release.
			logShipperView := map[string]string{
				"heritage":               "deckhouse",
				"module":                 "log-shipper",
				"rbac.deckhouse.io/kind": "use",
				"rbac.deckhouse.io/aggregate-to-kubernetes-as": "viewer",
			}
			logShipperEdit := map[string]string{
				"heritage":               "deckhouse",
				"module":                 "log-shipper",
				"rbac.deckhouse.io/kind": "use",
				"rbac.deckhouse.io/aggregate-to-kubernetes-as": "manager",
			}
			customView := map[string]string{
				"rbac.deckhouse.io/kind":                       "use",
				"rbac.deckhouse.io/aggregate-to-kubernetes-as": "viewer",
			}
			for _, name := range []string{"d8:use:role:viewer", "d8:use:role:viewer:kubernetes", "d8:use:role:user", "d8:use:role:admin:kubernetes"} {
				Expect(selects(name, logShipperView)).To(BeTrue(), name)
				Expect(selects(name, customView)).To(BeTrue(), name)
			}
			for _, name := range []string{"d8:use:role:viewer", "d8:use:role:user:kubernetes"} {
				Expect(selects(name, logShipperEdit)).To(BeFalse(), name)
			}
			for _, name := range []string{"d8:use:role:manager", "d8:use:role:admin"} {
				Expect(selects(name, logShipperEdit)).To(BeTrue(), name)
			}

			// What the previous use roles did not collect stays out: a manage capability on the
			// kubernetes lineage, the networking lineage that the use capabilities of cni-cilium
			// carried as well, a custom capability of the current scheme on the kubernetes lineage,
			// kind use with aggregate-to-all-as of a level below, which the use role of that level
			// collected instead, and a manage or custom object with aggregate-to-all-as.
			for _, object := range []map[string]string{
				{"rbac.deckhouse.io/kind": "manage", "rbac.deckhouse.io/aggregate-to-kubernetes-as": "viewer"},
				{"rbac.deckhouse.io/kind": "use", "rbac.deckhouse.io/aggregate-to-networking-as": "viewer"},
				{"rbac.deckhouse.io/kind": "custom-capability", "rbac.deckhouse.io/aggregate-to-kubernetes-as": "viewer"},
				{"rbac.deckhouse.io/kind": "use", "rbac.deckhouse.io/aggregate-to-all-as": "viewer"},
				{"rbac.deckhouse.io/kind": "manage", "rbac.deckhouse.io/aggregate-to-all-as": "admin"},
				{"rbac.deckhouse.io/kind": "custom-capability", "rbac.deckhouse.io/aggregate-to-all-as": "admin"},
			} {
				Expect(selects("d8:use:role:admin", object)).To(BeFalse(), "%v", object)
			}

			// The roles of the new model do not take them.
			for _, name := range []string{"d8:namespace:viewer", "d8:namespace:user", "d8:namespace:admin"} {
				Expect(f.KubernetesGlobalResource("ClusterRole", name).Exists()).To(BeTrue(), name)
				Expect(selects(name, logShipperView)).To(BeFalse(), name)
				Expect(selects(name, customView)).To(BeFalse(), name)
			}
		})

		It("Should leave the use capabilities out of the cluster-wide roles", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// selects reports whether a selector of the role matches an object with the given labels.
			selects := func(name string, objectLabels map[string]string) bool {
				role := f.KubernetesGlobalResource("ClusterRole", name)
				Expect(role.Exists()).To(BeTrue(), name)
				for _, raw := range role.Field("aggregationRule.clusterRoleSelectors").Array() {
					var selector metav1.LabelSelector
					Expect(json.Unmarshal([]byte(raw.Raw), &selector)).To(Succeed(), name)
					parsed, err := metav1.LabelSelectorAsSelector(&selector)
					Expect(err).ShouldNot(HaveOccurred(), name)
					if parsed.Matches(labels.Set(objectLabels)) {
						return true
					}
				}
				return false
			}
			// onLineage has the labels of an object of the given kind with the aggregation label of a
			// lineage, the way a module released on its own or a custom object may carry it.
			onLineage := func(kind, lineage, level string) map[string]string {
				return map[string]string{
					"heritage":               "deckhouse",
					"module":                 "some-module",
					"rbac.deckhouse.io/kind": kind,
					"rbac.deckhouse.io/aggregate-to-" + lineage + "-as": level,
				}
			}

			// A use capability gives rules in a namespace. With the label of a subsystem or of the
			// system scope it would give them in every namespace, so no cluster-wide role takes it,
			// while a capability of the same lineage is taken.
			for _, subsystem := range []string{"iam", "security", "cluster", "delivery", "network", "storage", "observability", "managed-services"} {
				for _, level := range []string{"viewer", "manager", "superadmin"} {
					name := "d8:subsystem:" + subsystem + ":" + level
					Expect(selects(name, onLineage("use", subsystem, level))).To(BeFalse(), name)
					Expect(selects(name, onLineage("capability", subsystem, level))).To(BeTrue(), name)
				}
			}
			for _, level := range []string{"viewer", "manager", "superadmin"} {
				name := "d8:system:" + level
				Expect(selects(name, onLineage("use", "system", level))).To(BeFalse(), name)
				Expect(selects(name, onLineage("capability", "system", level))).To(BeTrue(), name)
			}

			// The aliases of the old roles leave it out too, by the lineage of the new subsystem and
			// by the old one.
			for name, lineage := range map[string]string{
				"d8:manage:all:manager":           "system",
				"d8:manage:security:manager":      "security",
				"d8:manage:observability:manager": "observability",
				"d8:manage:storage:manager":       "storage",
				"d8:manage:networking:manager":    "network",
				"d8:manage:kubernetes:manager":    "kubernetes",
			} {
				Expect(selects(name, onLineage("use", lineage, "manager"))).To(BeFalse(), name)
				Expect(selects(name, onLineage("capability", lineage, "manager"))).To(BeTrue(), name)
			}
		})

		It("Should create aggregated custom ClusterRoles with cascading selectors and no rules of their own", func() {
			levels := []string{"User", "PrivilegedUser", "Editor", "Admin", "ClusterEditor", "ClusterAdmin"}
			names := []string{"user", "privileged-user", "editor", "admin", "cluster-editor", "cluster-admin"}
			for i, name := range names {
				role := f.KubernetesGlobalResource("ClusterRole", "user-authz:"+name+":custom")
				Expect(role.Exists()).To(BeTrue(), name)
				Expect(role.Field("rules").Exists()).To(BeFalse(), "%s: rules are owned by the aggregation controller", name)
				selectors := role.Field("aggregationRule.clusterRoleSelectors").Array()
				Expect(selectors).To(HaveLen(i+1), name)
				for j, selector := range selectors {
					Expect(selector.Get("matchLabels.user-authz\\.deckhouse\\.io/access-level").String()).To(Equal(levels[j]), name)
				}
			}
		})

		It("Should not render bindings for rules: user-authz-controller owns them", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:admin").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:admin:custom").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "user-authz:testenev:additional-role:cluster-write-all").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("RoleBinding", "testenv", "user-authz:testenev-namespaced:editor").Exists()).To(BeFalse())
		})

		It("Should deploy user-authz-controller with its RBAC", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", "d8-user-authz", "user-authz-controller")
			Expect(deployment.Exists()).To(BeTrue())
			Expect(deployment.Field("spec.template.spec.serviceAccountName").String()).To(Equal("controller"))
			Expect(deployment.Field("spec.template.spec.containers.0.name").String()).To(Equal("user-authz-controller"))
			Expect(f.KubernetesResource("ServiceAccount", "d8-user-authz", "controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("PodDisruptionBudget", "d8-user-authz", "user-authz-controller").Exists()).To(BeTrue())
			// The alert rules select job="user-authz-controller": the PodMonitor takes the job name from
			// the app label of the pods, so both must stay in place.
			podMonitor := f.KubernetesResource("PodMonitor", "d8-monitoring", "user-authz-controller")
			Expect(podMonitor.Exists()).To(BeTrue())
			Expect(podMonitor.Field("spec.jobLabel").String()).To(Equal("app"))
			Expect(deployment.Field("spec.template.metadata.labels.app").String()).To(Equal("user-authz-controller"))
			Expect(deployment.Field("spec.template.spec.containers.0.env").String()).ToNot(ContainSubstring("HA_MODE"))
			// readyz runs cache-sync and api-access in sequence, 2 s each; the probe must outlast both.
			Expect(deployment.Field("spec.template.spec.containers.0.readinessProbe.timeoutSeconds").Int()).To(BeNumerically(">=", 5))

			role := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:controller")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(ContainSubstring("clusterauthorizationrules/status"))
			Expect(role.Field("rules").String()).To(ContainSubstring(`"bind"`))
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:controller").Field("subjects.0.name").String()).To(Equal("controller"))

			// The leader-election lease lives in the module's own namespace, so its grant is a Role
			// and must not be part of the ClusterRole: cluster-wide, it would let the controller
			// take over the lease of any other component in the cluster.
			Expect(role.Field("rules").String()).NotTo(ContainSubstring("leases"))
			lease := f.KubernetesResource("Role", "d8-user-authz", "controller")
			Expect(lease.Exists()).To(BeTrue())
			Expect(lease.Field("rules").String()).To(ContainSubstring("leases"))
			Expect(f.KubernetesResource("RoleBinding", "d8-user-authz", "controller").
				Field("subjects.0.name").String()).To(Equal("controller"))
		})

		// The other half of "no component, no alerts": with multi-tenancy on, the components exist
		// and so must the rules that watch them.
		It("Should render the alerts of the components multi-tenancy deploys", func() {
			webhook := f.KubernetesResource("PrometheusRule", "d8-system", "user-authz-user-authz-webhook")
			Expect(webhook.Exists()).To(BeTrue())
			Expect(webhook.Field("spec.groups").String()).To(ContainSubstring("D8UserAuthzWebhookTargetDown"))
			Expect(webhook.Field("spec.groups").String()).To(ContainSubstring("D8UserAuthzRevocationCheckStale"))
			Expect(webhook.Field("spec.groups").String()).To(ContainSubstring(`user_authz_webhook_revocation_state{job=\"user-authz-webhook\", namespace=\"d8-user-authz\", state=\"stale\"}`))
			// An instance that never lists the revocations stays unsynced, never stale (it has nothing
			// to answer from), and fails open: a revoked token works through its master.
			Expect(webhook.Field("spec.groups").String()).To(ContainSubstring("D8UserAuthzRevocationCheckNotSynced"))
			Expect(webhook.Field("spec.groups").String()).To(ContainSubstring(`user_authz_webhook_revocation_state{job=\"user-authz-webhook\", namespace=\"d8-user-authz\", state=\"unsynced\"}`))
			// The label is the Prometheus template of the alert, not a Helm one.
			Expect(webhook.Field("spec.groups").String()).To(ContainSubstring("{{ $labels.pod }}"))

			browser := f.KubernetesResource("PrometheusRule", "d8-system", "user-authz-permission-browser-apiserver")
			Expect(browser.Exists()).To(BeTrue())
			Expect(browser.Field("spec.groups").String()).To(ContainSubstring("D8UserAuthzPermissionBrowserUnavailable"))
		})

		// An instance that is not ready is precisely the one the alerts are about — its rules are
		// unlisted or no longer tracking the cluster. Dropping it from the scrape would leave
		// RulesNotSynced, RulesWatchErrors, DirectoryDiverged and RulePropagationLag blind at the
		// moment each became true.
		It("Should scrape instances that are not ready", func() {
			for _, name := range []string{"user-authz-webhook", "permission-browser-apiserver"} {
				pm := f.KubernetesResource("PodMonitor", "d8-monitoring", name)
				Expect(pm.Exists()).To(BeTrue())
				Expect(pm.Field("spec.podMetricsEndpoints").String()).
					NotTo(ContainSubstring("pod_ready"), "PodMonitor %s must not drop unready pods", name)
			}
		})

		It("Should deploy authorization webhook and supporting objects", func() {
			Expect(f.KubernetesResource("DaemonSet", "d8-user-authz", "user-authz-webhook").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "control-plane-configurator").Field("data.ca").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "apiserver-authentication-requestheader-client-ca").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-user-authz", "user-authz-webhook").Exists()).To(BeTrue())

			// The rendered config.json is gone: both consumers read the rules from the API. Its
			// absence is asserted rather than merely unmentioned, because bringing it back would
			// quietly restore a second, slower source of the same truth.
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "user-authz-webhook").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("DaemonSet", "d8-user-authz", "user-authz-webhook").
				Field("spec.template.spec.volumes").String()).NotTo(ContainSubstring("user-authz-webhook-config"))
			Expect(f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver").
				Field("spec.template.spec.volumes").String()).NotTo(ContainSubstring("user-authz-webhook-config"))

			// Mirrors enableMultiTenancy for the multitenancy.py and identity_privilege.py
			// validating webhooks.
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "d8-user-authz-multitenancy-state").Field("data.enableMultiTenancy").String()).To(Equal("true"))
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "kube-rbac-proxy-ca.crt").Exists()).To(BeTrue())
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

		It("Should probe the webhook without depending on the API server for liveness", func() {
			ds := f.KubernetesResource("DaemonSet", "d8-user-authz", "user-authz-webhook")
			Expect(ds.Exists()).To(BeTrue())

			// Every probe needs an explicit timeout: /healthcheck talks over the network, and the
			// kubelet's default for an exec probe is one second.
			for _, probe := range []string{"livenessProbe", "readinessProbe"} {
				field := "spec.template.spec.containers.0." + probe
				Expect(ds.Field(field).Exists()).To(BeTrue(), probe)
				Expect(ds.Field(field+".timeoutSeconds").Int()).To(BeNumerically(">=", 5), probe)
			}

			// Liveness must ask the local question and readiness the one that involves the API
			// server; swapping them is what let the kubelet kill the webhook during an API server
			// restart.
			Expect(ds.Field("spec.template.spec.containers.0.livenessProbe.exec.command").String()).To(ContainSubstring("healthz"))
			Expect(ds.Field("spec.template.spec.containers.0.readinessProbe.exec.command").String()).To(ContainSubstring("readyz"))

			// And there must be no startupProbe. It ran readyz, which needs the API server, and
			// killed the container after five minutes of it failing - so a master coming back
			// without its API server yet put the fail-closed authorizer into CrashLoopBackOff.
			// It guarded nothing: liveness asks healthz, which answers from the process alone.
			Expect(ds.Field("spec.template.spec.containers.0.startupProbe").Exists()).To(BeFalse())

			// The metrics sidecar needs the same explicit timeout: a flapping sidecar makes the
			// Pod unready, which holds the rollout of the whole DaemonSet.
			for _, probe := range []string{"livenessProbe", "readinessProbe"} {
				field := "spec.template.spec.containers.1." + probe
				Expect(ds.Field(field).Exists()).To(BeTrue(), probe)
				Expect(ds.Field(field+".timeoutSeconds").Int()).To(BeNumerically(">=", 5), probe)
			}
		})

		It("Should expose the webhook metrics for Prometheus", func() {
			ds := f.KubernetesResource("DaemonSet", "d8-user-authz", "user-authz-webhook")
			Expect(ds.Exists()).To(BeTrue())

			// The webhook serves metrics on 127.0.0.1 and the sidecar publishes them on the node.
			Expect(ds.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
			Expect(ds.Field("spec.template.spec.containers.1.ports.0.containerPort").Int()).To(Equal(int64(4233)))
			Expect(ds.Field("spec.template.spec.containers.1.ports.0.name").String()).To(Equal("https-metrics"))
			Expect(ds.Field("spec.template.spec.containers.1.args").String()).To(ContainSubstring(":4233"))
			Expect(ds.Field("spec.template.spec.containers.1.env").String()).To(ContainSubstring("http://127.0.0.1:4243/metrics"))
			// The sidecar authorizes scrapers against the virtual subresource granted in rbac-to-us.
			Expect(ds.Field("spec.template.spec.containers.1.env").String()).To(ContainSubstring("prometheus-metrics"))
			role := f.KubernetesResource("Role", "d8-user-authz", "access-to-webhook-prometheus-metrics")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(ContainSubstring("daemonsets/prometheus-metrics"))
			Expect(role.Field("rules").String()).To(ContainSubstring("user-authz-webhook"))
			Expect(f.KubernetesResource("RoleBinding", "d8-user-authz", "access-to-webhook-prometheus-metrics").Exists()).To(BeTrue())
			// The sidecar needs d8:rbac-proxy to ask the API server about the scraper.
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:webhook:rbac-proxy").Exists()).To(BeTrue())

			// The alert rules select job="user-authz-webhook": the PodMonitor takes the job name from
			// the app label of the pods, so both must stay in place.
			podMonitor := f.KubernetesResource("PodMonitor", "d8-monitoring", "user-authz-webhook")
			Expect(podMonitor.Exists()).To(BeTrue())
			Expect(podMonitor.Field("spec.jobLabel").String()).To(Equal("app"))
			Expect(podMonitor.Field("spec.podMetricsEndpoints.0.port").String()).To(Equal("https-metrics"))
			Expect(ds.Field("spec.template.metadata.labels.app").String()).To(Equal("user-authz-webhook"))

			rule := f.KubernetesResource("PrometheusRule", "d8-system", "user-authz-user-authz-webhook")
			Expect(rule.Exists()).To(BeTrue())
			Expect(rule.Field("spec.groups").String()).To(ContainSubstring("D8UserAuthzWebhookRulesQuarantined"))
			// Field().String() hands back JSON, so the selector's quotes arrive escaped.
			Expect(rule.Field("spec.groups").String()).To(ContainSubstring(`job=\"user-authz-webhook\"`))
		})

		// A rollout of the single non-HA replica used to take the aggregated API down: the strategy
		// was hardcoded to maxSurge 0 / maxUnavailable 1, so the only pod was removed before its
		// replacement existed and the APIService had no endpoints for the whole restart (16 s
		// measured). Without HA the Deployment now keeps the default strategy, which surges first.
		It("Should roll permission-browser without taking its API down when there is one replica", func() {
			deployment := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
			Expect(deployment.Exists()).To(BeTrue())
			Expect(deployment.Field("spec.replicas").Int()).To(BeEquivalentTo(1))
			Expect(deployment.Field("spec.strategy").Exists()).To(BeFalse(),
				"no strategy at all: the Deployment default surges a pod before removing the old one; an explicit Recreate would take the API down harder")

			// The budget the chart already carries: with one replica it permits the eviction of
			// that replica, which is the same as a minAvailable of zero.
			pdb := f.KubernetesResource("PodDisruptionBudget", "d8-user-authz", "permission-browser-apiserver")
			Expect(pdb.Exists()).To(BeTrue())
			Expect(pdb.Field("spec.maxUnavailable").Int()).To(BeEquivalentTo(1))
		})

		It("Should expose the permission-browser metrics for Prometheus", func() {
			deployment := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
			Expect(deployment.Exists()).To(BeTrue())

			// The apiserver's own port is behind the aggregation layer, so the metrics come out of a
			// loopback endpoint through the sidecar instead.
			Expect(deployment.Field("spec.template.spec.containers.1.name").String()).To(Equal("kube-rbac-proxy"))
			Expect(deployment.Field("spec.template.spec.containers.1.ports.0.containerPort").Int()).To(Equal(int64(4276)))
			Expect(deployment.Field("spec.template.spec.containers.1.ports.0.name").String()).To(Equal("https-metrics"))
			// The upstream is a different port from the one the sidecar listens on. They used to be
			// the same number, which works only because one is bound to the pod IP and the other to
			// loopback: a sidecar told to listen on 0.0.0.0 would collide with the apiserver's own
			// listener and proxy to itself.
			Expect(deployment.Field("spec.template.spec.containers.1.env").String()).To(ContainSubstring("http://127.0.0.1:4277/metrics"))
			Expect(deployment.Field("spec.template.spec.containers.1.env").String()).NotTo(ContainSubstring("http://127.0.0.1:4276/metrics"))

			role := f.KubernetesResource("Role", "d8-user-authz", "access-to-permission-browser-apiserver-prometheus-metrics")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(ContainSubstring("deployments/prometheus-metrics"))
			Expect(f.KubernetesResource("RoleBinding", "d8-user-authz", "access-to-permission-browser-apiserver-prometheus-metrics").Exists()).To(BeTrue())

			// The alert rules select job="permission-browser-apiserver"; the PodMonitor takes the job
			// name from the app label of the pods.
			podMonitor := f.KubernetesResource("PodMonitor", "d8-monitoring", "permission-browser-apiserver")
			Expect(podMonitor.Exists()).To(BeTrue())
			Expect(podMonitor.Field("spec.jobLabel").String()).To(Equal("app"))
			Expect(deployment.Field("spec.template.metadata.labels.app").String()).To(Equal("permission-browser-apiserver"))
		})

		It("Should let the webhook and permission-browser watch the ClusterAuthorizationRules", func() {
			// Both read the multi-tenancy options straight from the rules instead of a rendered file.
			webhookRole := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:webhook")
			Expect(webhookRole.Exists()).To(BeTrue())
			Expect(webhookRole.Field("rules").String()).To(ContainSubstring("clusterauthorizationrules"))

			browserRole := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:permission-browser-apiserver")
			Expect(browserRole.Exists()).To(BeTrue())
			Expect(browserRole.Field("rules").String()).To(ContainSubstring("clusterauthorizationrules"))
		})

		// The revocation check of user-authn shares the webhook's process and listener. Its entry in
		// the API server's authorizer chain comes from control-plane-manager, which adds it only
		// when this ConfigMap names the URL.
		It("Should serve the revocation check of user-authn", func() {
			configurator := f.KubernetesResource("ConfigMap", "d8-user-authz", "control-plane-configurator")
			Expect(configurator.Field("data.url").String()).To(Equal("https://127.0.0.1:40443/"))
			Expect(configurator.Field("data.revocationURL").String()).To(Equal("https://127.0.0.1:40443/revocation"))

			// It follows the Revocation objects of user-authn, and only reads them.
			webhookRole := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:webhook")
			const dexRule = `rules.#(apiGroups.0=="dex.coreos.com")`
			Expect(webhookRole.Field(dexRule + ".resources").AsStringSlice()).To(Equal([]string{"revocations"}))
			Expect(webhookRole.Field(dexRule + ".verbs").AsStringSlice()).To(ConsistOf("get", "list", "watch"))
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
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:bulk-sar-creator").Field("rules").String()).
				To(ContainSubstring("bulksubjectaccessreviews/nonself"))
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:self-bulk-sar-creator").Field("rules").String()).
				ToNot(ContainSubstring("bulksubjectaccessreviews/nonself"))
			selfBulkSARBinding := f.KubernetesGlobalResource(
				"ClusterRoleBinding",
				"d8:user-authz:self-bulk-sar-creator:system-authenticated",
			)
			Expect(selfBulkSARBinding.Exists()).To(BeTrue())
			Expect(selfBulkSARBinding.Field("roleRef.name").String()).
				To(Equal("d8:user-authz:self-bulk-sar-creator"))
			Expect(selfBulkSARBinding.Field("subjects.0.kind").String()).To(Equal("Group"))
			Expect(selfBulkSARBinding.Field("subjects.0.name").String()).To(Equal("system:authenticated"))
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:permission-browser-apiserver").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:permission-browser-apiserver:auth-delegator").Exists()).To(BeTrue())
		})

		It("Should render permission-browser availability alert", func() {
			rule := f.KubernetesResource("PrometheusRule", "d8-system", "user-authz-permission-browser-apiserver")
			Expect(rule.Exists()).To(BeTrue())
			Expect(rule.Field("spec.groups").String()).To(ContainSubstring("D8UserAuthzPermissionBrowserUnavailable"))
		})

		It("Should configure permission-browser-apiserver deployment correctly", func() {
			deploy := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
			Expect(deploy.Field("spec.template.spec.containers.0.args").String()).To(ContainSubstring("--secure-port=8443"))
			Expect(deploy.Field("spec.template.spec.containers.0.ports.0.containerPort").Int()).To(Equal(int64(8443)))
			Expect(deploy.Field("spec.template.spec.containers.0.ports.0.name").String()).To(Equal("https"))
		})

		It("Should let every authenticated user create a SelfScopedAccessReview", func() {
			role := f.KubernetesGlobalResource("ClusterRole", "d8:user-authz:self-scoped-access-reviewer")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(MatchJSON(
				`[{"apiGroups":["authorization.deckhouse.io"],"resources":["selfscopedaccessreviews"],"verbs":["create"]}]`))

			binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authz:self-scoped-access-reviewer:system-authenticated")
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("roleRef.name").String()).To(Equal("d8:user-authz:self-scoped-access-reviewer"))
			Expect(binding.Field("subjects").String()).To(MatchJSON(
				`[{"apiGroup":"rbac.authorization.k8s.io","kind":"Group","name":"system:authenticated"}]`))
		})

		// The multitenancy-manager state is read at run time from the state ConfigMap, so toggling
		// the module does not roll permission-browser.
		It("Should let permission-browser read the multitenancy state, not take it as an argument", func() {
			deploy := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
			for _, arg := range deploy.Field("spec.template.spec.containers.0.args").AsStringSlice() {
				Expect(arg).NotTo(HavePrefix("--multitenancy-manager-enabled"))
			}

			role := f.KubernetesResource("Role", "d8-user-authz", "permission-browser-apiserver:multitenancy-state-reader")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(MatchJSON(
				`[{"apiGroups":[""],"resources":["configmaps"],"resourceNames":["d8-user-authz-multitenancy-state"],"verbs":["get","list","watch"]}]`))
			binding := f.KubernetesResource("RoleBinding", "d8-user-authz", "permission-browser-apiserver:multitenancy-state-reader")
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("roleRef.name").String()).To(Equal("permission-browser-apiserver:multitenancy-state-reader"))
			Expect(binding.Field("subjects").String()).To(MatchJSON(
				`[{"kind":"ServiceAccount","name":"permission-browser-apiserver","namespace":"d8-user-authz"}]`))
		})

		It("Should tell permission-browser that the authorization webhook is not in the chain without control-plane-manager", func() {
			deploy := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
			Expect(deploy.Field("spec.template.spec.containers.0.args").AsStringSlice()).
				To(ContainElement("--authorization-webhook-enabled=false"))
		})

		Context("With control-plane-manager", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd", "prometheus", "control-plane-manager"]`)
				f.HelmRender()
			})

			It("Should tell permission-browser that the authorization webhook is in the chain", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				deploy := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
				Expect(deploy.Field("spec.template.spec.containers.0.args").AsStringSlice()).
					To(ContainElement("--authorization-webhook-enabled=true"))
			})
		})

		Context("Without the control-plane configurator", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd", "prometheus", "control-plane-manager"]`)
				f.ValuesSet("userAuthz.controlPlaneConfigurator.enabled", false)
				f.HelmRender()
			})

			It("Should tell permission-browser that the authorization webhook is not in the chain", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())
				deploy := f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver")
				Expect(deploy.Field("spec.template.spec.containers.0.args").AsStringSlice()).
					To(ContainElement("--authorization-webhook-enabled=false"))
			})
		})

		It("Should configure APIService correctly", func() {
			apiService := f.KubernetesGlobalResource("APIService", "v1alpha1.authorization.deckhouse.io")
			Expect(apiService.Field("spec.group").String()).To(Equal("authorization.deckhouse.io"))
			Expect(apiService.Field("spec.version").String()).To(Equal("v1alpha1"))
			Expect(apiService.Field("spec.service.name").String()).To(Equal("permission-browser-apiserver"))
			Expect(apiService.Field("spec.service.namespace").String()).To(Equal("d8-user-authz"))
		})
	})

	// Both of these contexts used to be named for a ClusterAuthorizationRule they set up through
	// userAuthz.internal.crds. The rules do not travel through values any more - the webhook reads
	// them from the API - so the setup quietly became empty and the two tests rendered the
	// defaults, twice, under names that promised otherwise. What is still true and worth pinning
	// is what multi-tenancy being off does to the render, so that is what this checks, once.
	Context("Protecting the webhook configuration of the Deckhouse modules", func() {
		const policyName = "webhook-handler-configuration.user-authz.deckhouse.io"

		It("Should not render the policy without ValidatingAdmissionPolicy in the cluster", func() {
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", policyName).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", policyName).Exists()).To(BeFalse())
		})

		It("Should leave the configuration to webhook-handler, the platform components and the superadmins", func() {
			f.HelmRender(WithAPIVersions(
				"admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy",
				"admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding",
			))
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", policyName)
			Expect(policy.Exists()).To(BeTrue())
			Expect(policy.Field("metadata.labels.heritage").String()).To(Equal("deckhouse"))
			Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))
			rule := policy.Field("spec.matchConstraints.resourceRules.0")
			Expect(rule.Get("apiGroups").String()).To(MatchJSON(`["admissionregistration.k8s.io"]`))
			Expect(rule.Get("operations").String()).To(MatchJSON(`["CREATE","UPDATE","DELETE"]`))
			Expect(rule.Get("resources").String()).To(MatchJSON(`["validatingwebhookconfigurations"]`))

			// shell-operator names the configurations <name>-<configurationId>, "-hooks" by default.
			conditions := policy.Field("spec.matchConditions").String()
			Expect(conditions).To(ContainSubstring(`request.name.startsWith(\"d8-deckhouse-validating-webhook-handler-\")`))
			Expect(conditions).To(ContainSubstring(`system:serviceaccounts:d8-system`))
			Expect(conditions).To(ContainSubstring(`dhctl`))
			Expect(policy.Field("spec.validations.0.expression").String()).To(Equal(
				`authorizer.group("admissionregistration.k8s.io").resource("validatingwebhookconfigurations")` +
					`.subresource("deckhouse-webhook-handler").name(request.name).check("update").allowed()`))

			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", policyName)
			Expect(binding.Field("spec.policyName").String()).To(Equal(policyName))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))

			// The superadmins hold the permission the policy checks, and the managers no longer write
			// admission webhooks and policies at all: the policy and its binding are not subject to
			// admission, so only RBAC keeps them.
			admission := f.KubernetesGlobalResource("ClusterRole", "d8:subsystem-capability:security:manage_admission_resources")
			Expect(admission.Exists()).To(BeTrue())
			Expect(admission.Field("metadata.labels.rbac\\.deckhouse\\.io/aggregate-to-security-as").String()).To(Equal("superadmin"))
			Expect(admission.Field("metadata.labels.rbac\\.deckhouse\\.io/aggregate-to-cluster-as").String()).To(Equal("superadmin"))
			Expect(admission.Field("rules").String()).To(ContainSubstring(`validatingwebhookconfigurations/deckhouse-webhook-handler`))
			Expect(admission.Field("rules.0.resources").String()).To(MatchJSON(
				`["mutatingwebhookconfigurations","validatingadmissionpolicies","validatingadmissionpolicybindings","validatingwebhookconfigurations"]`))

			manage := f.KubernetesGlobalResource("ClusterRole", "d8:subsystem-capability:security:manage_resources")
			Expect(manage.Exists()).To(BeTrue())
			Expect(manage.Field("metadata.labels.rbac\\.deckhouse\\.io/aggregate-to-security-as").String()).To(Equal("manager"))
			Expect(manage.Field("rules").String()).NotTo(ContainSubstring("admissionregistration.k8s.io"))
		})
	})

	Context("With multi-tenancy off", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		// A rule whose multi-tenancy options cannot take effect used to stop the whole module from
		// rendering. One rule, which anybody allowed to create them can write, then froze every
		// other change to the module, and the message reached only whoever read the release logs.
		// The condition is reported by the alert_multitenancy_disabled hook instead.
		It("Should render, and deploy nothing that enforces the options that cannot take effect", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			Expect(f.KubernetesResource("DaemonSet", "d8-user-authz", "user-authz-webhook").Exists()).To(BeFalse(),
				"the webhook that would enforce those options is not deployed with multi-tenancy off")
			Expect(f.KubernetesResource("Deployment", "d8-user-authz", "permission-browser-apiserver").Exists()).To(BeFalse(),
				"and neither is the apiserver that reports them")
			// Their PodMonitors go with them: a scrape target for a component that is not there is
			// what made the TargetDown alerts fire on every cluster with the default settings.
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "user-authz-webhook").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "permission-browser-apiserver").Exists()).To(BeFalse())
		})
	})

	Context("Without enabledMultiTenancy", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd"]`)
			f.HelmRender()
		})

		// user-authz-controller is rendered whether or not MultiTenancy is on, so the objects
		// it needs — the namespace it lives in and the secret it pulls its image with — have
		// to be unconditional as well. While they were gated, the release could not converge:
		// server-side apply rejected the controller with `namespaces "d8-user-authz" not found`.
		It("Should create the namespace and registry secret the controller needs", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("Namespace", "d8-user-authz").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Secret", "d8-user-authz", "deckhouse-registry").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Deployment", "d8-user-authz", "user-authz-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("PodDisruptionBudget", "d8-user-authz", "user-authz-controller").Exists()).To(BeTrue())
		})

		// The state ConfigMap says "false" rather than going missing, so identity_privilege.py
		// can tell MultiTenancy off from a state it does not know yet.
		It("Should mirror the multitenancy state as false", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())
			state := f.KubernetesResource("ConfigMap", "d8-user-authz", "d8-user-authz-multitenancy-state")
			Expect(state.Exists()).To(BeTrue())
			Expect(state.Field("data.enableMultiTenancy").String()).To(Equal("false"))
			// The kube-rbac-proxy CA in the same template stays gated on MultiTenancy.
			Expect(f.KubernetesResource("ConfigMap", "d8-user-authz", "kube-rbac-proxy-ca.crt").Exists()).To(BeFalse())
		})
	})

	// permission-browser and the scope filter of kube-apiserver read multitenancyManagerEnabled to
	// refuse the projects and project scopes while multitenancy-manager is off. CE has no
	// permission browser, but the kube-apiserver reads the key there too, so it has to render in
	// every edition.
	Context("Multitenancy-manager state", func() {
		for _, edition := range []string{"CE", "BE", "SE", "EE", "FE"} {
			for _, enabled := range []bool{true, false} {
				Context("in "+edition+" with multitenancy-manager enabled="+strconv.FormatBool(enabled), func() {
					BeforeEach(func() {
						modules := `["operator-prometheus", "operator-prometheus-crd"]`
						if enabled {
							modules = `["operator-prometheus", "operator-prometheus-crd", "multitenancy-manager"]`
						}
						f.ValuesSetFromYaml("global.enabledModules", modules)
						f.ValuesSet("global.deckhouseEdition", edition)
						f.HelmRender()
					})

					It("Should mirror whether multitenancy-manager is enabled", func() {
						Expect(f.RenderError).ShouldNot(HaveOccurred())
						state := f.KubernetesResource("ConfigMap", "d8-user-authz", "d8-user-authz-multitenancy-state")
						Expect(state.Exists()).To(BeTrue())
						Expect(state.Field("data.multitenancyManagerEnabled").String()).To(Equal(strconv.FormatBool(enabled)))
					})
				})
			}
		}
	})

	Context("Namespace access permissions based on edition", func() {
		Context("EE edition (non-CE)", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd"]`)
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
				f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd"]`)
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

			// The alerts must NOT be installed here. Permission Browser and the authorization
			// webhook are rendered only with multi-tenancy on, and these rules used to be plain
			// YAML shipped either way — so on every cluster with the default
			// enableMultiTenancy: false there was no target to scrape, absent(up{...}) stayed
			// true, and TargetDown fired permanently five minutes after install. An alert about a
			// component that is not deployed is noise by construction.
			It("Should not render alerts about components multi-tenancy does not deploy", func() {
				for _, name := range []string{
					"user-authz-permission-browser-apiserver",
					"user-authz-user-authz-webhook",
				} {
					Expect(f.KubernetesResource("PrometheusRule", "d8-system", name).Exists()).
						To(BeFalse(), "PrometheusRule %s must not exist without multi-tenancy", name)
				}
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

		// A cluster-wide read of `projects` would be answered by RBAC before the
		// namespace ACL could narrow it, so it exists only where that ACL does not.
		Context("Project visibility with multitenancy-manager enabled", func() {
			BeforeEach(func() {
				f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd", "multitenancy-manager"]`)
				f.ValuesSet("global.deckhouseEdition", "EE")
			})

			It("user-authz:user should not read projects cluster-wide when the namespace ACL filters them", func() {
				f.ValuesSet("userAuthz.enableMultiTenancy", true)
				f.HelmRender()
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:user")
				Expect(cr.Exists()).To(BeTrue())
				Expect(cr.Field("rules").String()).NotTo(ContainSubstring(`"projects"`))
			})

			It("user-authz:user should read projects cluster-wide when nothing filters them", func() {
				f.ValuesSet("userAuthz.enableMultiTenancy", false)
				f.HelmRender()
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:user")
				Expect(cr.Exists()).To(BeTrue())
				Expect(cr.Field("rules").String()).To(ContainSubstring(`"projects"`))
			})

			It("user-authz:user should not read projects when multitenancy-manager is absent", func() {
				f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd"]`)
				f.ValuesSet("userAuthz.enableMultiTenancy", false)
				f.HelmRender()
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:user")
				Expect(cr.Exists()).To(BeTrue())
				Expect(cr.Field("rules").String()).NotTo(ContainSubstring(`"projects"`))
			})
		})
	})

	Context("namespaces/finalize access", func() {
		BeforeEach(func() {
			f.HelmRender()
		})

		It("user-authz:cluster-admin should have namespaces/finalize access", func() {
			cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:cluster-admin")
			Expect(cr.Exists()).To(BeTrue())
			Expect(cr.Field("rules").String()).To(ContainSubstring("namespaces/finalize"))
		})

		It("user-authz:cluster-editor should not have namespaces/finalize access", func() {
			cr := f.KubernetesGlobalResource("ClusterRole", "user-authz:cluster-editor")
			Expect(cr.Exists()).To(BeTrue())
			Expect(cr.Field("rules").String()).NotTo(ContainSubstring("namespaces/finalize"))
		})
	})

	Context("RBACv2 subsystem capabilities", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd"]`)
			f.ValuesSet("global.deckhouseEdition", "EE")
			f.HelmRender()
		})

		It("Should let the IAM and security managers inspect the access of any subject", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// inspect_access moved to iam with user-authz and stays with the security manager, who
			// audits access
			capability := f.KubernetesGlobalResource("ClusterRole", "d8:subsystem-capability:iam:inspect_access")
			Expect(capability.Exists()).To(BeTrue())
			Expect(capability.Field(`metadata.labels.rbac\.deckhouse\.io/aggregate-to-iam-as`).String()).To(Equal("manager"))
			Expect(capability.Field(`metadata.labels.rbac\.deckhouse\.io/aggregate-to-security-as`).String()).To(Equal("manager"))
			Expect(capability.Field(`metadata.labels.rbac\.deckhouse\.io/capability`).String()).To(Equal("subsystem-capability.iam.inspect_access"))
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:subsystem-capability:security:inspect_access").Exists()).To(BeFalse())
		})

		It("view_resources should not grant any node subresource", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cr := f.KubernetesGlobalResource("ClusterRole", "d8:subsystem-capability:cluster:view_resources")
			Expect(cr.Exists()).To(BeTrue())

			rules := cr.Field("rules").String()
			// nodes/proxy is a raw passthrough to the privileged kubelet API: it allows running
			// commands in any container on the node regardless of pods/exec permissions, and
			// nodes/log allows reading arbitrary files under /var/log. Neither belongs in a role
			// that aggregates into viewer. Upstream keeps them in system:kubelet-api-admin only.
			Expect(rules).To(ContainSubstring(`"nodes"`))
			Expect(rules).NotTo(ContainSubstring("nodes/proxy"))
			Expect(rules).NotTo(ContainSubstring("nodes/log"))
			Expect(rules).NotTo(ContainSubstring("nodes/metrics"))
			Expect(rules).NotTo(ContainSubstring("nodes/stats"))
		})
	})

	// The access levels grant the namespaced Gateway API resources, read from User and write from
	// Editor up. The namespace capabilities grant them at the matching levels: view_resources
	// aggregates into d8:namespace:viewer and manage_resources into d8:namespace:manager, and the
	// higher namespace and project roles aggregate those in turn.
	Context("RBACv2 namespace capabilities", func() {
		gatewayAPIResources := []string{
			"backendtlspolicies", "gateways", "grpcroutes", "httproutes", "listenersets",
			"referencegrants", "tcproutes", "tlsroutes", "udproutes",
		}

		asStrings := func(values gjson.Result) []string {
			out := []string{}
			for _, value := range values.Array() {
				out = append(out, value.String())
			}
			return out
		}

		// gatewayAPIRules returns the rules of the ClusterRole that name the Gateway API group.
		gatewayAPIRules := func(name string) []gjson.Result {
			role := f.KubernetesGlobalResource("ClusterRole", name)
			Expect(role.Exists()).To(BeTrue(), "expected ClusterRole %s to be rendered", name)
			out := []gjson.Result{}
			for _, rule := range role.Field("rules").Array() {
				for _, group := range rule.Get("apiGroups").Array() {
					if group.String() == "gateway.networking.k8s.io" {
						out = append(out, rule)
						break
					}
				}
			}
			return out
		}

		BeforeEach(func() {
			f.ValuesSetFromYaml("global.enabledModules", `["operator-prometheus", "operator-prometheus-crd"]`)
			f.ValuesSet("global.deckhouseEdition", "EE")
			f.HelmRender()
		})

		It("view_resources should grant viewers read access to the namespaced Gateway API resources", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			name := "d8:namespace-capability:kubernetes:view_resources"
			role := f.KubernetesGlobalResource("ClusterRole", name)
			Expect(role.Field(`metadata.labels.rbac\.deckhouse\.io/aggregate-to-namespace-as`).String()).To(Equal("viewer"))

			rules := gatewayAPIRules(name)
			Expect(rules).To(HaveLen(1))
			Expect(asStrings(rules[0].Get("apiGroups"))).To(ConsistOf("gateway.networking.k8s.io"))
			Expect(asStrings(rules[0].Get("resources"))).To(ConsistOf(gatewayAPIResources))
			Expect(asStrings(rules[0].Get("verbs"))).To(ConsistOf("get", "list", "watch"))
		})

		It("manage_resources should grant managers write access to the namespaced Gateway API resources", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			name := "d8:namespace-capability:kubernetes:manage_resources"
			role := f.KubernetesGlobalResource("ClusterRole", name)
			Expect(role.Field(`metadata.labels.rbac\.deckhouse\.io/aggregate-to-namespace-as`).String()).To(Equal("manager"))

			rules := gatewayAPIRules(name)
			Expect(rules).To(HaveLen(1))
			Expect(asStrings(rules[0].Get("apiGroups"))).To(ConsistOf("gateway.networking.k8s.io"))
			Expect(asStrings(rules[0].Get("resources"))).To(ConsistOf(gatewayAPIResources))
			Expect(asStrings(rules[0].Get("verbs"))).To(ConsistOf("create", "update", "patch", "delete", "deletecollection"))
		})
	})

})
