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
	"sort"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/api/resource"

	. "github.com/deckhouse/deckhouse/testing/helm"
	"github.com/deckhouse/deckhouse/testing/library/object_store"
)

// storagePodContainer returns the container of a rendered pod controller by name. Named for this file
// rather than generically: the package is shared by every registry template test, and a second
// helper of the same name in another file merges without a conflict and fails to compile.
func storagePodContainer(workload object_store.KubeObject, name string) gjson.Result {
	for _, c := range workload.Field("spec.template.spec.containers").Array() {
		if c.Get("name").String() == name {
			return c
		}
	}
	return gjson.Result{}
}

// quantity parses a resource quantity a template rendered, failing the test on one it cannot read.
func quantity(value gjson.Result) resource.Quantity {
	q, err := resource.ParseQuantity(value.String())
	ExpectWithOffset(1, err).ShouldNot(HaveOccurred(), "not a quantity: %q", value.String())
	return q
}

// A registry that hangs with its port open used to keep its place in every node's layout: the
// controller builds the agents' addresses from the replicas' reports, not from pod readiness, and
// nothing restarted the process. The registry container is therefore probed for liveness — on a
// path that tells a listener that answers from one that does not and nothing more, behind a startup
// probe long enough for what the registry legitimately does before it opens the port. A handler
// hung behind a listener that still answers is not caught: see the comment on the probe.
var _ = Describe("Module :: registry :: helm template :: v2 storage liveness", func() {
	f := SetupHelmConfig(``)

	BeforeEach(func() {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	})

	It("restarts a registry that stops answering on its serving port", func() {
		distribution := storagePodContainer(f.KubernetesResource("StatefulSet", "d8-system", "registry-storage"), "distribution")
		Expect(distribution.Exists()).To(BeTrue())

		for _, kind := range []string{"livenessProbe", "startupProbe"} {
			probe := distribution.Get(kind + ".httpGet")
			Expect(probe.Exists()).To(BeTrue(), "%s must be an HTTP probe", kind)

			// The path upstream answers ahead of its own handlers: no credentials, no store.
			Expect(probe.Get("path").String()).To(Equal("/"), kind)
			Expect(probe.Get("scheme").String()).To(Equal("HTTPS"), kind)
			// By name, so the probe follows the serving port wherever it is numbered.
			Expect(probe.Get("port").String()).To(Equal("registry"), kind)
			// No host: the default is the node address, where the registry listens. The loopback
			// port of the same number is the node's agent, which answers for a registry that is down.
			Expect(probe.Get("host").Exists()).To(BeFalse(), kind)
		}

		// And the name is the serving listener: the port the Service and the readiness probe point at.
		var serving gjson.Result
		for _, port := range distribution.Get("ports").Array() {
			if port.Get("name").String() == "registry" {
				serving = port
			}
		}
		Expect(serving.Exists()).To(BeTrue())
		Expect(serving.Get("containerPort").String()).
			To(Equal(distribution.Get("readinessProbe.tcpSocket.port").String()))
	})

	It("gives the registry time to wait for its configuration and measure its store", func() {
		distribution := storagePodContainer(f.KubernetesResource("StatefulSet", "d8-system", "registry-storage"), "distribution")

		window := func(probe gjson.Result) int64 {
			return probe.Get("periodSeconds").Int() * probe.Get("failureThreshold").Int()
		}

		// Both happen on every start of the container, and a large store takes minutes to walk.
		Expect(window(distribution.Get("startupProbe"))).To(BeNumerically(">=", 1800))
		// About the agent's response timeout: a registry whose listener stopped answering is
		// restarted at the pace the nodes notice it, not sooner.
		Expect(window(distribution.Get("livenessProbe"))).To(BeNumerically("~", 60, 30))
		Expect(distribution.Get("livenessProbe.timeoutSeconds").Int()).To(BeNumerically(">", 1))
	})

	It("probes neither the debug listener nor the write endpoint", func() {
		distribution := storagePodContainer(f.KubernetesResource("StatefulSet", "d8-system", "registry-storage"), "distribution")

		var writePort string
		for _, port := range distribution.Get("ports").Array() {
			if port.Get("name").String() == "registry-push" {
				writePort = port.Get("containerPort").String()
			}
		}
		Expect(writePort).NotTo(BeEmpty())

		for _, kind := range []string{"livenessProbe", "readinessProbe", "startupProbe"} {
			probe := distribution.Get(kind)
			Expect(probe.Exists()).To(BeTrue(), kind)
			for _, port := range []string{probe.Get("httpGet.port").String(), probe.Get("tcpSocket.port").String()} {
				// The debug listener may fail to open without stopping the registry, and on a node
				// leaving the previous implementation its port belongs to that implementation's proxy.
				Expect(port).NotTo(Equal("5002"), kind)
				// A registry that cannot take a push still serves every pull.
				Expect(port).NotTo(Equal("registry-push"), kind)
				Expect(port).NotTo(Equal(writePort), kind)
			}
		}
	})
})

