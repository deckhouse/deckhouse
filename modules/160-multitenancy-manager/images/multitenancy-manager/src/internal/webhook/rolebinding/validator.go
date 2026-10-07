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

// Package rolebinding holds the shared admission validation logic for ProjectRoleBinding and
// ClusterProjectRoleBinding.
package rolebinding

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebinding "controller/internal/rolebinding"
	projectnamespacewebhook "controller/internal/webhook/projectnamespace"
)

const (
	// ControllerServiceAccount/DeckhouseServiceAccount re-export the privileged identities from the
	// shared internal/rolebinding package, so the literal values live in exactly one place while the
	// existing consumers of this webhook package keep referencing them here.
	ControllerServiceAccount = rolebinding.ControllerServiceAccount
	DeckhouseServiceAccount  = rolebinding.DeckhouseServiceAccount

	LabelRBACKind  = "rbac.deckhouse.io/kind"
	LabelRBACScope = "rbac.deckhouse.io/scope"

	customRolePrefix = "d8:custom:"
)

// Input carries the binding fields needed for validation.
type Input struct {
	RoleRefKind string
	RoleRefName string
	// Subjects are the binding subjects (validated for kind and, for ServiceAccounts, namespace).
	Subjects []rbacv1.Subject
	// OldSubjects are the subjects of the stored binding on UPDATE. A ServiceAccount subject the
	// stored binding already has is not refused for project membership again, so an update that
	// does not touch it -- the controller's own finalizer updates among them -- is never refused
	// for it. The response warns about it instead when its namespace is no longer a namespace of
	// the project, as the fan-out then gives it no rights.
	OldSubjects []rbacv1.Subject
	// Name is the name of the binding. The administrators binding the controller writes (see
	// rolebinding.IsAdministratorsBinding) is told by it.
	Name string
	// Namespace is the request namespace for a ProjectRoleBinding; empty for a ClusterProjectRoleBinding.
	Namespace string
	// ManagedBy is the managed-by marking to judge the object by, as resolved by ResolveManagedBy.
	ManagedBy string
	// Reader reads the namespaces and the ProjectNamespace claims that the membership of a
	// ServiceAccount subject is judged by. The webhooks pass the direct API reader, so that a
	// ProjectNamespace created just before the binding is seen; without one the client is used.
	Reader client.Reader
}

// ResolveManagedBy returns the managed-by marking the protection must be judged by.
//
// On UPDATE the old object counts as well. Reading only the incoming object would let one request
// both strip the label and make the change the label exists to prevent, since by the time the
// validation runs the marking is already gone. modules/140-user-authz/webhooks/validating/
// system_resources.py reads its markings from both objects for the same reason.
func ResolveManagedBy(operation admissionv1.Operation, object, oldObject map[string]string) string {
	if object[v1alpha3.ResourceLabelManagedBy] == v1alpha3.ManagedByController {
		return v1alpha3.ManagedByController
	}
	if operation == admissionv1.Update && oldObject[v1alpha3.ResourceLabelManagedBy] == v1alpha3.ManagedByController {
		return v1alpha3.ManagedByController
	}

	return object[v1alpha3.ResourceLabelManagedBy]
}

