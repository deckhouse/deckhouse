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
	"maps"
	"os"
	"path/filepath"
	"strconv"
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
	reservedUsersPolicyName      = "d8-multitenancy-manager-reserved-users"
	keptObjectsPolicyName        = "d8-multitenancy-manager-kept-objects"
	namespaceOwnershipPolicyName = "d8-multitenancy-manager-namespace-ownership"
	manifestsPolicyName          = "d8-multitenancy-manager-manifests"
	manifestsObjectsPolicyName   = "d8-multitenancy-manager-manifests-objects"
	v1alpha1TemplateCreatePolicy = "d8-multitenancy-manager-v1alpha1-template-create"

	controllerServiceAccount = "system:serviceaccount:d8-multitenancy-manager:multitenancy-manager"
	deckhouseServiceAccount  = "system:serviceaccount:d8-system:deckhouse"
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
	hec.ValuesSet("multitenancyManager.internal.quotaLedgerCert.ca", "test")
	hec.ValuesSet("multitenancyManager.internal.quotaLedgerCert.crt", "test")
	hec.ValuesSet("multitenancyManager.internal.quotaLedgerCert.key", "test")
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

var _ = Describe("Module :: multitenancy-manager :: helm template :: namespace ownership policy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		renderPolicies(hec)
	})

	It("leaves the project label of a namespace to the controller, an empty value included", func() {
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", namespaceOwnershipPolicyName).ToYaml())

		// A nil label map stands for a namespace without the label, and a nil old map for a create.
		empty := map[string]string{"projects.deckhouse.io/project": ""}
		owned := map[string]string{"projects.deckhouse.io/project": "team"}
		other := map[string]string{"projects.deckhouse.io/project": "other"}
		none := map[string]string{}
		cases := []struct {
			name     string
			username string
			old, new map[string]string
			allowed  bool
		}{
			{name: "a user creates a namespace without the label", username: projectAdministrator, new: none, allowed: true},
			{name: "a user creates a namespace with the label", username: projectAdministrator, new: owned},
			{name: "a user creates a namespace with an empty label", username: projectAdministrator, new: empty},
			{name: "a user adds an empty label", username: projectAdministrator, old: none, new: empty},
			{name: "a user keeps an empty label", username: projectAdministrator, old: empty, new: empty, allowed: true},
			{name: "a user empties the label", username: projectAdministrator, old: owned, new: empty},
			{name: "a user fills an empty label", username: projectAdministrator, old: empty, new: owned},
			{name: "a user changes the label", username: projectAdministrator, old: owned, new: other},
			{name: "a user removes the label", username: projectAdministrator, old: owned, new: none, allowed: true},
			{name: "a user removes an empty label", username: projectAdministrator, old: empty, new: none, allowed: true},
			{name: "the controller creates a namespace with the label", username: controllerServiceAccount, new: owned, allowed: true},
			{name: "the controller changes the label", username: controllerServiceAccount, old: owned, new: other, allowed: true},
			{name: "deckhouse creates a namespace with an empty label", username: deckhouseServiceAccount, new: empty, allowed: true},
		}
		for _, tc := range cases {
			allowed, message, err := policy.admit(namespaceWrite(tc.username, tc.old, tc.new), nil)
			Expect(err).ToNot(HaveOccurred(), tc.name)
			Expect(allowed).To(Equal(tc.allowed), tc.name)
			if !allowed {
				Expect(message).To(HavePrefix("The projects.deckhouse.io/project label is set by the multitenancy-manager controller"), tc.name)
			}
		}
	})
})

