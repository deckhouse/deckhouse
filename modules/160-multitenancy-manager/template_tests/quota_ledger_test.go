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
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	quotaLedgerWebhook   = "d8-multitenancy-manager-quota-ledger"
	quotaLedgerRoute     = "d8-multitenancy-manager-quota-ledger-route"
	quotaLedgerService   = "multitenancy-manager-quota-ledger"
	quotaLedgerFlow      = "d8-multitenancy-manager-quota-ledger"
	quotaLedgerSyncFlow  = "d8-multitenancy-manager-quota-ledger-sync"
	quotaLedgerNamespace = "d8-multitenancy-manager"
)

var _ = Describe("Module :: multitenancy-manager :: helm template :: quota ledger ::", func() {
	hec := SetupHelmConfig("")

	It("calls the ledger on the load balancers, and on the virtual machines only where the virtualization module is enabled", func() {
		renderQuota(hec, true)
		Expect(hec.RenderError).ShouldNot(HaveOccurred())
		webhooks := hec.KubernetesGlobalResource("ValidatingWebhookConfiguration", quotaLedgerWebhook).Field("webhooks").Array()
		Expect(webhooks).To(HaveLen(2))
		lb := webhooks[1]
		Expect(lb.Get("name").String()).To(Equal("quota-lb.multitenancy-manager.deckhouse.io"))
		Expect(lb.Get("failurePolicy").String()).To(Equal("Fail"))
		Expect(lb.Get("rules.0.resources").String()).To(MatchJSON(`["services"]`))
		Expect(lb.Get("rules.0.operations").String()).To(MatchJSON(`["CREATE","UPDATE"]`))
		Expect(lb.Get("namespaceSelector").String()).To(MatchJSON(`{"matchLabels":{"quota.projects.deckhouse.io/lb":"enforced"}}`))
		Expect(lb.Get("matchConditions.0.expression").String()).To(ContainSubstring(`object.spec.type == "LoadBalancer"`))
		Expect(lb.Get("clientConfig.service.path").String()).To(Equal("/validate/quota-lb"))
		Expect(ledgerEnv(hec, "QUOTA_LEDGER_FAMILIES").Get("value").String()).To(Equal("lb"))
		// the controller reads the same families: the keys of a disabled family count nothing
		Expect(hec.KubernetesResource("ConfigMap", quotaLedgerNamespace, "quota-ledger-config").Field("data.families").String()).To(Equal("lb"))

		hec.ValuesSet("global.enabledModules", []string{"vertical-pod-autoscaler", "control-plane-manager", "virtualization"})
		hec.HelmRender(WithAPIVersions("admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy", "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding"))
		Expect(hec.RenderError).ShouldNot(HaveOccurred())
		webhooks = hec.KubernetesGlobalResource("ValidatingWebhookConfiguration", quotaLedgerWebhook).Field("webhooks").Array()
		Expect(webhooks).To(HaveLen(3))
		vm := webhooks[2]
		Expect(vm.Get("name").String()).To(Equal("quota-vm.multitenancy-manager.deckhouse.io"))
		Expect(vm.Get("failurePolicy").String()).To(Equal("Fail"))
		Expect(vm.Get("rules.0.apiGroups").String()).To(MatchJSON(`["virtualization.deckhouse.io"]`))
		Expect(vm.Get("rules.0.resources").String()).To(MatchJSON(`["virtualmachines"]`))
		Expect(vm.Get("rules.0.operations").String()).To(MatchJSON(`["CREATE","UPDATE"]`))
		Expect(vm.Get("namespaceSelector").String()).To(MatchJSON(`{"matchLabels":{"quota.projects.deckhouse.io/vm":"enforced"}}`))
		Expect(vm.Get("clientConfig.service.path").String()).To(Equal("/validate/quota-vm"))
		Expect(ledgerEnv(hec, "QUOTA_LEDGER_FAMILIES").Get("value").String()).To(Equal("lb,vm"))
		Expect(hec.KubernetesResource("ConfigMap", quotaLedgerNamespace, "quota-ledger-config").Field("data.families").String()).To(Equal("lb,vm"))
	})

	Context("on a control plane Deckhouse manages", func() {
		BeforeEach(func() {
			renderQuota(hec, true)
		})

		It("renders a fail-closed webhook on the status writes of d8-project-quota with the timeout the ledger counts with", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			vwc := hec.KubernetesGlobalResource("ValidatingWebhookConfiguration", quotaLedgerWebhook)
			Expect(vwc.Exists()).To(BeTrue())
			webhook := vwc.Field("webhooks.0")
			Expect(webhook.Get("name").String()).To(Equal("quota-ledger.multitenancy-manager.deckhouse.io"))
			Expect(webhook.Get("failurePolicy").String()).To(Equal("Fail"))
			Expect(webhook.Get("sideEffects").String()).To(Equal("None"))
			Expect(webhook.Get("rules.0.resources").String()).To(MatchJSON(`["resourcequotas/status"]`))
			Expect(webhook.Get("rules.0.operations").String()).To(MatchJSON(`["UPDATE"]`))
			Expect(webhook.Get("namespaceSelector.matchExpressions.0.values").String()).To(MatchJSON(`["pending","enforced","canary"]`))
			Expect(webhook.Get("matchConditions.0.expression").String()).To(ContainSubstring(`request.userInfo.username == "system:apiserver"`))
			Expect(webhook.Get("matchConditions.0.expression").String()).To(ContainSubstring(`request.name == "d8-project-quota"`))
			Expect(webhook.Get("matchConditions.0.expression").String()).To(ContainSubstring(`request.name == "quota-ledger-canary"`))

			canary := hec.KubernetesResource("ResourceQuota", quotaLedgerNamespace, "quota-ledger-canary")
			Expect(canary.Field("spec.hard").String()).To(MatchJSON(`{"count/quotaledgercanaries.multitenancy.deckhouse.io":"0"}`),
				"a resource nothing creates: no create of the module waits for the ledger")
			Expect(hec.KubernetesGlobalResource("Namespace", quotaLedgerNamespace).Field(`metadata.labels.projects\.deckhouse\.io/quota`).String()).To(Equal("canary"))
			Expect(webhook.Get("clientConfig.service.name").String()).To(Equal(quotaLedgerService))
			Expect(webhook.Get("clientConfig.service.path").String()).To(Equal("/validate/quota-ledger"))
			Expect(webhook.Get("clientConfig.caBundle").String()).To(Equal("bGVkZ2VyLXRlc3Q="), "the CA of the ledger certificate, not of the other webhooks")

			config := hec.KubernetesResource("ConfigMap", quotaLedgerNamespace, "quota-ledger-config")
			Expect(webhook.Get("timeoutSeconds").String()).To(Equal(config.Field("data.webhookTimeoutSeconds").String()))
			Expect(config.Field("data.leaseSeconds").Int()-config.Field("data.fenceSeconds").Int()-config.Field("data.webhookTimeoutSeconds").Int()).
				To(BeNumerically(">=", 9), "the window the ledger refuses to start without")
		})

		It("routes the webhook through a Service without a selector and runs the ledger in its own container", func() {
			service := hec.KubernetesResource("Service", quotaLedgerNamespace, quotaLedgerService)
			Expect(service.Exists()).To(BeTrue())
			Expect(service.Field("spec.selector").Exists()).To(BeFalse())
			Expect(service.Field("spec.ports.0.targetPort").Int()).To(BeEquivalentTo(9444))

			Expect(hec.KubernetesResource("Secret", quotaLedgerNamespace, "quota-ledger-certs").Exists()).To(BeTrue())

			deployment := hec.KubernetesResource("Deployment", quotaLedgerNamespace, "multitenancy-manager")
			Expect(deployment.Field("spec.replicas").Int()).To(BeEquivalentTo(2), "a second replica takes the lease over even on a single master")
			found := false
			for _, container := range deployment.Field("spec.template.spec.containers").Array() {
				if container.Get("name").String() != "quota-ledger" {
					continue
				}
				found = true
				Expect(container.Get("args").String()).To(MatchJSON(`["quota-ledger"]`))
				Expect(container.Get("livenessProbe.httpGet.path").String()).To(Equal("/livez"))
				Expect(container.Get("readinessProbe.httpGet.path").String()).To(Equal("/healthz"))
				Expect(container.Get("resources.requests.cpu").String()).To(Equal("100m"))
				Expect(container.Get("volumeMounts.0.mountPath").String()).To(Equal("/quota-ledger-certs"))
			}
			Expect(found).To(BeTrue())
			Expect(hec.KubernetesResource("PodDisruptionBudget", quotaLedgerNamespace, "multitenancy-manager").Field("spec.minAvailable").Int()).To(BeEquivalentTo(1))
		})

		It("keeps the lease renewals and the lists of the ledger out of the shared priority levels", func() {
			lease := hec.KubernetesGlobalResource("FlowSchema", quotaLedgerFlow)
			Expect(lease.Field("spec.matchingPrecedence").Int()).To(BeNumerically("<", 1000))
			Expect(lease.Field("spec.priorityLevelConfiguration.name").String()).To(Equal("leader-election"))
			sync := hec.KubernetesGlobalResource("FlowSchema", quotaLedgerSyncFlow)
			Expect(sync.Field("spec.matchingPrecedence").Int()).To(BeNumerically("<", 1000))
			Expect(sync.Field("spec.priorityLevelConfiguration.name").String()).To(Equal(quotaLedgerFlow))
			Expect(hec.KubernetesGlobalResource("PriorityLevelConfiguration", quotaLedgerFlow).Exists()).To(BeTrue())
			// the quorum lists of a new leader cover the objects of the families too
			var listed []string
			for _, rule := range sync.Field("spec.rules.0.resourceRules").Array() {
				for _, resource := range rule.Get("resources").Array() {
					listed = append(listed, resource.String())
				}
			}
			Expect(listed).To(ContainElements("resourcequotas", "namespaces", "projectquotas", "virtualmachines", "services", "metalloadbalancerclasses"))
		})

		It("lets only the ledger, Deckhouse and kube-system write the route and the lease of the ledger", func() {
			policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", quotaLedgerRoute).ToYaml())
			slices := schema.GroupVersionResource{Group: "discovery.k8s.io", Version: "v1", Resource: "endpointslices"}
			endpoints := schema.GroupVersionResource{Version: "v1", Resource: "endpoints"}
			services := schema.GroupVersionResource{Version: "v1", Resource: "services"}
			leases := schema.GroupVersionResource{Group: "coordination.k8s.io", Version: "v1", Resource: "leases"}
			cases := []struct {
				name    string
				attrs   admission.Attributes
				allowed bool
			}{
				{name: "the ledger writes the slice", attrs: write(slices, "", admission.Update, quotaLedgerNamespace, quotaLedgerService, controllerServiceAccount, nil), allowed: true},
				{name: "the garbage collector deletes it", attrs: write(slices, "", admission.Delete, quotaLedgerNamespace, quotaLedgerService, garbageCollector, nil), allowed: true},
				{name: "a cluster administrator writes it", attrs: write(slices, "", admission.Update, quotaLedgerNamespace, quotaLedgerService, "admin@example.com", nil)},
				{name: "a cluster administrator takes the lease", attrs: write(leases, "", admission.Update, quotaLedgerNamespace, quotaLedgerService, "admin@example.com", nil)},
				{name: "another slice", attrs: write(slices, "", admission.Update, quotaLedgerNamespace, "multitenancy-manager-abcde", "admin@example.com", nil), allowed: true},
				{name: "Endpoints of the ledger Service, which kube-controller-manager would mirror", attrs: write(endpoints, "", admission.Create, quotaLedgerNamespace,
					quotaLedgerService, "admin@example.com", nil)},
				{name: "a selector put on the ledger Service", attrs: write(services, "", admission.Update, quotaLedgerNamespace, quotaLedgerService, "admin@example.com", nil)},
				{name: "Deckhouse renders the ledger Service", attrs: write(services, "", admission.Update, quotaLedgerNamespace, quotaLedgerService, deckhouseServiceAccount, nil),
					allowed: true},
				{name: "another Service", attrs: write(services, "", admission.Update, quotaLedgerNamespace, "multitenancy-manager", "admin@example.com", nil), allowed: true},
				{name: "another slice of the ledger Service", attrs: write(slices, "", admission.Create, quotaLedgerNamespace, "multitenancy-manager-quota-ledger-x", "admin@example.com",
					map[string]any{"metadata": map[string]any{"labels": map[string]any{"kubernetes.io/service-name": quotaLedgerService}}})},
				{name: "the label taken off another slice of the ledger Service", attrs: relabel(slices, quotaLedgerNamespace, "multitenancy-manager-quota-ledger-x", "admin@example.com",
					map[string]any{"kubernetes.io/service-name": quotaLedgerService}, nil)},
				{name: "the garbage collector deletes another slice of the ledger Service", attrs: write(slices, "", admission.Delete, quotaLedgerNamespace,
					"multitenancy-manager-quota-ledger-x", garbageCollector, map[string]any{"metadata": map[string]any{"labels": map[string]any{"kubernetes.io/service-name": quotaLedgerService}}}),
					allowed: true},
			}
			for _, tc := range cases {
				allowed, _, err := policy.admit(tc.attrs, nil)
				Expect(err).ToNot(HaveOccurred(), tc.name)
				Expect(allowed).To(Equal(tc.allowed), tc.name)
			}
		})
	})

	Context("on a control plane Deckhouse does not manage", func() {
		BeforeEach(func() {
			renderQuota(hec, false)
		})

		It("renders neither the ledger nor its webhook", func() {
			Expect(hec.KubernetesGlobalResource("ValidatingWebhookConfiguration", quotaLedgerWebhook).Exists()).To(BeFalse())
			Expect(hec.KubernetesResource("Service", quotaLedgerNamespace, quotaLedgerService).Exists()).To(BeFalse())
			for _, container := range hec.KubernetesResource("Deployment", quotaLedgerNamespace, "multitenancy-manager").Field("spec.template.spec.containers").Array() {
				Expect(container.Get("name").String()).ToNot(Equal("quota-ledger"))
			}
		})
	})
})

