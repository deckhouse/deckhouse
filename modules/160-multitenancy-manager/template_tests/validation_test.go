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
	"os"
	"path/filepath"
	"strings"

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
	reservedUsersPolicyName = "d8-multitenancy-manager-reserved-users"
	keptObjectsPolicyName   = "d8-multitenancy-manager-kept-objects"

	controllerServiceAccount = "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"
	garbageCollector         = "system:serviceaccount:kube-system:generic-garbage-collector"
	userAuthzController      = "system:serviceaccount:d8-user-authz:controller"
	projectAdministrator     = "alice@example.com"

	projectUser = "system:multitenancy-manager:project:proj"
)

// bindingKind is a kind the policy guards, with where the kind keeps its subjects.
type bindingKind struct {
	gvr schema.GroupVersionResource
	gvk schema.GroupVersionKind
	// inSpec is true for the Deckhouse kinds, which keep their subjects in spec.subjects; the RBAC
	// bindings keep them at the top.
	inSpec     bool
	namespaced bool
}

var (
	roleBinding = bindingKind{
		gvr:        schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
		gvk:        schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "RoleBinding"},
		namespaced: true,
	}
	clusterRoleBinding = bindingKind{
		gvr: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterrolebindings"},
		gvk: schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"},
	}
	projectRoleBinding = bindingKind{
		gvr:        schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha3", Resource: "projectrolebindings"},
		gvk:        schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha3", Kind: "ProjectRoleBinding"},
		inSpec:     true,
		namespaced: true,
	}
	clusterProjectRoleBinding = bindingKind{
		gvr:    schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha3", Resource: "clusterprojectrolebindings"},
		gvk:    schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha3", Kind: "ClusterProjectRoleBinding"},
		inSpec: true,
	}
	authorizationRule = bindingKind{
		gvr:        schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "authorizationrules"},
		gvk:        schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "AuthorizationRule"},
		inSpec:     true,
		namespaced: true,
	}
	clusterAuthorizationRule = bindingKind{
		gvr:    schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1", Resource: "clusterauthorizationrules"},
		gvk:    schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1", Kind: "ClusterAuthorizationRule"},
		inSpec: true,
	}
)