var _ = Describe("Module :: multitenancy-manager :: helm template :: manifests policy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		renderPolicies(hec)
	})

	It("renders the policy and its binding for creates and updates of the refused kinds", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		policy := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", manifestsPolicyName)
		Expect(policy.Exists()).To(BeTrue())
		Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))
		Expect(policy.Field("spec.matchConstraints.resourceRules").String()).To(MatchJSON(
			`[{"apiGroups":["deckhouse.io"],"apiVersions":["*"],"operations":["CREATE","UPDATE"],"resources":["authorizationrules"]}]`))

		binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", manifestsPolicyName)
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("spec.policyName").String()).To(Equal(manifestsPolicyName))
		Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))
		Expect(binding.Field("spec.matchResources").Exists()).To(BeFalse(), "the binding matches every object of the kinds")
	})

	It("refuses an AuthorizationRule of the Admin level to the users the manifests are applied as", func() {
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", manifestsPolicyName).ToYaml())

		cases := []struct {
			name      string
			operation admission.Operation
			username  string
			level     string
			// oldLevel is the access level of the stored rule of an update; "-" stands for a rule
			// without a spec.
			oldLevel string
			// subjects and oldSubjects are the users the rule and the stored rule name; bob when empty.
			subjects, oldSubjects []string
			allowed               bool
		}{
			{name: "the manifests create an Admin rule", username: projectUser, level: "Admin"},
			{name: "the manifests of another project create an Admin rule", username: "system:multitenancy-manager:project:other", level: "Admin"},
			{name: "the manifests raise an Editor rule to Admin", operation: admission.Update, username: projectUser, level: "Admin", oldLevel: "Editor"},
			{name: "the manifests raise a rule without a level to Admin", operation: admission.Update, username: projectUser, level: "Admin"},
			{name: "the manifests raise a rule without a spec to Admin", operation: admission.Update, username: projectUser, level: "Admin", oldLevel: "-"},

			{name: "the manifests create a User rule", username: projectUser, level: "User", allowed: true},
			{name: "the manifests create a PrivilegedUser rule", username: projectUser, level: "PrivilegedUser", allowed: true},
			{name: "the manifests create an Editor rule", username: projectUser, level: "Editor", allowed: true},
			{name: "the manifests create a rule without a level", username: projectUser, allowed: true},
			{name: "the manifests update an Admin rule that stays Admin", operation: admission.Update, username: projectUser,
				level: "Admin", oldLevel: "Admin", allowed: true},
			{name: "the manifests add a subject to an Admin rule", operation: admission.Update, username: projectUser,
				level: "Admin", oldLevel: "Admin", subjects: []string{"bob", "eve"}},
			{name: "the manifests replace the subject of an Admin rule", operation: admission.Update, username: projectUser,
				level: "Admin", oldLevel: "Admin", subjects: []string{"eve"}},
			{name: "the manifests drop a subject of an Admin rule", operation: admission.Update, username: projectUser,
				level: "Admin", oldLevel: "Admin", oldSubjects: []string{"bob", "eve"}, allowed: true},
			{name: "the manifests lower an Admin rule to Editor", operation: admission.Update, username: projectUser,
				level: "Editor", oldLevel: "Admin", allowed: true},
			{name: "the manifests update an Editor rule", operation: admission.Update, username: projectUser,
				level: "Editor", oldLevel: "Editor", allowed: true},

			{name: "a project administrator creates an Admin rule", username: projectAdministrator, level: "Admin", allowed: true},
			{name: "a project administrator raises a rule to Admin", operation: admission.Update, username: projectAdministrator,
				level: "Admin", oldLevel: "Editor", allowed: true},
			{name: "the controller creates an Admin rule", username: controllerServiceAccount, level: "Admin", allowed: true},
			{name: "another user under the reserved prefix creates an Admin rule", username: "system:multitenancy-manager:other",
				level: "Admin", allowed: true},
		}
		for _, tc := range cases {
			operation := tc.operation
			if operation == "" {
				operation = admission.Create
			}
			allowed, message, err := policy.admit(ruleAttributes(operation, tc.username, tc.level, tc.oldLevel, tc.subjects, tc.oldSubjects), nil)
			Expect(err).ToNot(HaveOccurred(), tc.name)
			Expect(allowed).To(Equal(tc.allowed), tc.name)
			if !allowed {
				Expect(message).To(Equal("The manifests of a project template do not create an AuthorizationRule of the Admin level, "+
					"raise one to it or add subjects to one. Set the administrators of the project in .spec.administrators of the Project."), tc.name)
			}
		}
	})

	// The API server sends only the matched kinds; the expression keeps the others out all the same.
	It("lets the manifests create an object of another kind", func() {
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", manifestsPolicyName).ToYaml())
		configMap := bindingKind{
			gvr:        schema.GroupVersionResource{Version: "v1", Resource: "configmaps"},
			gvk:        schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
			namespaced: true,
		}
		allowed, _, err := policy.validate(configMap, admission.Create, projectUser, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(allowed).To(BeTrue())
	})
})