// Validate runs the shared validation for PRB/CPRB admission requests.
func Validate(ctx context.Context, c client.Client, req admission.Request, in Input) admission.Response {
	user := req.UserInfo.Username
	privileged := user == ControllerServiceAccount || user == DeckhouseServiceAccount

	// managed-by protection: controller-managed bindings can only be changed by the controller.
	if in.ManagedBy == v1alpha3.ManagedByController && !privileged {
		return admission.Denied(fmt.Sprintf("the binding is managed by the controller (label %s=%s) and cannot be modified",
			v1alpha3.ResourceLabelManagedBy, v1alpha3.ManagedByController))
	}

	// delete only needs the managed-by protection above
	if req.Operation == admissionv1.Delete {
		return admission.Allowed("")
	}

	if in.RoleRefKind != "ClusterRole" {
		return admission.Denied("roleRef.kind must be ClusterRole")
	}

	// only an UPDATE has a stored binding whose subjects are kept
	if req.Operation != admissionv1.Update {
		in.OldSubjects = nil
	}
	reader := client.Reader(c)
	if in.Reader != nil {
		reader = in.Reader
	}
	// The controller writes the administrators binding of a project from spec.administrators, which only
	// a writer of the Project sets, and the fan-out carries them whatever namespace they name. The
	// upgrade lifts administrators that name the ServiceAccounts of another project, which an
	// AuthorizationRule granted the rights before, so holding these writes to the project would leave
	// every administrator of such a project without the binding. Only the controller is let through
	// here; Deckhouse, which may change the binding too, is held to the project like everyone else.
	project := in.Namespace
	if user == ControllerServiceAccount && rolebinding.IsAdministratorsBinding(in.Name, in.ManagedBy) {
		project = ""
	}
	// The warnings go out with whichever response allows the request.
	denied, warnings, err := validateSubjects(ctx, reader, in, project)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}
	if denied != "" {
		return admission.Denied(denied)
	}

	prefixDenied := func() admission.Response {
		return admission.Denied(fmt.Sprintf(
			"ClusterRole %q cannot be granted via a project role binding; allowed: d8:project:*, d8:namespace:*, their capabilities and d8:custom:*",
			in.RoleRefName))
	}

	// The role is looked up before the prefix check so that a deprecated alias (d8:use:role:*) --
	// whose name is outside the granted prefixes -- still gets the message that names its
	// replacement, instead of the generic prefix refusal.
	clusterRole := &rbacv1.ClusterRole{}
	if err := c.Get(ctx, client.ObjectKey{Name: in.RoleRefName}, clusterRole); err != nil {
		if apierrors.IsNotFound(err) {
			if !rolebinding.IsRoleAllowed(in.RoleRefName) {
				return prefixDenied()
			}
			// Fail closed: a non-privileged user must not be able to pre-create a binding to a
			// not-yet-existing role and thus skip the scope/label and privilege-escalation checks
			// below. Only the controller/Deckhouse may reference an absent role.
			if privileged {
				return admission.Allowed("").WithWarnings(warnings...).WithWarnings(fmt.Sprintf("ClusterRole %q does not exist", in.RoleRefName))
			}
			return admission.Denied(fmt.Sprintf(
				"ClusterRole %q does not exist; it must exist before it can be granted via a project role binding",
				in.RoleRefName))
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if clusterRole.Annotations[rolebinding.AnnotationDisabledForProjects] == "true" {
		msg := fmt.Sprintf("ClusterRole %q is disabled for direct use in projects", in.RoleRefName)
		if replacement := clusterRole.Annotations[rolebinding.AnnotationDeprecatedReplacedBy]; replacement != "" {
			msg += fmt.Sprintf("; use %q instead", replacement)
		}
		return admission.Denied(msg)
	}

	if !rolebinding.IsRoleAllowed(in.RoleRefName) {
		return prefixDenied()
	}

	if strings.HasPrefix(in.RoleRefName, customRolePrefix) {
		kind := clusterRole.Labels[LabelRBACKind]
		if kind != "custom-role" && kind != "custom-capability" {
			return admission.Denied(fmt.Sprintf("ClusterRole %q must have label %s in {custom-role, custom-capability}", in.RoleRefName, LabelRBACKind))
		}
		if scope := clusterRole.Labels[LabelRBACScope]; scope == "system" || scope == "subsystem" {
			return admission.Denied(fmt.Sprintf("ClusterRole %q has scope %q which cannot be granted via a project role binding", in.RoleRefName, scope))
		}
	}

	// privilege escalation check: the requesting user must be allowed to bind the ClusterRole.
	if !privileged {
		allowed, reason, err := canBind(ctx, c, req, in)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
		if !allowed {
			return admission.Denied(fmt.Sprintf("user %q is not allowed to bind ClusterRole %q: %s", user, in.RoleRefName, reason))
		}
	}

	return admission.Allowed("").WithWarnings(warnings...)
}

// validateSubjects checks subject kinds and constrains the subjects that name ServiceAccounts to the
// project. It returns a denial message, or an empty string when the subjects are valid, and the
// warnings for the response that allows the request.
// projectNamespace is the project's main namespace for a ProjectRoleBinding, or empty for a
// cluster-scoped ClusterProjectRoleBinding (which spans all projects and cannot be constrained here)
// and for the administrators binding the controller writes.
// Besides a ServiceAccount subject, the User "system:serviceaccount:<namespace>:<name>" and the Group
// "system:serviceaccounts:<namespace>" name ServiceAccounts (see rolebinding.ServiceAccountNamespace),
// and the same rule holds for them: the fan-out would leave them out of the RoleBindings anyway.
// A subject in in.OldSubjects is not refused for project membership again, only warned about.
func validateSubjects(ctx context.Context, c client.Reader, in Input, projectNamespace string) (string, []string, error) {
	var warnings []string
	for _, s := range in.Subjects {
		if s.Name == "" {
			return "subject name must not be empty", nil, nil
		}
		switch s.Kind {
		case rbacv1.UserKind, rbacv1.GroupKind:
		case rbacv1.ServiceAccountKind:
			if s.Namespace == "" {
				return fmt.Sprintf("ServiceAccount subject %q must set a namespace", s.Name), nil, nil
			}
		default:
			return fmt.Sprintf("subject %q has invalid kind %q: must be User, Group or ServiceAccount", s.Name, s.Kind), nil, nil
		}

		namespace, namesServiceAccounts := rolebinding.ServiceAccountNamespace(s)
		if !namesServiceAccounts || projectNamespace == "" {
			continue
		}
		// For a ProjectRoleBinding the ServiceAccounts must belong to the project: its main
		// namespace (== project name) or one of its additional namespaces.
		belongs, err := namespaceOfProject(ctx, c, namespace, projectNamespace)
		if slices.Contains(in.OldSubjects, s) {
			// A stored subject is never refused, so a failed read costs the warning alone.
			if err != nil {
				ctrllog.FromContext(ctx).Error(err, "check the project membership of a stored ServiceAccount subject",
					"subject", s.Kind+" "+s.Name, "namespace", namespace)
				continue
			}
			if !belongs {
				warnings = append(warnings, noLongerInProject(s, namespace, projectNamespace))
			}
			continue
		}
		if err != nil {
			return "", nil, err
		}
		if !belongs {
			if s.Kind == rbacv1.ServiceAccountKind {
				return fmt.Sprintf("ServiceAccount subject namespace %q must belong to project %q", namespace, projectNamespace), nil, nil
			}
			return fmt.Sprintf("%s subject %q names ServiceAccounts of namespace %q, which must belong to project %q",
				s.Kind, s.Name, namespace, projectNamespace), nil, nil
		}
	}
	return "", warnings, nil
}

// noLongerInProject is the warning about a stored subject whose ServiceAccounts are in a namespace
// that is not the project's any more.
func noLongerInProject(s rbacv1.Subject, namespace, project string) string {
	if s.Kind == rbacv1.ServiceAccountKind {
		return fmt.Sprintf("ServiceAccount %s/%s is no longer in a namespace of project %q and gets no rights from this binding",
			namespace, s.Name, project)
	}
	return fmt.Sprintf("%s %q names ServiceAccounts of namespace %q, which is no longer in a namespace of project %q, "+
		"and gets no rights from this binding", s.Kind, s.Name, namespace, project)
}

// namespaceOfProject reports whether the namespace is the main namespace of the project or one of
// its additional namespaces. An additional namespace carries the project label; one a
// ProjectNamespace of the project has claimed but the controller has not created yet belongs to the
// project as well. A name that merely starts with the project name and a dash does not: it can be
// the main namespace of a separate project.
func namespaceOfProject(ctx context.Context, c client.Reader, namespace, project string) (bool, error) {
	if namespace == project {
		return true, nil
	}

	ns := new(corev1.Namespace)
	switch err := c.Get(ctx, client.ObjectKey{Name: namespace}, ns); {
	case err == nil:
		if owner := ns.Labels[v1alpha3.ResourceLabelProject]; owner != "" {
			return owner == project, nil
		}
	case !apierrors.IsNotFound(err):
		return false, fmt.Errorf("get the %q namespace: %w", namespace, err)
	}

	claim, err := projectnamespacewebhook.Claiming(ctx, c, namespace)
	if err != nil {
		return false, err
	}
	return claim != nil && claim.Namespace == project, nil
}

func canBind(ctx context.Context, c client.Client, req admission.Request, in Input) (bool, string, error) {
	extra := map[string]authorizationv1.ExtraValue{}
	for k, v := range req.UserInfo.Extra {
		extra[k] = authorizationv1.ExtraValue(v)
	}
	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   req.UserInfo.Username,
			Groups: req.UserInfo.Groups,
			UID:    req.UserInfo.UID,
			Extra:  extra,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: in.Namespace,
				Verb:      "bind",
				Group:     rbacv1.GroupName,
				Resource:  "clusterroles",
				Name:      in.RoleRefName,
			},
		},
	}
	if err := c.Create(ctx, sar); err != nil {
		return false, "", fmt.Errorf("create SubjectAccessReview: %w", err)
	}
	return sar.Status.Allowed, sar.Status.Reason, nil
}