// renderPolicies renders the module on a cluster that serves the ValidatingAdmissionPolicy API.
func renderPolicies(hec *Config) {
	hec.ValuesSet("global.modulesImages", GetModulesImages())
	hec.ValuesSet("global.enabledModules", []string{"vertical-pod-autoscaler"})
	hec.ValuesSetFromYaml("global.discovery.d8SpecificNodeCountByRole", `{}`)
	hec.ValuesSet("multitenancyManager.internal.admissionWebhookCert.ca", "test")
	hec.ValuesSet("multitenancyManager.internal.admissionWebhookCert.crt", "test")
	hec.ValuesSet("multitenancyManager.internal.admissionWebhookCert.key", "test")
	hec.HelmRender(WithAPIVersions(
		"admissionregistration.k8s.io/v1/ValidatingAdmissionPolicy",
		"admissionregistration.k8s.io/v1/ValidatingAdmissionPolicyBinding",
	))
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: reserved users policy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		renderPolicies(hec)
	})

	It("renders the policy and its binding for every binding kind", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		policy := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedUsersPolicyName)
		Expect(policy.Exists()).To(BeTrue())
		Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))
		Expect(policy.Field("spec.matchConstraints.resourceRules.0.resources").String()).To(MatchJSON(`["rolebindings","clusterrolebindings"]`))
		Expect(policy.Field("spec.matchConstraints.resourceRules.1.resources").String()).To(MatchJSON(
			`["projectrolebindings","clusterprojectrolebindings","authorizationrules","clusterauthorizationrules"]`))

		binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", reservedUsersPolicyName)
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("spec.policyName").String()).To(Equal(reservedUsersPolicyName))
		Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
		Expect(binding.Field("spec.matchResources").Exists()).To(BeFalse(), "the binding matches every object of the kinds")
	})

	It("lets only the controller bind the users it acts as", func() {
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", reservedUsersPolicyName).ToYaml())

		cases := []struct {
			name      string
			kind      bindingKind
			operation admission.Operation
			username  string
			subjects  []any
			old       []any
			allowed   bool
		}{
			{name: "a user binds the user of a project in a RoleBinding", kind: roleBinding, username: projectAdministrator,
				subjects: []any{userSubject(projectUser)}},
			{name: "a user binds the user of a project in a ClusterRoleBinding", kind: clusterRoleBinding, username: projectAdministrator,
				subjects: []any{userSubject("bob"), userSubject(projectUser)}},
			{name: "a user binds the user of a project in a ProjectRoleBinding", kind: projectRoleBinding, username: projectAdministrator,
				subjects: []any{userSubject(projectUser)}},
			{name: "a user binds the user of a project in a ClusterProjectRoleBinding", kind: clusterProjectRoleBinding, username: projectAdministrator,
				subjects: []any{userSubject(projectUser)}},
			{name: "a user names the user of a project in an AuthorizationRule", kind: authorizationRule, username: projectAdministrator,
				subjects: []any{userSubject(projectUser)}},
			{name: "a user names the user of a project in a ClusterAuthorizationRule", kind: clusterAuthorizationRule, username: projectAdministrator,
				subjects: []any{userSubject(projectUser)}},
			{name: "a group under the prefix is reserved too", kind: roleBinding, username: projectAdministrator,
				subjects: []any{map[string]any{"kind": "Group", "name": "system:multitenancy-manager:anything"}}},
			{name: "a user adds the user of a project to a binding", kind: roleBinding, operation: admission.Update, username: projectAdministrator,
				old: []any{userSubject("bob")}, subjects: []any{userSubject("bob"), userSubject(projectUser)}},
			{name: "a cluster administrator binds the user of a project", kind: clusterRoleBinding, username: "admin@example.com",
				subjects: []any{userSubject(projectUser)}},
			{name: "another service account of kube-system binds the user of a project", kind: roleBinding,
				username: "system:serviceaccount:kube-system:some-operator", subjects: []any{userSubject(projectUser)}},
			// user-authz-controller leaves the user out of its d8:dict grants, because this refusal
			// would come back on every reconcile of the dictionary
			{name: "user-authz-controller grants the dictionary to the user of a project", kind: clusterRoleBinding, username: userAuthzController,
				subjects: []any{userSubject(projectUser)}},

			{name: "the controller binds the user of a project", kind: projectRoleBinding, username: controllerServiceAccount,
				subjects: []any{userSubject(projectUser)}, allowed: true},
			{name: "the controller fans the binding out", kind: roleBinding, username: controllerServiceAccount,
				subjects: []any{userSubject(projectUser)}, allowed: true},
			{name: "a user binds other users", kind: roleBinding, username: projectAdministrator,
				subjects: []any{userSubject("bob"), userSubject("system:multitenancy-manager-team")}, allowed: true},
			{name: "a user takes the user of a project out of a binding", kind: roleBinding, operation: admission.Update, username: projectAdministrator,
				old: []any{userSubject("bob"), userSubject(projectUser)}, subjects: []any{userSubject("bob")}, allowed: true},
			{name: "the garbage collector updates a fanned-out binding", kind: roleBinding, operation: admission.Update, username: garbageCollector,
				old: []any{userSubject(projectUser)}, subjects: []any{userSubject(projectUser)}, allowed: true},
			{name: "kube-controller-manager without its service accounts updates a fanned-out binding", kind: roleBinding, operation: admission.Update,
				username: "system:kube-controller-manager", old: []any{userSubject(projectUser)}, subjects: []any{userSubject(projectUser)}, allowed: true},
			// the break-glass identity the other policies of the module let through
			{name: "system:sudouser binds the user of a project", kind: roleBinding, username: "system:sudouser",
				subjects: []any{userSubject(projectUser)}, allowed: true},
			{name: "a binding without subjects", kind: roleBinding, username: projectAdministrator, allowed: true},
		}

		for _, tc := range cases {
			operation := tc.operation
			if operation == "" {
				operation = admission.Create
			}
			allowed, message, err := policy.validate(tc.kind, operation, tc.username, tc.subjects, tc.old)
			Expect(err).ToNot(HaveOccurred(), tc.name)
			Expect(allowed).To(Equal(tc.allowed), tc.name)
			if !allowed {
				Expect(message).To(ContainSubstring("The system:multitenancy-manager:"), tc.name)
				Expect(message).To(ContainSubstring("only the controller binds the users whose names start with system:multitenancy-manager:"), tc.name)
			}
		}
	})
})

