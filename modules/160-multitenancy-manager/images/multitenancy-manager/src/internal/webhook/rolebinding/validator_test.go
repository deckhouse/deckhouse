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

package rolebinding

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	admissionv1 "k8s.io/api/admission/v1"
	authnv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebinding "controller/internal/rolebinding"
)

func newClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	return newClientBuilder(t, objs...).Build()
}

func newClientBuilder(t *testing.T, objs ...client.Object) *fake.ClientBuilder {
	t.Helper()
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		rbacv1.AddToScheme, authorizationv1.AddToScheme, corev1.AddToScheme, v1alpha3.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...)
}

func request(op admissionv1.Operation, user string) admission.Request {
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: op,
		UserInfo:  authnv1.UserInfo{Username: user},
	}}
}

func clusterRole(name string, labels, annotations map[string]string) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels, Annotations: annotations}}
}

func TestValidate_ManagedByProtection(t *testing.T) {
	c := newClient(t)
	in := Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer", ManagedBy: v1alpha3.ManagedByController}

	// a regular user cannot touch a controller-managed binding
	resp := Validate(context.Background(), c, request(admissionv1.Update, "alice"), in)
	assert.False(t, resp.Allowed)

	// the controller can
	resp = Validate(context.Background(), c, request(admissionv1.Update, ControllerServiceAccount), in)
	assert.True(t, resp.Allowed)
}

// The marking and the change it forbids can arrive in one request, so the incoming object alone is
// not enough to judge by: an UPDATE that drops the label would otherwise be judged as unmarked.
func TestResolveManagedBy(t *testing.T) {
	managed := map[string]string{v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController}

	tests := []struct {
		name      string
		operation admissionv1.Operation
		object    map[string]string
		oldObject map[string]string
		expected  string
	}{
		{name: "marked", operation: admissionv1.Update, object: managed, expected: v1alpha3.ManagedByController},
		{name: "unmarked", operation: admissionv1.Update, object: map[string]string{}, expected: ""},
		{
			name:      "the marking stripped in the same update",
			operation: admissionv1.Update,
			object:    map[string]string{},
			oldObject: managed,
			expected:  v1alpha3.ManagedByController,
		},
		{
			name:      "the marking replaced in the same update",
			operation: admissionv1.Update,
			object:    map[string]string{v1alpha3.ResourceLabelManagedBy: "somebody-else"},
			oldObject: managed,
			expected:  v1alpha3.ManagedByController,
		},
		{
			// There is no old object to read, and a binding created with the marking is judged by it.
			name:      "created with the marking",
			operation: admissionv1.Create,
			object:    managed,
			expected:  v1alpha3.ManagedByController,
		},
		{
			name:      "another value is carried through as it is",
			operation: admissionv1.Update,
			object:    map[string]string{v1alpha3.ResourceLabelManagedBy: "somebody-else"},
			expected:  "somebody-else",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ResolveManagedBy(tt.operation, tt.object, tt.oldObject))
		})
	}
}

func TestValidate_Delete(t *testing.T) {
	c := newClient(t)
	// delete of a non-managed binding is always allowed (no role checks)
	resp := Validate(context.Background(), c, request(admissionv1.Delete, "alice"), Input{})
	assert.True(t, resp.Allowed)
}

func TestValidate_RoleRefKind(t *testing.T) {
	c := newClient(t)
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "Role", RoleRefName: "d8:project:viewer"})
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "ClusterRole")
}

func TestValidate_RoleNotAllowed(t *testing.T) {
	c := newClient(t)
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "cluster-admin"})
	assert.False(t, resp.Allowed)
}

func TestValidate_ClusterRoleMissing(t *testing.T) {
	c := newClient(t)
	// privileged user, allowed prefix, role does not exist -> allowed with a warning
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer"})
	assert.True(t, resp.Allowed)
	assert.NotEmpty(t, resp.Warnings)
}

func TestValidate_ClusterRoleMissingFailsClosedForUser(t *testing.T) {
	c := newClient(t)
	// non-privileged user, allowed prefix, role does not exist -> denied (fail closed)
	resp := Validate(context.Background(), c, request(admissionv1.Create, "alice"),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer"})
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "does not exist")
}

