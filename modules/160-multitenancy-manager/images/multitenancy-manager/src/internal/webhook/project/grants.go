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

package project

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/controllers/projectgrants"
	"controller/internal/controllers/templategrants"
)

// grantPolicyResource is what spec.grants turns into, and so what changing it needs the right to create.
const grantPolicyResource = "clusterresourcegrantpolicies"

// v1alpha2GrantsErasure refuses a write through deckhouse.io/v1alpha2 of a project that has grants:
// that version cannot describe them, so the write would erase them, whoever makes it. The request
// cannot tell: the API server applies such a write to the stored project converted to v1alpha2, and
// converts both the old and the new object to v1alpha3 for the webhook, without the grants. So the
// stored project is read at v1alpha3, past the cache. It has to be the version oldObject shows, or
// this would judge a project the request does not replace; a changed one is refused as a conflict, the
// answer clients retry on.
func (v *validator) v1alpha2GrantsErasure(ctx context.Context, req admission.Request, project *v1alpha3.Project) admission.Response {
	if req.Operation != admissionv1.Update || !writtenThroughV1alpha2(req) {
		return admission.Allowed("")
	}
	old := new(v1alpha3.Project)
	if err := json.Unmarshal(req.OldObject.Raw, old); err != nil {
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("decode the old project: %w", err))
	}
	stored := new(v1alpha3.Project)
	switch err := v.apiReader().Get(ctx, client.ObjectKey{Name: project.Name}, stored); {
	case apierrors.IsNotFound(err):
		// deleted meanwhile: the update fails in the storage, and there is nothing to erase
		return admission.Allowed("")
	case err != nil:
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("get the '%s' project: %w", project.Name, err))
	}
	if stored.ResourceVersion != old.ResourceVersion {
		return admission.Response{AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Code:   http.StatusConflict,
				Reason: metav1.StatusReasonConflict,
				Message: fmt.Sprintf("the '%s' project changed while this request through %s was checked; retry the request",
					project.Name, v1alpha2.SchemeGroupVersion),
			},
		}}
	}
	if !stored.Spec.Grants.IsEmpty() {
		return admission.Denied(fmt.Sprintf("the '%s' project has spec.grants, which %s cannot describe; a request through %s would erase them, "+
			"so change the project through %s", project.Name, v1alpha2.SchemeGroupVersion, v1alpha2.SchemeGroupVersion, v1alpha3.SchemeGroupVersion))
	}
	return admission.Allowed("")
}

// statusValidator checks a write of the projects/status subresource. Such a write replaces the status
// only, but through deckhouse.io/v1alpha2 the API server applies it to the stored project converted to
// v1alpha2 and converts the result back, which drops spec.grants as well. So it gets the same erasure
// check as a write of the project itself, and nothing else: the status is the controller's.
type statusValidator struct {
	validator *validator
}

func (s *statusValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	project := new(v1alpha3.Project)
	if err := json.Unmarshal(req.Object.Raw, project); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	return s.validator.v1alpha2GrantsErasure(ctx, req, project)
}

// checkGrants checks spec.grants. A change of the grants needs the
// right to create ClusterResourceGrantPolicy objects, which the grants become: whoever may change a
// project may not grant it cluster-wide resources of their own choosing. The right is checked first,
// so that the answers of the checks below, which look at the cluster-wide policies, go only to who
// may create them. The grants are checked the way the grants of a ProjectTemplate are, and their
// managed policies must not take a name that is not theirs. A write that leaves the grants as they
// were is not checked, and the controller and Deckhouse are not asked for the right.
func (v *validator) checkGrants(ctx context.Context, req admission.Request, project, stored *v1alpha3.Project, privileged bool) (string, error) {
	var before *v1alpha3.ProjectGrants
	if stored != nil {
		before = stored.Spec.Grants
	}
	if equality.Semantic.DeepEqual(normalGrants(before), normalGrants(project.Spec.Grants)) {
		return "", nil
	}
	if reason, err := v.grantRight(ctx, req.UserInfo, project, privileged); reason != "" || err != nil {
		return reason, err
	}
	if project.Spec.Grants.IsEmpty() {
		return "", nil
	}

	if reason := inlineSelectors(project); reason != "" {
		return reason, nil
	}
	if reason, err := v.libraryPolicies(ctx, project); reason != "" || err != nil {
		return reason, err
	}
	return v.managedNamesFree(ctx, project)
}

// normalGrants is nil for grants that grant nothing, so that leaving out the field and writing it
// empty are the same.
func normalGrants(grants *v1alpha3.ProjectGrants) *v1alpha3.ProjectGrants {
	if grants.IsEmpty() {
		return nil
	}
	return grants
}

// grantRight asks the API server whether the user may create ClusterResourceGrantPolicy objects.
func (v *validator) grantRight(ctx context.Context, user authenticationv1.UserInfo, project *v1alpha3.Project, privileged bool) (string, error) {
	if privileged {
		return "", nil
	}
	extra := make(map[string]authorizationv1.ExtraValue, len(user.Extra))
	for key, values := range user.Extra {
		extra[key] = authorizationv1.ExtraValue(values)
	}
	review := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   user.Username,
			UID:    user.UID,
			Groups: user.Groups,
			Extra:  extra,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:    grantsv1alpha2.GroupVersion.Group,
				Resource: grantPolicyResource,
				Verb:     "create",
			},
		},
	}
	if err := v.client.Create(ctx, review); err != nil {
		return "", fmt.Errorf("review the right to create %s: %w", grantPolicyResource, err)
	}
	if !review.Status.Allowed {
		return fmt.Sprintf("Changing spec.grants of the '%s' project needs the right to create %s.%s, which the grants become",
			project.Name, grantPolicyResource, grantsv1alpha2.GroupVersion.Group), nil
	}
	return "", nil
}

