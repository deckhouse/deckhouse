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
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/modules/002-deckhouse/hooks/lib/publicdomain"
	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// The two ways a module in this repository asks for a hostname under publicDomainTemplate: the Helm
// helper in a template, and the certificate SAN helper in a hook. Both take the name portion as a
// literal, which is what makes $reservedNames checkable against them.
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
	reservedHostsConfigMapName     = "d8-reserved-public-hosts"
	reservedHostsIngressPolicy     = "reserved-public-hosts-ingress.deckhouse.io"
	reservedHostsHTTPRoutePolicy   = "reserved-public-hosts-httproute.deckhouse.io"
	reservedHostsListenerSetPolicy = "reserved-public-hosts-listenerset.deckhouse.io"

	validatingAdmissionPolicyAPI        = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"
	validatingAdmissionPolicyBindingAPI = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding"
	httpRouteAPI                        = "gateway.networking.k8s.io/v1/HTTPRoute"
	listenerSetAPI                      = "gateway.networking.k8s.io/v1/ListenerSet"
)

// schemaDefaultMode reads settings.reservedPublicHosts.mode's default out of the module's own schema,
// so that the checks below hold whichever of the two reservations a branch ships by default. A
// release branch that judges the wider one too risky for a patch release changes it there.
func schemaDefaultMode() string {
	content, err := os.ReadFile(filepath.Join(repositoryRoot(), "modules/002-deckhouse/openapi/config-values.yaml"))
	Expect(err).ShouldNot(HaveOccurred())

	var schema struct {
		Properties struct {
			ReservedPublicHosts struct {
				Properties struct {
					Mode struct {
						Default string   `json:"default"`
						Enum    []string `json:"enum"`
					} `json:"mode"`
				} `json:"properties"`
			} `json:"reservedPublicHosts"`
		} `json:"properties"`
	}
	Expect(yaml.Unmarshal(content, &schema)).To(Succeed())

	mode := schema.Properties.ReservedPublicHosts.Properties.Mode
	Expect(mode.Enum).To(ConsistOf("Template", "List"), "there are two reservations and no third one")
	Expect(mode.Default).To(BeElementOf(mode.Enum))
	return mode.Default
}

// expectCoversRepositoryPublicDomains fails when a module in this repository publishes a public
// domain $reservedNames does not name, pointing at the file that publishes it.
//
// Under mode: List that list is the reservation, so this is the check that keeps an in-repo module
// from adding a public domain nobody reserved. Under mode: Template the reservation no longer needs
// it, and what the check keeps correct is the vocabulary either side of it: platformHosts, which is
// how an operator reads what the platform serves, and the set of names excludedServices reports as
// published. Both modes therefore still want the literal in sync with the repository.
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
			"%s publishes the public domain %q, but %q is not named in $reservedNames in "+
				"modules/002-deckhouse/templates/reserved-public-hosts.yaml, so platformHosts does "+
				"not mention it and excludedServices reports it as unknown.", source, name, name)
	}
}