var _ = Describe("Module :: multitenancy-manager :: helm template :: kept objects policy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		renderPolicies(hec)
	})

	It("lets the manifests take a kept object over and keeps the users of the project off it", func() {
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", keptObjectsPolicyName).ToYaml())

		// The users of the project, the one the manifests apply as among them, have rights in its
		// namespace only, as a RoleBinding grants them; a cluster administrator has them everywhere.
		authz := authorizer.AuthorizerFunc(func(_ context.Context, attributes authorizer.Attributes) (authorizer.Decision, string, error) {
			if attributes.GetUser().GetName() == "admin@example.com" || attributes.GetNamespace() == "proj" {
				return authorizer.DecisionAllow, "", nil
			}
			return authorizer.DecisionNoOpinion, "", nil
		})

		cases := []struct {
			name     string
			username string
			allowed  bool
		}{
			{name: "the manifests update a kept object as the user of the project", username: projectUser, allowed: true},
			{name: "a project administrator updates a kept object", username: projectAdministrator},
			{name: "a cluster administrator updates a kept object", username: "admin@example.com", allowed: true},
		}
		for _, tc := range cases {
			allowed, message, err := policy.admit(keptObjectUpdate(tc.username), authz)
			Expect(err).ToNot(HaveOccurred(), tc.name)
			Expect(allowed).To(Equal(tc.allowed), tc.name)
			if !allowed {
				Expect(message).To(ContainSubstring("Only a requester who may update networkpolicies across the cluster may change it"), tc.name)
			}
		}
	})
})

// keptObjectUpdate is an update of a NetworkPolicy that a switch of the proj project from a Helm text
// left in the project namespace.
func keptObjectUpdate(username string) admission.Attributes {
	gvk := schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"}
	gvr := schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"}
	kept := func() *unstructured.Unstructured {
		object := new(unstructured.Unstructured)
		object.SetGroupVersionKind(gvk)
		object.SetNamespace("proj")
		object.SetName("deny-all")
		object.SetLabels(map[string]string{"projects.deckhouse.io/kept-from-helm-template": "true"})
		return object
	}
	return admission.NewAttributesRecord(kept(), kept(), gvk, "proj", "deny-all", gvr, "", admission.Update, nil, false,
		&user.DefaultInfo{Name: username, Groups: groupsOf(username)})
}

func userSubject(name string) map[string]any {
	return map[string]any{"apiGroup": "rbac.authorization.k8s.io", "kind": "User", "name": name}
}

type compiledPolicy struct {
	matchConditions admissioncel.ConditionEvaluator
	validations     admissioncel.ConditionEvaluator
	messages        admissioncel.ConditionEvaluator
}

func compileRenderedPolicy(renderedPolicy string) compiledPolicy {
	// Strict decoding also asserts that the rendered manifest carries no field the API does not know.
	var policy admissionregistrationv1.ValidatingAdmissionPolicy
	Expect(yaml.UnmarshalStrict([]byte(renderedPolicy), &policy)).To(Succeed())

	validationVars := admissioncel.OptionalVariableDeclarations{HasAuthorizer: true}
	messageVars := admissioncel.OptionalVariableDeclarations{}

	// The environment is pinned to the lowest Kubernetes version this branch supports, and the mode
	// is NewExpressions, the one kube-apiserver validates a freshly submitted policy against: with
	// failurePolicy: Fail, an expression that needs a newer CEL feature would refuse every binding on
	// the oldest supported cluster.
	compiler, err := admissioncel.NewCompositedCompiler(environment.MustBaseEnvSet(minimalSupportedKubernetesVersion()))
	Expect(err).ToNot(HaveOccurred())
	for _, variable := range policy.Spec.Variables {
		result := compiler.CompileAndStoreVariable(
			namedExpression{name: variable.Name, expression: variable.Expression}, validationVars, environment.NewExpressions)
		Expect(result.Error).To(BeNil(), "variable %q should compile", variable.Name)
	}

	// Match conditions cannot refer to the variables, so they get a compiler that has none.
	matchCompiler, err := admissioncel.NewCompositedCompiler(environment.MustBaseEnvSet(minimalSupportedKubernetesVersion()))
	Expect(err).ToNot(HaveOccurred())
	matchConditions := make([]admissioncel.ExpressionAccessor, 0, len(policy.Spec.MatchConditions))
	for _, condition := range policy.Spec.MatchConditions {
		expression := boolExpression{expression: condition.Expression}
		Expect(matchCompiler.CompileCELExpression(expression, validationVars, environment.NewExpressions).Error).To(BeNil(),
			"match condition %q should compile", condition.Name)
		matchConditions = append(matchConditions, expression)
	}

	validations := make([]admissioncel.ExpressionAccessor, 0, len(policy.Spec.Validations))
	messages := make([]admissioncel.ExpressionAccessor, 0, len(policy.Spec.Validations))
	for _, validation := range policy.Spec.Validations {
		condition := boolExpression{expression: validation.Expression}
		Expect(compiler.CompileCELExpression(condition, validationVars, environment.NewExpressions).Error).To(BeNil(),
			"validation %q should compile", validation.Expression)
		validations = append(validations, condition)

		message := stringExpression{expression: validation.MessageExpression}
		Expect(compiler.CompileCELExpression(message, messageVars, environment.NewExpressions).Error).To(BeNil(),
			"message expression %q should compile", validation.MessageExpression)
		messages = append(messages, message)
	}

	return compiledPolicy{
		matchConditions: matchCompiler.CompileCondition(matchConditions, validationVars, environment.NewExpressions),
		validations:     compiler.CompileCondition(validations, validationVars, environment.NewExpressions),
		messages:        compiler.CompileCondition(messages, messageVars, environment.NewExpressions),
	}
}

