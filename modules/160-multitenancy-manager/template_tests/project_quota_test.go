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
	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	quotaGuardPolicyName       = "d8-multitenancy-manager-quota-guard"
	quotaPendingPolicyName     = "d8-multitenancy-manager-quota-pending"
	projectQuotaPolicyName     = "d8-multitenancy-manager-project-quota"
	projectQuotaStatusPolicy   = "d8-multitenancy-manager-project-quota-status"
	projectQuotaCRStatusPolicy = "d8-multitenancy-manager-projectquota-status"

	resourceQuotaController = "system:serviceaccount:kube-system:resourcequota-controller"
	replicaSetController    = "system:serviceaccount:kube-system:replicaset-controller"
	namespaceController     = "system:serviceaccount:kube-system:namespace-controller"
)

// renderQuota renders the module on a cluster whose control plane Deckhouse manages or not.
func renderQuota(hec *Config, controlPlaneManaged bool) {
	modules := []string{"vertical-pod-autoscaler"}
	if controlPlaneManaged {
		modules = append(modules, "control-plane-manager")
	}
	hec.ValuesSet("global.modulesImages", GetModulesImages())
	hec.ValuesSet("global.enabledModules", modules)
	hec.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
	hec.ValuesSet("multitenancyManager.internal.admissionWebhookCert.ca", "test")
	hec.ValuesSet("multitenancyManager.internal.admissionWebhookCert.crt", "test")
	hec.ValuesSet("multitenancyManager.internal.admissionWebhookCert.key", "test")
	hec.HelmRender(WithAPIVersions(
		"admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy",
		"admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding",
	))
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: project quota ::", func() {
	hec := SetupHelmConfig("")

	cases := []struct {
		title        string
		managed      bool
		managedValue string
	}{
		{title: "on a control plane Deckhouse manages", managed: true, managedValue: "true"},
		{title: "on a control plane Deckhouse does not manage", managedValue: "false"},
	}
	for _, c := range cases {
		Context(c.title, func() {
			BeforeEach(func() {
				renderQuota(hec, c.managed)
			})

			It("tells the controller about the control plane and renders the guard only where the sum is enforced", func() {
				Expect(hec.RenderError).ShouldNot(HaveOccurred())

				config := hec.KubernetesResource("ConfigMap", "d8-multitenancy-manager", "quota-ledger-config")
				Expect(config.Exists()).To(BeTrue())
				Expect(config.Field("data.controlPlaneManaged").String()).To(Equal(c.managedValue))

				for _, name := range []string{quotaGuardPolicyName, quotaPendingPolicyName, projectQuotaStatusPolicy} {
					Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", name).Exists()).To(Equal(c.managed), name)
					Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", name).Exists()).To(Equal(c.managed), name)
				}

				for _, name := range []string{projectQuotaPolicyName, projectQuotaCRStatusPolicy} {
					Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", name).Exists()).To(BeTrue(), name)
					Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", name).Exists()).To(BeTrue(), name)
				}
			})
		})
	}

	Context("on a control plane Deckhouse manages", func() {
		BeforeEach(func() {
			renderQuota(hec, true)
		})

		It("binds the guards to the namespaces by their quota label and refuses a create without the quota", func() {
			enforced := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", quotaGuardPolicyName)
			Expect(enforced.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
			Expect(enforced.Field("spec.paramRef").String()).To(MatchJSON(`{"name":"d8-project-quota","parameterNotFoundAction":"Deny"}`))
			Expect(enforced.Field("spec.matchResources.namespaceSelector").String()).To(MatchJSON(`{"matchLabels":{"projects.deckhouse.io/quota":"enforced"}}`))

			pendingPolicy := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", quotaPendingPolicyName)
			guardPolicy := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", quotaGuardPolicyName)
			Expect(pendingPolicy.Field("spec.paramKind").Exists()).To(BeFalse())
			Expect(guardPolicy.Field("spec.paramKind").Exists()).To(BeTrue())
			for _, field := range []string{"spec.matchConstraints", "spec.matchConditions"} {
				Expect(pendingPolicy.Field(field).String()).To(MatchJSON(guardPolicy.Field(field).String()), field)
			}
			Expect(guardPolicy.Field("spec.matchConstraints.excludeResourceRules.#.resources").String()).To(
				MatchJSON(`[["resourcequotas","events"],["events"],["localsubjectaccessreviews"],["configmaps"],["serviceaccounts"]]`))
			Expect(guardPolicy.Field("spec.matchConstraints.excludeResourceRules.#.resourceNames").String()).To(MatchJSON(`[["kube-root-ca.crt"],["default"]]`))

			pending := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", quotaPendingPolicyName)
			Expect(pending.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
			Expect(pending.Field("spec.paramRef").Exists()).To(BeFalse(), "a missing parameter would refuse the exclusions too")
			Expect(pending.Field("spec.matchResources.namespaceSelector").String()).To(MatchJSON(`{"matchLabels":{"projects.deckhouse.io/quota":"pending"}}`))

		})

		// kube-apiserver looks the parameter of a binding up before it evaluates the match conditions:
		// a namespace guarded by a binding that refuses without its parameter refuses the controller the
		// very quota that parameter is.
		It("lets the controller make the quota of a new namespace", func() {
			guards := []string{quotaPendingPolicyName, quotaGuardPolicyName}
			admitted := func(namespaceLabels map[string]string, attrs admission.Attributes, params runtime.Object) (bool, string) {
				for _, name := range guards {
					if allowed, message := throughBinding(hec, name, namespaceLabels, attrs, params); !allowed {
						return false, message
					}
				}
				return true, ""
			}
			accountController := func(name string) string { return "system:serviceaccount:kube-system:" + name }

			born := map[string]string{"projects.deckhouse.io/quota": "pending"}
			for _, attrs := range []admission.Attributes{
				create(core("resourcequotas"), "d8-project-quota", controllerServiceAccount),
				create(schema.GroupVersionResource{Group: "multitenancy.deckhouse.io", Version: "v1alpha1", Resource: "availableclusterresources"}, "storageclasses",
					controllerServiceAccount),
				create(core("configmaps"), "kube-root-ca.crt", accountController("root-ca-cert-publisher")),
				create(core("serviceaccounts"), "default", accountController("service-account-controller")),
			} {
				allowed, message := admitted(born, attrs, nil)
				Expect(allowed).To(BeTrue(), "%s %s: %s", attrs.GetResource().Resource, attrs.GetName(), message)
			}
			allowed, message := admitted(born, create(core("pods"), "web", projectAdministrator), nil)
			Expect(allowed).To(BeFalse())
			Expect(message).To(ContainSubstring("not calculated"))

			// an enforced namespace whose quota went missing: the parameter is gone, and a Deny binding
			// refuses before its match conditions, the writers they leave out included; only what the
			// match constraints leave out passes, the quota itself among it
			enforced := map[string]string{"projects.deckhouse.io/quota": "enforced"}
			allowed, message = admitted(enforced, create(core("resourcequotas"), "d8-project-quota", controllerServiceAccount), nil)
			Expect(allowed).To(BeTrue(), message)
			allowed, message = admitted(enforced, create(core("configmaps"), "kube-root-ca.crt", accountController("root-ca-cert-publisher")), nil)
			Expect(allowed).To(BeTrue(), message)
			allowed, _ = admitted(enforced, create(core("pods"), "web", projectAdministrator), nil)
			Expect(allowed).To(BeFalse())
			allowed, _ = admitted(enforced, create(core("configmaps"), "kube-root-ca.crt-2", controllerServiceAccount), nil)
			Expect(allowed).To(BeFalse(), "the controller is left out by a match condition, which comes after the parameter")
			allowed, message = admitted(enforced, create(core("pods"), "web", projectAdministrator), quotaParams(map[string]any{"pods": "10"}, true, false))
			Expect(allowed).To(BeTrue(), message)
		})

		It("serves the keys webhook of ProjectQuota and grants the quotas to IAM managers only", func() {
			webhooks := hec.KubernetesGlobalResource("ValidatingWebhookConfiguration", "d8-multitenancy-manager-webhook")
			found := false
			for _, webhook := range webhooks.Field("webhooks").Array() {
				if webhook.Get("name").String() != "projectquotas.multitenancy-webhook.deckhouse.io" {
					continue
				}
				found = true
				Expect(webhook.Get("rules.0.resources").String()).To(MatchJSON(`["projectquotas"]`))
				Expect(webhook.Get("clientConfig.service.path").String()).To(Equal("/validate/v1alpha1/projectquotas"))
				Expect(webhook.Get("matchConditions.0.name").String()).To(Equal("exclude-system-writers"))
			}
			Expect(found).To(BeTrue())

			capability := hec.KubernetesGlobalResource("ClusterRole", "d8:system-capability:multitenancy-manager:manage_quotas")
			Expect(capability.Exists()).To(BeTrue())
			Expect(capability.Field(`metadata.labels.rbac\.deckhouse\.io/aggregate-to-iam-as`).String()).To(Equal("manager"))
			Expect(capability.Field("rules.0.resources").String()).To(MatchJSON(`["projectquotas"]`))
		})

		It("refuses the objects a quota counts until the quota is calculated", func() {
			policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", quotaGuardPolicyName).ToYaml())
			calculated := quotaParams(map[string]any{"pods": "10", "requests.cpu": "4", "services.loadbalancers": "1"}, true, false)
			pending := quotaParams(map[string]any{"pods": "10", "count/configmaps": "5"}, false, false)
			scoped := quotaParams(map[string]any{"pods": "10"}, true, true)

			cases := []struct {
				name     string
				attrs    admission.Attributes
				params   runtime.Object
				allowed  bool
				contains string
			}{
				{name: "a pod in a calculated namespace", attrs: create(core("pods"), "web", projectAdministrator), params: calculated, allowed: true},
				{name: "a pod before the quota is calculated", attrs: create(core("pods"), "web", projectAdministrator), params: pending, contains: "not calculated"},
				{name: "a ConfigMap the quota counts before it is calculated", attrs: create(core("configmaps"), "cm", projectAdministrator), params: pending,
					contains: "not calculated"},
				// the plugin of the API server skips the key until the controller calculates it; holding every
				// create of the project for that is the cost the guard does not take
				{name: "a pod while a key just added to a calculated quota is not calculated", attrs: create(core("pods"), "web", projectAdministrator),
					params: quotaParamsWithStatus(map[string]any{"pods": "10", "requests.cpu": "4"}, map[string]any{"pods": "10"}), allowed: true},
				{name: "a Secret the quota does not count", attrs: create(core("secrets"), "s", projectAdministrator), params: pending, allowed: true},
				{name: "a pod resize", attrs: resize(projectAdministrator), params: pending, contains: "not calculated"},
				{name: "a load balancer counted by services.loadbalancers", attrs: create(core("services"), "lb", projectAdministrator),
					params: quotaParams(map[string]any{"services.loadbalancers": "1"}, false, false), contains: "not calculated"},
				{name: "a pod under a quota with scopes", attrs: create(core("pods"), "web", projectAdministrator), params: scoped, contains: "scopes"},
				{name: "the quota itself", attrs: create(core("resourcequotas"), "own", projectAdministrator), params: pending, allowed: true},
				{name: "the controller", attrs: create(core("pods"), "web", controllerServiceAccount), params: pending, allowed: true},
				{name: "the root CA ConfigMap of a new namespace", attrs: create(core("configmaps"), "kube-root-ca.crt", "system:serviceaccount:kube-system:root-ca-cert-publisher"),
					params: quotaParams(map[string]any{"count/configmaps": "5"}, false, false), allowed: true},
				{name: "a review", attrs: create(schema.GroupVersionResource{Group: "authorization.k8s.io", Version: "v1", Resource: "localsubjectaccessreviews"}, "", projectAdministrator),
					params: pending, allowed: true},
			}
			for _, tc := range cases {
				allowed, message, err := policy.admitWithParams(tc.attrs, nil, tc.params)
				Expect(err).ToNot(HaveOccurred(), tc.name)
				Expect(allowed).To(Equal(tc.allowed), tc.name)
				if tc.contains != "" {
					Expect(message).To(ContainSubstring(tc.contains), tc.name)
				}
			}
		})

		It("keeps d8-project-quota to the controller and resourcequotas out of every quota of a project", func() {
			policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", projectQuotaPolicyName).ToYaml())
			cases := []struct {
				name    string
				attrs   admission.Attributes
				allowed bool
			}{
				{name: "a user creates d8-project-quota", attrs: quotaWrite(admission.Create, "d8-project-quota", projectAdministrator, map[string]any{"pods": "1000"})},
				{name: "a user deletes d8-project-quota", attrs: quotaWrite(admission.Delete, "d8-project-quota", projectAdministrator, nil)},
				{name: "the controller creates d8-project-quota", attrs: quotaWrite(admission.Create, "d8-project-quota", controllerServiceAccount, map[string]any{"pods": "10"}),
					allowed: true},
				{name: "the namespace controller deletes it with the namespace", attrs: quotaWrite(admission.Delete, "d8-project-quota", namespaceController, nil), allowed: true},
				{name: "kube-controller-manager without a service account for each controller deletes it with the namespace",
					attrs: quotaWrite(admission.Delete, "d8-project-quota", "system:kube-controller-manager", nil), allowed: true},
				{name: "a quota of a project administrator", attrs: quotaWrite(admission.Create, "team-limits", projectAdministrator, map[string]any{"pods": "5"}), allowed: true},
				{name: "a quota that limits resourcequotas", attrs: quotaWrite(admission.Create, "team-limits", projectAdministrator, map[string]any{"resourcequotas": "1"})},
				{name: "a quota that limits count/resourcequotas", attrs: quotaWrite(admission.Create, "team-limits", projectAdministrator,
					map[string]any{"count/resourcequotas": "1"})},
			}
			for _, tc := range cases {
				allowed, _, err := policy.admit(tc.attrs, nil)
				Expect(err).ToNot(HaveOccurred(), tc.name)
				Expect(allowed).To(Equal(tc.allowed), tc.name)
			}
		})

		It("lets only the quota plugin and the resource quota controller write the usage of d8-project-quota", func() {
			policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", projectQuotaStatusPolicy).ToYaml())
			cases := []struct {
				name     string
				quota    string
				username string
				allowed  bool
			}{
				{name: "the quota plugin", quota: "d8-project-quota", username: "system:apiserver", allowed: true},
				{name: "the resource quota controller", quota: "d8-project-quota", username: resourceQuotaController, allowed: true},
				{name: "kube-controller-manager without a service account for each controller", quota: "d8-project-quota", username: "system:kube-controller-manager",
					allowed: true},
				{name: "a cluster administrator", quota: "d8-project-quota", username: "admin@example.com"},
				{name: "another controller of kube-system", quota: "d8-project-quota", username: replicaSetController},
				{name: "the status of another quota", quota: "team-limits", username: "admin@example.com", allowed: true},
			}
			for _, tc := range cases {
				allowed, _, err := policy.admit(statusUpdate(core("resourcequotas"), "team", tc.quota, tc.username), nil)
				Expect(err).ToNot(HaveOccurred(), tc.name)
				Expect(allowed).To(Equal(tc.allowed), tc.name)
			}

			crStatus := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", projectQuotaCRStatusPolicy).ToYaml())
			projectQuotas := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "projectquotas"}
			allowed, _, err := crStatus.admit(statusUpdate(projectQuotas, "", "team", controllerServiceAccount), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
			allowed, _, err = crStatus.admit(statusUpdate(projectQuotas, "", "team", "admin@example.com"), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeFalse())
		})
	})
})

// throughBinding admits the request the way kube-apiserver applies the binding of the name and its
// policy: the excluded resources of the policy and the namespace selector first, then the parameter,
// whose absence a Deny binding answers before any match condition, then the policy. A nil params is a
// missing parameter.
func throughBinding(hec *Config, name string, namespaceLabels map[string]string, attrs admission.Attributes, params runtime.Object) (bool, string) {
	binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", name)
	Expect(binding.Exists()).To(BeTrue(), name)
	for _, rule := range hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", name).Field("spec.matchConstraints.excludeResourceRules").Array() {
		if excludes(rule, attrs) {
			return true, ""
		}
	}
	selector := new(metav1.LabelSelector)
	if raw := binding.Field("spec.matchResources.namespaceSelector"); raw.Exists() {
		Expect(json.Unmarshal([]byte(raw.Raw), selector)).To(Succeed())
	}
	matches, err := metav1.LabelSelectorAsSelector(selector)
	Expect(err).ToNot(HaveOccurred())
	if !matches.Matches(labels.Set(namespaceLabels)) {
		return true, ""
	}
	if paramRef := binding.Field("spec.paramRef"); paramRef.Exists() && params == nil {
		if paramRef.Get("parameterNotFoundAction").String() == "Deny" {
			return false, "no params found for policy binding with `Deny` parameterNotFoundAction"
		}
		return true, ""
	}
	policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", name).ToYaml())
	allowed, message, err := policy.admitWithParams(attrs, nil, params)
	Expect(err).ToNot(HaveOccurred(), name)
	return allowed, message
}

