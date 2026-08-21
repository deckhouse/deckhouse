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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	celgo "github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/version"
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
	// A cluster provisioner running as a platform component. Deckhouse Commander patches DexClients
	// in its own namespace during cluster bootstrap, which is what makes this case load-bearing.
	commanderServiceAccount  = "system:serviceaccount:d8-commander:commander"
	kubeSystemServiceAccount = "system:serviceaccount:kube-system:some-controller"
	tenantServiceAccount     = "system:serviceaccount:attacker-ns:tenant"
	namespaceEditor          = "editor@example.com"

	allowAccessSubresource = "allow-access-to-kubernetes"
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
				ContainSubstring("controls access to the Kubernetes API"),
			)
			Expect(policy.Field("spec.validations.0.messageExpression").String()).To(
				ContainSubstring("allow-access-to-kubernetes subresource"),
			)

			// Platform components have to be excluded, otherwise the gate denies the provisioning
			// flows that create these objects on the cluster's behalf.
			matchConditions := policy.Field("spec.matchConditions").String()
			Expect(matchConditions).To(ContainSubstring("system:serviceaccount:d8-"))
			Expect(matchConditions).To(ContainSubstring("system:serviceaccount:kube-"))

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
				name             string
				resource         string
				kind             string
				operation        admission.Operation
				username         string
				groups           []string
				canGrant         bool
				grantedResource  string
				grantedNamespace string
				annotations      map[string]string
				oldAnnotations   map[string]string
				allowed          bool
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
					name:        "annotation explicitly disabled on create",
					annotations: map[string]string{dexClientAnnotation: "false"},
					allowed:     false,
				},
				{
					name:        "annotation value is not a boolean",
					annotations: map[string]string{dexClientAnnotation: "yes"},
					allowed:     false,
				},
				{
					name:        "annotation value is empty",
					annotations: map[string]string{dexClientAnnotation: ""},
					allowed:     false,
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
					// Cluster provisioners run as platform components and patch these objects during
					// bootstrap. Gating them buys nothing, and denying them breaks cluster creation.
					name:        "platform service account provisioning a client grants access",
					username:    commanderServiceAccount,
					annotations: map[string]string{dexClientAnnotation: "true"},
					allowed:     true,
				},
				{
					name:        "service account of a kube-prefixed namespace grants access",
					username:    kubeSystemServiceAccount,
					annotations: map[string]string{dexClientAnnotation: "true"},
					allowed:     true,
				},
				{
					name:        "subject of a system group grants access",
					groups:      []string{"system:authenticated", "system:serviceaccounts:d8-system"},
					annotations: map[string]string{dexClientAnnotation: "true"},
					allowed:     true,
				},
				{
					// A tenant's own service account is not a platform component: the namespace
					// prefix is what separates them, and this is the case the policy exists for.
					name:        "tenant service account grants access",
					username:    tenantServiceAccount,
					annotations: map[string]string{dexClientAnnotation: "true"},
					allowed:     false,
				},
				{
					name:        "subject holding the allow-access subresource grants access",
					canGrant:    true,
					annotations: map[string]string{dexClientAnnotation: "true"},
					allowed:     true,
				},
				{
					name:            "subresource granted on the other kind does not carry over",
					canGrant:        true,
					grantedResource: "dexauthenticators",
					annotations:     map[string]string{dexClientAnnotation: "true"},
					allowed:         false,
				},
				{
					name:             "subresource granted in another namespace does not carry over",
					canGrant:         true,
					grantedNamespace: "other-ns",
					annotations:      map[string]string{dexClientAnnotation: "true"},
					allowed:          false,
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
					name:           "annotation is flipped from a value no hook reads as true to true",
					operation:      admission.Update,
					annotations:    map[string]string{dexClientAnnotation: "true"},
					oldAnnotations: map[string]string{dexClientAnnotation: "yes"},
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
					name:           "object that already carries a disabled annotation stays editable",
					operation:      admission.Update,
					annotations:    map[string]string{dexClientAnnotation: "false"},
					oldAnnotations: map[string]string{dexClientAnnotation: "false"},
					allowed:        true,
				},
				{
					name:           "disabled annotation is added to an object that had none",
					operation:      admission.Update,
					annotations:    map[string]string{dexClientAnnotation: "false"},
					oldAnnotations: nil,
					allowed:        false,
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
					name:        "dex client carrying the dex authenticator annotation is not gated",
					annotations: map[string]string{dexAuthenticatorAnnotation: "true"},
					allowed:     true,
				},
				{
					name:        "dex authenticator carrying the dex client annotation is not gated",
					resource:    "dexauthenticators",
					kind:        "DexAuthenticator",
					annotations: map[string]string{dexClientAnnotation: "true"},
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
					resource:         resource,
					kind:             kind,
					operation:        operation,
					username:         username,
					groups:           tc.groups,
					canGrant:         tc.canGrant,
					grantedResource:  tc.grantedResource,
					grantedNamespace: tc.grantedNamespace,
					annotations:      tc.annotations,
					oldAnnotations:   tc.oldAnnotations,
				})

				Expect(err).ToNot(HaveOccurred(), tc.name)
				Expect(allowed).To(Equal(tc.allowed), tc.name)
			}
		})

		It("Should deny every annotation value that any hook generation reads as a grant", func() {
			policy := compileRenderedPolicy(
				hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", allowAccessPolicyName).ToYaml(),
			)

			for _, value := range annotationValueCorpus {
				granting := make([]string, 0, len(hookGenerations))
				for _, generation := range hookGenerations {
					if generation.grants(value) {
						granting = append(granting, generation.name)
					}
				}

				if len(granting) == 0 {
					continue
				}

				By(fmt.Sprintf("value %q is honoured as a grant by the %v hook", value, granting))

				annotations := map[string]string{dexClientAnnotation: value}

				denied, err := policy.validate(dexAdmissionInput{
					resource:    "dexclients",
					kind:        "DexClient",
					operation:   admission.Create,
					username:    namespaceEditor,
					annotations: annotations,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(denied).To(BeFalse(),
					"a subject without the allow-access-to-kubernetes subresource must not set %q", value)

				// The gate must stop the unauthorised subject only, otherwise the loop above would
				// pass just as well against a policy that denies everything.
				granted, err := policy.validate(dexAdmissionInput{
					resource:    "dexclients",
					kind:        "DexClient",
					operation:   admission.Create,
					username:    namespaceEditor,
					canGrant:    true,
					annotations: annotations,
				})
				Expect(err).ToNot(HaveOccurred())
				Expect(granted).To(BeTrue(),
					"a subject holding the allow-access-to-kubernetes subresource must be able to set %q", value)
			}
		})
	})
})