// validate reports whether the policy admits the request for a binding, and the message of the first
// validation that refuses it.
func (p compiledPolicy) validate(kind bindingKind, operation admission.Operation, username string, subjects, old []any) (bool, string, error) {
	namespace := ""
	if kind.namespaced {
		namespace = "proj"
	}

	// The old object stays a nil interface on create: a typed nil pointer counts as present.
	var oldObject runtime.Object
	if operation == admission.Update {
		oldObject = bindingObject(kind, namespace, old)
	}
	attributes := admission.NewAttributesRecord(
		bindingObject(kind, namespace, subjects), oldObject, kind.gvk, namespace, "binding", kind.gvr, "", operation, nil, false,
		&user.DefaultInfo{Name: username, Groups: groupsOf(username)},
	)
	return p.admit(attributes, nil)
}

// admit reports whether the policy admits the request, with authz behind its authorizer variable, and
// the message of the first validation that refuses it. Match conditions are evaluated first, as
// kube-apiserver does: a request they do not select never reaches the validations.
func (p compiledPolicy) admit(attributes admission.Attributes, authz authorizer.Authorizer) (bool, string, error) {
	versioned, err := admission.NewVersionedAttributes(attributes, attributes.GetKind(), nil)
	if err != nil {
		return false, "", err
	}
	request := admissioncel.CreateAdmissionRequest(attributes,
		metav1.GroupVersionResource(attributes.GetResource()), metav1.GroupVersionKind(attributes.GetKind()))

	selected, err := allTrue(p.matchConditions, versioned, request)
	if err != nil || !selected {
		return true, "", err
	}
	results, _, err := p.validations.ForInput(context.Background(), versioned, request,
		admissioncel.OptionalVariableBindings{Authorizer: authz}, nil, celconfig.RuntimeCELCostBudget)
	if err != nil {
		return false, "", err
	}
	messages, _, err := p.messages.ForInput(context.Background(), versioned, request, admissioncel.OptionalVariableBindings{}, nil, celconfig.RuntimeCELCostBudget)
	if err != nil {
		return false, "", err
	}
	for i, result := range results {
		if result.Error != nil {
			return false, "", result.Error
		}
		if result.EvalResult != celtypes.True {
			if messages[i].Error != nil {
				return false, "", messages[i].Error
			}
			return false, messages[i].EvalResult.Value().(string), nil
		}
	}
	return true, "", nil
}

// allTrue reports whether every expression of the evaluator holds for the request.
func allTrue(evaluator admissioncel.ConditionEvaluator, attributes *admission.VersionedAttributes, request *admissionv1.AdmissionRequest) (bool, error) {
	results, _, err := evaluator.ForInput(context.Background(), attributes, request, admissioncel.OptionalVariableBindings{}, nil, celconfig.RuntimeCELCostBudget)
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

// groupsOf returns the groups kube-apiserver attaches to the user, the ones the exclusions of the
// policy are written against.
func groupsOf(username string) []string {
	groups := []string{"system:authenticated"}
	namespaced, isServiceAccount := strings.CutPrefix(username, "system:serviceaccount:")
	if !isServiceAccount {
		return groups
	}
	namespace, _, _ := strings.Cut(namespaced, ":")
	return append(groups, "system:serviceaccounts", "system:serviceaccounts:"+namespace)
}

func bindingObject(kind bindingKind, namespace string, subjects []any) *unstructured.Unstructured {
	object := new(unstructured.Unstructured)
	object.SetGroupVersionKind(kind.gvk)
	object.SetNamespace(namespace)
	object.SetName("binding")
	if subjects == nil {
		return object
	}
	if kind.inSpec {
		object.Object["spec"] = map[string]any{"subjects": subjects}
	} else {
		object.Object["subjects"] = subjects
	}
	return object
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