// excludes reports whether an excluded resource rule of a policy covers the request: its group, its
// resource with the subresource, and its name when the rule names some.
func excludes(rule gjson.Result, attrs admission.Attributes) bool {
	resource := attrs.GetResource().Resource
	if attrs.GetSubresource() != "" {
		resource += "/" + attrs.GetSubresource()
	}
	in := func(field, value string) bool {
		values := rule.Get(field).Array()
		if len(values) == 0 {
			return field == "resourceNames"
		}
		for _, v := range values {
			if v.String() == "*" || v.String() == value {
				return true
			}
		}
		return false
	}
	return in("apiGroups", attrs.GetResource().Group) && in("resources", resource) && in("resourceNames", attrs.GetName())
}

func core(resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Version: "v1", Resource: resource}
}

// quotaParams is a d8-project-quota with the limits, calculated when status.hard carries them.
func quotaParams(hard map[string]any, calculated, scoped bool) runtime.Object {
	spec := map[string]any{"hard": hard}
	if scoped {
		spec["scopes"] = []any{"BestEffort"}
	}
	object := map[string]any{"apiVersion": "v1", "kind": "ResourceQuota", "metadata": map[string]any{"name": "d8-project-quota", "namespace": "team"}, "spec": spec}
	if calculated {
		object["status"] = map[string]any{"hard": hard}
	}
	return &unstructured.Unstructured{Object: object}
}

