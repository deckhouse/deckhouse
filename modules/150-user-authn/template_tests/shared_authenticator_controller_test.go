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

	"github.com/tidwall/gjson"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	admissioncel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	dexClientControllerPolicyName     = "dexclient-controller.user-authn.deckhouse.io"
	dexClientControllerServiceAccount = "system:serviceaccount:d8-user-authn:dexclient-controller"
	managedByLabel                    = "app.kubernetes.io/managed-by"
	managedByController               = "user-authn-controller"
)

// objectWriteAdmitted reports whether the policy admits the write of an object of the kind by the
// user. A nil map of labels is no object: there is no new one on a delete and no old one on a
// create. spec is the spec of the new object, if it has one.
func objectWriteAdmitted(p compiledPolicy, username string, gvk schema.GroupVersionKind, resource string, operation admission.Operation, oldLabels, newLabels map[string]string, spec map[string]any) (bool, error) {
	gvr := gvk.GroupVersion().WithResource(resource)
	object := func(labels map[string]string, spec map[string]any) runtime.Object {
		if labels == nil {
			// A nil interface: the admission machinery takes a typed nil pointer for an object.
			return nil
		}
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		obj.SetNamespace("shop")
		obj.SetName("app-dex-authenticator")
		if len(labels) > 0 {
			obj.SetLabels(labels)
		}
		if spec != nil {
			obj.Object["spec"] = spec
		}
		return obj
	}

	attributes := admission.NewAttributesRecord(object(newLabels, spec), object(oldLabels, nil), gvk, "shop", "app-dex-authenticator", gvr, "",
		operation, nil, false, &user.DefaultInfo{Name: username, Groups: defaultGroups(username)})
	versionedAttributes, err := admission.NewVersionedAttributes(attributes, gvk, nil)
	if err != nil {
		return false, err
	}
	request := admissioncel.CreateAdmissionRequest(attributes, metav1.GroupVersionResource(gvr), metav1.GroupVersionKind(gvk))
	selected, err := p.evaluate(p.matchConditions, versionedAttributes, request, admissioncel.OptionalVariableBindings{})
	if err != nil || !selected {
		return true, err
	}
	return p.evaluate(p.validations, versionedAttributes, request, admissioncel.OptionalVariableBindings{})
}

// referenceGrantAPIVersion is the ReferenceGrant kind of the Gateway API the controller writes, and
// referenceGrantV1beta1APIVersion the older one, which a cluster may serve alone.
const (
	referenceGrantAPIVersion        = "gateway.networking.k8s.io/v1/ReferenceGrant"
	referenceGrantV1beta1APIVersion = "gateway.networking.k8s.io/v1beta1/ReferenceGrant"
)