var _ = Describe("Module :: multitenancy-manager :: helm template :: v1alpha1 template create policy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		renderPolicies(hec)
	})

	It("refuses every create of a ProjectTemplate through deckhouse.io/v1alpha1 and nothing else", func() {
		Expect(hec.RenderError).ShouldNot(HaveOccurred())

		policy := hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", v1alpha1TemplateCreatePolicy)
		Expect(policy.Exists()).To(BeTrue())
		Expect(policy.Field("spec.failurePolicy").String()).To(Equal("Fail"))
		// With Equivalent, the API server would match a request through v1alpha2 as well.
		Expect(policy.Field("spec.matchConstraints.matchPolicy").String()).To(Equal("Exact"))
		Expect(policy.Field("spec.matchConstraints.resourceRules").String()).To(MatchJSON(
			`[{"apiGroups":["deckhouse.io"],"apiVersions":["v1alpha1"],"operations":["CREATE"],"resources":["projecttemplates"]}]`))
		Expect(policy.Field("spec.matchConditions").Exists()).To(BeFalse(), "nobody is exempt")

		binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", v1alpha1TemplateCreatePolicy)
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("spec.policyName").String()).To(Equal(v1alpha1TemplateCreatePolicy))
		Expect(binding.Field("spec.validationActions").String()).To(MatchJSON(`["Deny","Audit"]`))

		compiled := compileRenderedPolicy(policy.ToYaml())
		template := bindingKind{
			gvr: schema.GroupVersionResource{Group: "deckhouse.io", Version: "v1alpha1", Resource: "projecttemplates"},
			gvk: schema.GroupVersionKind{Group: "deckhouse.io", Version: "v1alpha1", Kind: "ProjectTemplate"},
		}
		for _, username := range []string{"admin@example.com", "system:sudouser", controllerServiceAccount, "system:serviceaccount:d8-system:deckhouse"} {
			allowed, message, err := compiled.validate(template, admission.Create, username, nil, nil)
			Expect(err).ToNot(HaveOccurred(), username)
			Expect(allowed).To(BeFalse(), username)
			Expect(message).To(HavePrefix("A ProjectTemplate is created through deckhouse.io/v1alpha2."), username)
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

// namespaceWrite is a write of the team namespace with the labels: a create when oldLabels is nil, an
// update from them otherwise.
func namespaceWrite(username string, oldLabels, newLabels map[string]string) admission.Attributes {
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	namespace := func(labels map[string]string) *unstructured.Unstructured {
		object := new(unstructured.Unstructured)
		object.SetGroupVersionKind(gvk)
		object.SetName("team")
		object.SetLabels(labels)
		return object
	}
	// The old object stays a nil interface on create: a typed nil pointer counts as present.
	operation := admission.Create
	var oldObject runtime.Object
	if oldLabels != nil {
		operation = admission.Update
		oldObject = namespace(oldLabels)
	}
	return admission.NewAttributesRecord(namespace(newLabels), oldObject, gvk, "", "team", gvr, "", operation, nil, false,
		&user.DefaultInfo{Name: username, Groups: groupsOf(username)})
}

// ruleAttributes is a write of an AuthorizationRule of the level in the main namespace of the proj
// project. An update replaces a stored rule of oldLevel, where "-" stands for a rule without a spec.
// An empty level leaves spec.accessLevel out.
func ruleAttributes(operation admission.Operation, username, level, oldLevel string, subjects, oldSubjects []string) admission.Attributes {
	rule := func(level string, names []string) *unstructured.Unstructured {
		object := new(unstructured.Unstructured)
		object.SetGroupVersionKind(authorizationRule.gvk)
		object.SetNamespace("proj")
		object.SetName("rule")
		if level == "-" {
			return object
		}
		if len(names) == 0 {
			names = []string{"bob"}
		}
		ruleSubjects := make([]any, 0, len(names))
		for _, name := range names {
			ruleSubjects = append(ruleSubjects, userSubject(name))
		}
		spec := map[string]any{"subjects": ruleSubjects}
		if level != "" {
			spec["accessLevel"] = level
		}
		object.Object["spec"] = spec
		return object
	}
	// The old object stays a nil interface on create: a typed nil pointer counts as present.
	var oldObject runtime.Object
	if operation == admission.Update {
		oldObject = rule(oldLevel, oldSubjects)
	}
	return admission.NewAttributesRecord(rule(level, subjects), oldObject, authorizationRule.gvk, "proj", "rule", authorizationRule.gvr, "",
		operation, nil, false, &user.DefaultInfo{Name: username, Groups: groupsOf(username)})
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

	// A policy with a paramKind reads its params in the variables and validations.
	hasParams := policy.Spec.ParamKind != nil
	validationVars := admissioncel.OptionalVariableDeclarations{HasAuthorizer: true, HasParams: hasParams}
	messageVars := admissioncel.OptionalVariableDeclarations{HasParams: hasParams}

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

		// A validation without a messageExpression is refused with its static message, which a CEL
		// string literal stands for here.
		message := stringExpression{expression: validation.MessageExpression}
		if message.expression == "" {
			message.expression = strconv.Quote(validation.Message)
		}
		Expect(compiler.CompileCELExpression(message, messageVars, environment.NewExpressions).Error).To(BeNil(),
			"message expression %q should compile", message.expression)
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
	return p.admitWithParams(attributes, authz, nil)
}

// admitWithParams is admit for a policy with a paramKind, params being the object its binding refers to.
func (p compiledPolicy) admitWithParams(attributes admission.Attributes, authz authorizer.Authorizer, params runtime.Object) (bool, string, error) {
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
		admissioncel.OptionalVariableBindings{Authorizer: authz, VersionedParams: params}, nil, celconfig.RuntimeCELCostBudget)
	if err != nil {
		return false, "", err
	}
	messages, _, err := p.messages.ForInput(context.Background(), versioned, request, admissioncel.OptionalVariableBindings{VersionedParams: params}, nil,
		celconfig.RuntimeCELCostBudget)
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

var _ = Describe("Module :: multitenancy-manager :: helm template :: heritage policy on a project namespace", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		renderPolicies(hec)
	})

	It("lets a user relabel a project namespace but not change its ownerReferences", func() {
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", "d8-multitenancy-manager").ToYaml())

		owner := map[string]any{"apiVersion": "example.com/v1", "kind": "Tenant", "name": "team", "uid": "1"}
		cases := []struct {
			name     string
			username string
			// owned stores the namespace with the owner before the edit
			owned   bool
			edit    func(*unstructured.Unstructured)
			allowed bool
		}{
			{name: "a user adds a label of their own", username: "admin@example.com", allowed: true,
				edit: func(ns *unstructured.Unstructured) {
					ns.SetLabels(map[string]string{"heritage": "multitenancy-manager", "team": "a"})
				}},
			{name: "a user adds an owner", username: "admin@example.com",
				edit: func(ns *unstructured.Unstructured) {
					Expect(unstructured.SetNestedSlice(ns.Object, []any{owner}, "metadata", "ownerReferences")).To(Succeed())
				}},
			{name: "the controller adds an owner", username: controllerServiceAccount, allowed: true,
				edit: func(ns *unstructured.Unstructured) {
					Expect(unstructured.SetNestedSlice(ns.Object, []any{owner}, "metadata", "ownerReferences")).To(Succeed())
				}},
			{name: "a user removes an owner", username: "admin@example.com", owned: true,
				edit: func(ns *unstructured.Unstructured) { ns.SetOwnerReferences(nil) }},
			{name: "a user changes the UID of an owner", username: "admin@example.com", owned: true,
				edit: func(ns *unstructured.Unstructured) {
					changed := maps.Clone(owner)
					changed["uid"] = "2"
					Expect(unstructured.SetNestedSlice(ns.Object, []any{changed}, "metadata", "ownerReferences")).To(Succeed())
				}},
			{name: "the garbage collector removes an owner", username: garbageCollector, owned: true, allowed: true,
				edit: func(ns *unstructured.Unstructured) { ns.SetOwnerReferences(nil) }},
		}
		for _, tc := range cases {
			var owners []any
			if tc.owned {
				owners = []any{owner}
			}
			allowed, message, err := policy.admit(projectNamespaceUpdate(tc.username, owners, tc.edit), nil)
			Expect(err).ToNot(HaveOccurred(), tc.name)
			Expect(allowed).To(Equal(tc.allowed), tc.name)
			if !allowed {
				Expect(message).To(ContainSubstring("its spec, finalizers, ownerReferences and the labels"), tc.name)
			}
		}
	})

	It("lets a rollout restart of a heritage workload through only when nothing else changes", func() {
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", "d8-multitenancy-manager").ToYaml())
		labels := map[string]string{"heritage": "multitenancy-manager"}
		restart := func(deployment *unstructured.Unstructured) {
			Expect(unstructured.SetNestedStringMap(deployment.Object, map[string]string{"kubectl.kubernetes.io/restartedAt": "2026-10-30T12:00:00Z"},
				"spec", "template", "metadata", "annotations")).To(Succeed())
		}
		restartAndScale := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			Expect(unstructured.SetNestedField(deployment.Object, int64(5), "spec", "replicas")).To(Succeed())
		}
		// An image change is a field of the pod template, next to the annotation a restart sets.
		restartAndChangeImage := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			Expect(unstructured.SetNestedSlice(deployment.Object, []any{map[string]any{"name": "web", "image": "evil:latest"}},
				"spec", "template", "spec", "containers")).To(Succeed())
		}
		cases := []struct {
			name      string
			edit      func(*unstructured.Unstructured)
			newLabels map[string]string
			allowed   bool
		}{
			{name: "a project administrator restarts a rollout", edit: restart, allowed: true},
			{name: "a project administrator restarts a rollout and scales", edit: restartAndScale},
			{name: "a project administrator restarts a rollout and changes the image", edit: restartAndChangeImage},
			{name: "a project administrator restarts a rollout and takes the heritage label off", edit: restart,
				newLabels: map[string]string{"app": "web"}},
			{name: "a project administrator scales", edit: func(deployment *unstructured.Unstructured) {
				Expect(unstructured.SetNestedField(deployment.Object, int64(5), "spec", "replicas")).To(Succeed())
			}},
		}
		for _, tc := range cases {
			newLabels := labels
			if tc.newLabels != nil {
				newLabels = tc.newLabels
			}
			allowed, message, err := policy.admit(manifestsObjectWrite(projectAdministrator, admission.Update, labels, newLabels, tc.edit, nil), nil)
			Expect(err).ToNot(HaveOccurred(), tc.name)
			Expect(allowed).To(Equal(tc.allowed), tc.name)
			if !allowed {
				Expect(message).To(ContainSubstring("This resource is managed by proj Project"), tc.name)
			}
		}
	})
})