func TestValidate_Subjects(t *testing.T) {
	c := newClient(t, clusterRole("d8:project:viewer", nil, nil))
	base := func(subjects ...rbacv1.Subject) Input {
		return Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer", Namespace: "proj", Subjects: subjects}
	}

	// invalid subject kind is rejected
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		base(rbacv1.Subject{Kind: "Robot", Name: "x"}))
	assert.False(t, resp.Allowed)

	// ServiceAccount without namespace is rejected
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		base(rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "sa"}))
	assert.False(t, resp.Allowed)

	// ServiceAccount from a foreign namespace is rejected
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		base(rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "sa", Namespace: "other"}))
	assert.False(t, resp.Allowed)

	// ServiceAccount from the project's main namespace is accepted
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		base(rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "sa", Namespace: "proj"}))
	assert.True(t, resp.Allowed, resp.Result)
}

// A ServiceAccount belongs to the project when its namespace does, and a name that starts with the
// project name and a dash does not make it so: such a namespace can be the main namespace of a
// separate project, and a binding would then hand rights in this project to another tenant's
// workloads.
func TestValidate_ServiceAccountOfTheProject(t *testing.T) {
	sa := func(namespace string) rbacv1.Subject {
		return rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "sa", Namespace: namespace}
	}
	saUser := func(namespace string) rbacv1.Subject {
		return rbacv1.Subject{Kind: rbacv1.UserKind, Name: "system:serviceaccount:" + namespace + ":sa"}
	}
	saGroup := func(namespace string) rbacv1.Subject {
		return rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:" + namespace}
	}
	namespace := func(name, project string) *corev1.Namespace {
		return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{v1alpha3.ResourceLabelProject: project}}}
	}
	claim := &v1alpha3.ProjectNamespace{
		ObjectMeta: metav1.ObjectMeta{Name: "queue", Namespace: "proj"},
		Spec:       v1alpha3.ProjectNamespaceSpec{Name: "queue"},
	}
	// A ProjectNamespace of another project whose name reads like one of proj's.
	foreignClaim := &v1alpha3.ProjectNamespace{
		ObjectMeta: metav1.ObjectMeta{Name: "backend", Namespace: "proj-a"},
		Spec:       v1alpha3.ProjectNamespaceSpec{Name: "backend"},
	}
	c := newClient(t,
		clusterRole("d8:project:viewer", nil, nil),
		namespace("proj-extra", "proj"),
		namespace("proj-other", "proj-other"),
		claim,
		foreignClaim,
	)
	input := func(subjects, old []rbacv1.Subject) Input {
		return Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer", Namespace: "proj", Subjects: subjects, OldSubjects: old}
	}

	const noLongerOther = `ServiceAccount proj-other/sa is no longer in a namespace of project "proj" and gets no rights from this binding`
	tests := []struct {
		name      string
		operation admissionv1.Operation
		subjects  []rbacv1.Subject
		old       []rbacv1.Subject
		allowed   bool
		warnings  []string
	}{
		{name: "an additional namespace of the project", operation: admissionv1.Create, subjects: []rbacv1.Subject{sa("proj-extra")}, allowed: true},
		{name: "a namespace a ProjectNamespace of the project claims", operation: admissionv1.Create, subjects: []rbacv1.Subject{sa("proj-queue")}, allowed: true},
		{name: "a separate project named like an additional namespace", operation: admissionv1.Create, subjects: []rbacv1.Subject{sa("proj-other")}},
		{name: "an unclaimed name under the project name", operation: admissionv1.Create, subjects: []rbacv1.Subject{sa("proj-future")}},
		{name: "a namespace a ProjectNamespace of another project claims", operation: admissionv1.Create, subjects: []rbacv1.Subject{sa("proj-a-backend")}},
		{
			// The finalizer updates of the controller carry the stored subjects; refusing them would
			// leave the binding impossible to delete. The fan-out gives the subject no rights, and
			// the warning says so to whoever applies the binding.
			name:      "a subject the stored binding already has is not refused again, only warned about",
			operation: admissionv1.Update,
			subjects:  []rbacv1.Subject{sa("proj-other")},
			old:       []rbacv1.Subject{sa("proj-other")},
			allowed:   true,
			warnings:  []string{noLongerOther},
		},
		{
			name:      "a stored subject of the project has no warning",
			operation: admissionv1.Update,
			subjects:  []rbacv1.Subject{sa("proj-extra"), sa("proj-queue")},
			old:       []rbacv1.Subject{sa("proj-extra"), sa("proj-queue")},
			allowed:   true,
		},
		{
			name:      "a subject added on update is checked",
			operation: admissionv1.Update,
			subjects:  []rbacv1.Subject{sa("proj-other"), sa("proj-future")},
			old:       []rbacv1.Subject{sa("proj-other")},
		},
		// The User and the Group that stand for ServiceAccounts are held to the same rule; the fan-out
		// would leave them out of the RoleBindings otherwise, and they would count against the names
		// of other projects.
		{name: "the user of a ServiceAccount of the project", operation: admissionv1.Create, subjects: []rbacv1.Subject{saUser("proj-extra")}, allowed: true},
		{name: "the group of the ServiceAccounts of the project", operation: admissionv1.Create, subjects: []rbacv1.Subject{saGroup("proj-queue")}, allowed: true},
		{name: "the user of a ServiceAccount of another project", operation: admissionv1.Create, subjects: []rbacv1.Subject{saUser("proj-other")}},
		{name: "the group of the ServiceAccounts of another project", operation: admissionv1.Create, subjects: []rbacv1.Subject{saGroup("proj-other")}},
		{
			name:     "a user or a group that names no ServiceAccounts is not checked",
			subjects: []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}, {Kind: rbacv1.GroupKind, Name: "system:serviceaccounts"}},
			allowed:  true, operation: admissionv1.Create,
		},
		{
			name:      "a stored group of another project is only warned about",
			operation: admissionv1.Update,
			subjects:  []rbacv1.Subject{saGroup("proj-other")},
			old:       []rbacv1.Subject{saGroup("proj-other")},
			allowed:   true,
			warnings: []string{`Group "system:serviceaccounts:proj-other" names ServiceAccounts of namespace "proj-other", which is no ` +
				`longer in a namespace of project "proj", and gets no rights from this binding`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := Validate(context.Background(), c, request(tt.operation, ControllerServiceAccount), input(tt.subjects, tt.old))
			assert.Equal(t, tt.allowed, resp.Allowed, resp.Result)
			if !tt.allowed {
				assert.Contains(t, resp.Result.Message, `must belong to project "proj"`)
			}
			assert.Equal(t, tt.warnings, resp.Warnings)
		})
	}

	// A cluster-wide binding has no project to judge a ServiceAccount by, stored or not.
	cluster := input([]rbacv1.Subject{sa("proj-other")}, []rbacv1.Subject{sa("proj-other")})
	cluster.Namespace = ""
	resp := Validate(context.Background(), c, request(admissionv1.Update, ControllerServiceAccount), cluster)
	assert.True(t, resp.Allowed, resp.Result)
	assert.Empty(t, resp.Warnings)
}

