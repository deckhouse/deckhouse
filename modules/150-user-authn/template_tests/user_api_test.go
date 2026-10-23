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
	"encoding/base64"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/tidwall/gjson"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	admissioncel "k8s.io/apiserver/pkg/admission/plugin/cel"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/cel/environment"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	userAPIPolicyName     = "user-api-self-service.deckhouse.io"
	userAPIServiceAccount = "system:serviceaccount:d8-user-authn:user-api"
	userAPIImage          = "registry.example.com@imageHash-userAuthn-userApi"
)

var _ = Describe("Module :: user-authn :: helm template :: user-api", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.33.0")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "dex-crt")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "dex-key")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "dex-ca")
		hec.ValuesSet("userAuthn.internal.userAPITLS.crt", "user-api-crt")
		hec.ValuesSet("userAuthn.internal.userAPITLS.key", "user-api-key")
		hec.ValuesSet("userAuthn.internal.userAPITLS.ca", "user-api-ca")
	})

	// devices.trustedAfter is one setting for user-api (the device age of the self-service actions), the
	// controller (the trusted devices kept first in a cut list) and Dex (the untrusted devices evicted first).
	DescribeTable("Should trust the devices of users after devices.trustedAfter everywhere",
		func(trustedAfter, want string) {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapProvider)
			if trustedAfter != "" {
				hec.ValuesSet("userAuthn.devices.trustedAfter", trustedAfter)
			}
			hec.HelmRender()
			Expect(hec.RenderError).ToNot(HaveOccurred())

			userAPI := hec.KubernetesResource("Deployment", "d8-user-authn", "user-api")
			Expect(userAPI.Field(`spec.template.spec.containers.#(name=="user-api").args`).AsStringSlice()).
				To(ContainElement("--min-device-age=" + want))
			controller := hec.KubernetesResource("Deployment", "d8-user-authn", "user-authn-controller")
			Expect(controller.Field(`spec.template.spec.containers.0.env.#(name=="MIN_DEVICE_AGE").value`).String()).To(Equal(want))
			Expect(gjson.GetBytes(renderedDexConfig(hec), "devices.trustedAfter").String()).To(Equal(want))
		},
		Entry("a day by default", "", "24h"),
		Entry("an hour and a half", "1h30m", "1h30m"),
		Entry("at once", "0s", "0s"),
	)

	// The limits of devices, the life of an unused refresh token and the retention of the records reach Dex and the
	// controller.
	DescribeTable("Should pass the limits and the retention on",
		func(settings map[string]string, want map[string]string) {
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapProvider)
			for path, value := range settings {
				hec.ValuesSetFromYaml("userAuthn."+path, value)
			}
			hec.HelmRender()
			Expect(hec.RenderError).ToNot(HaveOccurred())

			dex := renderedDexConfig(hec)
			controller := hec.KubernetesResource("Deployment", "d8-user-authn", "user-authn-controller")
			env := func(name string) string {
				return controller.Field(`spec.template.spec.containers.0.env.#(name=="` + name + `").value`).String()
			}
			Expect(gjson.GetBytes(dex, "devices.maxPerClient").String()).To(Equal(want["maxPerClient"]))
			Expect(gjson.GetBytes(dex, "devices.maxKnown").String()).To(Equal(want["maxKnown"]))
			Expect(gjson.GetBytes(dex, "expiry.refreshTokens.validIfNotUsedFor").String()).To(Equal(want["refreshIdle"]))
			Expect(env("USER_OPERATION_RETENTION")).To(Equal(want["operations"]))
			Expect(env("REVOCATION_RETENTION")).To(Equal(want["revocations"]))
		},
		Entry("the defaults", map[string]string{}, map[string]string{
			"maxPerClient": "10", "maxKnown": "20", "refreshIdle": "2190h", "operations": "24h", "revocations": "",
		}),
		Entry("set", map[string]string{
			"refreshTokenValidIfNotUsedFor": "720h",
			"devices":                       "{maxPerApplication: 3, rememberedBrowsers: 0}",
			"retention":                     "{userOperations: 168h, revocations: 24h}",
		}, map[string]string{
			"maxPerClient": "3", "maxKnown": "0", "refreshIdle": "720h", "operations": "168h", "revocations": "24h",
		}),
	)

	Context("Without static users and without ValidatingAdmissionPolicy support", func() {
		BeforeEach(func() {
			// Only external providers: user-api serves them as well.
			hec.ValuesSetFromYaml("userAuthn.internal.providers", ldapProvider)
			hec.HelmRender()
		})

		It("Should deploy user-api all the same", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.ValuesGet("userAuthn.internal.dexUsersCRDs").Array()).To(BeEmpty())

			Expect(hec.KubernetesResource("Deployment", "d8-user-authn", "user-api").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Service", "d8-user-authn", "user-api").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ServiceAccount", "d8-user-authn", "user-api").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("PodDisruptionBudget", "d8-user-authn", "user-api").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("Secret", "d8-user-authn", "user-api-tls").Exists()).To(BeTrue())
			Expect(hec.KubernetesResource("ConfigMap", "d8-user-authn", "user-api-ca").Exists()).To(BeTrue())
			Expect(hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:user-api").Exists()).To(BeTrue())
			Expect(hec.KubernetesGlobalResource("ClusterRoleBinding", "d8:user-authn:user-api").Exists()).To(BeTrue())
		})

		It("Should run user-api with the certificate from the hook and without an init container", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			deployment := hec.KubernetesResource("Deployment", "d8-user-authn", "user-api")
			Expect(deployment.Field("spec.template.spec.initContainers").Exists()).To(BeFalse())
			Expect(deployment.Field("spec.template.metadata.annotations.checksum/tls").String()).ToNot(BeEmpty())

			container := deployment.Field(`spec.template.spec.containers.#(name=="user-api")`)
			Expect(container.Get("image").String()).To(Equal(userAPIImage))
			args := deployment.Field(`spec.template.spec.containers.#(name=="user-api").args`).AsStringSlice()
			Expect(args).To(ContainElements(
				"--tls-cert-file=/certs/tls.crt",
				"--tls-key-file=/certs/tls.key",
				"--dex-url=https://dex.d8-user-authn.svc",
				"--dex-ca-file=/certs/dex-ca.crt",
				"--console-client-id=console-d8-console-dex-authenticator",
			))
			Expect(container.Get(`volumeMounts.#(name=="tls").mountPath`).String()).To(Equal("/certs"))
			Expect(deployment.Field(`spec.template.spec.volumes.#(name=="tls").secret.secretName`).String()).To(Equal("user-api-tls"))
			Expect(deployment.Field(`spec.template.spec.volumes.#(name=="tls").emptyDir`).Exists()).To(BeFalse())
		})

		It("Should put the certificate and the CA of Dex into the Secret", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			secret := hec.KubernetesResource("Secret", "d8-user-authn", "user-api-tls")
			Expect(secret.Field("type").String()).To(Equal("kubernetes.io/tls"))
			for key, want := range map[string]string{
				`tls\.crt`:    "user-api-crt",
				`tls\.key`:    "user-api-key",
				`ca\.crt`:     "user-api-ca",
				`dex-ca\.crt`: "dex-ca",
			} {
				Expect(secret.Field("data."+key).String()).To(Equal(base64.StdEncoding.EncodeToString([]byte(want))), key)
			}
		})

		It("Should publish the CA of user-api to every service account", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			configMap := hec.KubernetesResource("ConfigMap", "d8-user-authn", "user-api-ca")
			Expect(configMap.Field(`data.ca\.crt`).String()).To(Equal("user-api-ca"))
			Expect(configMap.Field("data").Map()).To(HaveLen(1))

			role := hec.KubernetesResource("Role", "d8-user-authn", "user-api-ca-reader")
			Expect(role.Exists()).To(BeTrue())
			Expect(role.Field("rules").String()).To(MatchJSON(`[{
				"apiGroups": [""],
				"resources": ["configmaps"],
				"resourceNames": ["user-api-ca"],
				"verbs": ["get"]
			}]`))

			roleBinding := hec.KubernetesResource("RoleBinding", "d8-user-authn", "user-api-ca-reader")
			Expect(roleBinding.Exists()).To(BeTrue())
			Expect(roleBinding.Field("roleRef.kind").String()).To(Equal("Role"))
			Expect(roleBinding.Field("roleRef.name").String()).To(Equal("user-api-ca-reader"))
			Expect(roleBinding.Field("subjects").String()).To(MatchJSON(`[{
				"apiGroup": "rbac.authorization.k8s.io",
				"kind": "Group",
				"name": "system:serviceaccounts"
			}]`))
		})

		It("Should let user-api create and read UserOperations, read a named UserAccount, and nothing more", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			clusterRole := hec.KubernetesGlobalResource("ClusterRole", "d8:user-authn:user-api")
			Expect(clusterRole.Field("rules").String()).To(MatchJSON(`[
				{"apiGroups": ["deckhouse.io"], "resources": ["useroperations"], "verbs": ["create", "get"]},
				{"apiGroups": ["deckhouse.io"], "resources": ["useraccounts"], "verbs": ["get"]},
				{"apiGroups": ["dex.coreos.com"], "resources": ["passwords"], "verbs": ["get", "list", "watch"]}
			]`))
		})

		It("Should not render the policy without its API", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())
			Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", userAPIPolicyName).Exists()).To(BeFalse())
			Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", userAPIPolicyName).Exists()).To(BeFalse())
		})
	})

	Context("With ValidatingAdmissionPolicy support", func() {
		BeforeEach(func() {
			hec.HelmRender(WithAPIVersions(
				validatingAdmissionPolicyAPIVersion,
				validatingAdmissionPolicyBindingAPIVersion,
			))
		})

		It("Should bind the policy for UserOperations of the user-api service account", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			policy := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", userAPIPolicyName)
			Expect(policy.Exists()).To(BeTrue())
			Expect(policy.Field("apiVersion").String()).To(Equal("admissionregistration.k8s.io/v1"))
			Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))

			rule := policy.Field("spec.matchConstraints.resourceRules.0")
			Expect(rule.Get("apiGroups").String()).To(MatchJSON(`["deckhouse.io"]`))
			Expect(rule.Get("operations").String()).To(MatchJSON(`["CREATE"]`))
			Expect(rule.Get("resources").String()).To(MatchJSON(`["useroperations"]`))
			Expect(policy.Field("spec.matchConditions.0.expression").String()).To(ContainSubstring(userAPIServiceAccount))

			binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", userAPIPolicyName)
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("spec.policyName").String()).To(Equal(userAPIPolicyName))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))
		})

		type object = map[string]interface{}
		admissions := []struct {
			name        string
			username    string
			spec        object
			wantAllowed bool
		}{
			{
				name:        "Logout of a local user",
				username:    userAPIServiceAccount,
				spec:        object{"type": "Logout", "user": "jane", "initiatorType": "self"},
				wantAllowed: true,
			},
			{
				name:     "Logout of an external account",
				username: userAPIServiceAccount,
				spec: object{"type": "Logout", "initiatorType": "self", "target": object{
					"email": "jdoe@example.org", "connectorID": "ldap", "userID": "cn=jdoe,ou=people,dc=example,dc=org",
				}},
				wantAllowed: true,
			},
			{
				name:        "RequirePasswordChange of a local user",
				username:    userAPIServiceAccount,
				spec:        object{"type": "RequirePasswordChange", "user": "jane", "initiatorType": "self"},
				wantAllowed: true,
			},
			{
				name:     "ResetPassword",
				username: userAPIServiceAccount,
				spec: object{"type": "ResetPassword", "user": "jane", "initiatorType": "self",
					"resetPassword": object{"newPasswordHash": "$2a$10$x"}},
			},
			{
				name:     "Lock",
				username: userAPIServiceAccount,
				spec: object{"type": "Lock", "user": "jane", "initiatorType": "self",
					"lock": object{"for": "permanent"}},
			},
			{
				name:     "Unlock",
				username: userAPIServiceAccount,
				spec:     object{"type": "Unlock", "user": "jane", "initiatorType": "self"},
			},
			{
				name:     "Reset2FA",
				username: userAPIServiceAccount,
				spec:     object{"type": "Reset2FA", "user": "jane", "initiatorType": "self"},
			},
			{
				name:     "Logout on behalf of an administrator",
				username: userAPIServiceAccount,
				spec:     object{"type": "Logout", "user": "jane", "initiatorType": "admin"},
			},
			{
				name:     "Logout without an initiator",
				username: userAPIServiceAccount,
				spec:     object{"type": "Logout", "user": "jane"},
			},
			{
				name:     "Logout of one browser",
				username: userAPIServiceAccount,
				spec: object{"type": "Logout", "user": "jane", "initiatorType": "self",
					"logout": object{"session": "fehzwdh5qy4oyqcn7qhkcgzbm"}},
				wantAllowed: true,
			},
			{
				name:     "Logout of one device without a browser",
				username: userAPIServiceAccount,
				spec: object{"type": "Logout", "user": "jane", "initiatorType": "self",
					"logout": object{"grant": "fehzwdh5qy4oyqcn7qhkcgzbm"}},
				wantAllowed: true,
			},
			{
				name:     "Logout of the other devices",
				username: userAPIServiceAccount,
				spec: object{"type": "Logout", "user": "jane", "initiatorType": "self",
					"logout": object{"exceptSession": "fehzwdh5qy4oyqcn7qhkcgzbm"}},
				wantAllowed: true,
			},
			{
				name:     "Logout of the other devices from a device without a browser",
				username: userAPIServiceAccount,
				spec: object{"type": "Logout", "user": "jane", "initiatorType": "self",
					"logout": object{"exceptGrant": "fehzwdh5qy4oyqcn7qhkcgzbm"}},
				wantAllowed: true,
			},
			{
				name:     "RequirePasswordChange carrying a password hash",
				username: userAPIServiceAccount,
				spec: object{"type": "RequirePasswordChange", "user": "jane", "initiatorType": "self",
					"resetPassword": object{"newPasswordHash": "$2a$10$x"}},
			},
			{
				name:     "RequirePasswordChange carrying a lock",
				username: userAPIServiceAccount,
				spec: object{"type": "RequirePasswordChange", "user": "jane", "initiatorType": "self",
					"lock": object{"for": "1h"}},
			},
			{
				name:     "Lock by an administrator is not the policy's business",
				username: "admin@example.com",
				spec: object{"type": "Lock", "user": "jane", "initiatorType": "admin",
					"lock": object{"for": "permanent"}},
				wantAllowed: true,
			},
			{
				name:        "Lock by another service account of user-authn is not the policy's business",
				username:    "system:serviceaccount:d8-user-authn:controller",
				spec:        object{"type": "Lock", "user": "jane", "lock": object{"for": "1h"}},
				wantAllowed: true,
			},
		}

		for _, admission := range admissions {
			It("Should admit only self-service UserOperations from user-api: "+admission.name, func() {
				Expect(hec.RenderError).ToNot(HaveOccurred())

				policy := compileUserAPIPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", userAPIPolicyName).ToYaml())
				allowed, err := policy.admitUserOperation(admission.username, admission.spec)
				Expect(err).ToNot(HaveOccurred())
				Expect(allowed).To(Equal(admission.wantAllowed))
			})
		}

		It("Should keep its expressions within what the API server checks", func() {
			Expect(hec.RenderError).ToNot(HaveOccurred())

			// Strict decoding asserts that the rendered manifest carries no field the API does not know.
			var policy admissionregistrationv1.ValidatingAdmissionPolicy
			Expect(yaml.UnmarshalStrict([]byte(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", userAPIPolicyName).ToYaml()), &policy)).To(Succeed())
			Expect(policy.Spec.Validations).To(HaveLen(3))
			for _, validation := range policy.Spec.Validations {
				Expect(validation.Reason).ToNot(BeNil())
				Expect(*validation.Reason).To(Equal(metav1.StatusReasonForbidden))
				Expect(validation.Message).ToNot(BeEmpty())
			}
		})
	})
})

