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
	"context"

	celgo "github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	admissioncel "k8s.io/apiserver/pkg/admission/plugin/cel"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/cel/environment"
	"sigs.k8s.io/yaml"

	. "github.com/deckhouse/deckhouse/testing/helm"
)

const (
	validatingAdmissionPolicyAPIVersion        = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy"
	validatingAdmissionPolicyBindingAPIVersion = "admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding"

	allowAccessPolicyName = "dex-allow-access-to-kubernetes.deckhouse.io"

	dexClientAnnotation        = "dexclient.deckhouse.io/allow-access-to-kubernetes"
	dexAuthenticatorAnnotation = "dexauthenticator.deckhouse.io/allow-access-to-kubernetes"

	deckhouseServiceAccount = "system:serviceaccount:d8-system:deckhouse"
	namespaceEditor         = "editor@example.com"
)

var _ = Describe("Module :: user-authn :: helm template :: validation", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		hec.ValuesSet("global.discovery.kubernetesVersion", "1.32.0")
		hec.ValuesSet("global.modules.publicDomainTemplate", "%s.example.com")
		hec.ValuesSet("global.modules.https.mode", "CertManager")
		hec.ValuesSet("global.modules.https.certManager.clusterIssuerName", "letsencrypt")
		hec.ValuesSet("global.modulesImages.registry.base", "registry.example.com")
		hec.ValuesSet("global.enabledModules", []string{"cert-manager", "vertical-pod-autoscaler"})
		hec.ValuesSet("global.discovery.d8SpecificNodeCountByRole.system", 2)
		hec.ValuesSet("global.discovery.kubernetesCA", "plainstring")

		hec.ValuesSet("userAuthn.internal.kubernetesDexClientAppSecret", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.crt", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.key", "plainstring")
		hec.ValuesSet("userAuthn.internal.dexTLS.ca", "plainstring")
	})

	Context("Without ValidatingAdmissionPolicy support in the cluster", func() {
		BeforeEach(func() {
			hec.HelmRender()
		})

		It("Should not render the policies", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())
			Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", allowAccessPolicyName).Exists()).To(BeFalse())
			Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", allowAccessPolicyName).Exists()).To(BeFalse())
			Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", "system-users.deckhouse.io").Exists()).To(BeFalse())
		})
	})

	Context("With ValidatingAdmissionPolicy support in the cluster", func() {
		BeforeEach(func() {
			hec.HelmRender(WithAPIVersions(
				validatingAdmissionPolicyAPIVersion,
				validatingAdmissionPolicyBindingAPIVersion,
			))
		})

		It("Should render the policy gating the allow-access-to-kubernetes annotation", func() {
			Expect(hec.RenderError).ShouldNot(HaveOccurred())

			policy := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", allowAccessPolicyName)
			Expect(policy.Exists()).To(BeTrue())
			Expect(policy.Field("apiVersion").String()).To(Equal("admissionregistration.k8s.io/v1"))
			Expect(policy.Field("metadata.labels.heritage").String()).To(Equal("deckhouse"))
			Expect(policy.Field("metadata.labels.module").String()).To(Equal("user-authn"))
			Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))

			rule := policy.Field("spec.matchConstraints.resourceRules.0")
			Expect(rule.Get("apiGroups").String()).To(MatchJSON(`["deckhouse.io"]`))
			Expect(rule.Get("apiVersions").String()).To(MatchJSON(`["*"]`))
			Expect(rule.Get("operations").String()).To(MatchJSON(`["CREATE","UPDATE"]`))
			Expect(rule.Get("resources").String()).To(MatchJSON(`["dexclients","dexauthenticators"]`))

			Expect(policy.Field("spec.validations.0.reason").String()).To(Equal("Forbidden"))
			Expect(policy.Field("spec.validations.0.messageExpression").String()).To(
				ContainSubstring("grants access to the Kubernetes API"),
			)
			Expect(policy.Field("spec.validations.0.messageExpression").String()).To(
				ContainSubstring("must be applied by a cluster administrator"),
			)

			binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", allowAccessPolicyName)
			Expect(binding.Exists()).To(BeTrue())
			Expect(binding.Field("apiVersion").String()).To(Equal("admissionregistration.k8s.io/v1"))
			Expect(binding.Field("spec.policyName").String()).To(Equal(allowAccessPolicyName))
			Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny"]`))
		})

		It("Should keep rendering the pre-existing user validation policy", func() {
			Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", "system-users.deckhouse.io").Exists()).To(BeTrue())
			Expect(hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", "system-users.deckhouse.io").Exists()).To(BeTrue())
		})

		It("Should evaluate the rendered CEL expressions as intended", func() {
			policy := compileRenderedPolicy(
				hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", allowAccessPolicyName).ToYaml(),
			)

			cases := []struct {
				name           string
				resource       string
				kind           string
				operation      admission.Operation
				username       string
				canManage      bool
				annotations    map[string]string
				oldAnnotations map[string]string
				allowed        bool
			}{
				{
					name:        "unauthorised subject grants access on create",
					annotations: map[string]string{dexClientAnnotation: "true"},
					allowed:     false,
				},
				{
					name:        "unauthorised subject creates a client without the annotation",
					annotations: nil,
					allowed:     true,
				},
				{
					name:        "annotation explicitly disabled",
					annotations: map[string]string{dexClientAnnotation: "false"},
					allowed:     true,
				},
				{
					name:        "annotation value is not a boolean the hooks would honour",
					annotations: map[string]string{dexClientAnnotation: "yes"},
					allowed:     true,
				},
				{
					name:        "truthy value in another casing",
					annotations: map[string]string{dexClientAnnotation: "True"},
					allowed:     false,
				},
				{
					name:        "truthy value as a single character",
					annotations: map[string]string{dexClientAnnotation: "t"},
					allowed:     false,
				},
				{
					name:        "truthy value as a digit",
					annotations: map[string]string{dexClientAnnotation: "1"},
					allowed:     false,
				},
				{
					name:        "deckhouse service account grants access",
					username:    deckhouseServiceAccount,
					annotations: map[string]string{dexClientAnnotation: "true"},
					allowed:     true,
				},
				{
					name:        "subject allowed to update the user-authn ModuleConfig grants access",
					canManage:   true,
					annotations: map[string]string{dexClientAnnotation: "true"},
					allowed:     true,
				},
				{
					name:           "existing grant is carried over unchanged",
					operation:      admission.Update,
					annotations:    map[string]string{dexClientAnnotation: "true"},
					oldAnnotations: map[string]string{dexClientAnnotation: "true"},
					allowed:        true,
				},
				{
					name:           "existing grant is rewritten with an equivalent value",
					operation:      admission.Update,
					annotations:    map[string]string{dexClientAnnotation: "1"},
					oldAnnotations: map[string]string{dexClientAnnotation: "true"},
					allowed:        true,
				},
				{
					name:           "annotation is added by an update",
					operation:      admission.Update,
					annotations:    map[string]string{dexClientAnnotation: "true"},
					oldAnnotations: nil,
					allowed:        false,
				},
				{
					name:           "annotation is flipped from false to true",
					operation:      admission.Update,
					annotations:    map[string]string{dexClientAnnotation: "true"},
					oldAnnotations: map[string]string{dexClientAnnotation: "false"},
					allowed:        false,
				},
				{
					name:           "annotation is flipped from true to false",
					operation:      admission.Update,
					annotations:    map[string]string{dexClientAnnotation: "false"},
					oldAnnotations: map[string]string{dexClientAnnotation: "true"},
					allowed:        true,
				},
				{
					name:           "annotation is dropped by an update",
					operation:      admission.Update,
					annotations:    nil,
					oldAnnotations: map[string]string{dexClientAnnotation: "true"},
					allowed:        true,
				},
				{
					name:        "dex authenticator grants access on create",
					resource:    "dexauthenticators",
					kind:        "DexAuthenticator",
					annotations: map[string]string{dexAuthenticatorAnnotation: "true"},
					allowed:     false,
				},
				{
					name:           "dex authenticator keeps an existing grant",
					resource:       "dexauthenticators",
					kind:           "DexAuthenticator",
					operation:      admission.Update,
					annotations:    map[string]string{dexAuthenticatorAnnotation: "true"},
					oldAnnotations: map[string]string{dexAuthenticatorAnnotation: "true"},
					allowed:        true,
				},
				{
					name:        "annotation of the other kind is not gated",
					annotations: map[string]string{dexAuthenticatorAnnotation: "true"},
					allowed:     true,
				},
			}

			for _, tc := range cases {
				By(tc.name)

				resource, kind := tc.resource, tc.kind
				if resource == "" {
					resource, kind = "dexclients", "DexClient"
				}
				operation := tc.operation
				if operation == "" {
					operation = admission.Create
				}
				username := tc.username
				if username == "" {
					username = namespaceEditor
				}

				allowed, err := policy.validate(dexAdmissionInput{
					resource:       resource,
					kind:           kind,
					operation:      operation,
					username:       username,
					canManage:      tc.canManage,
					annotations:    tc.annotations,
					oldAnnotations: tc.oldAnnotations,
				})

				Expect(err).ToNot(HaveOccurred(), tc.name)
				Expect(allowed).To(Equal(tc.allowed), tc.name)
			}
		})
	})
})