// quotaParamsWithStatus is a d8-project-quota with the limits in spec and the ones the controller
// has calculated in status.hard.
func quotaParamsWithStatus(hard, calculated map[string]any) runtime.Object {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "ResourceQuota", "metadata": map[string]any{"name": "d8-project-quota", "namespace": "team"},
		"spec": map[string]any{"hard": hard}, "status": map[string]any{"hard": calculated},
	}}
}

// write is a request for an object of the resource; content is the rest of the object, and a nil one
// leaves the object out, as a delete does.
func write(resource schema.GroupVersionResource, subresource string, operation admission.Operation, namespace, name, username string,
	content map[string]any,
) admission.Attributes {
	gvk := schema.GroupVersionKind{Group: resource.Group, Version: resource.Version, Kind: "Object"}
	object := &unstructured.Unstructured{Object: map[string]any{}}
	for key, value := range content {
		object.Object[key] = value
	}
	object.SetGroupVersionKind(gvk)
	object.SetName(name)
	object.SetNamespace(namespace)

	// The old object stays a nil interface on create: a typed nil pointer counts as present.
	var newObject, oldObject runtime.Object = object, nil
	switch operation {
	case admission.Update:
		oldObject = object.DeepCopy()
	case admission.Delete:
		newObject, oldObject = nil, object
	}
	return admission.NewAttributesRecord(newObject, oldObject, gvk, namespace, name, resource, subresource, operation, nil, false,
		&user.DefaultInfo{Name: username, Groups: groupsOf(username)})
}

// create is the create of an object of the resource in the team namespace.
func create(resource schema.GroupVersionResource, name, username string) admission.Attributes {
	return write(resource, "", admission.Create, "team", name, username, nil)
}

// resize is a resize of a pod in the team namespace.
func resize(username string) admission.Attributes {
	return write(core("pods"), "resize", admission.Update, "team", "web", username, nil)
}

// quotaWrite is a write of a ResourceQuota in the team namespace; nil hard leaves spec out.
func quotaWrite(operation admission.Operation, name, username string, hard map[string]any) admission.Attributes {
	content := map[string]any{}
	if hard != nil {
		content["spec"] = map[string]any{"hard": hard}
	}
	return write(core("resourcequotas"), "", operation, "team", name, username, content)
}

// statusUpdate is an update of the status of the named object of the resource.
func statusUpdate(resource schema.GroupVersionResource, namespace, name, username string) admission.Attributes {
	return write(resource, "status", admission.Update, namespace, name, username, nil)
}