// hookGeneration models how a released version of the hooks decides that the annotation grants
// access to the Kubernetes API. The policy's soundness depends on it denying at least everything
// the hooks honour, so both sides are derived here instead of being restated in a comment: if a
// hook is ever widened, this list is the single place that has to change, and the assertions
// following from it start failing until the policy is widened too.
type hookGeneration struct {
	name   string
	grants func(value string) bool
}

var hookGenerations = []hookGeneration{
	{
		// modules/150-user-authn/hooks/get_dex_client_crds.go and get_dex_authenticator_crds.go
		// parse the value with strconv.ParseBool and fail closed on anything else.
		name: "value honouring",
		grants: func(value string) bool {
			granted, err := strconv.ParseBool(value)

			return err == nil && granted
		},
	},
	{
		// Earlier revisions of the same hooks looked the key up and ignored its value entirely.
		name:   "presence honouring",
		grants: func(string) bool { return true },
	},
}

// annotationValueCorpus covers everything strconv.ParseBool accepts, in both polarities, plus
// spellings around it that a user could plausibly write.
var annotationValueCorpus = []string{
	"1", "t", "T", "TRUE", "true", "True",
	"0", "f", "F", "FALSE", "false", "False",
	"", " ", "yes", "no", "on", "off", "enabled", "2", "TrUe", "true ",
}

// compiledPolicy evaluates the match conditions and the validation expressions of a
// ValidatingAdmissionPolicy through the same CEL machinery kube-apiserver uses, so the expressions
// the module ships are under test.
type compiledPolicy struct {
	matchConditions admissioncel.ConditionEvaluator
	validations     admissioncel.ConditionEvaluator
}