var _ = Describe("Module :: deckhouse :: reserved public hosts ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	// reservedPublicHosts is the deckhouse ModuleConfig section under test and snapshot the recorded
	// grandfathering; an empty string leaves either absent, which is what a cluster that never
	// configured one looks like.
	renderWith := func(publicDomainTemplate, reservedPublicHosts, snapshot string, apiVersions ...string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
		if publicDomainTemplate != "" {
			f.ValuesSet("global.modules.publicDomainTemplate", publicDomainTemplate)
		}
		if reservedPublicHosts != "" {
			f.ValuesSetFromYaml("deckhouse.reservedPublicHosts", reservedPublicHosts)
		}
		if snapshot != "" {
			f.ValuesSetFromYaml("deckhouse.internal.reservedPublicHosts", snapshot)
		}
		f.HelmRender(WithAPIVersions(apiVersions...))
	}

	renderWithSettings := func(publicDomainTemplate, reservedPublicHosts string, apiVersions ...string) {
		renderWith(publicDomainTemplate, reservedPublicHosts, "", apiVersions...)
	}

	// render asks for Template mode explicitly. Every context below that is about Template mode says
	// so, rather than leaning on whatever this branch defaults to, so that a release branch which
	// ships the other default keeps testing both and has nothing to edit here.
	render := func(publicDomainTemplate string, apiVersions ...string) {
		renderWith(publicDomainTemplate, `{mode: Template}`, "", apiVersions...)
	}

	admissionAPIs := []string{validatingAdmissionPolicyAPI, validatingAdmissionPolicyBindingAPI}

	configMap := func() object_store.KubeObject {
		Expect(f.RenderError).ShouldNot(HaveOccurred())
		return f.KubernetesResource("ConfigMap", "d8-system", reservedHostsConfigMapName)
	}

	// Which reservation a cluster gets when its ModuleConfig says nothing is decided by the schema
	// default, because addon-operator fills the section in before Helm sees it. The template carries
	// a fallback for the same choice, which is what a bare render without that defaulting uses. The
	// two are compared against the schema rather than against a literal, so that a branch which
	// ships the other default has to move both and cannot leave the tests exercising the one it does
	// not ship.
	Context("The mode a cluster gets when it asks for nothing", func() {
		It("is the one the schema defaults to, once the defaults are applied", func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
			f.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
			f.ApplyOpenAPIDefaults()
			f.HelmRender(WithAPIVersions(admissionAPIs...))

			Expect(configMap().Field("data.mode").String()).To(Equal(schemaDefaultMode()))
		})

		It("is the same one the template falls back to without them", func() {
			renderWith("%s.example.com", "", "", admissionAPIs...)
			Expect(configMap().Field("data.mode").String()).To(Equal(schemaDefaultMode()),
				"the fallback in templates/reserved-public-hosts.yaml has to agree with the schema, "+
					"or a bare render of the chart would reserve something else than a cluster does")
		})
	})

	Context("The reservation follows publicDomainTemplate by default", func() {
		BeforeEach(func() {
			render("%s.example.com", admissionAPIs...)
		})

		It("derives the namespace of the template as a regex", func() {
			cm := configMap()
			Expect(cm.Field("data.mode").String()).To(Equal("Template"))
			// The literal string the policies feed to CEL matches(), which is RE2. Asserted whole,
			// because it is the security boundary: a stray character in it either stops reserving or
			// starts reserving somebody else's domain. The backslashes survive the ConfigMap, which
			// is the escaping this depends on.
			Expect(cm.Field("data.hostPattern").String()).
				To(Equal(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?\.example\.com$`))
			_, err := regexp.Compile(cm.Field("data.hostPattern").String())
			Expect(err).ShouldNot(HaveOccurred(), "CEL matches() is RE2, so Go must accept it")
		})

		It("derives it from the whole template when the %s shares its label", func() {
			render("kube-%s.company.my", admissionAPIs...)
			Expect(configMap().Field("data.hostPattern").String()).
				To(Equal(`^kube-[a-z0-9]([-a-z0-9]*[a-z0-9])?\.company\.my$`))
		})

		// The hook that records what tenants already serve has to decide which hostnames the
		// reservation is about to claim, and it cannot read the regex out of a ConfigMap that does
		// not exist yet, so it derives the same set in Go. Same derivation, two languages: compared
		// here rather than trusted, because a drift between them grandfathers the wrong hostnames.
		It("derives the same namespace the hook does", func() {
			for _, domainTemplate := range []string{
				"%s.example.com",
				"kube-%s.company.my",
				"%s-kube.company.my",
				"pre-%s-post.a-b.c.example.com",
				"%s.a.b.c.d.example.com",
			} {
				render(domainTemplate, admissionAPIs...)
				cm := configMap()

				namespace, err := publicdomain.ParseNamespace(domainTemplate)
				Expect(err).ShouldNot(HaveOccurred(), domainTemplate)
				Expect(cm.Field("data.hostPattern").String()).To(Equal(namespace.Pattern.String()), domainTemplate)

				wildcards := []string{}
				for _, host := range strings.Fields(cm.Field("data.hosts").String()) {
					if strings.HasPrefix(host, "*") {
						wildcards = append(wildcards, host)
					}
				}
				if namespace.Wildcard == "" {
					Expect(wildcards).To(BeEmpty(), domainTemplate)
				} else {
					Expect(wildcards).To(Equal([]string{namespace.Wildcard}), domainTemplate)
				}
			}
		})

		It("quotes what it puts in the regex, so a dotted domain is not a wildcard", func() {
			render("%s.a-b.c.example.com", admissionAPIs...)
			pattern := configMap().Field("data.hostPattern").String()
			Expect(pattern).To(Equal(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?\.a-b\.c\.example\.com$`))
			Expect(regexp.MustCompile(pattern).MatchString("shop.axb.c.example.com")).To(BeFalse(),
				"an unquoted dot would have matched any character here")
		})

		It("reserves the wildcard form of the namespace by exact match", func() {
			hosts := strings.Fields(configMap().Field("data.hosts").String())
			Expect(hosts).To(ContainElement("*.example.com"),
				"the label the pattern derives cannot match a wildcard, and in Template mode a "+
					"tenant holding one shadows every platform hostname at once")
		})

		It("leaves the wildcard alone where it would cover more than the platform's namespace", func() {
			render("kube-%s.company.my", admissionAPIs...)
			hosts := strings.Fields(configMap().Field("data.hosts").String())
			Expect(hosts).To(BeEmpty(),
				"kube-*.company.my is not a hostname any API server accepts, and *.company.my "+
					"covers a domain the platform does not own")
		})

		It("keeps the list of what the platform publishes for an operator to read", func() {
			cm := configMap()
			platform := strings.Fields(cm.Field("data.platformHosts").String())
			Expect(platform).To(ContainElements(
				"api.example.com",
				"console.example.com",
				"dex.example.com",
				"grafana.example.com",
				"kubeconfig.example.com",
				"prometheus.example.com",
			))
			// The policies lowercase the claimed hostname before comparing, so anything here that
			// kept any uppercase would silently never match.
			for _, host := range platform {
				Expect(host).To(Equal(strings.ToLower(host)))
			}
		})

		It("covers every public domain the repository itself publishes", func() {
			expectCoversRepositoryPublicDomains(configMap())
		})

		It("records that no grandfathering has happened yet", func() {
			cm := configMap()
			Expect(cm.Field("data.grandfatherRecorded").String()).To(Equal("false"))
			Expect(strings.Fields(cm.Field("data.grandfatheredHosts").String())).To(BeEmpty())
			Expect(strings.Fields(cm.Field("data.allowedHosts").String())).To(BeEmpty())
		})

		It("denies an Ingress claiming a hostname in that namespace", func() {
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
			Expect(vap.Field(`spec.variables.#(name=="conflicts").expression`).String()).To(ContainSubstring("matches(variables.reservedPattern)"))
			Expect(vap.Field("spec.validations.0.reason").String()).To(Equal("Forbidden"))
		})

		It("exempts the writers that create the platform's own objects", func() {
			vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy)
			Expect(vap.Field("spec.matchConditions").Array()).To(HaveLen(3))
			Expect(vap.Field(`spec.matchConditions.#(name=="exclude-system-serviceaccounts").expression`).String()).
				To(ContainSubstring("system:serviceaccount:d8-"))
		})

		It("binds the policy to the parameters and skips the platform's own namespaces", func() {
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

		It("leaves the Gateway API kinds alone while they are not installed", func() {
			for _, name := range []string{reservedHostsHTTPRoutePolicy, reservedHostsListenerSetPolicy} {
				Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", name).Exists()).To(BeFalse(), name)
				Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", name).Exists()).To(BeFalse(), name)
			}
		})
	})

	Context("The Gateway API is installed", func() {
		BeforeEach(func() {
			render("%s.example.com", append(admissionAPIs, httpRouteAPI, listenerSetAPI)...)
		})

		// Every difference between the three policies is a hostname a tenant can claim on the kind
		// that got the weaker one, so the fields that must not differ are compared rather than
		// asserted one by one.
		It("reserves the same hostnames on every kind that carries one", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			ingress := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy)
			for name, expected := range map[string]struct{ apiGroup, resource, hostField string }{
				reservedHostsHTTPRoutePolicy:   {"gateway.networking.k8s.io", "httproutes", "object.spec.hostnames"},
				reservedHostsListenerSetPolicy: {"gateway.networking.k8s.io", "listenersets", "object.spec.listeners"},
			} {
				vap := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", name)
				Expect(vap.Exists()).To(BeTrue(), name)
				Expect(vap.Field("spec.matchConstraints.resourceRules.0.apiGroups").String()).To(MatchJSON(`["` + expected.apiGroup + `"]`))
				Expect(vap.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(MatchJSON(`["` + expected.resource + `"]`))
				Expect(vap.Field("spec.matchConstraints.resourceRules.0.operations").String()).To(MatchJSON(`["CREATE","UPDATE"]`))
				Expect(vap.Field(`spec.variables.#(name=="claimedHosts").expression`).String()).To(ContainSubstring(expected.hostField), name)

				for _, field := range []string{
					"spec.failurePolicy",
					"spec.matchConditions",
					"spec.validations",
					`spec.variables.#(name=="reservedHosts").expression`,
					`spec.variables.#(name=="reservedPattern").expression`,
					`spec.variables.#(name=="allowedHosts").expression`,
					`spec.variables.#(name=="conflicts").expression`,
				} {
					Expect(vap.Field(field).String()).To(Equal(ingress.Field(field).String()),
						"%s must not differ between %s and %s", field, name, reservedHostsIngressPolicy)
				}

				binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", name)
				Expect(binding.Exists()).To(BeTrue(), name)
				ingressBinding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedHostsIngressPolicy)
				for _, field := range []string{"spec.validationActions", "spec.paramRef", "spec.matchResources"} {
					Expect(binding.Field(field).String()).To(Equal(ingressBinding.Field(field).String()),
						"%s must not differ between the %s binding and the Ingress one", field, name)
				}
			}
		})

		It("renders the ListenerSet policy only where the kind exists", func() {
			render("%s.example.com", append(admissionAPIs, httpRouteAPI)...)
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsHTTPRoutePolicy).Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsListenerSetPolicy).Exists()).To(BeFalse())
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

	Context("An operator reserves their own hostname while the platform publishes none", func() {
		BeforeEach(func() {
			renderWithSettings("", `{mode: Template, additionalHosts: ["admin.example.com"]}`, admissionAPIs...)
		})

		It("reserves it and derives no pattern, so nothing else becomes reserved", func() {
			cm := configMap()
			Expect(cm.Exists()).To(BeTrue())
			Expect(strings.Fields(cm.Field("data.hosts").String())).To(Equal([]string{"admin.example.com"}))
			Expect(cm.Field("data.hostPattern").String()).To(BeEmpty(),
				"an empty pattern is what makes the policies reserve nothing rather than everything")
			Expect(strings.Fields(cm.Field("data.platformHosts").String())).To(BeEmpty())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedHostsIngressPolicy).Exists()).To(BeTrue())
		})
	})

	Context("The settings are present but empty, as the schema defaults leave them", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com", `{mode: Template, additionalHosts: [], excludedServices: []}`, admissionAPIs...)
		})

		It("behaves exactly as if the section had never been written", func() {
			cm := configMap()
			Expect(cm.Field("data.mode").String()).To(Equal("Template"))
			Expect(cm.Field("data.hostPattern").String()).To(Equal(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?\.example\.com$`))
			Expect(strings.Fields(cm.Field("data.hosts").String())).To(Equal([]string{"*.example.com"}))
			Expect(strings.Fields(cm.Field("data.allowedHosts").String())).To(BeEmpty())
		})
	})

	Context("An operator reserves hostnames of their own", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com",
				`{mode: Template, additionalHosts: ["admin.corp.example.org", "billing.corp.example.com"]}`, admissionAPIs...)
		})

		It("adds them to what the policies match exactly", func() {
			cm := configMap()
			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).To(ContainElements("admin.corp.example.org", "billing.corp.example.com"))
			// The policies compare against lowercase, so anything that kept its case would never
			// match whatever the request claims.
			for _, host := range hosts {
				Expect(host).To(Equal(strings.ToLower(host)))
			}
			Expect(strings.Fields(cm.Field("data.platformHosts").String())).NotTo(ContainElement("admin.corp.example.org"))
		})
	})

	Context("An operator gives a hostname back to a tenant", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com", `{mode: Template, excludedServices: ["grafana"]}`, admissionAPIs...)
		})

		It("renders the hostname the name stands for into the allowlist", func() {
			cm := configMap()
			Expect(strings.Fields(cm.Field("data.excludedHosts").String())).To(Equal([]string{"grafana.example.com"}),
				"excludedServices takes a service name, so the hostname has to be rendered for the "+
					"pattern to be able to let it back out")
			Expect(strings.Fields(cm.Field("data.allowedHosts").String())).To(ContainElement("grafana.example.com"))
		})

		It("keeps the excluded hostname visible as one the platform publishes", func() {
			Expect(strings.Fields(configMap().Field("data.platformHosts").String())).To(ContainElement("grafana.example.com"))
		})

		It("still answers whether the literal covers the repository, exclusion or not", func() {
			// The excluded service has to be one this repository publishes, otherwise the check
			// below would hold whichever key it read and would prove nothing.
			Expect(publishedPublicDomains()).To(HaveKey("grafana"),
				"this context excludes grafana to show that an exclusion cannot quiet the coverage "+
					"check, which needs the repository to still publish grafana")

			cm := configMap()
			Expect(strings.Fields(cm.Field("data.allowedHosts").String())).To(ContainElement("grafana.example.com"))
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

	Context("An operator excludes a service the platform does not publish", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com", `{mode: Template, excludedServices: ["graphana"]}`, admissionAPIs...)
		})

		It("keeps rendering, since a typo must not stop the module that deploys Deckhouse", func() {
			cm := configMap()
			Expect(cm.Exists()).To(BeTrue())
			// The reservation the operator meant to lift is untouched: grafana is still covered by
			// the pattern and nothing let it back out.
			Expect(strings.Fields(cm.Field("data.allowedHosts").String())).NotTo(ContainElement("grafana.example.com"))
		})

		It("publishes the name so that the misspelling is visible", func() {
			Expect(strings.Fields(configMap().Field("data.unknownExcludedServices").String())).To(Equal([]string{"graphana"}))
		})

		It("reports nothing when every excluded name is published", func() {
			renderWithSettings("%s.example.com", `{mode: Template, excludedServices: ["grafana"]}`, admissionAPIs...)
			cm := configMap()
			Expect(strings.Fields(cm.Field("data.unknownExcludedServices").String())).To(BeEmpty())
			Expect(strings.Fields(cm.Field("data.allowedHosts").String())).To(ContainElement("grafana.example.com"))
		})
	})

	Context("The upgrade recorded what tenants already served", func() {
		BeforeEach(func() {
			renderWith("%s.example.com", `{mode: Template, excludedServices: ["grafana"]}`,
				`{recorded: true, hosts: ["shop.example.com", "store.example.com"]}`, admissionAPIs...)
		})

		It("keeps the recorded hostnames apart from the ones an operator wrote", func() {
			cm := configMap()
			Expect(cm.Field("data.grandfatherRecorded").String()).To(Equal("true"))
			Expect(strings.Fields(cm.Field("data.grandfatheredHosts").String())).
				To(Equal([]string{"shop.example.com", "store.example.com"}))
			Expect(strings.Fields(cm.Field("data.excludedHosts").String())).To(Equal([]string{"grafana.example.com"}))
			Expect(strings.Fields(cm.Field("data.allowedHosts").String())).
				To(Equal([]string{"grafana.example.com", "shop.example.com", "store.example.com"}),
					"the policies read one key, but which of the two put a hostname there stays readable")
		})
	})

	Context("A cluster asks for the reservation the module shipped before", func() {
		BeforeEach(func() {
			renderWithSettings("%s.example.com", `{mode: List}`, admissionAPIs...)
		})

		It("reserves the hostnames of the named services by exact match and derives no pattern", func() {
			cm := configMap()
			Expect(cm.Field("data.mode").String()).To(Equal("List"))
			Expect(cm.Field("data.hostPattern").String()).To(BeEmpty())
			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).To(ContainElements(
				"api.example.com",
				"console.example.com",
				"dex.example.com",
				"grafana.example.com",
				"kubeconfig.example.com",
				"prometheus.example.com",
			))
			Expect(hosts).To(Equal(strings.Fields(cm.Field("data.platformHosts").String())),
				"with no setting applied, List reserves exactly what the platform publishes")
			Expect(hosts).NotTo(ContainElement("*.example.com"),
				"wildcards were out of scope of the list, and List has to stay what it was")
		})

		It("subtracts an exclusion from the list rather than allowing it back out", func() {
			renderWithSettings("%s.example.com", `{mode: List, excludedServices: ["grafana"]}`, admissionAPIs...)
			cm := configMap()
			hosts := strings.Fields(cm.Field("data.hosts").String())
			Expect(hosts).NotTo(ContainElement("grafana.example.com"))
			Expect(hosts).To(ContainElements("console.example.com", "prometheus.example.com"))
			Expect(strings.Fields(cm.Field("data.allowedHosts").String())).To(BeEmpty(),
				"nothing may be verdictAllowed back out of a pattern that does not exist")
			Expect(strings.Fields(cm.Field("data.excludedHosts").String())).To(Equal([]string{"grafana.example.com"}),
				"still published, so that the effect of the setting reads the same in both modes")
		})

		It("does not apply the grandfathering, there is nothing to grandfather", func() {
			renderWith("%s.example.com", `{mode: List}`, `{recorded: true, hosts: ["console.example.com"]}`, admissionAPIs...)
			cm := configMap()
			Expect(strings.Fields(cm.Field("data.grandfatheredHosts").String())).To(Equal([]string{"console.example.com"}),
				"kept, so that a later switch to Template mode has it and does not snapshot again")
			Expect(strings.Fields(cm.Field("data.allowedHosts").String())).To(BeEmpty())
		})

		It("still answers whether the literal covers the repository", func() {
			expectCoversRepositoryPublicDomains(configMap())
		})

		It("applies both settings at once, and a hostname named twice is reserved once", func() {
			renderWithSettings("%s.example.com",
				`{mode: List, additionalHosts: ["admin.example.com", "console.example.com"], excludedServices: ["grafana", "hubble"]}`,
				admissionAPIs...)
			hosts := strings.Fields(configMap().Field("data.hosts").String())
			Expect(hosts).To(ContainElement("admin.example.com"))
			Expect(hosts).NotTo(ContainElements("grafana.example.com", "hubble.example.com"))
			Expect(hosts).To(Equal(sortedUnique(hosts)))
		})
	})

	// The fallback for a template the global schema rejects, such as one with two %s in the same
	// label, has no render test: values validation refuses such a value before the chart is
	// rendered. The equivalent derivation the hook performs is covered by hooks/lib/publicdomain.
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