// The disruption budget of the cache follows the number of its replicas, which is the number of
// masters. It used to follow the cluster's high-availability flag, a different question, and the two
// part ways whenever `global.highAvailability` is set against the master count.
var _ = Describe("Module :: registry :: helm template :: v2 storage disruption budget", func() {
	f := SetupHelmConfig(``)

	// The flag explicitly, both the override and what discovery says, so that a case of the two
	// disagreeing is that case and not the default.
	renderWith := func(masters int, discoveredHA bool, overrideHA *bool) {
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		if masters > 0 {
			f.ValuesSet("global.discovery.clusterMasterCount", masters)
		}
		f.ValuesSet("global.discovery.clusterControlPlaneIsHighlyAvailable", discoveredHA)
		if overrideHA != nil {
			f.ValuesSet("global.highAvailability", *overrideHA)
		}
		f.ValuesSetFromYaml("registry", v2Enabled)
		f.HelmRender()
		Expect(f.RenderError).ShouldNot(HaveOccurred())
	}

	budget := func() (replicas, maxUnavailable string) {
		set := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
		pdb := f.KubernetesResource("PodDisruptionBudget", "d8-system", "registry-storage")
		Expect(set.Exists()).To(BeTrue())
		Expect(pdb.Exists()).To(BeTrue())
		return set.Field("spec.replicas").String(), pdb.Field("spec.maxUnavailable").String()
	}

	yes, no := true, false

	It("lets one of three replicas go", func() {
		renderWith(3, true, nil)
		replicas, maxUnavailable := budget()
		Expect(replicas).To(Equal("3"))
		Expect(maxUnavailable).To(Equal("1"))
	})

	It("keeps the only replica", func() {
		renderWith(1, false, nil)
		replicas, maxUnavailable := budget()
		Expect(replicas).To(Equal("1"))
		Expect(maxUnavailable).To(Equal("0"))
	})

	It("keeps the only replica before the master count is known", func() {
		renderWith(0, false, nil)
		replicas, maxUnavailable := budget()
		Expect(replicas).To(Equal("1"))
		Expect(maxUnavailable).To(Equal("0"))
	})

	// Three replicas behind a budget of zero: every drain of a master stopped on the cache until it
	// timed out.
	It("lets one of three replicas go with high availability turned off", func() {
		renderWith(3, true, &no)
		replicas, maxUnavailable := budget()
		Expect(replicas).To(Equal("3"))
		Expect(maxUnavailable).To(Equal("1"))
	})

	// The only replica behind a budget of one: in an air-gapped cluster, evicting it leaves the
	// cluster with nothing to pull from.
	It("keeps the only replica with high availability turned on", func() {
		renderWith(1, false, &yes)
		replicas, maxUnavailable := budget()
		Expect(replicas).To(Equal("1"))
		Expect(maxUnavailable).To(Equal("0"))
	})
})