func compileRenderedPolicy(renderedPolicy string) compiledPolicy {
	// Strict decoding also asserts that the rendered manifest carries no field the API does not know.
	var policy admissionregistrationv1.ValidatingAdmissionPolicy
	Expect(yaml.UnmarshalStrict([]byte(renderedPolicy), &policy)).To(Succeed())

	// kube-apiserver declares the authorizer for validations but not for message expressions.
	validationVars := admissioncel.OptionalVariableDeclarations{HasAuthorizer: true, StrictCost: true}
	messageVars := admissioncel.OptionalVariableDeclarations{StrictCost: true}

	// The environment is pinned to the lowest Kubernetes version this branch supports, not to the
	// compatibility version of the vendored apiserver, which is higher. With failurePolicy: Fail an
	// expression needing a newer CEL feature would not merely fail to evaluate, it would deny every
	// DexClient and DexAuthenticator write on the oldest supported cluster.
	//
	// The mode has to be NewExpressions for the pin to mean anything: StoredExpressions deliberately
	// keeps every library available so that policies already persisted in etcd keep evaluating, and
	// it is NewExpressions that kube-apiserver validates a freshly submitted policy against.
	compiler, err := admissioncel.NewCompositedCompiler(
		environment.MustBaseEnvSet(minimalSupportedKubernetesVersion(), true),
	)
	Expect(err).ToNot(HaveOccurred())

	// Variables come first: expressions referring to them are type-checked against the types
	// collected here.
	for _, variable := range policy.Spec.Variables {
		result := compiler.CompileAndStoreVariable(
			namedExpression{name: variable.Name, expression: variable.Expression},
			validationVars, environment.NewExpressions,
		)
		Expect(result.Error).To(BeNil(), "variable %q should compile", variable.Name)
	}

	matchConditions := make([]admissioncel.ExpressionAccessor, 0, len(policy.Spec.MatchConditions))
	for _, condition := range policy.Spec.MatchConditions {
		expression := boolExpression{expression: condition.Expression}
		Expect(compiler.CompileCELExpression(expression, validationVars, environment.NewExpressions).Error).To(BeNil(),
			"match condition %q should compile", condition.Name)

		matchConditions = append(matchConditions, expression)
	}

	validations := make([]admissioncel.ExpressionAccessor, 0, len(policy.Spec.Validations))
	for _, validation := range policy.Spec.Validations {
		condition := boolExpression{expression: validation.Expression}
		Expect(compiler.CompileCELExpression(condition, validationVars, environment.NewExpressions).Error).To(BeNil(),
			"validation %q should compile", validation.Expression)

		message := stringExpression{expression: validation.MessageExpression}
		Expect(compiler.CompileCELExpression(message, messageVars, environment.NewExpressions).Error).To(BeNil(),
			"message expression %q should compile", validation.MessageExpression)

		validations = append(validations, condition)
	}

	return compiledPolicy{
		matchConditions: compiler.CompileCondition(matchConditions, validationVars, environment.NewExpressions),
		validations:     compiler.CompileCondition(validations, validationVars, environment.NewExpressions),
	}
}

// minimalSupportedKubernetesVersion reads the lowest Kubernetes version listed in
// candi/version_map.yml, which is the oldest cluster a Deckhouse release from this branch runs on.
func minimalSupportedKubernetesVersion() *version.Version {
	raw, err := os.ReadFile(filepath.Join(repositoryRoot(), "candi", "version_map.yml"))
	Expect(err).ToNot(HaveOccurred())

	var versionMap struct {
		K8s map[string]json.RawMessage `json:"k8s"`
	}
	Expect(yaml.Unmarshal(raw, &versionMap)).To(Succeed())
	Expect(versionMap.K8s).ToNot(BeEmpty())

	var minimal *version.Version
	for supported := range versionMap.K8s {
		parsed, err := version.ParseGeneric(supported)
		Expect(err).ToNot(HaveOccurred(), "kubernetes version %q should parse", supported)

		if minimal == nil || parsed.LessThan(minimal) {
			minimal = parsed
		}
	}

	return minimal
}