// compiledPolicy evaluates the validation expressions of a ValidatingAdmissionPolicy through the
// same CEL machinery kube-apiserver uses, so the expressions the module ships are under test.
type compiledPolicy struct {
	validations admissioncel.ConditionEvaluator
}

func compileRenderedPolicy(renderedPolicy string) compiledPolicy {
	// Strict decoding also asserts that the rendered manifest carries no field the API does not know.
	var policy admissionregistrationv1.ValidatingAdmissionPolicy
	Expect(yaml.UnmarshalStrict([]byte(renderedPolicy), &policy)).To(Succeed())

	// kube-apiserver declares the authorizer for validations but not for message expressions.
	validationVars := admissioncel.OptionalVariableDeclarations{HasAuthorizer: true, StrictCost: true}
	messageVars := admissioncel.OptionalVariableDeclarations{StrictCost: true}

	compiler, err := admissioncel.NewCompositedCompiler(
		environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion(), true),
	)
	Expect(err).ToNot(HaveOccurred())

	// Variables come first: expressions referring to them are type-checked against the types
	// collected here.
	for _, variable := range policy.Spec.Variables {
		result := compiler.CompileAndStoreVariable(
			namedExpression{name: variable.Name, expression: variable.Expression},
			validationVars, environment.StoredExpressions,
		)
		Expect(result.Error).To(BeNil(), "variable %q should compile", variable.Name)
	}

	validations := make([]admissioncel.ExpressionAccessor, 0, len(policy.Spec.Validations))
	for _, validation := range policy.Spec.Validations {
		condition := boolExpression{expression: validation.Expression}
		Expect(compiler.CompileCELExpression(condition, validationVars, environment.StoredExpressions).Error).To(BeNil(),
			"validation %q should compile", validation.Expression)

		message := stringExpression{expression: validation.MessageExpression}
		Expect(compiler.CompileCELExpression(message, messageVars, environment.StoredExpressions).Error).To(BeNil(),
			"message expression %q should compile", validation.MessageExpression)

		validations = append(validations, condition)
	}

	return compiledPolicy{
		validations: compiler.CompileCondition(validations, validationVars, environment.StoredExpressions),
	}
}

