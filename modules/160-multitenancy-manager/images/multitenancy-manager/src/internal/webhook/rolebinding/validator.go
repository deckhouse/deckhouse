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
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebinding "controller/internal/rolebinding"
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
	// Namespace is the request namespace for a ProjectRoleBinding; empty for a ClusterProjectRoleBinding.
	Namespace string
	// ManagedBy is the value of the projects.deckhouse.io/managed-by label on the object (or its old version on delete).
	ManagedBy string
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

	if denied := validateSubjects(in.Subjects, in.Namespace); denied != "" {
		return admission.Denied(denied)
	}

	if !rolebinding.IsRoleAllowed(in.RoleRefName) {
		return admission.Denied(fmt.Sprintf(
			"ClusterRole %q cannot be granted via a project role binding; allowed: d8:project:*, d8:namespace:*, their capabilities and d8:custom:*",
			in.RoleRefName))
	}

	clusterRole := &rbacv1.ClusterRole{}
	if err := c.Get(ctx, client.ObjectKey{Name: in.RoleRefName}, clusterRole); err != nil {
		if apierrors.IsNotFound(err) {
			// Fail closed: a non-privileged user must not be able to pre-create a binding to a
			// not-yet-existing role and thus skip the scope/label and privilege-escalation checks
			// below. Only the controller/Deckhouse may reference an absent role.
			if privileged {
				return admission.Allowed("").WithWarnings(fmt.Sprintf("ClusterRole %q does not exist", in.RoleRefName))
			}
			return admission.Denied(fmt.Sprintf(
				"ClusterRole %q does not exist; it must exist before it can be granted via a project role binding",
				in.RoleRefName))
		}
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if clusterRole.Annotations[rolebinding.AnnotationDisabledForProjects] == "true" {
		return admission.Denied(fmt.Sprintf("ClusterRole %q is disabled for direct use in projects", in.RoleRefName))
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

	return admission.Allowed("")
}

// validateSubjects checks subject kinds and constrains ServiceAccount subjects to the project.
// projectNamespace is the project's main namespace for a ProjectRoleBinding, or empty for a
// cluster-scoped ClusterProjectRoleBinding (which spans all projects and cannot be constrained here).
func validateSubjects(subjects []rbacv1.Subject, projectNamespace string) string {
	for _, s := range subjects {
		if s.Name == "" {
			return "subject name must not be empty"
		}
		switch s.Kind {
		case rbacv1.UserKind, rbacv1.GroupKind:
		case rbacv1.ServiceAccountKind:
			if s.Namespace == "" {
				return fmt.Sprintf("ServiceAccount subject %q must set a namespace", s.Name)
			}
			// For a ProjectRoleBinding the ServiceAccount must belong to the project: its main
			// namespace (== project name) or one of its additional "<project>-*" namespaces.
			if projectNamespace != "" &&
				s.Namespace != projectNamespace &&
				!strings.HasPrefix(s.Namespace, projectNamespace+"-") {
				return fmt.Sprintf("ServiceAccount subject namespace %q must belong to project %q", s.Namespace, projectNamespace)
			}
		default:
			return fmt.Sprintf("subject %q has invalid kind %q: must be User, Group or ServiceAccount", s.Name, s.Kind)
		}
	}
	return ""
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