// Every pod controller has an autoscaler, and every container in it a policy.
//
// The module linter checks exactly this for every other module, and does not see these objects: it
// renders the module with its default values, where this implementation is off. A container without
// a policy is still one the autoscaler manages, only with nothing bounding what it recommends — and
// the template leaves its requests to the autoscaler, so until the first recommendation it has none.
var _ = Describe("Module :: registry :: helm template :: v2 vertical pod autoscaling", func() {
	f := SetupHelmConfig(``)

	type workload struct {
		kind, name string
		containers []string
	}

	// What the module renders, file by file, as kind/name → the containers of the pod controllers,
	// and kind/name → the container names of each autoscaler's policies.
	inventory := func(registryValues string) (map[string]workload, map[string][]string) {
		rendered := map[string]string{}
		f.ValuesSetFromYaml("global", globalValues)
		f.ValuesSet("global.modulesImages", GetModulesImages())
		f.ValuesSetFromYaml("registry", registryValues)
		f.HelmRender(WithRenderOutput(rendered))
		Expect(f.RenderError).ShouldNot(HaveOccurred())

		workloads := map[string]workload{}
		policies := map[string][]string{}
		for _, manifests := range rendered {
			for _, doc := range strings.Split(manifests, "\n---") {
				if strings.TrimSpace(doc) == "" {
					continue
				}
				var obj struct {
					Kind     string `yaml:"kind"`
					Metadata struct {
						Name string `yaml:"name"`
					} `yaml:"metadata"`
					Spec struct {
						TargetRef struct {
							Kind string `yaml:"kind"`
							Name string `yaml:"name"`
						} `yaml:"targetRef"`
						ResourcePolicy struct {
							ContainerPolicies []struct {
								ContainerName string `yaml:"containerName"`
							} `yaml:"containerPolicies"`
						} `yaml:"resourcePolicy"`
						Template struct {
							Spec struct {
								Containers []struct {
									Name string `yaml:"name"`
								} `yaml:"containers"`
							} `yaml:"spec"`
						} `yaml:"template"`
					} `yaml:"spec"`
				}
				if err := yaml.Unmarshal([]byte(doc), &obj); err != nil || obj.Kind == "" {
					continue
				}
				switch obj.Kind {
				case "Deployment", "StatefulSet", "DaemonSet":
					w := workload{kind: obj.Kind, name: obj.Metadata.Name}
					for _, c := range obj.Spec.Template.Spec.Containers {
						w.containers = append(w.containers, c.Name)
					}
					workloads[obj.Kind+"/"+obj.Metadata.Name] = w
				case "VerticalPodAutoscaler":
					var names []string
					for _, p := range obj.Spec.ResourcePolicy.ContainerPolicies {
						names = append(names, p.ContainerName)
					}
					policies[obj.Spec.TargetRef.Kind+"/"+obj.Spec.TargetRef.Name] = names
				}
			}
		}
		return workloads, policies
	}

	for name, fixture := range map[string]string{
		"a pass-through cache": v2Enabled,
		"an air-gapped cache":  v2AirGap,
	} {
		name, fixture := name, fixture

		It("gives every pod controller an autoscaler covering all of its containers with "+name, func() {
			workloads, policies := inventory(fixture)

			// Not vacuous: the controller and the storage are both here.
			Expect(workloads).To(HaveKey("Deployment/registry-controller"))
			Expect(workloads).To(HaveKey("StatefulSet/registry-storage"))

			for key, w := range workloads {
				covered, ok := policies[key]
				Expect(ok).To(BeTrue(), "%s has no VerticalPodAutoscaler", key)
				sort.Strings(covered)
				containers := append([]string(nil), w.containers...)
				sort.Strings(containers)
				Expect(covered).To(Equal(containers),
					"the VerticalPodAutoscaler of %s must have a policy for every container and none for a container it does not have", key)
			}
		})
	}

	Context("the storage", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", v2Enabled)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		// Never evicted by the autoscaler: the replicas are replaced by the update controller alone,
		// followers first and the leader last.
		It("applies a recommendation only to a replica being created anyway", func() {
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "registry-storage")
			Expect(vpa.Exists()).To(BeTrue())
			Expect(vpa.Field("spec.targetRef.kind").String()).To(Equal("StatefulSet"))
			Expect(vpa.Field("spec.targetRef.name").String()).To(Equal("registry-storage"))
			Expect(vpa.Field("spec.updatePolicy.updateMode").String()).To(Equal("Initial"))
			Expect(vpa.Field(`metadata.labels.workload-resource-policy\.deckhouse\.io`).String()).To(Equal("master"))
		})

		It("keeps what it may recommend small enough for a four-core master", func() {
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "registry-storage")

			total := resource.MustParse("0")
			for _, policy := range vpa.Field("spec.resourcePolicy.containerPolicies").Array() {
				name := policy.Get("containerName").String()
				for _, kind := range []string{"cpu", "memory"} {
					minimum := quantity(policy.Get("minAllowed." + kind))
					maximum := quantity(policy.Get("maxAllowed." + kind))
					Expect(minimum.Cmp(maximum)).To(BeNumerically("<=", 0), "%s %s: min above max", name, kind)
				}
				total.Add(quantity(policy.Get("maxAllowed.cpu")))
			}
			Expect(total.MilliValue()).To(BeNumerically("<=", 400),
				"the storage could be recommended more CPU than a four-core master holds beside its control plane")
		})

		// The storage starts before the autoscaler during bootstrap, so its requests are written into
		// the pod whether the autoscaler is there or not — and limits never.
		It("keeps the requests in the pod and sets no limits", func() {
			set := f.KubernetesResource("StatefulSet", "d8-system", "registry-storage")
			for _, c := range set.Field("spec.template.spec.containers").Array() {
				name := c.Get("name").String()
				Expect(c.Get("resources.requests.cpu").Exists()).To(BeTrue(), name)
				Expect(c.Get("resources.requests.memory").Exists()).To(BeTrue(), name)
				Expect(c.Get("resources.limits").Exists()).To(BeFalse(), name)
			}
		})
	})

	Context("the controller", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSetFromYaml("registry", v2Enabled)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("bounds the proxy the way every other module's is bounded", func() {
			vpa := f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "registry-controller")
			Expect(vpa.Exists()).To(BeTrue())
			Expect(vpa.Field(`metadata.labels.workload-resource-policy\.deckhouse\.io`).String()).To(Equal("master"))

			var proxy gjson.Result
			for _, policy := range vpa.Field("spec.resourcePolicy.containerPolicies").Array() {
				if policy.Get("containerName").String() == "kube-rbac-proxy" {
					proxy = policy
				}
			}
			Expect(proxy.Exists()).To(BeTrue())
			Expect(proxy.Get("minAllowed.cpu").String()).To(Equal("10m"))
			Expect(proxy.Get("minAllowed.memory").String()).To(Equal("25Mi"))
			Expect(proxy.Get("maxAllowed.cpu").String()).To(Equal("25m"))
			Expect(proxy.Get("maxAllowed.memory").String()).To(Equal("64Mi"))
		})
	})

	Context("without the autoscaler", func() {
		BeforeEach(func() {
			f.ValuesSetFromYaml("global", globalValues)
			f.ValuesSet("global.modulesImages", GetModulesImages())
			f.ValuesSet("global.enabledModules", []string{"registry", "cert-manager"})
			f.ValuesSetFromYaml("registry", v2Enabled)
			f.HelmRender()
			Expect(f.RenderError).ShouldNot(HaveOccurred())
		})

		It("renders no autoscaler and gives every container its requests", func() {
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "registry-storage").Exists()).To(BeFalse())
			Expect(f.KubernetesResource("VerticalPodAutoscaler", "d8-system", "registry-controller").Exists()).To(BeFalse())

			for _, w := range []object_store.KubeObject{
				f.KubernetesResource("StatefulSet", "d8-system", "registry-storage"),
				f.KubernetesResource("Deployment", "d8-system", "registry-controller"),
			} {
				Expect(w.Exists()).To(BeTrue())
				for _, c := range w.Field("spec.template.spec.containers").Array() {
					name := c.Get("name").String()
					Expect(c.Get("resources.requests.cpu").Exists()).To(BeTrue(), name)
					Expect(c.Get("resources.requests.memory").Exists()).To(BeTrue(), name)
				}
			}

			proxy := storagePodContainer(f.KubernetesResource("Deployment", "d8-system", "registry-controller"), "kube-rbac-proxy")
			Expect(proxy.Get("resources.requests.cpu").String()).To(Equal("10m"))
			Expect(proxy.Get("resources.requests.memory").String()).To(Equal("25Mi"))
		})
	})
})
