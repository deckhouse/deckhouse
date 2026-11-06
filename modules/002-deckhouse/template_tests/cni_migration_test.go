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
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// cni-migration/ switches a cluster from one CNI to another: a manager on a master and an agent on
// every node. global-hooks/detect_cni_migration.go sets cniMigrationEnabled and cniMigrationName while
// a CNIMigration exists, and cniMigrationWebhooksDisable until it has succeeded.

const (
	cniMigrationName           = "cilium-to-flannel"
	securityPolicyExceptionAPI = "deckhouse.io/v1alpha1/SecurityPolicyException"
	cniMigrationAgent          = "cni-migration-agent"
	cniMigrationManager        = "cni-migration-manager"
)

var _ = Describe("Module :: deckhouse :: CNI migration ::", func() {
	f := SetupHelmConfig(`{deckhouse: {internal: {currentReleaseImageName: test }}}`)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("deckhouse", moduleValuesForMasterNode)
	})

	migrating := func(apiVersions ...string) {
		f.ValuesSet("global.internal.cniMigrationEnabled", true)
		f.ValuesSet("global.internal.cniMigrationName", cniMigrationName)
		f.HelmRender(WithAPIVersions(apiVersions...))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	agent := func() object_store.KubeObject {
		return f.KubernetesResource("DaemonSet", "d8-system", cniMigrationAgent)
	}
	manager := func() object_store.KubeObject {
		return f.KubernetesResource("Deployment", "d8-system", cniMigrationManager)
	}
	exception := func(name string) object_store.KubeObject {
		return f.KubernetesResource("SecurityPolicyException", "d8-system", name)
	}

	It("renders nothing while no migration is in progress", func() {
		f.HelmRender(WithAPIVersions(securityPolicyExceptionAPI))
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		Expect(agent().Exists()).To(BeFalse())
		Expect(manager().Exists()).To(BeFalse())
		for _, name := range []string{cniMigrationAgent, cniMigrationManager} {
			Expect(f.KubernetesResource("ServiceAccount", "d8-system", name).Exists()).To(BeFalse(), name)
			Expect(f.KubernetesGlobalResource("ClusterRole", "d8:"+name).Exists()).To(BeFalse(), name)
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:"+name).Exists()).To(BeFalse(), name)
			Expect(exception(name).Exists()).To(BeFalse(), name)
		}
	})

	Context("while a migration is in progress", func() {
		BeforeEach(func() {
			migrating(securityPolicyExceptionAPI)
		})

		It("runs the manager and an agent on every node for that migration", func() {
			Expect(manager().Exists()).To(BeTrue())
			Expect(manager().Field("spec.replicas").Int()).To(Equal(int64(1)))
			Expect(manager().Field("spec.template.spec.nodeSelector").String()).To(MatchJSON(`{"node-role.kubernetes.io/control-plane":""}`))
			Expect(manager().Field("spec.template.spec.containers.0.args").String()).
				To(MatchJSON(`["--mode=manager","--migration-name=` + cniMigrationName + `"]`))

			Expect(agent().Exists()).To(BeTrue())
			Expect(agent().Field("spec.template.spec.tolerations").String()).To(MatchJSON(`[{"operator":"Exists"}]`),
				"every node has to be switched, whatever taints it has")
			Expect(agent().Field("spec.template.spec.containers.0.args").String()).
				To(MatchJSON(`["--mode=agent","--migration-name=` + cniMigrationName + `"]`))
		})

		It("reaches the API server through the node-local proxy, which stays up while the pod network is replaced", func() {
			for _, workload := range []object_store.KubeObject{manager(), agent()} {
				Expect(workload.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
				container := workload.Field("spec.template.spec.containers.0")
				Expect(container.Get(`env.#(name=="KUBERNETES_SERVICE_HOST").value`).String()).To(Equal("127.0.0.1"))
				Expect(container.Get(`env.#(name=="KUBERNETES_SERVICE_PORT").value`).String()).To(Equal("6445"))
			}
		})

		It("lets the manager drive the migration and the CNI modules", func() {
			Expect(roleGrants(f.KubernetesGlobalResource("ClusterRole", "d8:cni-migration-manager"))).To(Equal(map[string][]string{
				"network.deckhouse.io/cnimigrations":            {"get", "list", "patch", "update", "watch"},
				"network.deckhouse.io/cninodemigrations":        {"get", "list", "patch", "update", "watch"},
				"network.deckhouse.io/cnimigrations/status":     {"get", "patch", "update"},
				"network.deckhouse.io/cninodemigrations/status": {"get", "patch", "update"},
				"deckhouse.io/moduleconfigs":                    {"get", "list", "patch", "update", "watch"},

				"pods":            rbacReadVerbs,
				"services":        rbacReadVerbs,
				"nodes":           rbacReadVerbs,
				"apps/daemonsets": rbacReadVerbs,

				"admissionregistration.k8s.io/validatingwebhookconfigurations": rbacReadVerbs,
				"admissionregistration.k8s.io/mutatingwebhookconfigurations":   rbacReadVerbs,
			}))
		})

		It("lets the agent report the migration of its node and restart the pods on it", func() {
			Expect(roleGrants(f.KubernetesGlobalResource("ClusterRole", "d8:cni-migration-agent"))).To(Equal(map[string][]string{
				"network.deckhouse.io/cnimigrations":            rbacReadVerbs,
				"network.deckhouse.io/cninodemigrations":        {"create", "get", "list", "patch", "update", "watch"},
				"network.deckhouse.io/cninodemigrations/status": {"get", "patch", "update"},

				"pods":  {"delete", "get", "list", "patch", "update", "watch"},
				"nodes": rbacReadVerbs,
			}))
		})

		It("binds each role to the service account of its own workload", func() {
			for name, workload := range map[string]object_store.KubeObject{cniMigrationManager: manager(), cniMigrationAgent: agent()} {
				Expect(workload.Field("spec.template.spec.serviceAccountName").String()).To(Equal(name))
				Expect(f.KubernetesResource("ServiceAccount", "d8-system", name).Exists()).To(BeTrue(), name)

				binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:"+name)
				Expect(binding.Exists()).To(BeTrue(), name)
				Expect(binding.Field("roleRef").String()).To(MatchJSON(`{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"d8:` + name + `"}`))
				Expect(binding.Field("subjects").String()).To(MatchJSON(`[{"kind":"ServiceAccount","name":"` + name + `","namespace":"d8-system"}]`))
			}
		})

		It("points each pod at a security policy exception that exists", func() {
			for _, workload := range []object_store.KubeObject{manager(), agent()} {
				name := workload.Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).String()
				Expect(name).ToNot(BeEmpty())
				Expect(exception(name).Exists()).To(BeTrue(), "the %s exception should be rendered", name)
			}
		})

		It("lets the manager run in the host network", func() {
			Expect(exception(cniMigrationManager).Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
		})

		It("lets the agent into the host namespaces it enters, privileged and as root", func() {
			spec := agent().Field("spec.template.spec")
			allowed := exception(cniMigrationAgent).Field("spec")
			for _, namespace := range []string{"hostNetwork", "hostPID", "hostIPC"} {
				Expect(spec.Get(namespace).Bool()).To(BeTrue(), namespace)
				Expect(allowed.Get("network."+namespace+".allowedValue").Bool()).To(BeTrue(), namespace)
			}

			Expect(spec.Get("containers.0.securityContext.privileged").Bool()).To(BeTrue())
			Expect(allowed.Get("securityContext.privileged.allowedValue").Bool()).To(BeTrue())
			Expect(allowed.Get("securityContext.runAsUser.allowedValues").String()).
				To(MatchJSON(`[` + spec.Get("containers.0.securityContext.runAsUser").String() + `]`))
		})

		// The exception matches a hostPath only when its readOnly equals the one on the mount, so an
		// entry with the wrong value reads as no entry at all.
		It("lists every host path of the agent in its exception, with the readOnly of the mount", func() {
			spec := agent().Field("spec.template.spec")

			mountReadOnly := map[string]bool{}
			for _, mount := range spec.Get("containers.0.volumeMounts").Array() {
				mountReadOnly[mount.Get("name").String()] = mount.Get("readOnly").Bool()
			}

			allowed := map[string]bool{}
			for _, entry := range exception(cniMigrationAgent).Field("spec.volumes.hostPath.allowedValues").Array() {
				allowed[entry.Get("path").String()] = entry.Get("readOnly").Bool()
			}

			hostPaths := 0
			for _, volume := range spec.Get("volumes").Array() {
				path := volume.Get("hostPath.path")
				if !path.Exists() {
					continue
				}
				hostPaths++
				name := volume.Get("name").String()
				Expect(mountReadOnly).To(HaveKey(name), "the %s volume should be mounted", name)
				Expect(allowed).To(HaveKeyWithValue(path.String(), mountReadOnly[name]),
					"%s is mounted with readOnly=%v and the exception must say the same", path.String(), mountReadOnly[name])
			}
			Expect(allowed).To(HaveLen(hostPaths), "the exception should allow no host path the agent does not mount")
		})
	})

	It("renders the exceptions when the SecurityPolicyException kind comes from the CRDs of the modules", func() {
		f.ValuesSetFromYaml("global.discovery.apiVersions", `["`+securityPolicyExceptionAPI+`"]`)
		migrating()

		Expect(exception(cniMigrationAgent).Exists()).To(BeTrue())
		Expect(exception(cniMigrationManager).Exists()).To(BeTrue())
	})

	It("renders no exceptions while the SecurityPolicyException kind is not known", func() {
		migrating()

		Expect(agent().Exists()).To(BeTrue())
		Expect(exception(cniMigrationAgent).Exists()).To(BeFalse())
		Expect(exception(cniMigrationManager).Exists()).To(BeFalse())
	})

	Context("admission webhooks", func() {
		configurator := func() object_store.KubeObject {
			return f.KubernetesResource("ConfigMap", "d8-system", "control-plane-configurator")
		}

		It("asks control-plane-manager to turn the admission webhooks off until the migration has succeeded", func() {
			f.ValuesSet("global.internal.cniMigrationWebhooksDisable", true)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			Expect(configurator().Exists()).To(BeTrue())
			Expect(configurator().Field(`metadata.labels.control-plane-configurator`).Exists()).To(BeTrue(),
				"control-plane-manager reads the ConfigMaps of d8-system by this label")
			Expect(configurator().Field(`metadata.labels.control-plane-configurator`).String()).To(BeEmpty())
			Expect(configurator().Field("data").String()).
				To(MatchJSON(`{"disableAdmissionPlugins":"MutatingAdmissionWebhook,ValidatingAdmissionWebhook"}`))
		})

		It("leaves the admission webhooks on otherwise", func() {
			migrating()

			Expect(configurator().Exists()).To(BeFalse())
		})
	})
})