// inlineSelectors refuses an empty or invalid selector in spec.grants.resources: an empty selector
// would select every cluster-wide resource of the type.
func inlineSelectors(project *v1alpha3.Project) string {
	for i := range project.Spec.Grants.Resources {
		entry := &project.Spec.Grants.Resources[i]
		for _, sel := range []struct {
			field    string
			selector *metav1.LabelSelector
		}{
			{"allowedSelector", entry.AllowedSelector},
			{"deniedSelector", entry.DeniedSelector},
		} {
			if sel.selector == nil {
				continue
			}
			if len(sel.selector.MatchLabels) == 0 && len(sel.selector.MatchExpressions) == 0 {
				return fmt.Sprintf("the '%s' project field 'spec.grants.resources[%d].%s' (%s) is an empty selector: a selector must carry "+
					"matchLabels or matchExpressions; omit the field to grant nothing through it", project.Name, i, sel.field, entry.ResourceName)
			}
			if _, err := metav1.LabelSelectorAsSelector(sel.selector); err != nil {
				return fmt.Sprintf("the '%s' project field 'spec.grants.resources[%d].%s' (%s) is not a valid selector: %v",
					project.Name, i, sel.field, entry.ResourceName, err)
			}
		}
	}
	return ""
}

// libraryPolicies requires every policy of spec.grants.grantPolicies to exist and to have no
// selector: a policy with one is bound to its own namespaces already. "inline" names the policy of
// the inline resources. The managed policy of a reference must be a valid object name, which a long
// library policy name makes impossible: the controller could never create it.
func (v *validator) libraryPolicies(ctx context.Context, project *v1alpha3.Project) (string, error) {
	for _, name := range project.Spec.Grants.GrantPolicies {
		if name == templategrants.GrantSourceInline {
			return fmt.Sprintf("the '%s' project references a grant policy named '%s', which is reserved for the policy of its inline resources; "+
				"rename the referenced policy", project.Name, name), nil
		}
		if problems := validation.IsDNS1123Subdomain(projectgrants.PolicyName(project.Name, name)); len(problems) > 0 {
			return fmt.Sprintf("the '%s' project references ClusterResourceGrantPolicy '%s', whose managed policy name '%s' is not a valid object name (%s); "+
				"use a shorter policy or project name", project.Name, name, projectgrants.PolicyName(project.Name, name), strings.Join(problems, "; ")), nil
		}
		policy := new(grantsv1alpha2.ClusterResourceGrantPolicy)
		if err := v.apiReader().Get(ctx, client.ObjectKey{Name: name}, policy); err != nil {
			if apierrors.IsNotFound(err) {
				return fmt.Sprintf("the '%s' project references ClusterResourceGrantPolicy '%s', which does not exist", project.Name, name), nil
			}
			return "", fmt.Errorf("get the '%s' grant policy: %w", name, err)
		}
		if !policy.Spec.IsLibrary() {
			return fmt.Sprintf("the '%s' project references ClusterResourceGrantPolicy '%s', which has a projectSelector or a namespaceSelector; "+
				"spec.grants.grantPolicies may only reference library policies (without either selector)", project.Name, name), nil
		}
	}
	return "", nil
}

// managedNamesFree refuses grants whose managed policies would take a name that is not the project's:
// one of an existing policy the project does not manage, or one that the grants of another project
// produce. "project-<project>-<source>" joins its parts with '-', so project "a" with policy "b-c" and
// project "a-b" with policy "c" make the same name.
func (v *validator) managedNamesFree(ctx context.Context, project *v1alpha3.Project) (string, error) {
	own := projectgrants.ManagedNames(project)
	for _, name := range own {
		existing := new(grantsv1alpha2.ClusterResourceGrantPolicy)
		switch err := v.apiReader().Get(ctx, client.ObjectKey{Name: name}, existing); {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return "", fmt.Errorf("get the '%s' grant policy: %w", name, err)
		}
		if existing.Labels[projectgrants.LabelManagedByProject] != project.Name {
			return fmt.Sprintf("the grants of the '%s' project would produce the managed ClusterResourceGrantPolicy '%s', which exists and is not "+
				"the project's; rename that policy or the referenced one", project.Name, name), nil
		}
	}

	// The projects come from the API server: a project created just before this one, as a GitOps sync
	// applies both, may not be in the cache yet, and on a replica that is not the leader a cache read
	// would start the Project informer inside the request.
	projects := new(v1alpha3.ProjectList)
	if err := v.apiReader().List(ctx, projects); err != nil {
		return "", fmt.Errorf("list the projects: %w", err)
	}
	ownNames := make(map[string]struct{}, len(own))
	for _, name := range own {
		ownNames[name] = struct{}{}
	}
	for i := range projects.Items {
		other := &projects.Items[i]
		if other.Name == project.Name {
			continue
		}
		for _, name := range projectgrants.ManagedNames(other) {
			if _, clash := ownNames[name]; clash {
				return fmt.Sprintf("the grants of the '%s' project would produce the managed ClusterResourceGrantPolicy '%s', which the grants of the "+
					"'%s' project produce; rename the referenced policy", project.Name, name, other.Name), nil
			}
		}
	}
	return "", nil
}