// projectNamespaceUpdate is an update of the main namespace of the proj project that edit makes. The stored
// namespace has the given ownerReferences.
func projectNamespaceUpdate(username string, owners []any, edit func(*unstructured.Unstructured)) admission.Attributes {
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
	stored := new(unstructured.Unstructured)
	stored.SetGroupVersionKind(gvk)
	stored.SetName("proj")
	stored.SetLabels(map[string]string{"heritage": "multitenancy-manager"})
	if owners != nil {
		Expect(unstructured.SetNestedSlice(stored.Object, owners, "metadata", "ownerReferences")).To(Succeed())
	}
	updated := stored.DeepCopy()
	edit(updated)
	return admission.NewAttributesRecord(updated, stored, gvk, "", "proj", gvr, "", admission.Update, nil, false,
		&user.DefaultInfo{Name: username, Groups: groupsOf(username)})
}

var _ = Describe("Module :: multitenancy-manager :: helm template :: manifests objects policy", func() {
	hec := SetupHelmConfig("")

	BeforeEach(func() {
		renderPolicies(hec)
	})

	It("binds the objects that carry the label of the manifests", func() {
		binding := hec.KubernetesGlobalResource("ValidatingAdmissionPolicyBinding", manifestsObjectsPolicyName)
		Expect(binding.Exists()).To(BeTrue())
		Expect(binding.Field("spec.policyName").String()).To(Equal(manifestsObjectsPolicyName))
		Expect(binding.Field("spec.matchResources.objectSelector.matchLabels").String()).
			To(MatchJSON(`{"projects.deckhouse.io/source":"manifests"}`))
	})

	It("leaves the objects of the manifests to the manifests of their project", func() {
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", manifestsObjectsPolicyName).ToYaml())

		// A cluster administrator may update deployments everywhere; the users of the project only in
		// its namespace, as a RoleBinding grants them.
		authz := authorizer.AuthorizerFunc(func(_ context.Context, attributes authorizer.Attributes) (authorizer.Decision, string, error) {
			if attributes.GetUser().GetName() == "admin@example.com" || attributes.GetNamespace() == "proj" {
				return authorizer.DecisionAllow, "", nil
			}
			return authorizer.DecisionNoOpinion, "", nil
		})
		ofManifests := map[string]string{"projects.deckhouse.io/project": "proj", "projects.deckhouse.io/source": "manifests"}
		ofAnotherProject := map[string]string{"projects.deckhouse.io/project": "other", "projects.deckhouse.io/source": "manifests"}
		own := map[string]string{"app": "web"}
		// A switch from a Helm text leaves an object with the label of its project.
		leftFromHelm := map[string]string{"projects.deckhouse.io/project": "proj", "projects.deckhouse.io/kept-from-helm-template": "true"}
		restart := func(deployment *unstructured.Unstructured) {
			Expect(unstructured.SetNestedStringMap(deployment.Object, map[string]string{"kubectl.kubernetes.io/restartedAt": "2026-10-30T12:00:00Z"},
				"spec", "template", "metadata", "annotations")).To(Succeed())
		}
		scale := func(deployment *unstructured.Unstructured) {
			Expect(unstructured.SetNestedField(deployment.Object, int64(5), "spec", "replicas")).To(Succeed())
		}
		restartAndScale := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			scale(deployment)
		}
		// An image change is a field of the pod template, next to the annotation a restart sets.
		restartAndChangeImage := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			Expect(unstructured.SetNestedSlice(deployment.Object, []any{map[string]any{"name": "app", "image": "nginx:other"}},
				"spec", "template", "spec", "containers")).To(Succeed())
		}
		restartAndRelabelPod := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			Expect(unstructured.SetNestedStringMap(deployment.Object, map[string]string{"app": "other"},
				"spec", "template", "metadata", "labels")).To(Succeed())
		}
		restartAndAnnotateObject := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			deployment.SetAnnotations(map[string]string{"note": "changed"})
		}
		restartKeepingPodAnnotation := func(deployment *unstructured.Unstructured) {
			Expect(unstructured.SetNestedStringMap(deployment.Object, map[string]string{
				"app": "keep", "kubectl.kubernetes.io/restartedAt": "2026-10-30T12:00:00Z",
			}, "spec", "template", "metadata", "annotations")).To(Succeed())
		}
		restartChangingPodAnnotation := func(deployment *unstructured.Unstructured) {
			Expect(unstructured.SetNestedStringMap(deployment.Object, map[string]string{
				"app": "changed", "kubectl.kubernetes.io/restartedAt": "2026-10-30T12:00:00Z",
			}, "spec", "template", "metadata", "annotations")).To(Succeed())
		}
		// The API server writes these on the update, the generation bump of a pod-template change included.
		restartWithApiserverFields := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			deployment.SetResourceVersion("9")
			deployment.SetGeneration(4)
			deployment.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: "kubectl"}})
		}
		restartAndSetStatus := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			Expect(unstructured.SetNestedField(deployment.Object, int64(1), "status", "replicas")).To(Succeed())
		}
		restartAndChangeFinalizers := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			deployment.SetFinalizers([]string{"example.com/keep"})
		}
		restartAndChangeOwnerReferences := func(deployment *unstructured.Unstructured) {
			restart(deployment)
			Expect(unstructured.SetNestedSlice(deployment.Object, []any{map[string]any{
				"apiVersion": "apps/v1", "kind": "ReplicaSet", "name": "web", "uid": "abc",
			}}, "metadata", "ownerReferences")).To(Succeed())
		}
		podAnnotation := func(deployment *unstructured.Unstructured) {
			Expect(unstructured.SetNestedStringMap(deployment.Object, map[string]string{"app": "keep"},
				"spec", "template", "metadata", "annotations")).To(Succeed())
		}

		cases := []struct {
			name      string
			username  string
			operation admission.Operation
			oldLabels map[string]string
			newLabels map[string]string
			edit      func(*unstructured.Unstructured)
			// seed changes the stored object and the new one, before edit.
			seed    func(*unstructured.Unstructured)
			allowed bool
		}{
			{name: "the manifests create an object", username: projectUser, operation: admission.Create,
				newLabels: ofManifests, allowed: true},
			{name: "the manifests change an object", username: projectUser, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: scale, allowed: true},
			{name: "the manifests delete an object", username: projectUser, operation: admission.Delete,
				oldLabels: ofManifests, allowed: true},
			{name: "the manifests take over an object left from a Helm text", username: projectUser, operation: admission.Update,
				oldLabels: leftFromHelm, newLabels: ofManifests, allowed: true},
			{name: "the manifests of another project change an object", username: "system:multitenancy-manager:project:other",
				operation: admission.Update, oldLabels: ofManifests, newLabels: ofManifests, edit: scale},
			{name: "the manifests move an object to another project", username: projectUser, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofAnotherProject},
			{name: "the manifests put the project label back", username: projectUser, operation: admission.Update,
				oldLabels: map[string]string{"projects.deckhouse.io/source": "manifests"}, newLabels: ofManifests, allowed: true},
			{name: "the manifests put an empty project label back", username: projectUser, operation: admission.Update,
				oldLabels: map[string]string{"projects.deckhouse.io/source": "manifests", "projects.deckhouse.io/project": ""}, newLabels: ofManifests, allowed: true},
			{name: "a project administrator puts the project label back", username: projectAdministrator, operation: admission.Update,
				oldLabels: map[string]string{"projects.deckhouse.io/source": "manifests"}, newLabels: ofManifests},
			{name: "a cluster administrator puts the project label back", username: "admin@example.com", operation: admission.Update,
				oldLabels: map[string]string{"projects.deckhouse.io/source": "manifests"}, newLabels: ofManifests},
			{name: "the controller hands an object over to the release", username: controllerServiceAccount, operation: admission.Update,
				oldLabels: ofManifests, newLabels: own, allowed: true},
			{name: "the namespace controller deletes an object", username: "system:serviceaccount:kube-system:namespace-controller",
				operation: admission.Delete, oldLabels: ofManifests, allowed: true},
			{name: "a project administrator changes an object", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: scale},
			{name: "a module service account changes an object", username: "system:serviceaccount:d8-virtualization:virtualization-controller",
				operation: admission.Update, oldLabels: ofManifests, newLabels: ofManifests, edit: scale, allowed: true},
			{name: "a project administrator deletes an object", username: projectAdministrator, operation: admission.Delete,
				oldLabels: ofManifests},
			{name: "a project administrator takes the label off", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: own},
			{name: "a project administrator restarts a rollout", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: restart, allowed: true},
			{name: "a project administrator restarts a rollout and keeps another pod annotation", username: projectAdministrator,
				operation: admission.Update, oldLabels: ofManifests, newLabels: ofManifests, edit: restartKeepingPodAnnotation, seed: podAnnotation, allowed: true},
			{name: "a project administrator restarts a rollout while the API server bumps its fields", username: projectAdministrator,
				operation: admission.Update, oldLabels: ofManifests, newLabels: ofManifests, edit: restartWithApiserverFields, allowed: true},
			{name: "the manifests restart a rollout and scale", username: projectUser, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: restartAndScale, allowed: true},
			{name: "a project administrator restarts a rollout and scales", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: restartAndScale},
			{name: "a project administrator restarts a rollout and changes the image", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: restartAndChangeImage},
			{name: "a project administrator restarts a rollout and relabels the pod", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: restartAndRelabelPod},
			{name: "a project administrator restarts a rollout and annotates the object", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: restartAndAnnotateObject},
			{name: "a project administrator restarts a rollout and changes another pod annotation", username: projectAdministrator,
				operation: admission.Update, oldLabels: ofManifests, newLabels: ofManifests, edit: restartChangingPodAnnotation, seed: podAnnotation},
			{name: "a project administrator restarts a rollout and changes the status", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: restartAndSetStatus},
			{name: "a project administrator restarts a rollout and changes finalizers", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: restartAndChangeFinalizers},
			{name: "a project administrator restarts a rollout and changes ownerReferences", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: restartAndChangeOwnerReferences},
			{name: "a project administrator restarts a rollout and takes the label off", username: projectAdministrator, operation: admission.Update,
				oldLabels: ofManifests, newLabels: own, edit: restart},
			{name: "a project administrator creates an object with the label", username: projectAdministrator,
				operation: admission.Create, newLabels: ofManifests},
			{name: "a project administrator hands an object over", username: projectAdministrator, operation: admission.Update,
				oldLabels: own, newLabels: ofManifests},
			{name: "a cluster administrator changes an object", username: "admin@example.com", operation: admission.Update,
				oldLabels: ofManifests, newLabels: ofManifests, edit: scale},
			{name: "a cluster administrator creates an object with the label", username: "admin@example.com",
				operation: admission.Create, newLabels: ofManifests},
			{name: "a cluster administrator hands an object over", username: "admin@example.com", operation: admission.Update,
				oldLabels: own, newLabels: ofManifests, allowed: true},
		}
		for _, tc := range cases {
			allowed, message, err := policy.admit(manifestsObjectWrite(tc.username, tc.operation, tc.oldLabels, tc.newLabels, tc.edit, tc.seed), authz)
			Expect(err).ToNot(HaveOccurred(), tc.name)
			Expect(allowed).To(Equal(tc.allowed), tc.name)
			if !allowed {
				Expect(message).To(SatisfyAny(
					ContainSubstring("Manual modification is forbidden: change spec.manifests of the ProjectTemplate"),
					ContainSubstring("Only the manifests of a project put the projects.deckhouse.io/source: manifests label"),
				), tc.name)
			}
		}
	})

	It("evaluates the rollout restart on objects that have no pod template", func() {
		policy := compileRenderedPolicy(hec.KubernetesGlobalResource("ValidatingAdmissionPolicy", manifestsObjectsPolicyName).ToYaml())
		labels := map[string]string{"projects.deckhouse.io/project": "proj", "projects.deckhouse.io/source": "manifests"}
		write := func(gvk schema.GroupVersionKind, gvr schema.GroupVersionResource, operation admission.Operation, mutate func(*unstructured.Unstructured)) admission.Attributes {
			object := func() *unstructured.Unstructured {
				item := new(unstructured.Unstructured)
				item.SetGroupVersionKind(gvk)
				item.SetNamespace("proj")
				item.SetName("job")
				item.SetLabels(labels)
				return item
			}
			var oldObject, newObject runtime.Object
			if operation != admission.Create {
				oldObject = object()
			}
			if operation != admission.Delete {
				updated := object()
				if mutate != nil {
					mutate(updated)
				}
				newObject = updated
			}
			return admission.NewAttributesRecord(newObject, oldObject, gvk, "proj", "job", gvr, "", operation, nil, false,
				&user.DefaultInfo{Name: projectAdministrator, Groups: groupsOf(projectAdministrator)})
		}
		cronGVK := schema.GroupVersionKind{Group: "batch", Version: "v1", Kind: "CronJob"}
		cronGVR := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"}
		cmGVK := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
		cmGVR := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}

		// The pod template of a CronJob is not spec.template. Setting restartedAt there is not a
		// rollout restart, and the comparison must not fail closed with an evaluation error.
		allowed, _, err := policy.admit(write(cronGVK, cronGVR, admission.Update, func(object *unstructured.Unstructured) {
			Expect(unstructured.SetNestedStringMap(object.Object, map[string]string{"kubectl.kubernetes.io/restartedAt": "2026-10-30T12:00:00Z"},
				"spec", "jobTemplate", "spec", "template", "metadata", "annotations")).To(Succeed())
		}), nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(allowed).To(BeFalse())

		allowed, _, err = policy.admit(write(cmGVK, cmGVR, admission.Update, func(object *unstructured.Unstructured) {
			object.Object["data"] = map[string]any{"team": "other"}
		}), nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(allowed).To(BeFalse())

		allowed, _, err = policy.admit(write(cmGVK, cmGVR, admission.Create, nil), nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(allowed).To(BeFalse())

		allowed, _, err = policy.admit(write(cmGVK, cmGVR, admission.Delete, nil), nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(allowed).To(BeFalse())
	})
})

// manifestsObjectWrite is a write of the web Deployment in the main namespace of the project: a create
// when oldLabels is nil, a delete when newLabels is nil, an update otherwise, with edit applied to
// the new object.
func manifestsObjectWrite(username string, operation admission.Operation, oldLabels, newLabels map[string]string,
	edit func(*unstructured.Unstructured), seed func(*unstructured.Unstructured)) admission.Attributes {
	gvk := schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	deployment := func(labels map[string]string) *unstructured.Unstructured {
		object := new(unstructured.Unstructured)
		object.SetGroupVersionKind(gvk)
		object.SetNamespace("proj")
		object.SetName("web")
		object.SetLabels(labels)
		object.Object["spec"] = map[string]any{"replicas": int64(1), "template": map[string]any{"metadata": map[string]any{}}}
		return object
	}
	// A missing side stays a nil interface: a typed nil pointer counts as present.
	var oldObject, newObject runtime.Object
	if oldLabels != nil {
		stored := deployment(oldLabels)
		if seed != nil {
			seed(stored)
		}
		oldObject = stored
	}
	if newLabels != nil {
		updated := deployment(newLabels)
		if seed != nil {
			seed(updated)
		}
		if edit != nil {
			edit(updated)
		}
		newObject = updated
	}
	return admission.NewAttributesRecord(newObject, oldObject, gvk, "proj", "web", gvr, "", operation, nil, false,
		&user.DefaultInfo{Name: username, Groups: groupsOf(username)})
}