// repositoryRoot walks up from the package directory to the checkout root.
func repositoryRoot() string {
	directory, err := os.Getwd()
	Expect(err).ToNot(HaveOccurred())

	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory
		}

		parent := filepath.Dir(directory)
		Expect(parent).ToNot(Equal(directory), "the repository root should be reachable from %s", directory)
		directory = parent
	}
}

type dexAdmissionInput struct {
	resource       string
	kind           string
	operation      admission.Operation
	username       string
	groups         []string
	// canGrant hands the subject the allow-access-to-kubernetes subresource on the resource it
	// writes, in the namespace it writes to.
	canGrant bool
	// grantedResource and grantedNamespace narrow that permission, so that a grant issued for one
	// kind or one namespace can be shown not to carry over to another.
	grantedResource  string
	grantedNamespace string
	annotations      map[string]string
	oldAnnotations   map[string]string
}

const (
	admissionNamespace = "attacker-ns"
	admissionName      = "app"
)

// validate reports whether the policy admits the request. Match conditions are evaluated first, the
// way kube-apiserver does it: a request they do not select never reaches the validations at all, so
// the policy leaves it alone.
func (p compiledPolicy) validate(input dexAdmissionInput) (bool, error) {
	gvk := schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: input.kind}
	gvr := schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: input.resource}

	// The old object has to stay a nil interface on create: a typed nil pointer is treated as
	// present by the admission machinery.
	var oldObject runtime.Object
	if input.operation == admission.Update {
		oldObject = dexObject(gvk, admissionNamespace, admissionName, input.oldAnnotations)
	}

	groups := input.groups
	if groups == nil {
		groups = []string{"system:authenticated"}
	}

	attributes := admission.NewAttributesRecord(
		dexObject(gvk, admissionNamespace, admissionName, input.annotations), oldObject,
		gvk, admissionNamespace, admissionName, gvr, "", input.operation, nil, false,
		&user.DefaultInfo{Name: input.username, Groups: groups},
	)
	versionedAttributes, err := admission.NewVersionedAttributes(attributes, gvk, nil)
	if err != nil {
		return false, err
	}

	request := admissioncel.CreateAdmissionRequest(
		attributes, metav1.GroupVersionResource(gvr), metav1.GroupVersionKind(gvk),
	)
	bindings := admissioncel.OptionalVariableBindings{Authorizer: allowAccessAuthorizer(input)}

	selected, err := p.evaluate(p.matchConditions, versionedAttributes, request, bindings)
	if err != nil || !selected {
		return true, err
	}

	return p.evaluate(p.validations, versionedAttributes, request, bindings)
}

// evaluate reports whether every expression of the evaluator holds for the request.
func (p compiledPolicy) evaluate(
	evaluator admissioncel.ConditionEvaluator,
	attributes *admission.VersionedAttributes,
	request *admissionv1.AdmissionRequest,
	bindings admissioncel.OptionalVariableBindings,
) (bool, error) {
	results, _, err := evaluator.ForInput(
		context.Background(), attributes, request, bindings, nil, celconfig.RuntimeCELCostBudget,
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

// allowAccessAuthorizer mimics RBAC for a subject that holds the allow-access-to-kubernetes
// subresource on one resource in one namespace, and holds nothing else. Both are narrowed on
// purpose: a grant on DexClients must not carry over to DexAuthenticators, and a grant in one
// namespace must not carry over to another.
func allowAccessAuthorizer(input dexAdmissionInput) authorizer.Authorizer {
	grantedResource := input.grantedResource
	if grantedResource == "" {
		grantedResource = input.resource
	}
	grantedNamespace := input.grantedNamespace
	if grantedNamespace == "" {
		grantedNamespace = admissionNamespace
	}

	return authorizerFunc(func(attributes authorizer.Attributes) bool {
		return input.canGrant &&
			attributes.GetAPIGroup() == "deckhouse.io" &&
			attributes.GetResource() == grantedResource &&
			attributes.GetSubresource() == allowAccessSubresource &&
			attributes.GetNamespace() == grantedNamespace &&
			attributes.GetVerb() == "create"
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