type dexAdmissionInput struct {
	resource       string
	kind           string
	operation      admission.Operation
	username       string
	canManage      bool
	annotations    map[string]string
	oldAnnotations map[string]string
}

// validate reports whether every validation of the policy admits the request.
func (p compiledPolicy) validate(input dexAdmissionInput) (bool, error) {
	const (
		namespace = "attacker-ns"
		name      = "app"
	)

	gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: input.kind}
	gvr := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: input.resource}

	// The old object has to stay a nil interface on create: a typed nil pointer is treated as
	// present by the admission machinery.
	var oldObject runtime.Object
	if input.operation == admission.Update {
		oldObject = dexObject(gvk, namespace, name, input.oldAnnotations)
	}

	attributes := admission.NewAttributesRecord(
		dexObject(gvk, namespace, name, input.annotations), oldObject,
		gvk, namespace, name, gvr, "", input.operation, nil, false,
		&user.DefaultInfo{Name: input.username, Groups: []string{"system:authenticated"}},
	)
	versionedAttributes, err := admission.NewVersionedAttributes(attributes, gvk, nil)
	if err != nil {
		return false, err
	}

	results, _, err := p.validations.ForInput(
		context.Background(), versionedAttributes,
		admissioncel.CreateAdmissionRequest(attributes, metav1.GroupVersionResource(gvr), metav1.GroupVersionKind(gvk)),
		admissioncel.OptionalVariableBindings{Authorizer: userAuthnModuleConfigAuthorizer(input.canManage)},
		nil, celconfig.RuntimeCELCostBudget,
	)
	if err != nil {
		return false, err
	}

	for _, result := range results {
		if result.Error != nil {
			return false, result.Error
		}
		if result.EvalResult != celtypes.True {
			return false, nil
		}
	}

	return true, nil
}

func dexObject(gvk schema.GroupVersionKind, namespace, name string, annotations map[string]string) *unstructured.Unstructured {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(gvk)
	object.SetNamespace(namespace)
	object.SetName(name)
	object.SetAnnotations(annotations)
	object.Object["spec"] = map[string]interface{}{
		"redirectURIs": []interface{}{"https://app.example.com/callback"},
	}

	return object
}

// userAuthnModuleConfigAuthorizer mimics RBAC for a subject that either holds or lacks the right to
// update the user-authn ModuleConfig, and holds nothing else.
func userAuthnModuleConfigAuthorizer(canManage bool) authorizer.Authorizer {
	return authorizerFunc(func(attributes authorizer.Attributes) bool {
		return canManage &&
			attributes.GetAPIGroup() == "deckhouse.io" &&
			attributes.GetResource() == "moduleconfigs" &&
			attributes.GetName() == "user-authn" &&
			attributes.GetVerb() == "update"
	})
}

type authorizerFunc func(authorizer.Attributes) bool

func (f authorizerFunc) Authorize(_ context.Context, attributes authorizer.Attributes) (authorizer.Decision, string, error) {
	if f(attributes) {
		return authorizer.DecisionAllow, "", nil
	}

	return authorizer.DecisionNoOpinion, "", nil
}

type namedExpression struct {
	name       string
	expression string
}

func (e namedExpression) GetName() string       { return e.name }
func (e namedExpression) GetExpression() string { return e.expression }
func (e namedExpression) ReturnTypes() []*celgo.Type {
	return []*celgo.Type{celgo.AnyType, celgo.DynType}
}

type boolExpression struct {
	expression string
}

func (e boolExpression) GetExpression() string      { return e.expression }
func (e boolExpression) ReturnTypes() []*celgo.Type { return []*celgo.Type{celgo.BoolType} }

type stringExpression struct {
	expression string
}

func (e stringExpression) GetExpression() string      { return e.expression }
func (e stringExpression) ReturnTypes() []*celgo.Type { return []*celgo.Type{celgo.StringType} }
