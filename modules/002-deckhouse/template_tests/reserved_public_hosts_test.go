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
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// The two ways a module in this repository asks for a hostname under publicDomainTemplate: the Helm
// helper in a template, and the certificate SAN helper in a hook. Both take the name portion as a
// literal, which is what makes the reserved list checkable against them.
var publicDomainCallSites = []*regexp.Regexp{
	regexp.MustCompile(`helm_lib_module_public_domain"\s+\(list\s+\S+\s+"([a-z0-9-]+)"`),
	regexp.MustCompile(`PublicDomainSAN\("([a-z0-9-]+)"\)`),
}

func repositoryRoot() string {
	dir, err := os.Getwd()
	Expect(err).ShouldNot(HaveOccurred())

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		Expect(parent).ToNot(Equal(dir), "the repository root should be an ancestor of %s", dir)
		dir = parent
	}
}

// publishedPublicDomains maps every name portion the repository renders under
// publicDomainTemplate to a file that asks for it, so a failure can name the culprit.
func publishedPublicDomains() map[string]string {
	root := repositoryRoot()
	found := map[string]string{}

	for _, tree := range []string{"modules", "ee"} {
		tree = filepath.Join(root, tree)
		if _, err := os.Stat(tree); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(tree, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			// The library chart, whose own definition of the helper would otherwise be read as a
			// call site. A symlink in a checkout, a real copy in the build image.
			if entry.IsDir() && entry.Name() == "helm_lib" {
				return fs.SkipDir
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".yaml", ".yml", ".tpl", ".go":
			default:
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, callSite := range publicDomainCallSites {
				for _, match := range callSite.FindAllSubmatch(content, -1) {
					name := string(match[1])
					if _, seen := found[name]; !seen {
						found[name], _ = filepath.Rel(root, path)
					}
				}
			}
			return nil
		})
		Expect(err).ShouldNot(HaveOccurred())
	}

	return found
}

const (
	reservedHostsConfigMapName   = "d8-reserved-public-hosts"
	reservedHostsIngressPolicy   = "reserved-public-hosts-ingress.deckhouse.io"
	reservedHostsHTTPRoutePolicy = "reserved-public-hosts-httproute.deckhouse.io"

	validatingAdmissionPolicyAPI        = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"
	validatingAdmissionPolicyBindingAPI = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding"
	httpRouteAPI                        = "gateway.networking.k8s.io/v1/HTTPRoute"
)

// expectCoversRepositoryPublicDomains fails when a module in this repository publishes a public
// domain the literal in the template does not name, pointing at the file that publishes it.
//
// It reads platformHosts and never hosts: the check is about the literal keeping up with the
// repository, so a cluster that excluded a service through its ModuleConfig must not be able to
// answer it. The context that renders with an exclusion calls this too, which is what keeps that
// distinction from being an unenforced comment.
func expectCoversRepositoryPublicDomains(cm object_store.KubeObject) {
	platformHosts := strings.Fields(cm.Field("data.platformHosts").String())

	published := publishedPublicDomains()
	Expect(published).ToNot(BeEmpty(), "the call site patterns should still match this repository")

	for name, source := range published {
		Expect(platformHosts).To(ContainElement(name+".example.com"),
			"%s publishes the public domain %q, but it is not reserved. Add %q to $reservedNames "+
				"in modules/002-deckhouse/templates/reserved-public-hosts.yaml, otherwise any "+
				"namespace can claim that hostname.", source, name, name)
	}
}

