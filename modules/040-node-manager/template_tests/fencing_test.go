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
	"os"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

const (
	fencingNamespace = "d8-cloud-instance-manager"
	fencingGroup     = "node-manager.deckhouse.io"
	fencingVAPName   = "fencing-agent-own-node-only.deckhouse.io"

	fencingStatesVAPName = "fencing-agent-failed-node-states.deckhouse.io"

	fencingCriticalModeVAPName = "fencing-critical-mode.deckhouse.io"
)

var _ = Describe("Module :: node-manager :: helm template :: fencing agent", func() {
	f := SetupHelmConfig(``)

	// renderIn renders the module in the edition. An empty edition keeps the one of globalValues.
	renderIn := func(edition, fencing string) {
		f.ValuesSetFromYaml("global", globalValues)
		if edition != "" {
			f.ValuesSet("global.deckhouseEdition", edition)
		}
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("global.discovery.apiVersions", `["deckhouse.io/v1alpha1/SecurityPolicyException"]`)
		f.ValuesSetFromYaml("nodeManager", nodeManagerConfigValues+fencingDataInternalValues)
		f.ValuesSetFromYaml("nodeManager.internal.fencing", fencing)
		f.ApplyOpenAPIDefaults()
		setBashibleAPIServerTLSValues(f)
		f.HelmRender(WithAPIVersions("autoscaling.k8s.io/v1/VerticalPodAutoscaler", "monitoring.coreos.com/v1/PodMonitor"))
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	render := func(fencing string) { renderIn("", fencing) }

	agent := func(nodeGroup string) object_store.KubeObject {
		return f.KubernetesResource("DaemonSet", fencingNamespace, "fencing-agent-"+nodeGroup)
	}

	exception := func(nodeGroup string) object_store.KubeObject {
		return f.KubernetesResource("SecurityPolicyException", fencingNamespace, "fencing-agent-"+nodeGroup)
	}

	clusterRole := func(name string) object_store.KubeObject {
		return f.KubernetesGlobalResource("ClusterRole", name)
	}

	hostPaths := func(obj object_store.KubeObject, path string) []string {
		var paths []string
		for _, p := range obj.Field(path).Array() {
			paths = append(paths, p.Get("path").String())
		}
		return paths
	}

	envNames := func(ds object_store.KubeObject) []string {
		var names []string
		for _, e := range ds.Field("spec.template.spec.containers.0.env").Array() {
			names = append(names, e.Get("name").String())
		}
		return names
	}

	podTemplate := func(nodeGroup string) string {
		return agent(nodeGroup).Field("spec.template").Raw
	}

	agentBindings := func() []object_store.KubeObject {
		return []object_store.KubeObject{
			f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:fencing-agent"),
			f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:fencing-agent:rbac-proxy"),
			f.KubernetesResource("RoleBinding", fencingNamespace, "fencing-agent"),
			f.KubernetesResource("RoleBinding", "default", "d8:node-manager:fencing-agent"),
		}
	}

	// occupiedPorts returns the ports that the containers of a workload listen on: the container
	// and host ports, the ports of the HTTP probes, and the ports in the address and port arguments
	// and in the host:port values of the environment.
	occupiedPorts := func(workload object_store.KubeObject) []string {
		argPort := regexp.MustCompile(`^--[a-z-]*(?:address|port)=(?:.*:)?([0-9]+)$`)
		envPort := regexp.MustCompile(`^[^\s]*:([0-9]+)$`)
		var ports []string
		for _, container := range workload.Field("spec.template.spec.containers").Array() {
			for _, port := range container.Get("ports.#.containerPort").Array() {
				ports = append(ports, port.String())
			}
			for _, port := range container.Get("ports.#.hostPort").Array() {
				ports = append(ports, port.String())
			}
			for _, value := range container.Get("env.#.value").Array() {
				if m := envPort.FindStringSubmatch(value.String()); m != nil {
					ports = append(ports, m[1])
				}
			}
			for _, probe := range []string{"livenessProbe", "readinessProbe"} {
				if port := container.Get(probe + ".httpGet.port"); port.Exists() {
					ports = append(ports, port.String())
				}
			}
			for _, arg := range container.Get("args").Array() {
				if m := argPort.FindStringSubmatch(arg.String()); m != nil {
					ports = append(ports, m[1])
				}
			}
		}
		return ports
	}

	// awaited returns the objects that nelm must see deleted before it deletes obj.
	awaited := func(obj object_store.KubeObject) []string {
		var objects []string
		for key, value := range obj.Field("metadata.annotations").Map() {
			if !strings.HasPrefix(key, "werf.io/delete-dependency-") {
				continue
			}
			props := map[string]string{}
			for _, prop := range strings.Split(value.String(), ",") {
				k, v, _ := strings.Cut(prop, "=")
				props[k] = v
			}
			Expect(props["state"]).To(Equal("absent"), key)
			objects = append(objects, props["group"]+"/"+props["kind"]+"/"+props["namespace"]+"/"+props["name"])
		}
		return objects
	}

	Context("every node group is Disabled", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Disabled}}`)
		})

		It("deploys no agent", func() {
			Expect(agent("master").Exists()).To(BeFalse())
			Expect(agent("worker").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", fencingNamespace, "fencing-agent-master").Exists()).To(BeFalse())
			Expect(exception("master").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "fencing-agent").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("NodeGroupConfiguration", "enable-watchdog-for-nodegroup-worker.sh").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("NodeGroupConfiguration", "fencing-agent-socket-directory.sh").Exists()).To(BeFalse())
		})

		// Helm deletes the DaemonSets when fencing turns off. A stopping agent must
		// still write AgentStopping to its Node. If not, its watchdog stays armed. The
		// policies limit the rights that stay.
		It("keeps the ServiceAccount of the agent, its right to write the Node condition and its policies", func() {
			for _, name := range []string{fencingVAPName, fencingStatesVAPName} {
				Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", name).Exists()).To(BeTrue(), name)
				Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", name).Field("spec.policyName").String()).To(Equal(name))
			}
			Expect(f.KubernetesResource("ServiceAccount", fencingNamespace, "fencing-agent").Exists()).To(BeTrue())
			binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:fencing-agent")
			Expect(binding.Field("roleRef.name").String()).To(Equal("d8:node-manager:fencing-agent"))
			Expect(binding.Field("subjects.0.name").String()).To(Equal("fencing-agent"))
			Expect(binding.Field("subjects.0.namespace").String()).To(Equal(fencingNamespace))
			Expect(roleVerbs(clusterRole("d8:node-manager:fencing-agent"), "", "nodes/status")).To(ContainElement("patch"))
		})

		It("lets nelm delete the bindings of the agent without a wait, because no DaemonSet of the agent exists", func() {
			for _, binding := range agentBindings() {
				Expect(binding.Exists()).To(BeTrue())
				Expect(awaited(binding)).To(BeEmpty(), binding.Field("metadata.name").String())
			}
		})

		It("deploys nothing of the controller", func() {
			Expect(f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", fencingNamespace, "fencing-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodDisruptionBudget", fencingNamespace, "fencing-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("ServiceAccount", fencingNamespace, "fencing-controller").Exists()).To(BeFalse())
			Expect(clusterRole("d8:node-manager:fencing-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:fencing-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:fencing-controller:rbac-proxy").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Role", fencingNamespace, "fencing-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("RoleBinding", fencingNamespace, "fencing-controller").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Role", fencingNamespace, "fencing-controller-leader-election").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("RoleBinding", fencingNamespace, "fencing-controller-leader-election").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("Role", fencingNamespace, "access-to-fencing-controller-prometheus-metrics").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("RoleBinding", fencingNamespace, "access-to-fencing-controller-prometheus-metrics").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "fencing-controller").Exists()).To(BeFalse())
		})
	})

	DescribeTable("the Critical mode is available only in the commercial editions",
		func(edition string, commercial bool) {
			renderIn(edition, `nodeGroups: {master: {mode: Disabled}, worker: {mode: Normal}}`)

			Expect(f.KubernetesGlobalResource("FencingSLAProfile", "critical").Exists()).To(Equal(commercial))
			Expect(f.KubernetesGlobalResource("FencingSLAProfile", "normal").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("FencingSLAProfile", "slow").Exists()).To(BeTrue())
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", fencingCriticalModeVAPName).Exists()).To(Equal(!commercial))
			Expect(f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", fencingCriticalModeVAPName).Exists()).To(Equal(!commercial))
			Expect(agent("worker").Exists()).To(BeTrue())
		},
		Entry("CE", "CE", false),
		Entry("EE", "EE", true),
		Entry("FE", "FE", true),
	)

	Context("a Slow group among Disabled groups", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Slow}, system: {mode: Disabled}}`)
		})

		It("deploys a DaemonSet and a VPA for every group", func() {
			for _, ng := range []string{"master", "worker", "system"} {
				Expect(agent(ng).Exists()).To(BeTrue(), ng)
				Expect(f.KubernetesResource("VerticalPodAutoscaler", fencingNamespace, "fencing-agent-"+ng).Exists()).To(BeTrue(), ng)
			}
		})

		It("labels the pods as agents of version 2 and keeps the selector of the DaemonSet", func() {
			ds := agent("worker")
			Expect(ds.Field("spec.template.metadata.labels.app").String()).To(Equal("fencing-agent"))
			Expect(ds.Field(`spec.template.metadata.labels.node-manager\.deckhouse\.io/fencing-agent-version`).String()).To(Equal("2"))
			Expect(ds.Field("spec.selector.matchLabels").Map()).To(HaveLen(2))
			Expect(ds.Field("spec.selector.matchLabels.app").String()).To(Equal("fencing-agent"))
			Expect(ds.Field("spec.selector.matchLabels.node-group-name").String()).To(Equal("worker"))
		})

		It("runs on the host network without a host port", func() {
			ds := agent("worker")
			Expect(ds.Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			for _, c := range ds.Field("spec.template.spec.containers").Array() {
				for _, p := range c.Get("ports").Array() {
					Expect(p.Get("hostPort").Exists()).To(BeFalse(), c.Get("name").String())
				}
			}
		})

		It("has no environment variable of the configuration", func() {
			for _, ng := range []string{"master", "worker", "system"} {
				Expect(envNames(agent(ng))).To(ConsistOf("LOG_LEVEL", "WATCHDOG_DEVICE", "API_SOCKET_PATH", "HEALTH_PROBE_BIND_ADDRESS", "NODE_NAME",
					"KUBERNETES_SERVICE_HOST", "KUBERNETES_SERVICE_PORT"), ng)
			}
		})

		It("connects the agent to the API server through the local kubernetes-api-proxy", func() {
			for _, ng := range []string{"master", "worker", "system"} {
				env := agent(ng).Field(`spec.template.spec.containers.#(name=="fencing-agent").env`)
				Expect(env.Get(`#(name=="KUBERNETES_SERVICE_HOST").value`).String()).To(Equal("127.0.0.1"), ng)
				Expect(env.Get(`#(name=="KUBERNETES_SERVICE_PORT").value`).String()).To(Equal("6445"), ng)
			}
		})

		It("has no checksum and no pod-reloader annotation in the pod template", func() {
			for _, ng := range []string{"master", "worker", "system"} {
				for key := range agent(ng).Field("spec.template.metadata.annotations").Map() {
					Expect(key).NotTo(MatchRegexp("checksum|reloader"), ng)
				}
			}
		})

		It("updates a group by a rolling update without surge", func() {
			strategy := agent("worker").Field("spec.updateStrategy")
			Expect(strategy.Get("type").String()).To(Equal("RollingUpdate"))
			Expect(strategy.Get("rollingUpdate.maxSurge").Int()).To(BeZero())
		})

		It("has the critical priority and a memory limit", func() {
			ds := agent("worker")
			Expect(ds.Field("spec.template.spec.priorityClassName").String()).To(Equal("system-node-critical"))
			Expect(ds.Field("spec.template.spec.containers.0.resources.limits.memory").Exists()).To(BeTrue())
		})

		It("has a VPA for the agent container", func() {
			policy := f.KubernetesResource("VerticalPodAutoscaler", fencingNamespace, "fencing-agent-worker").Field("spec.resourcePolicy.containerPolicies.0")
			Expect(policy.Get("containerName").String()).To(Equal("fencing-agent"))
		})

		It("resolves names of cluster services on the host network", func() {
			Expect(agent("worker").Field("spec.template.spec.hostNetwork").Bool()).To(BeTrue())
			Expect(agent("worker").Field("spec.template.spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
		})

		It("gives the watchdog and the privileges only to the armed group", func() {
			armed := agent("worker").Field("spec.template.spec.containers.0.securityContext")
			Expect(armed.Get("privileged").Bool()).To(BeTrue())
			Expect(armed.Get("capabilities.drop").Array()).To(HaveLen(1))
			Expect(armed.Get("capabilities.drop.0").String()).To(Equal("ALL"))
			Expect(armed.Get("capabilities.add").Array()).To(HaveLen(1))
			Expect(armed.Get("capabilities.add.0").String()).To(Equal("NET_RAW"))
			Expect(armed.Get("seLinuxOptions.type").String()).To(Equal("spc_t"))
			Expect(agent("worker").Field(`spec.template.metadata.annotations.container\.apparmor\.security\.beta\.kubernetes\.io/fencing-agent`).String()).To(Equal("unconfined"))
			Expect(hostPaths(agent("worker"), "spec.template.spec.volumes.#.hostPath")).To(ConsistOf("/dev/watchdog", "/var/run/fencing-agent"))

			disabled := agent("master").Field("spec.template.spec.containers.0.securityContext")
			Expect(disabled.Get("privileged").Exists()).To(BeFalse())
			Expect(disabled.Get("allowPrivilegeEscalation").Bool()).To(BeFalse())
			Expect(disabled.Get("capabilities.drop").Array()).To(HaveLen(1))
			Expect(disabled.Get("capabilities.drop.0").String()).To(Equal("ALL"))
			Expect(disabled.Get("capabilities.add").Exists()).To(BeFalse())
			Expect(agent("master").Field("spec.template.metadata.annotations").Exists()).To(BeFalse())
			Expect(hostPaths(agent("master"), "spec.template.spec.volumes.#.hostPath")).To(ConsistOf("/var/run/fencing-agent"))
		})

		It("asks for SYS_ADMIN in no pod and no exception", func() {
			for _, ng := range []string{"master", "worker", "system"} {
				Expect(agent(ng).ToYaml()).NotTo(ContainSubstring("SYS_ADMIN"), ng)
				Expect(exception(ng).ToYaml()).NotTo(ContainSubstring("SYS_ADMIN"), ng)
			}
		})

		It("exposes the metrics through kube-rbac-proxy", func() {
			proxy := agent("worker").Field(`spec.template.spec.containers.#(name=="kube-rbac-proxy")`)
			Expect(proxy.Exists()).To(BeTrue())
			var listenPort string
			for _, arg := range proxy.Get("args").Array() {
				if v, ok := strings.CutPrefix(arg.String(), "--secure-listen-address="); ok {
					listenPort = v[strings.LastIndex(v, ":")+1:]
				}
			}
			Expect(listenPort).NotTo(BeEmpty())
			Expect(listenPort).To(Equal(proxy.Get(`ports.#(name=="https-metrics").containerPort`).String()))
			// On the host network kubelet gives an empty status.podIP while the node has no
			// addresses, and the proxy then listens on 0.0.0.0. status.hostIP fails instead.
			Expect(agent("worker").Field(rbacProxyListenAddressFieldPath).String()).To(Equal("status.hostIP"))
			// The proxy forwards only /metrics: /healthz and /readyz of the agent stay local.
			Expect(proxy.Get(`env.#(name=="KUBE_RBAC_PROXY_CONFIG").value`).String()).To(MatchYAML(`
upstreams:
- upstream: http://127.0.0.1:4250/metrics
  path: /metrics
  authorization:
    resourceAttributes:
      namespace: d8-cloud-instance-manager
      apiGroup: apps
      apiVersion: v1
      resource: daemonsets
      subresource: prometheus-metrics
      name: fencing-agent
`))
			Expect(proxy.Get("ports.0.name").String()).To(Equal("https-metrics"))

			monitor := f.KubernetesResource("PodMonitor", "d8-monitoring", "fencing-agent")
			Expect(monitor.Exists()).To(BeTrue())
			Expect(monitor.Field("spec.podMetricsEndpoints.0.port").String()).To(Equal("https-metrics"))
			Expect(monitor.Field("spec.podMetricsEndpoints.0.path").String()).To(BeElementOf("", "/metrics"))
			Expect(monitor.Field("spec.selector.matchLabels.app").String()).To(Equal("fencing-agent"))

			role := f.KubernetesResource("Role", fencingNamespace, "access-to-fencing-agent-prometheus-metrics")
			Expect(roleVerbs(role, "apps", "daemonsets/prometheus-metrics")).To(ConsistOf("get"))
		})

		It("serves the health and the metrics of the agent only on the loopback address", func() {
			container := agent("worker").Field(`spec.template.spec.containers.#(name=="fencing-agent")`)
			Expect(container.Get(`env.#(name=="HEALTH_PROBE_BIND_ADDRESS").value`).String()).To(Equal("127.0.0.1:4250"))
			for _, probe := range []string{"livenessProbe", "readinessProbe"} {
				Expect(container.Get(probe+".httpGet.host").String()).To(Equal("127.0.0.1"), probe)
				Expect(container.Get(probe+".httpGet.port").Int()).To(Equal(int64(4250)), probe)
			}
		})

		It("registers the ports that the agent occupies on the node", func() {
			ports, err := os.ReadFile("../../../docs/documentation/_data/deckhouse-ports.yml")
			Expect(err).NotTo(HaveOccurred())
			// 8500 is the memberlist port. It is set in the code of the agent, not in the pod.
			for _, port := range append(occupiedPorts(agent("worker")), "8500") {
				Expect(string(ports)).To(ContainSubstring(`ports: "`+port+`"`), port)
			}
		})
	})

	Context("a group on Deckhouse Engine", func() {
		BeforeEach(func() {
			render(`nodeGroups: {engine: {mode: Critical, engine: true}, worker: {mode: Critical, engine: false}}`)
		})

		It("uses the ClusterIP of the API server, not the local kubernetes-api-proxy", func() {
			Expect(envNames(agent("engine"))).NotTo(ContainElement("KUBERNETES_SERVICE_HOST"))
			Expect(envNames(agent("engine"))).NotTo(ContainElement("KUBERNETES_SERVICE_PORT"))
			Expect(envNames(agent("worker"))).To(ContainElements("KUBERNETES_SERVICE_HOST", "KUBERNETES_SERVICE_PORT"))
		})
	})

	Context("the key of memberlist", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Slow}}`)
		})

		It("is the whole Secret mounted as a volume, without subPath, in every group", func() {
			for _, ng := range []string{"master", "worker"} {
				ds := agent(ng)
				Expect(ds.Field(`spec.template.spec.volumes.#(name=="memberlist-key").secret.secretName`).String()).To(Equal("fencing-memberlist-key"), ng)
				Expect(ds.Field(`spec.template.spec.volumes.#(name=="memberlist-key").secret.items`).Exists()).To(BeFalse(), ng)
				mount := ds.Field(`spec.template.spec.containers.0.volumeMounts.#(name=="memberlist-key")`)
				Expect(mount.Get("mountPath").String()).To(Equal("/etc/fencing/memberlist-key"), ng)
				Expect(mount.Get("subPath").Exists()).To(BeFalse(), ng)
				Expect(mount.Get("readOnly").Bool()).To(BeTrue(), ng)
			}
		})
	})

	Context("the pod template", func() {
		It("does not change when a group changes between armed modes", func() {
			render(`nodeGroups: {worker: {mode: Slow}, db: {mode: Normal}}`)
			slow := podTemplate("worker")

			render(`nodeGroups: {worker: {mode: Critical}, db: {mode: Normal}}`)
			Expect(podTemplate("worker")).To(Equal(slow))
		})

		It("does not change when the profile of memberlist changes", func() {
			render(`nodeGroups: {worker: {mode: Slow}, db: {mode: Slow}}`)
			slow := podTemplate("worker")

			render(`nodeGroups: {worker: {mode: Slow}, db: {mode: Critical}}`)
			Expect(podTemplate("worker")).To(Equal(slow))
		})

		It("changes when a group changes between Disabled and an armed mode", func() {
			render(`nodeGroups: {worker: {mode: Slow}, db: {mode: Critical}}`)
			armed := podTemplate("worker")

			render(`nodeGroups: {worker: {mode: Disabled}, db: {mode: Critical}}`)
			Expect(podTemplate("worker")).NotTo(Equal(armed))
		})

		It("does not change with the migration from 1.0, while the update strategy does", func() {
			render(`nodeGroups: {worker: {mode: Slow}}`)
			steady := podTemplate("worker")
			Expect(agent("worker").Field("spec.updateStrategy.rollingUpdate.maxUnavailable").String()).To(Equal("10%"))

			render(`{nodeGroups: {worker: {mode: Slow}}, migrationFrom10: true}`)
			Expect(podTemplate("worker")).To(Equal(steady))
			strategy := agent("worker").Field("spec.updateStrategy.rollingUpdate")
			Expect(strategy.Get("maxSurge").Int()).To(BeZero())
			Expect(strategy.Get("maxUnavailable").String()).To(Equal("100%"))
		})
	})

	Context("the exception of the security policy", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Critical}}`)
		})

		It("allows the host network and only NET_RAW to the armed group", func() {
			armed := exception("worker")
			Expect(armed.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			Expect(armed.Field("spec.securityContext.privileged.allowedValue").Bool()).To(BeTrue())
			Expect(armed.Field("spec.securityContext.capabilities.allowedValues.add").Array()).To(HaveLen(1))
			Expect(armed.Field("spec.securityContext.capabilities.allowedValues.add.0").String()).To(Equal("NET_RAW"))
			Expect(hostPaths(armed, "spec.volumes.hostPath.allowedValues")).To(ConsistOf("/dev/watchdog", "/var/run/fencing-agent"))
		})

		It("allows the host network and no privilege to a Disabled group", func() {
			disabled := exception("master")
			Expect(disabled.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			Expect(disabled.Field("spec.securityContext").Exists()).To(BeFalse())
			Expect(hostPaths(disabled, "spec.volumes.hostPath.allowedValues")).To(ConsistOf("/var/run/fencing-agent"))
		})

		It("is named by the pods of every group", func() {
			for _, ng := range []string{"master", "worker"} {
				Expect(agent(ng).Field(`spec.template.metadata.labels.security\.deckhouse\.io/security-policy-exception`).String()).To(Equal("fencing-agent-" + ng))
			}
		})

		It("allows the container ports of the agent", func() {
			for _, ng := range []string{"master", "worker"} {
				var allowed []int64
				for _, port := range exception(ng).Field("spec.network.hostPorts.#.port").Array() {
					allowed = append(allowed, port.Int())
				}
				for _, port := range agent(ng).Field("spec.template.spec.containers.#.ports.#.containerPort|@flatten").Array() {
					Expect(allowed).To(ContainElement(port.Int()), ng)
				}
			}
		})
	})

	// When the module is disabled, a stopping armed agent must still write
	// AgentStopping to its Node. So nelm deletes its rights only after its pods.
	Context("the deletion order of the agent", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Slow}}`)
		})

		It("deletes the bindings of the agent only after every DaemonSet of the agent is gone", func() {
			var daemonSets []string
			for _, ng := range []string{"master", "worker"} {
				Expect(agent(ng).Exists()).To(BeTrue(), ng)
				daemonSets = append(daemonSets, "apps/DaemonSet/"+fencingNamespace+"/fencing-agent-"+ng)
			}
			for _, binding := range agentBindings() {
				Expect(binding.Exists()).To(BeTrue())
				Expect(awaited(binding)).To(ConsistOf(daemonSets), binding.Field("metadata.name").String())
			}
		})

		It("deletes a DaemonSet of the agent after its pods", func() {
			for _, ng := range []string{"master", "worker"} {
				Expect(agent(ng).Field(`metadata.annotations.werf\.io/delete-propagation`).String()).To(Equal("Foreground"), ng)
			}
		})
	})

	Context("the RBAC of the agent", func() {
		BeforeEach(func() {
			render(`nodeGroups: {worker: {mode: Slow}}`)
		})

		// The agent watches its own Node, patches it and its status, reads the
		// three built-in profiles by name and does not update or patch a
		// FencingFailedNodeState except its status.
		It("gives the rights of the cluster", func() {
			role := clusterRole("d8:node-manager:fencing-agent")
			Expect(roleVerbs(role, "", "nodes")).To(ConsistOf("list", "watch", "patch"))
			Expect(roleVerbs(role, "", "nodes/status")).To(ConsistOf("patch"))
			Expect(roleVerbs(role, "", "events")).To(BeEmpty())
			Expect(roleVerbs(role, fencingGroup, "fencingslaprofiles")).To(ConsistOf("get"))
			Expect(role.Field(`rules.#(resources.#(=="fencingslaprofiles")).resourceNames`).String()).To(MatchJSON(`["critical","normal","slow"]`))
			Expect(roleVerbs(role, fencingGroup, "fencingfailednodestates")).To(ConsistOf("get", "list", "watch", "create", "delete"))
			Expect(roleVerbs(role, fencingGroup, "fencingfailednodestates/status")).To(ConsistOf("patch"))
		})

		// The agent writes the events of its Node into the namespace default,
		// watches the agent pods, and watches its ConfigMap and the EndpointSlice
		// kubernetes by name.
		It("gives the rights of the namespaces", func() {
			own := f.KubernetesResource("Role", fencingNamespace, "fencing-agent")
			Expect(roleVerbs(own, "", "pods")).To(ConsistOf("list", "watch"))
			Expect(roleVerbs(own, "", "configmaps")).To(ConsistOf("list", "watch"))
			Expect(own.Field(`rules.#(resources.#(=="configmaps")).resourceNames`).String()).To(MatchJSON(`["fencing-agent-config"]`))

			system := f.KubernetesResource("Role", "default", "d8:node-manager:fencing-agent")
			Expect(roleVerbs(system, "discovery.k8s.io", "endpointslices")).To(ConsistOf("list", "watch"))
			Expect(system.Field(`rules.#(resources.#(=="endpointslices")).resourceNames`).String()).To(MatchJSON(`["kubernetes"]`))
			Expect(roleVerbs(system, "", "events")).To(ConsistOf("create", "patch"))
		})

		It("gives no right on NodeGroups and Secrets", func() {
			for _, role := range []object_store.KubeObject{
				clusterRole("d8:node-manager:fencing-agent"),
				f.KubernetesResource("Role", fencingNamespace, "fencing-agent"),
				f.KubernetesResource("Role", "default", "d8:node-manager:fencing-agent"),
			} {
				Expect(role.Exists()).To(BeTrue())
				Expect(roleVerbs(role, "deckhouse.io", "nodegroups")).To(BeEmpty())
				Expect(roleVerbs(role, "", "secrets")).To(BeEmpty())
			}
		})

		It("binds the rights to the ServiceAccount of the agent", func() {
			bindings := []object_store.KubeObject{
				f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:fencing-agent"),
				f.KubernetesResource("RoleBinding", fencingNamespace, "fencing-agent"),
				f.KubernetesResource("RoleBinding", "default", "d8:node-manager:fencing-agent"),
			}
			for _, binding := range bindings {
				Expect(binding.Exists()).To(BeTrue())
				Expect(binding.Field("subjects.0.name").String()).To(Equal("fencing-agent"))
				Expect(binding.Field("subjects.0.namespace").String()).To(Equal(fencingNamespace))
			}
		})
	})

	Context("the NodeGroupConfiguration of the watchdog", func() {
		It("carries the labels of the module", func() {
			render(`nodeGroups: {worker: {mode: Slow}}`)
			ngc := f.KubernetesGlobalResource("NodeGroupConfiguration", "enable-watchdog-for-nodegroup-worker.sh")
			Expect(ngc.Exists()).To(BeTrue())
			Expect(ngc.Field("metadata.labels.app").String()).To(Equal("fencing-agent"))
			Expect(ngc.Field("metadata.labels.module").String()).To(Equal("node-manager"))
			Expect(ngc.Field("metadata.labels.heritage").String()).To(Equal("deckhouse"))
		})
	})

	Context("the NodeGroupConfiguration of the socket directory", func() {
		It("gives the directory of the local API socket to the agent user on every node", func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Slow}}`)
			ngc := f.KubernetesGlobalResource("NodeGroupConfiguration", "fencing-agent-socket-directory.sh")
			Expect(ngc.Exists()).To(BeTrue())
			Expect(ngc.Field("spec.nodeGroups").String()).To(MatchJSON(`["*"]`))
			Expect(ngc.Field("metadata.labels.app").String()).To(Equal("fencing-agent"))
			Expect(ngc.Field("spec.content").String()).To(ContainSubstring("d /run/fencing-agent 0700 64535 root -"))
			Expect(ngc.Field("spec.content").String()).To(ContainSubstring("systemd-tmpfiles --create /etc/tmpfiles.d/fencing-agent.conf"))
		})
	})

	Context("the RBAC of the metrics proxy of the agent", func() {
		It("binds the ServiceAccount of the agent to the rights of kube-rbac-proxy", func() {
			render(`nodeGroups: {worker: {mode: Slow}}`)
			binding := f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:fencing-agent:rbac-proxy")
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("roleRef.kind").String()).To(Equal("ClusterRole"))
			Expect(binding.Field("roleRef.name").String()).To(Equal("d8:rbac-proxy"))
			Expect(binding.Field("subjects.0.kind").String()).To(Equal("ServiceAccount"))
			Expect(binding.Field("subjects.0.name").String()).To(Equal("fencing-agent"))
			Expect(binding.Field("subjects.0.namespace").String()).To(Equal(fencingNamespace))
		})
	})

	Context("the admission policy of the own Node", func() {
		BeforeEach(func() {
			render(`nodeGroups: {worker: {mode: Slow}}`)
		})

		It("denies the agent every Node but the Node of its pod, and a token without the claim", func() {
			policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", fencingVAPName)
			Expect(policy.Exists()).To(BeTrue())
			Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(policy.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(MatchJSON(`["nodes", "nodes/status"]`))

			expression := policy.Field("spec.validations.0.expression").String()
			Expect(expression).To(ContainSubstring("'authentication.kubernetes.io/node-name' in request.userInfo.extra"))
			Expect(expression).To(ContainSubstring("n == request.name"))
		})

		It("limits only the agent", func() {
			policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", fencingVAPName)
			Expect(policy.Field("spec.matchConditions").Array()).To(HaveLen(1))
			Expect(policy.Field("spec.matchConditions.0.expression").String()).To(Equal(`request.userInfo.username == "system:serviceaccount:d8-cloud-instance-manager:fencing-agent"`))
		})

		It("is bound to deny", func() {
			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", fencingVAPName)
			Expect(binding.Field("spec.policyName").String()).To(Equal(fencingVAPName))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))
		})

		const (
			legacyEnabled = "node-manager.deckhouse.io/fencing-enabled"
			legacyMode    = "node-manager.deckhouse.io/fencing-mode"
		)
		deckhouse := &user.DefaultInfo{Name: "system:serviceaccount:d8-system:deckhouse"}

		DescribeTable("admits a change of a Node",
			func(who user.Info, operation admission.Operation, subresource string, oldNode, node map[string]any, want bool) {
				policy := compileNodePolicy(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", fencingVAPName).ToYaml())
				Expect(policy.admits(who, operation, subresource, oldNode, node)).To(Equal(want))
			},
			Entry("the agent sets an annotation of its Node", agentOf("node-a"), admission.Update, "", testNode("node-a"), testNode("node-a", annotate), true),
			Entry("the agent sets the Fencing condition of its Node", agentOf("node-a"), admission.Update, "status", testNode("node-a"), testNode("node-a", setFencingCondition), true),
			Entry("the agent removes a label of fencing 1.0", agentOf("node-a"), admission.Update, "", testNode("node-a"), testNode("node-a", dropLabel(legacyEnabled)), true),
			Entry("the agent removes both labels of fencing 1.0", agentOf("node-a"), admission.Update, "", testNode("node-a"), testNode("node-a", dropLabel(legacyEnabled), dropLabel(legacyMode)), true),
			Entry("the agent removes the last labels, they are of fencing 1.0", agentOf("node-a"), admission.Update, "",
				testNode("node-a", dropLabel("kubernetes.io/hostname")), testNode("node-a", dropLabels), true),
			Entry("the agent sets an annotation of its Node without labels", agentOf("node-a"), admission.Update, "",
				testNode("node-a", dropLabels), testNode("node-a", dropLabels, annotate), true),
			Entry("the agent changes another Node", agentOf("node-a"), admission.Update, "", testNode("node-b"), testNode("node-b", annotate), false),
			Entry("the agent sets the label of a master", agentOf("node-a"), admission.Update, "",
				testNode("node-a"), testNode("node-a", setLabel("node-role.kubernetes.io/control-plane", "")), false),
			Entry("the agent changes a label", agentOf("node-a"), admission.Update, "", testNode("node-a"), testNode("node-a", setLabel(legacyMode, "Watchdog")), false),
			Entry("the agent removes another label", agentOf("node-a"), admission.Update, "", testNode("node-a"), testNode("node-a", dropLabel("kubernetes.io/hostname")), false),
			Entry("the agent sets a label of a Node without labels", agentOf("node-a"), admission.Update, "",
				testNode("node-a", dropLabels), testNode("node-a", dropLabels, func(n map[string]any) { nodeMetadata(n)["labels"] = map[string]any{"a": "b"} }), false),
			Entry("the agent changes the spec", agentOf("node-a"), admission.Update, "", testNode("node-a"), testNode("node-a", cordon), false),
			Entry("the agent changes a label with the status", agentOf("node-a"), admission.Update, "status",
				testNode("node-a"), testNode("node-a", setFencingCondition, setLabel("node-role.kubernetes.io/control-plane", "")), false),
			Entry("the agent creates its Node", agentOf("node-a"), admission.Create, "", nil, testNode("node-a"), false),
			Entry("the agent deletes its Node", agentOf("node-a"), admission.Delete, "", testNode("node-a"), nil, false),
			Entry("Deckhouse sets the label of a master", deckhouse, admission.Update, "",
				testNode("node-a"), testNode("node-a", setLabel("node-role.kubernetes.io/control-plane", "")), true),
		)
	})

	Context("the admission policy of the failed node states", func() {
		BeforeEach(func() {
			render(`nodeGroups: {worker: {mode: Slow}}`)
		})

		It("checks the states and their status and is bound to deny", func() {
			policy := f.KubernetesGlobalResource("ValidatingAdmissionPolicy", fencingStatesVAPName)
			Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))
			Expect(policy.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(MatchJSON(`["fencingfailednodestates", "fencingfailednodestates/status"]`))

			binding := f.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", fencingStatesVAPName)
			Expect(binding.Field("spec.policyName").String()).To(Equal(fencingStatesVAPName))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))
		})

		const (
			t1 = "2026-10-29T10:00:01.000000Z"
			t2 = "2026-10-29T10:00:02.000000Z"
		)
		noClaim := &user.DefaultInfo{Name: "system:serviceaccount:d8-cloud-instance-manager:fencing-agent"}
		controller := &user.DefaultInfo{Name: "system:serviceaccount:d8-cloud-instance-manager:fencing-controller"}
		admin := &user.DefaultInfo{Name: "admin@example.com"}

		DescribeTable("admits a change of a FencingFailedNodeState",
			func(who user.Info, operation admission.Operation, subresource string, oldState, state map[string]any, want bool) {
				policy := compileStatesPolicy(f.KubernetesGlobalResource("ValidatingAdmissionPolicy", fencingStatesVAPName).ToYaml())
				Expect(policy.admits(who, operation, subresource, oldState, state)).To(Equal(want))
			},
			Entry("a writer creates a state", agentOf("node-w"), admission.Create, "", nil, testState("node-v"), true),
			Entry("a writer deletes a state", agentOf("node-w"), admission.Delete, "", testState("node-v"), nil, true),
			Entry("a token without the claim creates a state", noClaim, admission.Create, "", nil, testState("node-v"), false),
			Entry("a token without the claim deletes a state", noClaim, admission.Delete, "", testState("node-v"), nil, false),

			Entry("the failed node writes its heartbeat", agentOf("node-v"), admission.Update, "status",
				testState("node-v", heartbeat(t1)), testState("node-v", heartbeat(t2)), true),
			Entry("the failed node writes its first heartbeat next to the attestation of another node", agentOf("node-v"), admission.Update, "status",
				testState("node-v", attestation("node-w", t1)), testState("node-v", attestation("node-w", t1), heartbeat(t2)), true),
			Entry("another node writes the heartbeat", agentOf("node-x"), admission.Update, "status",
				testState("node-v", heartbeat(t1)), testState("node-v", heartbeat(t2)), false),
			Entry("another node removes the heartbeat", agentOf("node-x"), admission.Update, "status",
				testState("node-v", heartbeat(t1)), testState("node-v"), false),

			Entry("the witness writes the attestation with its own name", agentOf("node-w"), admission.Update, "status",
				testState("node-v"), testState("node-v", attestation("node-w", t1)), true),
			Entry("the witness writes the attestation next to the heartbeat", agentOf("node-w"), admission.Update, "status",
				testState("node-v", heartbeat(t1)), testState("node-v", heartbeat(t1), attestation("node-w", t2)), true),
			Entry("a node writes the attestation with the name of another node", agentOf("node-x"), admission.Update, "status",
				testState("node-v"), testState("node-v", attestation("node-w", t1)), false),
			Entry("a node changes the time of the attestation of another node", agentOf("node-x"), admission.Update, "status",
				testState("node-v", attestation("node-w", t1)), testState("node-v", attestation("node-w", t2)), false),
			Entry("the failed node writes an attestation", agentOf("node-v"), admission.Update, "status",
				testState("node-v"), testState("node-v", attestation("node-w", t1)), false),

			Entry("a writer writes the contact with its own name", agentOf("node-w"), admission.Update, "status",
				testState("node-v"), testState("node-v", contact("node-w", t1)), true),
			Entry("a node writes the contact with the name of another node", agentOf("node-x"), admission.Update, "status",
				testState("node-v"), testState("node-v", contact("node-w", t1)), false),

			Entry("a writer creates a state with the approval", agentOf("node-w"), admission.Create, "", nil, testState("node-v", approve("true")), false),
			Entry("the agent adds the approval", agentOf("node-v"), admission.Update, "", testState("node-v"), testState("node-v", approve("true")), false),
			Entry("the agent changes the approval", agentOf("node-v"), admission.Update, "",
				testState("node-v", approve("true")), testState("node-v", approve("false")), false),
			Entry("the agent removes the approval", agentOf("node-v"), admission.Update, "", testState("node-v", approve("true")), testState("node-v"), false),
			Entry("the agent keeps the approval", agentOf("node-v"), admission.Update, "status",
				testState("node-v", approve("true")), testState("node-v", approve("true"), heartbeat(t1)), true),

			Entry("the agent sets the phase", agentOf("node-x"), admission.Update, "status",
				testState("node-v"), testState("node-v", setPhase("Proven")), false),
			Entry("the failed node changes its phase", agentOf("node-v"), admission.Update, "status",
				testState("node-v", setPhase("FallbackAlive")), testState("node-v", setPhase("Suspected")), false),
			Entry("the agent changes the conditions", agentOf("node-x"), admission.Update, "status",
				testState("node-v", setConditions("ConcurrencyLimit")), testState("node-v", setConditions("MassFailure")), false),
			Entry("the agent sets the episode", agentOf("node-x"), admission.Update, "status",
				testState("node-v"), testState("node-v", setEpisode("fe-1")), false),
			Entry("the agent sets the episode label", agentOf("node-x"), admission.Update, "",
				testState("node-v"), testState("node-v", episodeLabel("fe-1")), false),
			Entry("a writer creates a state with the episode label", agentOf("node-w"), admission.Create, "", nil, testState("node-v", episodeLabel("fe-1")), false),
			Entry("the failed node writes its heartbeat next to the parts of the controller", agentOf("node-v"), admission.Update, "status",
				testState("node-v", episodeLabel("fe-1"), setPhase("FallbackAlive"), setConditions("MassFailure"), setEpisode("fe-1"), heartbeat(t1)),
				testState("node-v", episodeLabel("fe-1"), setPhase("FallbackAlive"), setConditions("MassFailure"), setEpisode("fe-1"), heartbeat(t2)), true),

			Entry("the controller writes the phase and the heartbeat", controller, admission.Update, "status",
				testState("node-v", heartbeat(t1)), testState("node-v", heartbeat(t2), setPhase("Proven")), true),
			Entry("the controller writes the conditions, the episode and its label", controller, admission.Update, "",
				testState("node-v"), testState("node-v", setConditions("MassFailure"), setEpisode("fe-1"), episodeLabel("fe-1")), true),
			Entry("a user adds the approval", admin, admission.Update, "", testState("node-v"), testState("node-v", approve("true")), true),
		)
	})

	Context("the controller", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled}, worker: {mode: Slow}}`)
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "control-plane-manager"]`)
			f.HelmRender(WithAPIVersions("autoscaling.k8s.io/v1/VerticalPodAutoscaler", "monitoring.coreos.com/v1/PodMonitor"))
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("deploys the controller when one node group has fencing", func() {
			Expect(f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", fencingNamespace, "fencing-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("PodDisruptionBudget", fencingNamespace, "fencing-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("ServiceAccount", fencingNamespace, "fencing-controller").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("Role", fencingNamespace, "access-to-fencing-controller-prometheus-metrics").Exists()).To(BeTrue())
			Expect(f.KubernetesResource("PodMonitor", "d8-monitoring", "fencing-controller").Exists()).To(BeTrue())
		})

		It("gives the rights of the cluster, and no right to create a state", func() {
			role := clusterRole("d8:node-manager:fencing-controller")
			Expect(roleVerbs(role, fencingGroup, "fencingfailednodestates")).To(ConsistOf("get", "list", "watch", "update", "patch", "delete"))
			Expect(roleVerbs(role, fencingGroup, "fencingfailednodestates/status")).To(ConsistOf("get", "update", "patch"))
			Expect(roleVerbs(role, fencingGroup, "fencingslaprofiles")).To(ConsistOf("get", "list", "watch"))
			Expect(roleVerbs(role, "", "nodes")).To(ConsistOf("get", "list", "watch", "update", "patch", "delete"))
			Expect(roleVerbs(role, "", "pods")).To(ConsistOf("get", "list", "watch", "delete"))
			Expect(roleVerbs(role, "", "pods/status")).To(ConsistOf("patch"))
			Expect(roleVerbs(role, "storage.k8s.io", "volumeattachments")).To(ConsistOf("list"))
			Expect(roleVerbs(role, "deckhouse.io", "instances")).To(ConsistOf("get", "list", "watch"))
			Expect(roleVerbs(role, "", "events")).To(ConsistOf("create", "patch"))
			Expect(roleVerbs(role, "", "configmaps")).To(BeEmpty())
			Expect(roleVerbs(role, "coordination.k8s.io", "leases")).To(BeEmpty())
		})

		It("gives the rights of the namespace of the module, and no right on ConfigMaps", func() {
			own := f.KubernetesResource("Role", fencingNamespace, "fencing-controller")
			Expect(roleVerbs(own, "", "configmaps")).To(BeEmpty())
			Expect(roleVerbs(own, "machine.sapcloud.io", "machines")).To(ConsistOf("get", "delete"))
			Expect(roleVerbs(own, "cluster.x-k8s.io", "machinehealthchecks")).To(ConsistOf("get", "list"))

			election := f.KubernetesResource("Role", fencingNamespace, "fencing-controller-leader-election")
			Expect(roleVerbs(election, "coordination.k8s.io", "leases")).To(ContainElements("get", "create", "update"))
		})

		It("elects a leader also with one replica", func() {
			f.ValuesSet("global.highAvailability", false)
			f.HelmRender(WithAPIVersions("autoscaling.k8s.io/v1/VerticalPodAutoscaler", "monitoring.coreos.com/v1/PodMonitor"))
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			deployment := f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller")
			Expect(deployment.Field("spec.replicas").Int()).To(Equal(int64(1)))
			Expect(deployment.Field(`spec.template.spec.containers.#(name=="fencing-controller").args`).String()).To(ContainSubstring(`"--leader-elect=true"`))
		})

		It("binds the rights and the rights of kube-rbac-proxy to the ServiceAccount of the controller", func() {
			bindings := []object_store.KubeObject{
				f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:fencing-controller"),
				f.KubernetesGlobalResource("ClusterRoleBinding", "d8:node-manager:fencing-controller:rbac-proxy"),
				f.KubernetesResource("RoleBinding", fencingNamespace, "fencing-controller"),
				f.KubernetesResource("RoleBinding", fencingNamespace, "fencing-controller-leader-election"),
			}
			for _, binding := range bindings {
				Expect(binding.Exists()).To(BeTrue())
				Expect(binding.Field("subjects.0.name").String()).To(Equal("fencing-controller"))
				Expect(binding.Field("subjects.0.namespace").String()).To(Equal(fencingNamespace))
			}
			Expect(bindings[1].Field("roleRef.name").String()).To(Equal("d8:rbac-proxy"))
		})

		It("connects to the API server through the local kubernetes-api-proxy on the host network", func() {
			pod := f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller").Field("spec.template")
			Expect(pod.Get("spec.hostNetwork").Bool()).To(BeTrue())
			Expect(pod.Get("spec.dnsPolicy").String()).To(Equal("ClusterFirstWithHostNet"))
			container := pod.Get(`spec.containers.#(name=="fencing-controller")`)
			Expect(container.Get(`env.#(name=="KUBERNETES_SERVICE_HOST").value`).String()).To(Equal("127.0.0.1"))
			Expect(container.Get(`env.#(name=="KUBERNETES_SERVICE_PORT").value`).String()).To(Equal("6445"))
		})

		It("connects to the ClusterIP of the API server when control-plane-manager is disabled", func() {
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler"]`)
			f.HelmRender(WithAPIVersions("autoscaling.k8s.io/v1/VerticalPodAutoscaler", "monitoring.coreos.com/v1/PodMonitor"))
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			controller := f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller")
			Expect(controller.Exists()).To(BeTrue())
			Expect(envNames(controller)).NotTo(ContainElement("KUBERNETES_SERVICE_HOST"))
			Expect(envNames(controller)).NotTo(ContainElement("KUBERNETES_SERVICE_PORT"))
		})

		It("serves the probes on the loopback address and passes only the metrics through kube-rbac-proxy", func() {
			pod := f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller").Field("spec.template.spec")
			controller := pod.Get(`containers.#(name=="fencing-controller")`)
			var address string
			for _, arg := range controller.Get("args").Array() {
				if v, ok := strings.CutPrefix(arg.String(), "--metrics-bind-address="); ok {
					address = v
				}
			}
			host, port, found := strings.Cut(address, ":")
			Expect(found).To(BeTrue(), address)
			Expect(host).To(Equal("127.0.0.1"))
			for _, probe := range []string{"livenessProbe", "readinessProbe"} {
				Expect(controller.Get(probe+".httpGet.host").String()).To(Equal("127.0.0.1"), probe)
				Expect(controller.Get(probe+".httpGet.port").String()).To(Equal(port), probe)
			}

			proxy := pod.Get(`containers.#(name=="kube-rbac-proxy")`)
			Expect(proxy.Get(`env.#(name=="KUBE_RBAC_PROXY_CONFIG").value`).String()).To(MatchYAML(`
upstreams:
- upstream: http://` + address + `/metrics
  path: /metrics
  authorization:
    resourceAttributes:
      namespace: d8-cloud-instance-manager
      apiGroup: apps
      apiVersion: v1
      resource: deployments
      subresource: prometheus-metrics
      name: fencing-controller
`))
		})

		It("does not use the ports of the other pods of the module on the host network of the masters", func() {
			f.ValuesSet("nodeManager.internal.capiControllerManagerEnabled", true)
			f.ValuesSet("nodeManager.internal.machineControllerManagerEnabled", true)
			f.ValuesSetFromYaml("nodeManager.internal.cloudProvider", `{type: openstack}`)
			f.HelmRender(WithAPIVersions("autoscaling.k8s.io/v1/VerticalPodAutoscaler", "monitoring.coreos.com/v1/PodMonitor"))
			Expect(f.RenderError).ShouldNot(HaveOccurred())

			// node-controller uses the host network before the bootstrap of the cluster is complete.
			others := []object_store.KubeObject{
				f.KubernetesResource("Deployment", fencingNamespace, "node-controller"),
				f.KubernetesResource("Deployment", fencingNamespace, "capi-controller-manager"),
				f.KubernetesResource("Deployment", fencingNamespace, "machine-controller-manager"),
				f.KubernetesResource("Deployment", fencingNamespace, "bashible-apiserver"),
				agent("master"),
			}
			var otherPorts []string
			for _, workload := range others {
				Expect(workload.Exists()).To(BeTrue(), workload.Field("metadata.name").String())
				ports := occupiedPorts(workload)
				Expect(ports).NotTo(BeEmpty(), workload.Field("metadata.name").String())
				otherPorts = append(otherPorts, ports...)
			}

			controllerPorts := occupiedPorts(f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller"))
			Expect(controllerPorts).NotTo(BeEmpty())
			for _, port := range controllerPorts {
				Expect(otherPorts).NotTo(ContainElement(port))
			}
		})

		It("registers the ports that the controller occupies on the node", func() {
			ports, err := os.ReadFile("../../../docs/documentation/_data/deckhouse-ports.yml")
			Expect(err).NotTo(HaveOccurred())
			for _, port := range occupiedPorts(f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller")) {
				Expect(string(ports)).To(ContainSubstring(`ports: "`+port+`"`), port)
			}
		})

		It("allows the host network and the container ports of the controller by its SecurityPolicyException", func() {
			pod := f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller").Field("spec.template")
			name := pod.Get(`metadata.labels.security\.deckhouse\.io/security-policy-exception`).String()
			spe := f.KubernetesResource("SecurityPolicyException", fencingNamespace, name)
			Expect(spe.Exists()).To(BeTrue())
			Expect(spe.Field("spec.network.hostNetwork.allowedValue").Bool()).To(BeTrue())
			allowed := spe.Field("spec.network.hostPorts.#.port").Array()
			var allowedPorts []int64
			for _, port := range allowed {
				allowedPorts = append(allowedPorts, port.Int())
			}
			for _, port := range pod.Get("spec.containers.#.ports.#.containerPort|@flatten").Array() {
				Expect(allowedPorts).To(ContainElement(port.Int()))
			}
		})

		// Kubernetes evicts the controller from a failed master only when no toleration holds the pod there without a time limit.
		// The scheduler can put the pod on a master that is not ready, but not on a master that does not respond.
		It("leaves a failed master and is not scheduled on it again", func() {
			tolerations := f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller").Field("spec.template.spec.tolerations").Array()
			Expect(tolerations).NotTo(BeEmpty())
			for key, wantNoSchedule := range map[string]bool{"node.kubernetes.io/unreachable": false, "node.kubernetes.io/not-ready": true} {
				var noSchedule bool
				for _, t := range tolerations {
					matchesKey := t.Get("key").String() == key || (t.Get("key").String() == "" && t.Get("operator").String() == "Exists")
					if !matchesKey {
						continue
					}
					effect := t.Get("effect").String()
					if effect == "" || effect == "NoExecute" {
						Expect(t.Get("tolerationSeconds").Exists()).To(BeTrue(), "%s: %s", key, t.Raw)
					}
					if effect == "" || effect == "NoSchedule" {
						noSchedule = true
					}
				}
				Expect(noSchedule).To(Equal(wantNoSchedule), key)
			}
			var keys []string
			for _, t := range tolerations {
				keys = append(keys, t.Get("key").String())
			}
			Expect(keys).To(ContainElements("node-role.kubernetes.io/control-plane", "node.deckhouse.io/uninitialized"))
		})
	})

	Context("the masters on Deckhouse Engine", func() {
		BeforeEach(func() {
			render(`nodeGroups: {master: {mode: Disabled, engine: true}, worker: {mode: Slow}}`)
			f.ValuesSetFromYaml("global.enabledModules", `["vertical-pod-autoscaler", "control-plane-manager"]`)
			f.HelmRender(WithAPIVersions("autoscaling.k8s.io/v1/VerticalPodAutoscaler", "monitoring.coreos.com/v1/PodMonitor"))
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("connects the controller to the ClusterIP of the API server, not to the local kubernetes-api-proxy", func() {
			controller := f.KubernetesResource("Deployment", fencingNamespace, "fencing-controller")
			Expect(controller.Exists()).To(BeTrue())
			Expect(envNames(controller)).NotTo(ContainElement("KUBERNETES_SERVICE_HOST"))
			Expect(envNames(controller)).NotTo(ContainElement("KUBERNETES_SERVICE_PORT"))
		})
	})

	Context("the RBAC of the users", func() {
		BeforeEach(func() {
			render(`nodeGroups: {worker: {mode: Disabled}}`)
		})

		It("lets the level User and the capability view read the states and the profiles, and nothing more", func() {
			for _, name := range []string{"d8:user-authz:node-manager:user", "d8:system-capability:node-manager:view"} {
				role := clusterRole(name)
				Expect(role.Exists()).To(BeTrue(), name)
				Expect(roleVerbs(role, fencingGroup, "fencingfailednodestates")).To(ConsistOf("get", "list", "watch"), name)
				Expect(roleVerbs(role, fencingGroup, "fencingslaprofiles")).To(ConsistOf("get", "list", "watch"), name)
			}
		})

		It("lets the level ClusterAdmin and the capability edit change the states and not the profiles", func() {
			for _, name := range []string{"d8:user-authz:node-manager:cluster-admin", "d8:system-capability:node-manager:edit"} {
				role := clusterRole(name)
				Expect(role.Exists()).To(BeTrue(), name)
				Expect(roleVerbs(role, fencingGroup, "fencingfailednodestates")).To(ConsistOf("update", "patch"), name)
				Expect(roleVerbs(role, fencingGroup, "fencingslaprofiles")).To(BeEmpty(), name)
			}
		})

		It("gives the other levels of user-authz no right on the states and the profiles", func() {
			for _, level := range []string{"privileged-user", "editor", "admin", "cluster-editor"} {
				role := clusterRole("d8:user-authz:node-manager:" + level)
				Expect(roleVerbs(role, fencingGroup, "fencingfailednodestates")).To(BeEmpty(), level)
				Expect(roleVerbs(role, fencingGroup, "fencingslaprofiles")).To(BeEmpty(), level)
			}
		})
	})
})