// compileUserAPIPolicy compiles the policy the way compileRenderedPolicy does, for a policy with
// static messages and without variables or an authorizer.
func compileUserAPIPolicy(renderedPolicy string) compiledPolicy {
	// Strict decoding also asserts that the rendered manifest carries no field the API does not know.
	var policy admissionregistrationv1.ValidatingAdmissionPolicy
	Expect(yaml.UnmarshalStrict([]byte(renderedPolicy), &policy)).To(Succeed())
	Expect(policy.Spec.Variables).To(BeEmpty())

	// Pinned to the oldest supported Kubernetes, as in compileRenderedPolicy: with failurePolicy
	// Fail, an expression the oldest API server cannot evaluate would refuse every UserOperation of
	// user-api there.
	newCompiler := func() *admissioncel.CompositedCompiler {
		compiler, err := admissioncel.NewCompositedCompiler(
			environment.MustBaseEnvSet(minimalSupportedKubernetesVersion()),
		)
		Expect(err).ToNot(HaveOccurred())
		return compiler
	}
	vars := admissioncel.OptionalVariableDeclarations{}

	compile := func(compiler *admissioncel.CompositedCompiler, expressions []string) admissioncel.ConditionEvaluator {
		accessors := make([]admissioncel.ExpressionAccessor, 0, len(expressions))
		for _, expression := range expressions {
			accessor := boolExpression{expression: expression}
			Expect(compiler.CompileCELExpression(accessor, vars, environment.NewExpressions).Error).To(BeNil(),
				"expression %q should compile", expression)
			accessors = append(accessors, accessor)
		}
		return compiler.CompileCondition(accessors, vars, environment.NewExpressions)
	}

	matchConditions := make([]string, 0, len(policy.Spec.MatchConditions))
	for _, condition := range policy.Spec.MatchConditions {
		matchConditions = append(matchConditions, condition.Expression)
	}
	validations := make([]string, 0, len(policy.Spec.Validations))
	for _, validation := range policy.Spec.Validations {
		validations = append(validations, validation.Expression)
	}

	return compiledPolicy{
		matchConditions: compile(newCompiler(), matchConditions),
		validations:     compile(newCompiler(), validations),
	}
}

// admitUserOperation reports whether the policy admits the creation of a UserOperation with spec by
// username.
func (p compiledPolicy) admitUserOperation(username string, spec map[string]interface{}) (bool, error) {
	gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "UserOperation"}
	gvr := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "useroperations"}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)
	object.SetGenerateName("self-")
	object.Object["spec"] = spec

	attributes := admission.NewAttributesRecord(
		object, nil, gvk, "", "", gvr, "", admission.Create, nil, false,
		&user.DefaultInfo{Name: username, Groups: defaultGroups(username)},
	)
	versionedAttributes, err := admission.NewVersionedAttributes(attributes, gvk, nil)
	if err != nil {
		return false, err
	}

	request := admissioncel.CreateAdmissionRequest(
		attributes, metav1.GroupVersionResource(gvr), metav1.GroupVersionKind(gvk),
	)
	bindings := admissioncel.OptionalVariableBindings{}

	selected, err := p.evaluate(p.matchConditions, versionedAttributes, request, bindings)
	if err != nil || !selected {
		return true, err
	}

	return p.evaluate(p.validations, versionedAttributes, request, bindings)
}