var _ = Describe("Module :: deckhouse :: reserved public hosts ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	// reservedPublicHosts is the deckhouse ModuleConfig section under test; an empty string leaves it
	// absent, which is what a cluster that never configured it looks like.
	renderWithSettings := func(publicDomainTemplate, reservedPublicHosts string, apiVersions ...string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		if publicDomainTemplate != "" {
			f.ValuesSet("global.modules.publicDomainTemplate", publicDomainTemplate)
		}
		if reservedPublicHosts != "" {
			f.ValuesSetFromYaml("deckhouse.reservedPublicHosts", reservedPublicHosts)
		}
		f.HelmRender(WithAPIVersions(apiVersions...))
	}

	render := func(publicDomainTemplate string, apiVersions ...string) {
		renderWithSettings(publicDomainTemplate, "", apiVersions...)
	}

	admissionAPIs := []string{validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI}

	Context("Ingress hosts are reserved when the platform publishes any", func() {
		BeforeEach(func() {
			render("%s.example.com", admissionAPIs...)
		})

		It("reserves the hostname of every service the platform publishes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			Expect(cm.Exists()).To(BeTrue())

			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).To(ContainElements(
				"api.example.com",
				"console.example.com",
				"dashboard.example.com",
				"dex.example.com",
				"grafana.example.com",
				"kubeconfig.example.com",
				"prometheus.example.com",
			))
			// The policies lowercase the claimed hostname before comparing, so a reserved one that
			// kept any uppercase would silently never match.
			for _, host := range hosts {
				Expect(host).To(Equal(strings.ToLower(host)))
			}
		})

		It("covers every public domain the repository itself publishes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			expectCoversRepositoryPublicDomains(cm)
		})

		It("denies an Ingress claiming one of them", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy)
			Expect(vap.Exists()).To(BeTrue())
			Expect(vap.Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(vap.Field("spec.paramKind.kind").String()).To(Equal("ConfigMap"))
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.apiGroups").String()).To(MatchJSON(`["networking.k8s.io"]`))
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(MatchJSON(`["ingresses"]`))
			// DELETE is deliberately absent: an Ingress that predates the policy stays routable
			// until somebody rewrites it.
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.operations").String()).To(MatchJSON(`["CREATE","UPDATE"]`))
			Expect(vap.Field(`spec.variables.#(name=="claimedHosts").expression`).String()).To(ContainSubstring("object.spec.rules"))
			Expect(vap.Field("spec.validations.0.reason").String()).To(Equal("Forbidden"))
		})

		It("exempts the writers that create the platform's own objects", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy)
			Expect(vap.Field("spec.matchConditions").Array()).To(HaveLen(3))
			Expect(vap.Field(`spec.matchConditions.#(name=="exclude-system-serviceaccounts").expression`).String()).
				To(ContainSubstring("system:serviceaccount:d8-"))
		})

		It("binds the policy to the parameters and skips the platform's own namespaces", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsIngressPolicy)
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("spec.policyName").String()).To(Equal(reservedHostsIngressPolicy))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
			Expect(binding.Field("spec.paramRef.name").String()).To(Equal(reservedHostsConfigMapName))
			Expect(binding.Field("spec.paramRef.namespace").String()).To(Equal("d8-system"))
			// Without this, losing the ConfigMap would take every Ingress write in the cluster
			// down with it.
			Expect(binding.Field("spec.paramRef.parameterNotFoundAction").String()).To(Equal("Allow"))
			Expect(binding.Field("spec.matchResources.namespaceSelector.matchExpressions").String()).To(MatchJSON(`[
				{"key": "heritage", "operator": "NotIn", "values": ["deckhouse"]},
				{"key": "security.deckhouse.io/reserved-hosts-bypass", "operator": "NotIn", "values": ["true"]}
			]`))
		})

		It("leaves HTTPRoute alone while the Gateway API is not installed", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsHTTPRoutePolicy).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsHTTPRoutePolicy).Exists()).To(BeFalse())
		})
	})

	Context("The hostnames follow publicDomainTemplate, whatever shape it has", func() {
		BeforeEach(func() {
			render("%s-kube.example.com", admissionAPIs...)
		})

		It("reserves what the template actually renders, not a fixed subdomain", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).To(ContainElement("console-kube.example.com"))
			Expect(hosts).NotTo(ContainElement("console.example.com"))
		})
	})

	Context("The Gateway API is installed", func() {
		BeforeEach(func() {
			render("%s.example.com", append(admissionAPIs, httpRouteAPI)...)
		})

		It("reserves the same hostnames on HTTPRoute", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsHTTPRoutePolicy)
			Expect(vap.Exists()).To(BeTrue())
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.apiGroups").String()).To(MatchJSON(`["gateway.networking.k8s.io"]`))
			Expect(vap.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(MatchJSON(`["httproutes"]`))
			Expect(vap.Field(`spec.variables.#(name=="claimedHosts").expression`).String()).To(ContainSubstring("object.spec.hostnames"))

			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsHTTPRoutePolicy)
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("spec.paramRef.name").String()).To(Equal(reservedHostsConfigMapName))
		})
	})

	Context("The cluster has no ValidatingAdmissionPolicy", func() {
		BeforeEach(func() {
			render("%s.example.com")
		})

		It("renders nothing rather than a ConfigMap nobody reads", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsIngressPolicy).Exists()).To(BeFalse())
		})
	})

	Context("The platform publishes nothing", func() {
		BeforeEach(func() {
			render("", admissionAPIs...)
		})

		It("reserves nothing, there are no system hostnames to protect", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy).Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsIngressPolicy).Exists()).To(BeFalse())
		})
	})

	Context("The settings are present but empty, as the schema defaults leave them", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com", `{additionalHosts: [], excludedServices: []}`, admissionAPIs...)
		})

		It("reserves exactly what the platform publishes, as before the settings existed", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			hosts := strings.Fields(cm.Field("data.hosts").String())
			platform := strings.Fields(cm.Field("data.platformHosts").String())
			Expect(platform).ToNot(BeEmpty())
			Expect(hosts).To(ConsistOf(platform))
		})
	})

	Context("An operator reserves hostnames of their own", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com",
				`{additionalHosts: ["admin.example.com", "billing.corp.example.com"]}`, admissionAPIs...)
		})

		It("adds them to the list the policies read", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).To(ContainElements("admin.example.com", "billing.corp.example.com"))
			Expect(hosts).To(ContainElement("console.example.com"))
			// The policies compare against lowercase, so anything that kept its case would never
			// match whatever the request claims.
			for _, host := range hosts {
				Expect(host).To(Equal(strings.ToLower(host)))
			}
		})

		It("leaves the hostnames the platform publishes out of it", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			platform := strings.Fields(cm.Field("data.platformHosts").String())
			Expect(platform).NotTo(ContainElement("admin.example.com"))
			Expect(platform).To(ContainElement("console.example.com"))
		})
	})

	Context("An operator gives a hostname back to a tenant", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com", `{excludedServices: ["grafana"]}`, admissionAPIs...)
		})

		It("stops reserving that hostname and nothing else", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).NotTo(ContainElement("grafana.example.com"))
			Expect(hosts).To(ContainElements("console.example.com", "prometheus.example.com"))
		})

		It("keeps the excluded hostname visible as one the platform publishes", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			Expect(strings.Fields(cm.Field("data.platformHosts").String())).To(ContainElement("grafana.example.com"))
		})

		It("still answers whether the literal covers the repository, exclusion or not", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// The excluded service has to be one this repository publishes, otherwise the check
			// below would hold whichever key it read and would prove nothing.
			Expect(publishedPublicDomains()).To(HaveKey("grafana"),
				"this context excludes grafana to show that an exclusion cannot quiet the coverage "+
					"check, which needs the repository to still publish grafana")

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			Expect(strings.Fields(cm.Field("data.hosts").String())).NotTo(ContainElement("grafana.example.com"))
			expectCoversRepositoryPublicDomains(cm)
		})

		It("does not touch the per-namespace bypass, the two are independent", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsIngressPolicy)
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
			Expect(binding.Field("spec.matchResources.namespaceSelector.matchExpressions").String()).To(MatchJSON(`[
				{"key": "heritage", "operator": "NotIn", "values": ["deckhouse"]},
				{"key": "security.deckhouse.io/reserved-hosts-bypass", "operator": "NotIn", "values": ["true"]}
			]`))
		})
	})

	Context("An operator adjusts the reservation in both directions at once", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com",
				`{additionalHosts: ["admin.example.com", "console.example.com"], excludedServices: ["grafana", "hubble"]}`,
				admissionAPIs...)
		})

		It("applies both, and a hostname named on both sides stays reserved once", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).To(ContainElement("admin.example.com"))
			Expect(hosts).NotTo(ContainElements("grafana.example.com", "hubble.example.com"))
			Expect(hosts).To(Equal(sortedUnique(hosts)))

			consoleCount := 0
			for _, host := range hosts {
				if host == "console.example.com" {
					consoleCount++
				}
			}
			Expect(consoleCount).To(Equal(1))
		})
	})

	Context("An operator excludes a service the platform does not publish", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com", `{excludedServices: ["graphana"]}`, admissionAPIs...)
		})

		It("refuses to render rather than leave the hostname reserved unnoticed", func() {
			Expect(f.RenderError).Should(HaveOccurred())
			Expect(f.RenderError.Error()).To(ContainSubstring("graphana"))
			Expect(f.RenderError.Error()).To(ContainSubstring("deckhouse.reservedPublicHosts.excludedServices"))
			// The message carries the names that would have worked, they exist nowhere else.
			Expect(f.RenderError.Error()).To(ContainSubstring("grafana"))
		})
	})

	Context("An operator reserves their own hostname while the platform publishes none", func() {
		BeforeEach(func() {
			renderWithSettings("", `{additionalHosts: ["admin.example.com"]}`, admissionAPIs...)
		})

		It("reserves it anyway, it does not depend on publicDomainTemplate", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			cm := f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
			Expect(cm.Exists()).To(BeTrue())
			Expect(strings.Fields(cm.Field("data.hosts").String())).To(Equal([]string{"admin.example.com"}))
			Expect(strings.Fields(cm.Field("data.platformHosts").String())).To(BeEmpty())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy).Exists()).To(BeTrue())
		})
	})
})

func sortedUnique(hosts []string) []string {
	seen := make(map[string]struct{}, len(hosts))
	unique := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		unique = append(unique, host)
	}
	sort.Strings(unique)
	return unique
}