// relabel is an update of an object of the resource that changes its labels from old to labels.
func relabel(resource schema.GroupVersionResource, namespace, name, username string, old, labels map[string]any) admission.Attributes {
	gvk := schema.GroupVersionKind{Group: resource.Group, Version: resource.Version, Kind: "Object"}
	object := func(labels map[string]any) *unstructured.Unstructured {
		o := &unstructured.Unstructured{Object: map[string]any{"metadata": map[string]any{"labels": labels}}}
		o.SetGroupVersionKind(gvk)
		o.SetName(name)
		o.SetNamespace(namespace)
		return o
	}
	return admission.NewAttributesRecord(object(labels), object(old), gvk, namespace, name, resource, "", admission.Update, nil, false,
		&user.DefaultInfo{Name: username, Groups: groupsOf(username)})
}

// ledgerEnv is the environment variable of the quota-ledger container.
func ledgerEnv(hec *Config, name string) gjson.Result {
	deployment := hec.KubernetesResource("Deployment", quotaLedgerNamespace, "multitenancy-manager")
	for _, container := range deployment.Field("spec.template.spec.containers").Array() {
		if container.Get("name").String() != "quota-ledger" {
			continue
		}
		for _, env := range container.Get("env").Array() {
			if env.Get("name").String() == name {
				return env
			}
		}
	}
	return gjson.Result{}
}