// A stored subject is never refused, so a failed read of its namespace costs the warning alone. A new
// subject is still refused with the error.
func TestValidate_StoredSubjectReadError(t *testing.T) {
	failNamespaces := interceptor.Funcs{Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
		if _, ok := obj.(*corev1.Namespace); ok {
			return errors.New("the API server is unavailable")
		}
		return cl.Get(ctx, key, obj, opts...)
	}}
	c := newClientBuilder(t, clusterRole("d8:project:viewer", nil, nil)).WithInterceptorFuncs(failNamespaces).Build()
	stored := []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "deployer", Namespace: "proj-other"}}
	in := Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer", Namespace: "proj", Subjects: stored, OldSubjects: stored}

	resp := Validate(context.Background(), c, request(admissionv1.Update, ControllerServiceAccount), in)
	assert.True(t, resp.Allowed, resp.Result)
	assert.Empty(t, resp.Warnings)

	in.OldSubjects = nil
	resp = Validate(context.Background(), c, request(admissionv1.Update, ControllerServiceAccount), in)
	assert.False(t, resp.Allowed)
	assert.Equal(t, int32(http.StatusInternalServerError), resp.Result.Code)
}

// The warning goes out with the response that allows the request, whichever it is. Here it is the
// response to a non-privileged update, after the check of the right to bind the role.
func TestValidate_StoredSubjectWarningForUser(t *testing.T) {
	allowBind := interceptor.Funcs{Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
		if sar, ok := obj.(*authorizationv1.SubjectAccessReview); ok {
			sar.Status.Allowed = true
			return nil
		}
		return cl.Create(ctx, obj, opts...)
	}}
	c := newClientBuilder(t,
		clusterRole("d8:project:viewer", nil, nil),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj-other", Labels: map[string]string{v1alpha3.ResourceLabelProject: "proj-other"}}},
	).WithInterceptorFuncs(allowBind).Build()
	stored := []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "deployer", Namespace: "proj-other"}}
	in := Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer", Namespace: "proj", Subjects: stored, OldSubjects: stored}

	resp := Validate(context.Background(), c, request(admissionv1.Update, "alice"), in)
	assert.True(t, resp.Allowed, resp.Result)
	assert.Equal(t, []string{`ServiceAccount proj-other/deployer is no longer in a namespace of project "proj" and gets no rights from this binding`}, resp.Warnings)
}