var _ = Describe("Module :: user-authn :: helm template :: DexAuthenticator reconcilers of dexclient-controller", func() {
	hec := SetupHelmConfig("")

	const namespace = "d8-user-authn"

	setValues := func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.33.1")
		hec.ValuesSet("global.discovery.clusterDomain", "cluster.local")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler", "operator-prometheus"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")

		hec.ValuesSetFromYaml("userAuthn.internal.sharedDexAuthenticator.redisPasswords", `
dex-authenticator-system: dh-test-secret-0123456789abcdef0
dex-authenticator: dh-test-secret-0123456789abcdef1
`)
	}

	policyAPIVersions := []string{validatingAdmissionPolicyAPIVersion, validatingAdmissionPolicyBindingAPIVersion}

	// controllerEnv returns the environment of the dexclient-controller container.
	controllerEnv := func() map[string]string {
		deployment := hec.KubernetesResource("Deployment", namespace, "dexclient-controller")
		Expect(deployment.Exists()).To(BeTrue())
		env := map[string]string{}
		for _, variable := range deployment.Field("spec.template.spec.containers.0.env").Array() {
			env[variable.Get("name").String()] = variable.Get("value").String()
		}
		return env
	}

	// grants returns what the ClusterRole and the Role of dexclient-controller grant on the resource.
	grants := func(resource string) (cluster, namespaced []string) {
		clusterRole := hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:dexclient-controller")
		Expect(clusterRole.Exists()).To(BeTrue())
		for _, rule := range rulesFor(clusterRole.Field("rules").Result, resource) {
			cluster = append(cluster, verbsOf(rule)...)
		}
		role := hec.KubernetesResource("Role", namespace, "dexclient-controller")
		Expect(role.Exists()).To(BeTrue())
		for _, rule := range rulesFor(role.Field("rules").Result, resource) {
			namespaced = append(namespaced, verbsOf(rule)...)
		}
		return cluster, namespaced
	}

	settings := func() gjson.Result {
		cm := hec.KubernetesResource("ConfigMap", namespace, "shared-dex-authenticator")
		Expect(cm.Exists()).To(BeTrue())
		raw := cm.Field("data.settings\\.json").String()
		Expect(json.Valid([]byte(raw))).To(BeTrue(), raw)
		return gjson.Parse(raw)
	}

	Context("while the module renders the shared instances", func() {
		BeforeEach(func() {
			setValues()
			hec.HelmRender(WithAPIVersions(policyAPIVersions...))
		})

		It("turns the reconcilers on and passes them the settings", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			Expect(controllerEnv()).To(HaveKeyWithValue("SHARED_DEX_AUTHENTICATOR", "true"))
			// No HTTPRoute kind in the cluster: the controller must not watch it.
			Expect(controllerEnv()).NotTo(HaveKey("GATEWAY_API"))

			Expect(settings().String()).To(MatchJSON(`{
				"clusterDomain": "cluster.local",
				"httpsMode": "CertManager",
				"ingressEnabled": true,
				"ingressNginxHSTSAnnotation": false,
				"gatewayAPI": false,
				"dexSessions": false,
				"platformSSO": true,
				"idTokenTTL": "10m"
			}`))
		})

		It("grants what the reconcilers read and write, and no write to pods", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			cluster, _ := grants("dexauthenticators")
			Expect(cluster).To(ConsistOf("get", "list", "watch"))
			cluster, _ = grants("dexauthenticators/status")
			Expect(cluster).To(ConsistOf("update"))
			for _, resource := range []string{"services", "ingresses"} {
				cluster, namespaced := grants(resource)
				Expect(cluster).To(ConsistOf("get", "list", "watch", "create", "update", "delete"), resource)
				Expect(namespaced).To(BeEmpty(), resource)
			}
			cluster, _ = grants("httproutes")
			Expect(cluster).To(BeEmpty(), "no Gateway API in the cluster")

			// The pods of the instances are asked which table they serve and never changed: a right to
			// change pods of the module namespace would let the controller replace the image of dex.
			for _, resource := range []string{"pods", "configmaps"} {
				cluster, namespaced := grants(resource)
				Expect(cluster).To(BeEmpty(), resource)
				Expect(namespaced).To(ConsistOf("get", "list", "watch"), resource)
			}
			for _, resource := range []string{"pods/exec", "pods/ephemeralcontainers", "pods/log", "endpointslices"} {
				cluster, namespaced := grants(resource)
				Expect(cluster).To(BeEmpty(), resource)
				Expect(namespaced).To(BeEmpty(), resource)
			}
		})

		It("narrows the writes of dexclient-controller to the objects it has labelled", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			rendered := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", dexClientControllerPolicyName)
			Expect(rendered.Exists()).To(BeTrue())
			Expect(rendered.Field("spec.failurePolicy").String()).To(Equal("Fail"))
			resources := map[string]string{}
			for _, rule := range rendered.Field("spec.matchConstraints.resourceRules").Array() {
				Expect(rule.Get("operations").String()).To(MatchJSON(`["CREATE","UPDATE","DELETE"]`))
				Expect(rule.Get("apiVersions").String()).To(MatchJSON(`["*"]`))
				resources[rule.Get("resources.0").String()] = rule.Get("apiGroups.0").String()
			}
			// Every kind the ClusterRole lets the controller write outside the module namespace.
			Expect(resources).To(Equal(map[string]string{"services": "", "ingresses": "networking.k8s.io", "httproutes": "gateway.networking.k8s.io"}))

			binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", dexClientControllerPolicyName)
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("spec.policyName").String()).To(Equal(dexClientControllerPolicyName))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))
			Expect(binding.Field("spec.matchResources").Exists()).To(BeFalse(), "the policy holds in every namespace")

			policy := compileRenderedPolicy(rendered.ToYaml())
			own := map[string]string{managedByLabel: managedByController, "app": "dex-authenticator"}
			helm := map[string]string{managedByLabel: "Helm", "app": "dex-authenticator"}
			other := map[string]string{"app": "shop"}
			none := map[string]string{}

			kinds := []struct {
				gvk      schema.GroupVersionKind
				resource string
			}{
				{schema.GroupVersionKind{Version: "v1", Kind: "Service"}, "services"},
				{schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"}, "ingresses"},
				{schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "HTTPRoute"}, "httproutes"},
			}
			for _, tc := range []struct {
				name      string
				username  string
				operation admission.Operation
				old, new  map[string]string
				allowed   bool
			}{
				{name: "creates its own object", operation: admission.Create, new: own, allowed: true},
				{name: "cannot create an object without the label", operation: admission.Create, new: other},
				{name: "cannot create an object without labels", operation: admission.Create, new: none},
				{name: "cannot create an object of another manager", operation: admission.Create, new: helm},
				{name: "changes its own object", operation: admission.Update, old: own, new: own, allowed: true},
				{name: "cannot take an object of somebody else", operation: admission.Update, old: other, new: own},
				{name: "cannot take an object without labels", operation: admission.Update, old: none, new: own},
				{name: "cannot take an object of the chart", operation: admission.Update, old: helm, new: own},
				{name: "cannot take the label off its own object", operation: admission.Update, old: own, new: other},
				{name: "cannot change an object of somebody else", operation: admission.Update, old: other, new: other},
				{name: "deletes its own object", operation: admission.Delete, old: own, allowed: true},
				{name: "cannot delete an object of somebody else", operation: admission.Delete, old: other},
				{name: "cannot delete an object without labels", operation: admission.Delete, old: none},
				{name: "leaves another controller of the module alone", username: "system:serviceaccount:d8-user-authn:controller", operation: admission.Update, old: other, new: other, allowed: true},
				{name: "leaves a user alone", username: "editor@example.com", operation: admission.Delete, old: other, allowed: true},
				{name: "leaves the account of the same name in another namespace alone", username: "system:serviceaccount:shop:dexclient-controller", operation: admission.Create, new: other, allowed: true},
			} {
				username := tc.username
				if username == "" {
					username = dexClientControllerServiceAccount
				}
				for _, kind := range kinds {
					By(tc.name + ": " + kind.resource)
					// The Service the controller writes for a DexAuthenticator: the label is what
					// these cases are about.
					var spec map[string]any
					if kind.resource == "services" {
						spec = map[string]any{"type": "ExternalName", "externalName": "dex-authenticator.d8-user-authn.svc.cluster.local"}
					}
					allowed, err := objectWriteAdmitted(policy, username, kind.gvk, kind.resource, tc.operation, tc.old, tc.new, spec)
					Expect(err).ToNot(HaveOccurred())
					Expect(allowed).To(Equal(tc.allowed))
				}
			}
		})

		It("lets dexclient-controller write only the Service a DexAuthenticator needs", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", dexClientControllerPolicyName).ToYaml())
			own := map[string]string{managedByLabel: managedByController, "app": "dex-authenticator"}
			service := schema.GroupVersionKind{Version: "v1", Kind: "Service"}
			userInstance := "dex-authenticator.d8-user-authn.svc.cluster.local"
			systemInstance := "dex-authenticator-system.d8-user-authn.svc.cluster.local"

			for _, tc := range []struct {
				name     string
				username string
				spec     map[string]any
				allowed  bool
			}{
				{name: "an ExternalName to the instance of the user namespaces", spec: map[string]any{"type": "ExternalName", "externalName": userInstance}, allowed: true},
				{name: "an ExternalName to the instance of the system namespaces", spec: map[string]any{"type": "ExternalName", "externalName": systemInstance}, allowed: true},
				{name: "an ExternalName with an empty list of external IPs", spec: map[string]any{"type": "ExternalName", "externalName": userInstance, "externalIPs": []any{}}, allowed: true},
				{name: "an ExternalName to another Service of the cluster", spec: map[string]any{"type": "ExternalName", "externalName": "kubernetes.default.svc.cluster.local"}},
				{name: "an ExternalName to dex itself", spec: map[string]any{"type": "ExternalName", "externalName": "dex.d8-user-authn.svc.cluster.local"}},
				{name: "an ExternalName out of the cluster", spec: map[string]any{"type": "ExternalName", "externalName": "attacker.example.com"}},
				{name: "an ExternalName to an instance under another domain", spec: map[string]any{"type": "ExternalName", "externalName": "dex-authenticator.d8-user-authn.svc.example.com"}},
				{name: "an ExternalName without a name", spec: map[string]any{"type": "ExternalName"}},
				{name: "an ExternalName with external IPs", spec: map[string]any{"type": "ExternalName", "externalName": userInstance, "externalIPs": []any{"192.0.2.1"}}},
				{name: "a ClusterIP Service", spec: map[string]any{"type": "ClusterIP", "selector": map[string]any{"app": "shop"}}},
				{name: "a NodePort Service", spec: map[string]any{"type": "NodePort"}},
				{name: "a LoadBalancer Service", spec: map[string]any{"type": "LoadBalancer"}},
				{name: "a Service without a type", spec: map[string]any{}},
				{name: "a LoadBalancer Service of a user", username: "editor@example.com", spec: map[string]any{"type": "LoadBalancer"}, allowed: true},
			} {
				username := tc.username
				if username == "" {
					username = dexClientControllerServiceAccount
				}
				for _, operation := range []admission.Operation{admission.Create, admission.Update} {
					By(tc.name + ": " + string(operation))
					old := map[string]string(nil)
					if operation == admission.Update {
						old = own
					}
					allowed, err := objectWriteAdmitted(policy, username, service, "services", operation, old, own, tc.spec)
					Expect(err).ToNot(HaveOccurred())
					Expect(allowed).To(Equal(tc.allowed))
				}
			}

			// The spec of the Service that goes away is not looked at, and neither is the spec of
			// a route: the condition is about the Services the controller writes.
			allowed, err := objectWriteAdmitted(policy, dexClientControllerServiceAccount, service, "services", admission.Delete, own, nil, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
			ingress := schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"}
			allowed, err = objectWriteAdmitted(policy, dexClientControllerServiceAccount, ingress, "ingresses", admission.Create, nil, own, map[string]any{"ingressClassName": "nginx"})
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
		})
	})

	Context("while the module renders the shared instances in a cluster with the Gateway API and other settings", func() {
		BeforeEach(func() {
			setValues()
			hec.ValuesSet("global.modules.https.mode", "OnlyInURI")
			hec.ValuesSet("global.discovery.ingressNginxAnnotationsVersion", "1.5.0")
			hec.ValuesSet("userAuthn.idTokenTTL", "2h30m")
			hec.ValuesSet("userAuthn.sessions.enabled", true)
			hec.ValuesSet("userAuthn.sessions.sso", "None")
			hec.HelmRender(WithAPIVersions(append(policyAPIVersions, httpRouteAPIVersion)...))
		})

		It("tells the reconcilers about the Gateway API and the settings", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			Expect(controllerEnv()).To(HaveKeyWithValue("SHARED_DEX_AUTHENTICATOR", "true"))
			Expect(controllerEnv()).To(HaveKeyWithValue("GATEWAY_API", "true"))
			cluster, _ := grants("httproutes")
			Expect(cluster).To(ConsistOf("get", "list", "watch", "create", "update", "delete"))

			Expect(settings().String()).To(MatchJSON(`{
				"clusterDomain": "cluster.local",
				"httpsMode": "OnlyInURI",
				"ingressEnabled": true,
				"ingressNginxHSTSAnnotation": true,
				"gatewayAPI": true,
				"dexSessions": true,
				"platformSSO": false,
				"idTokenTTL": "2h30m"
			}`))
		})
	})

	Context("while the module renders the shared instances in a cluster with ReferenceGrant objects", func() {
		BeforeEach(func() {
			setValues()
			hec.HelmRender(WithAPIVersions(append(policyAPIVersions, httpRouteAPIVersion, referenceGrantAPIVersion)...))
		})

		It("lets dexclient-controller keep the ReferenceGrants of d8-user-authn and nothing else", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			Expect(controllerEnv()).To(HaveKeyWithValue("REFERENCE_GRANTS", "true"))
			cluster, namespaced := grants("referencegrants")
			Expect(cluster).To(BeEmpty(), "the grants live in the module namespace only")
			Expect(namespaced).To(ConsistOf("list", "watch", "create", "update", "delete"))

			rendered := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", dexClientControllerPolicyName)
			Expect(rendered.Exists()).To(BeTrue())
			resources := map[string]string{}
			for _, rule := range rendered.Field("spec.matchConstraints.resourceRules").Array() {
				resources[rule.Get("resources.0").String()] = rule.Get("apiGroups.0").String()
			}
			Expect(resources).To(HaveKeyWithValue("referencegrants", "gateway.networking.k8s.io"))

			policy := compileRenderedPolicy(rendered.ToYaml())
			own := map[string]string{managedByLabel: managedByController, "app": "dex-authenticator-reference-grant"}
			other := map[string]string{"app": "shop"}
			referenceGrant := schema.GroupVersionKind{Group: "gateway.networking.k8s.io", Version: "v1", Kind: "ReferenceGrant"}
			from := func(kind string) []any {
				return []any{map[string]any{"group": "gateway.networking.k8s.io", "kind": kind, "namespace": "shop"}}
			}
			to := func(names ...string) []any {
				out := make([]any, 0, len(names))
				for _, name := range names {
					out = append(out, map[string]any{"group": "", "kind": "Service", "name": name})
				}
				return out
			}

			for _, tc := range []struct {
				name    string
				labels  map[string]string
				spec    map[string]any
				allowed bool
			}{
				{name: "a grant to the instance of the user namespaces", labels: own, spec: map[string]any{"from": from("HTTPRoute"), "to": to("dex-authenticator")}, allowed: true},
				{name: "a grant to both instances", labels: own, spec: map[string]any{"from": from("HTTPRoute"), "to": to("dex-authenticator", "dex-authenticator-system")}, allowed: true},
				{name: "a grant to dex", labels: own, spec: map[string]any{"from": from("HTTPRoute"), "to": to("dex")}},
				{name: "a grant to every Service", labels: own, spec: map[string]any{"from": from("HTTPRoute"), "to": []any{map[string]any{"group": "", "kind": "Service"}}}},
				{name: "a grant to a Secret", labels: own, spec: map[string]any{"from": from("HTTPRoute"), "to": []any{map[string]any{"group": "", "kind": "Secret", "name": "dex-authenticator"}}}},
				{name: "a grant from a Gateway", labels: own, spec: map[string]any{"from": from("Gateway"), "to": to("dex-authenticator")}},
				{name: "a grant without targets", labels: own, spec: map[string]any{"from": from("HTTPRoute")}},
				{name: "a grant without the label", labels: other, spec: map[string]any{"from": from("HTTPRoute"), "to": to("dex-authenticator")}},
			} {
				for _, operation := range []admission.Operation{admission.Create, admission.Update} {
					By(tc.name + ": " + string(operation))
					old := map[string]string(nil)
					if operation == admission.Update {
						old = own
					}
					allowed, err := objectWriteAdmitted(policy, dexClientControllerServiceAccount, referenceGrant, "referencegrants", operation, old, tc.labels, tc.spec)
					Expect(err).ToNot(HaveOccurred())
					Expect(allowed).To(Equal(tc.allowed))
				}
			}
			allowed, err := objectWriteAdmitted(policy, dexClientControllerServiceAccount, referenceGrant, "referencegrants", admission.Delete, own, nil, nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowed).To(BeTrue())
		})
	})

	Context("while the module renders the shared instances in a cluster that serves ReferenceGrant as v1beta1 only", func() {
		BeforeEach(func() {
			setValues()
			hec.HelmRender(WithAPIVersions(append(policyAPIVersions, httpRouteAPIVersion, referenceGrantV1beta1APIVersion)...))
		})

		It("keeps no ReferenceGrant: the controller writes the v1 one", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			Expect(controllerEnv()).NotTo(HaveKey("REFERENCE_GRANTS"))
			cluster, namespaced := grants("referencegrants")
			Expect(cluster).To(BeEmpty())
			Expect(namespaced).To(BeEmpty())

			rendered := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", dexClientControllerPolicyName)
			Expect(rendered.Exists()).To(BeTrue())
			for _, rule := range rendered.Field("spec.matchConstraints.resourceRules").Array() {
				Expect(rule.Get("resources.0").String()).NotTo(Equal("referencegrants"))
			}
		})
	})

	Context("while the module renders the shared instances in a cluster without the admission policy kinds", func() {
		BeforeEach(func() {
			setValues()
			hec.HelmRender(WithAPIVersions(httpRouteAPIVersion))
		})

		It("gives dexclient-controller no right to write what the policy would hold", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", dexClientControllerPolicyName).Exists()).To(BeFalse())
			for _, resource := range []string{"services", "ingresses", "httproutes"} {
				cluster, namespaced := grants(resource)
				Expect(cluster).To(ConsistOf("get", "list", "watch"), resource)
				Expect(namespaced).To(BeEmpty(), resource)
			}
		})
	})

	Context("while the module renders the shared instances with the Gateway API and Ingress turned off in the settings", func() {
		BeforeEach(func() {
			setValues()
			hec.ValuesSet("global.modules.gatewayAPI.enabled", false)
			hec.ValuesSet("global.modules.ingress.enabled", false)
			hec.HelmRender(WithAPIVersions(append(policyAPIVersions, httpRouteAPIVersion)...))
		})

		It("lets the reconcilers remove the HTTPRoutes they made and tells them to make no new one", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			// The kind is in the cluster: the routes made before the setting was turned off are still
			// watched and can be deleted.
			Expect(controllerEnv()).To(HaveKeyWithValue("GATEWAY_API", "true"))
			cluster, _ := grants("httproutes")
			Expect(cluster).To(ContainElement("delete"))
			Expect(settings().Get("gatewayAPI").Bool()).To(BeFalse())
			Expect(settings().Get("ingressEnabled").Bool()).To(BeFalse())
		})
	})
})
