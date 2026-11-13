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
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	onDemandVersionLocation = `location ~ ^/(ru|en)/modules/[a-z0-9-]+/(v[0-9]+\.[0-9]+\.[0-9]+[^/]*/|versions/$) {`
	genericModuleLocation   = `/modules/(?<module>[^/]+)/((alpha|beta|early-access|stable|rock-solid|latest)/)?(?<doc_path>.*)$ {`
)

var _ = Describe("Module :: documentation :: helm template :: on-demand module versions", func() {
	f := SetupHelmConfig(``)

	render := func(edition string) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSet("global.clusterIsBootstrapped", true)
		f.ValuesSet("global.deckhouseEdition", edition)
		f.ValuesSet("documentation.internal.auth.password", "plainstring")
		// The Ingress is not what these tests are about.
		f.ValuesSet("global.modules.ingress.enabled", false)
		f.HelmRender()
	}

	Context("Default edition", func() {
		BeforeEach(func() { render("EE") })

		It("Gives docs-builder a cache volume that nginx reads", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deploy := f.KubernetesResource("Deployment", "d8-system", "documentation")
			Expect(deploy.Exists()).To(BeTrue())

			Expect(deploy.Field(`spec.template.spec.containers.#(name=="builder").args`).String()).
				To(ContainSubstring("--ondemand-cache-dir=/app/ondemand"))

			Expect(deploy.Field(`spec.template.spec.volumes.#(name=="ondemand-cache").emptyDir.sizeLimit`).String()).To(Equal("4Gi"))
			// docs-builder reports the room of the pinned versions from it.
			Expect(deploy.Field(`spec.template.spec.containers.#(name=="builder").args`).String()).
				To(ContainSubstring("--ondemand-cache-size=4Gi"))

			Expect(deploy.Field(`spec.template.spec.containers.#(name=="builder").volumeMounts.#(name=="ondemand-cache").mountPath`).String()).To(Equal("/app/ondemand"))
			Expect(deploy.Field(`spec.template.spec.containers.#(name=="builder").volumeMounts.#(name=="ondemand-cache").readOnly`).Bool()).To(BeFalse())

			Expect(deploy.Field(`spec.template.spec.containers.#(name=="web").volumeMounts.#(name=="ondemand-cache").mountPath`).String()).To(Equal("/app/ondemand"))
			Expect(deploy.Field(`spec.template.spec.containers.#(name=="web").volumeMounts.#(name=="ondemand-cache").readOnly`).Bool()).To(BeTrue())
		})

		It("Gives the builder a core to render versions, also under VPA", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deploy := f.KubernetesResource("Deployment", "d8-system", "documentation")
			Expect(deploy.Field(`spec.template.spec.containers.#(name=="builder").resources.limits.cpu`).String()).To(Equal("1"))
			Expect(deploy.Field(`spec.template.spec.containers.#(name=="kube-rbac-proxy").resources.requests.cpu`).Exists()).To(BeTrue())

			Expect(deploy.Field(`spec.template.spec.containers.#(name=="builder").resources.requests.cpu`).String()).To(Equal("250m"))

			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "documentation")
			Expect(vpa.Exists()).To(BeTrue())
			Expect(vpa.Field("spec.updatePolicy.updateMode").String()).To(Equal("InPlaceOrRecreate"))
			// Limits follow the requests (CPU 250m:1): the request stays 250m,
			// the limit one core.
			builder := vpa.Field(`spec.resourcePolicy.containerPolicies.#(containerName=="builder")`)
			Expect(builder.Get("controlledValues").Exists()).To(BeFalse())
			Expect(builder.Get("minAllowed.cpu").String()).To(Equal("250m"))
			Expect(builder.Get("maxAllowed.cpu").String()).To(Equal("250m"))
			Expect(builder.Get("minAllowed.memory").String()).To(Equal(builder.Get("maxAllowed.memory").String()))
		})

		It("Keeps the docs-builder API and metrics off the pod address", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deploy := f.KubernetesResource("Deployment", "d8-system", "documentation")
			builder := deploy.Field(`spec.template.spec.containers.#(name=="builder")`)
			Expect(builder.Get("args").String()).To(ContainSubstring(`"--address=127.0.0.1:8081"`))
			Expect(builder.Get("args").String()).To(ContainSubstring(`"--metrics-address=127.0.0.1:9090"`))
			Expect(builder.Get("ports").Exists()).To(BeFalse())
			Expect(builder.Get("livenessProbe.httpGet.port").Int()).To(Equal(int64(8443)))
			Expect(builder.Get("readinessProbe.httpGet.port").Int()).To(Equal(int64(8443)))

			proxy := deploy.Field(`spec.template.spec.containers.#(name=="kube-rbac-proxy").env.#(name=="KUBE_RBAC_PROXY_CONFIG").value`).String()
			Expect(proxy).To(ContainSubstring("upstream: http://127.0.0.1:9090/metrics"))
			Expect(proxy).To(ContainSubstring("subresource: prometheus-metrics"))

		})

		It("Keeps the ServiceAccount token away from nginx", func() {
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			const tokenDir = "/var/run/secrets/kubernetes.io/serviceaccount"
			deploy := f.KubernetesResource("Deployment", "d8-system", "documentation")
			pod := deploy.Field("spec.template.spec")
			Expect(pod.Get("automountServiceAccountToken").Exists()).To(BeTrue())
			Expect(pod.Get("automountServiceAccountToken").Bool()).To(BeFalse())
			Expect(pod.Get("initContainers").Exists()).To(BeFalse())

			sources := pod.Get(`volumes.#(name=="kube-api-access").projected.sources`)
			Expect(sources.Get("#").Int()).To(Equal(int64(3)))
			Expect(sources.Get("0.serviceAccountToken.path").String()).To(Equal("token"))
			Expect(sources.Get("0.serviceAccountToken.audience").Exists()).To(BeFalse())
			Expect(sources.Get("1.configMap.name").String()).To(Equal("kube-root-ca.crt"))
			Expect(sources.Get("1.configMap.items.0.path").String()).To(Equal("ca.crt"))
			Expect(sources.Get("2.downwardAPI.items.0.path").String()).To(Equal("namespace"))

			for _, name := range []string{"builder", "kube-rbac-proxy"} {
				mount := pod.Get(`containers.#(name=="` + name + `").volumeMounts.#(name=="kube-api-access")`)
				Expect(mount.Get("mountPath").String()).To(Equal(tokenDir), name)
				Expect(mount.Get("readOnly").Bool()).To(BeTrue(), name)
			}

			web := pod.Get(`containers.#(name=="web").volumeMounts`)
			Expect(web.Get(`#(name=="kube-api-access")`).Exists()).To(BeFalse())
			Expect(web.String()).NotTo(ContainSubstring(tokenDir))
			// Every container is accounted for: a new one gets no token by default.
			Expect(pod.Get("containers.#").Int()).To(Equal(int64(3)))
		})

		It("Scrapes the docs-builder metrics through kube-rbac-proxy", func() {
			f.ValuesSet("global.discovery.apiVersions", []string{"monitoring.coreos.com/v1/PodMonitor"})
			f.ValuesSet("global.enabledModules", []string{"documentation", "prometheus"})
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "documentation").Field("spec.podMetricsEndpoints.0.port").String()).To(Equal("https"))
			Expect(f.KubernetesResource("RoleBinding", "d8-system", "access-to-documentation-prometheus-metrics").Exists()).To(BeTrue())
		})

		It("Lets docs-builder read modules and their sources", func() {
			role := f.KubernetesGlobalResource("ClusterRole", "d8:documentation:module-sources")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(MatchJSON(`[{"apiGroups":["deckhouse.io"],"resources":["modules","modulesources","modulepackages","packagerepositories"],"verbs":["get"]},{"apiGroups":["deckhouse.io"],"resources":["modules","modulepulloverrides","modulereleases","modulepackages"],"verbs":["list"]},{"apiGroups":["deckhouse.io"],"resources":["modulesources/sensitive","packagerepositories/sensitive"],"verbs":["get"]},{"apiGroups":["deckhouse.io"],"resources":["registryconfigs"],"resourceNames":["registry"],"verbs":["get"]}]`))

			binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:documentation:module-sources")
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("roleRef.name").String()).To(Equal("d8:documentation:module-sources"))
			Expect(binding.Field("subjects").String()).To(MatchJSON(`[{"kind":"ServiceAccount","name":"documentation","namespace":"d8-system"}]`))
		})

		It("Lets docs-builder read the store of the registry module and nothing else of it", func() {
			role := f.KubernetesResource("Role", "d8-system", "documentation:registry-storage-client")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(MatchJSON(`[{"apiGroups":[""],"resources":["secrets"],"resourceNames":["registry-storage-client"],"verbs":["get"]}]`))

			binding := f.KubernetesResource("RoleBinding", "d8-system", "documentation:registry-storage-client")
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("roleRef").String()).To(MatchJSON(`{"kind":"Role","name":"documentation:registry-storage-client","apiGroup":"rbac.authorization.k8s.io"}`))
			Expect(binding.Field("subjects").String()).To(MatchJSON(`[{"kind":"ServiceAccount","name":"documentation","namespace":"d8-system"}]`))
		})
	})

	for _, edition := range []string{"EE", "CSE"} {
		Context("nginx config of the "+edition+" edition", func() {
			BeforeEach(func() { render(edition) })

			It("Routes module version pages to the cache volume before the generic module location", func() {
				Expect(f.RenderError).ShouldNot(HaveOccurred())

				conf := f.KubernetesResource("ConfigMap", "d8-system", "documentation-nginx-config").Field(`data.nginx\.conf`).String()

				Expect(conf).To(ContainSubstring("log_format ondemand_access '$uri';"))
				Expect(conf).To(ContainSubstring("root /app/ondemand/.site;"))
				Expect(conf).To(ContainSubstring("try_files $uri $uri/ @ondemand_render;"))
				Expect(conf).To(ContainSubstring("access_log syslog:server=127.0.0.1:5514,nohostname,tag=docs_ondemand ondemand_access;"))
				Expect(conf).To(ContainSubstring("location ^~ /api/v2/ {"))
				Expect(conf).NotTo(ContainSubstring("/api/v1"))

				version := strings.Index(conf, onDemandVersionLocation)
				generic := strings.Index(conf, genericModuleLocation)
				Expect(version).To(BeNumerically(">", 0))
				Expect(generic).To(BeNumerically(">", version))

				// The pages are compressed after SSI: a gzip copy sent as is
				// would keep the includes of the header unexpanded.
				location := conf[version:]
				location = location[:strings.Index(location, "}")]
				Expect(location).NotTo(ContainSubstring("gzip_static"))
				Expect(location).To(ContainSubstring("gzip on;"))
				Expect(location).To(ContainSubstring("gzip_vary on;"))
				Expect(location).To(ContainSubstring("gzip_proxied any;"))
			})

			It("Looks a development branch up among the rendered versions in the generic module location", func() {
				conf := f.KubernetesResource("ConfigMap", "d8-system", "documentation-nginx-config").Field(`data.nginx\.conf`).String()

				generic := conf[strings.Index(conf, genericModuleLocation):]
				generic = generic[:strings.Index(generic, "set $embedded_module_path")]
				Expect(generic).To(ContainSubstring("set $ondemand_branch_dir /app/ondemand/.site/$ondemand_lang/modules/$ondemand_module/$ondemand_tag;"))
				Expect(generic).To(ContainSubstring("rewrite ^ /ondemand-branch$uri last;"))
				// A directory of the module pages is not taken for a branch.
				Expect(generic).To(ContainSubstring("set $ondemand_doc_dir /app/ondemand/.site/$ondemand_lang/modules/$ondemand_module/stable/$ondemand_tag;"))
				Expect(generic).To(ContainSubstring("if (-d $ondemand_doc_dir) {"))
				Expect(conf).To(ContainSubstring("location ^~ /ondemand-branch/ {"))
				branch := conf[strings.Index(conf, "location ^~ /ondemand-branch/ {"):]
				branch = branch[:strings.Index(branch, "}")]
				Expect(branch).NotTo(ContainSubstring("gzip_static"))
				Expect(branch).To(ContainSubstring("gzip on;"))
				Expect(branch).To(ContainSubstring("gzip_proxied any;"))
			})

			It("Serves the search index and the list of modules docs-builder builds, nothing pushed", func() {
				conf := f.KubernetesResource("ConfigMap", "d8-system", "documentation-nginx-config").Field(`data.nginx\.conf`).String()

				Expect(conf).To(MatchRegexp(`try_files /ondemand/\.index/\$(lang|redirection_lang)/search\.json =404;`))
				Expect(conf).To(ContainSubstring("try_files /ondemand/.index/$list_lang/modules/index.html @ondemand_render;"))
				list := conf[strings.Index(conf, "^/(?<list_lang>ru|en)/modules/$ {"):]
				list = list[:strings.Index(list, "}")]
				// Compressed after SSI, like the version pages.
				Expect(list).NotTo(ContainSubstring("gzip_static"))
				Expect(list).To(ContainSubstring("gzip on;"))
				for _, pushed := range []string{"/external-modules/", "/search-external-modules-index/", "external_module_path"} {
					Expect(conf).NotTo(ContainSubstring(pushed))
				}
			})

			It("Serves a module without a version from the version docs-builder links, then embedded, then docs-builder", func() {
				conf := f.KubernetesResource("ConfigMap", "d8-system", "documentation-nginx-config").Field(`data.nginx\.conf`).String()

				generic := conf[strings.Index(conf, genericModuleLocation):]
				generic = generic[:strings.Index(generic, "\n}")]
				linked := strings.Index(generic, "if (-d /app/ondemand/.site/$module_lang/modules/$module/stable) {")
				embedded := strings.Index(generic, "if (-d $document_root/$embedded_module_path) {")
				Expect(linked).To(BeNumerically(">", 0))
				Expect(embedded).To(BeNumerically(">", linked))
				Expect(generic).To(ContainSubstring("rewrite ^ /ondemand-branch/$module_lang/modules/$module/stable/$doc_path last;"))
				Expect(generic[embedded:]).To(ContainSubstring("proxy_pass http://127.0.0.1:8081;"))
			})
		})
	}
})