// webhookCondition evaluates a match condition of a webhook the way the API server binds it.
func webhookCondition(expression string, operation string, object, oldObject map[string]any) bool {
	env, err := cel.NewEnv(ext.Strings(), cel.Variable("object", cel.DynType), cel.Variable("oldObject", cel.DynType), cel.Variable("request", cel.DynType))
	Expect(err).ShouldNot(HaveOccurred())
	ast, issues := env.Compile(expression)
	Expect(issues.Err()).ShouldNot(HaveOccurred(), expression)
	program, err := env.Program(ast)
	Expect(err).ShouldNot(HaveOccurred())
	var old any
	if oldObject != nil {
		old = oldObject
	}
	out, _, err := program.Eval(map[string]any{"object": object, "oldObject": old, "request": map[string]any{"operation": operation}})
	Expect(err).ShouldNot(HaveOccurred(), expression)
	return out.Value() == true
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: quota ledger :: object webhooks ::", func() {
	hec := SetupHelmConfig("")

	It("calls the ledger only on a change the quota counts", func() {
		hec.ValuesSet("global.enabledModules", []string{"vertical-pod-autoscaler", "control-plane-manager", "virtualization"})
		renderQuota(hec, true)
		hec.ValuesSet("global.enabledModules", []string{"vertical-pod-autoscaler", "control-plane-manager", "virtualization"})
		hec.HelmRender(WithAPIVersions("admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy", "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding"))
		Expect(hec.RenderError).ShouldNot(HaveOccurred())
		webhooks := hec.KubernetesGlobalResource("ValidatingWebhookConfiguration", quotaLedgerWebhook).Field("webhooks").Array()
		Expect(webhooks).To(HaveLen(3))

		lb := webhooks[1].Get("matchConditions.1.expression").String()
		svc := func(annotations map[string]any, finalizers ...any) map[string]any {
			return map[string]any{"metadata": map[string]any{"annotations": annotations, "finalizers": finalizers},
				"spec": map[string]any{"type": "LoadBalancer", "ports": []any{map[string]any{"port": int64(80)}}}}
		}
		base := map[string]any{"network.deckhouse.io/load-balancer-ips": "10.0.0.1"}
		Expect(webhookCondition(lb, "CREATE", svc(base), nil)).To(BeTrue())
		Expect(webhookCondition(lb, "UPDATE", svc(base, "metallb"), svc(base))).To(BeFalse(), "a finalizer is not counted")
		Expect(webhookCondition(lb, "UPDATE", svc(map[string]any{"network.deckhouse.io/load-balancer-ips": "10.0.0.1,10.0.0.2"}), svc(base))).To(BeTrue())
		Expect(webhookCondition(lb, "UPDATE", svc(map[string]any{"network.deckhouse.io/load-balancer-shared-ip-key": "k"}), svc(map[string]any{}))).To(BeFalse(),
			"a shared address counts per Service, so the key is not counted")

		vm := webhooks[2].Get("matchConditions.0.expression").String()
		machine := func(cores int64, labels map[string]any) map[string]any {
			return map[string]any{"metadata": map[string]any{"labels": labels}, "spec": map[string]any{"cpu": map[string]any{"cores": cores}}}
		}
		Expect(webhookCondition(vm, "CREATE", machine(2, nil), nil)).To(BeTrue())
		Expect(webhookCondition(vm, "UPDATE", machine(2, map[string]any{"a": "b"}), machine(2, nil))).To(BeFalse())
		Expect(webhookCondition(vm, "UPDATE", machine(4, nil), machine(2, nil))).To(BeTrue())
	})
})