// The controller writes the administrators binding of a project from spec.administrators, which the
// upgrade fills with the administrators of the previous release, a User or a Group that names the
// ServiceAccounts of another project among them. Those writes are not held to the project, and their
// stored subjects get no warning, as the fan-out carries the administrators. Every other writer, and
// every other binding the controller writes, is held to the project as before, and the other subject
// checks still apply to the administrators binding.
func TestValidate_AdministratorsBindingOfTheController(t *testing.T) {
	c := newClient(t,
		clusterRole(v1alpha3.ProjectAdministratorsRoleName, nil, nil),
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "proj-other", Labels: map[string]string{v1alpha3.ResourceLabelProject: "proj-other"}}},
	)
	administrators := []rbacv1.Subject{
		{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "system:serviceaccount:proj-other:runner"},
		{APIGroup: rbacv1.GroupName, Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:proj-other"},
		{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Name: "alice@example.com"},
	}
	input := func(name, managedBy string, subjects []rbacv1.Subject) Input {
		return Input{
			RoleRefKind: "ClusterRole",
			RoleRefName: v1alpha3.ProjectAdministratorsRoleName,
			Name:        name,
			Namespace:   "proj",
			ManagedBy:   managedBy,
			Subjects:    subjects,
		}
	}
	const outsideTheProject = `names ServiceAccounts of namespace "proj-other", which must belong to project "proj"`

	tests := []struct {
		name      string
		user      string
		operation admissionv1.Operation
		in        Input
		old       []rbacv1.Subject
		// denied is a part of the refusal; empty means the request is allowed.
		denied string
	}{
		{
			name:      "the controller creates the binding",
			user:      ControllerServiceAccount,
			operation: admissionv1.Create,
			in:        input(v1alpha3.ProjectAdministratorsBinding, v1alpha3.ManagedByController, administrators),
		},
		{
			name:      "the controller updates the stored binding",
			user:      ControllerServiceAccount,
			operation: admissionv1.Update,
			in:        input(v1alpha3.ProjectAdministratorsBinding, v1alpha3.ManagedByController, administrators),
			old:       administrators,
		},
		{
			name:      "Deckhouse writes the binding",
			user:      DeckhouseServiceAccount,
			operation: admissionv1.Create,
			in:        input(v1alpha3.ProjectAdministratorsBinding, v1alpha3.ManagedByController, administrators),
			denied:    outsideTheProject,
		},
		{
			name:      "the controller writes another binding with the label",
			user:      ControllerServiceAccount,
			operation: admissionv1.Create,
			in:        input("ci", v1alpha3.ManagedByController, administrators),
			denied:    outsideTheProject,
		},
		{
			name:      "the controller writes a binding of that name without the label",
			user:      ControllerServiceAccount,
			operation: admissionv1.Create,
			in:        input(v1alpha3.ProjectAdministratorsBinding, "", administrators),
			denied:    outsideTheProject,
		},
		{
			name:      "a user writes a binding of her own",
			user:      "alice@example.com",
			operation: admissionv1.Create,
			in:        input("ci", "", administrators),
			denied:    outsideTheProject,
		},
		{
			name:      "the kinds of the subjects are still checked",
			user:      ControllerServiceAccount,
			operation: admissionv1.Create,
			in:        input(v1alpha3.ProjectAdministratorsBinding, v1alpha3.ManagedByController, []rbacv1.Subject{{Kind: "Robot", Name: "r2d2"}}),
			denied:    `has invalid kind "Robot"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := tt.in
			in.OldSubjects = tt.old
			resp := Validate(context.Background(), c, request(tt.operation, tt.user), in)
			if tt.denied == "" {
				assert.True(t, resp.Allowed, resp.Result)
				assert.Empty(t, resp.Warnings)
				return
			}
			assert.False(t, resp.Allowed)
			assert.Contains(t, resp.Result.Message, tt.denied)
		})
	}
}

// The membership of a ServiceAccount is read through the reader of the input, the direct API reader
// in the webhooks: a ProjectNamespace created just before the binding may not be in the cache yet.
func TestValidate_ServiceAccountMembershipReadsTheReader(t *testing.T) {
	cached := newClient(t, clusterRole("d8:project:viewer", nil, nil))
	live := newClient(t, &v1alpha3.ProjectNamespace{
		ObjectMeta: metav1.ObjectMeta{Name: "queue", Namespace: "proj"},
		Spec:       v1alpha3.ProjectNamespaceSpec{Name: "queue"},
	})
	in := Input{
		RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer", Namespace: "proj",
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "sa", Namespace: "proj-queue"}},
	}

	resp := Validate(context.Background(), cached, request(admissionv1.Create, ControllerServiceAccount), in)
	assert.False(t, resp.Allowed, "the cache does not have the claim yet")

	in.Reader = live
	resp = Validate(context.Background(), cached, request(admissionv1.Create, ControllerServiceAccount), in)
	assert.True(t, resp.Allowed, resp.Result)
}

func TestValidate_DisabledForProjects(t *testing.T) {
	c := newClient(t, clusterRole("d8:project:viewer", nil, map[string]string{rolebinding.AnnotationDisabledForProjects: "true"}))
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer"})
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "disabled")
}

// TestValidate_DisabledAliasOutsidePrefixes: a compat alias (d8:use:role:*) is outside the granted
// prefixes, and the prefix refusal used to win, so the author never saw the deprecation hint. The
// alias is refused with the message that names its replacement.
func TestValidate_DisabledAliasOutsidePrefixes(t *testing.T) {
	c := newClient(t, clusterRole("d8:use:role:admin", nil, map[string]string{
		rolebinding.AnnotationDisabledForProjects:  "true",
		rolebinding.AnnotationDeprecatedReplacedBy: "d8:namespace:admin",
	}))
	resp := Validate(context.Background(), c, request(admissionv1.Create, "alice"),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:use:role:admin"})
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "disabled for direct use in projects")
	assert.Contains(t, resp.Result.Message, `"d8:namespace:admin"`)

	// A missing role outside the prefixes is still the prefix refusal, for everyone.
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "cluster-admin"})
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "cannot be granted via a project role binding")
}

func TestValidate_CustomRoleLabels(t *testing.T) {
	// custom role without the required kind label is rejected
	c := newClient(t, clusterRole("d8:custom:bad", nil, nil))
	resp := Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:custom:bad"})
	assert.False(t, resp.Allowed)

	// custom role with a system scope is rejected
	c = newClient(t, clusterRole("d8:custom:sys", map[string]string{LabelRBACKind: "custom-role", LabelRBACScope: "system"}, nil))
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:custom:sys"})
	assert.False(t, resp.Allowed)

	// a well-formed custom role for the controller is allowed
	c = newClient(t, clusterRole("d8:custom:good", map[string]string{LabelRBACKind: "custom-role", LabelRBACScope: "namespace"}, nil))
	resp = Validate(context.Background(), c, request(admissionv1.Create, ControllerServiceAccount),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:custom:good"})
	assert.True(t, resp.Allowed)
}

func TestValidate_PrivilegeEscalationDeniedForUser(t *testing.T) {
	// a non-privileged user with a valid role still needs bind permission; the fake SAR returns
	// Allowed=false, so the request is denied.
	c := newClient(t, clusterRole("d8:project:viewer", nil, nil))
	resp := Validate(context.Background(), c, request(admissionv1.Create, "alice"),
		Input{RoleRefKind: "ClusterRole", RoleRefName: "d8:project:viewer"})
	assert.False(t, resp.Allowed)
}
