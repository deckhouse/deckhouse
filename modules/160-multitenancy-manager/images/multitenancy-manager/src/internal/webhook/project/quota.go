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
	"fmt"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/quota"
)

// checkQuota checks a change of spec.quota. The quota becomes the ProjectQuota of the project, so its
// keys follow the rules of a ProjectQuota, and the requester needs the right to write that
// ProjectQuota: whoever may change a project may not give it a quota of their own choosing. A write
// that leaves spec.quota as it was is not checked, and neither are the controller and Deckhouse. A quota
// is refused while a ProjectQuota created directly bears the name of the project: spec.quota does not
// take such an object over, so it would not apply.
func (v *validator) checkQuota(ctx context.Context, user authenticationv1.UserInfo, project, stored *v1alpha3.Project, privileged bool) (string, []string, error) {
	var before corev1.ResourceList
	if stored != nil {
		before = stored.Spec.Quota
	}
	if equality.Semantic.DeepEqual(before, project.Spec.Quota) {
		return "", nil, nil
	}

	check, err := quota.CheckKeys(project.Spec.Quota, before, v.quotaKeys)
	if err != nil {
		return "", nil, err
	}
	if len(check.Denied) > 0 {
		return fmt.Sprintf("The quota of the '%s' project is invalid: %s", project.Name, strings.Join(check.Denied, "; ")), nil, nil
	}
	if privileged {
		return "", check.Warnings, nil
	}

	verb := "update"
	switch {
	case len(before) == 0:
		verb = "create"
	case len(project.Spec.Quota) == 0:
		verb = "delete"
	}
	allowed, err := v.mayWriteProjectQuota(ctx, user, verb, project.Name)
	if err != nil {
		return "", nil, err
	}
	if !allowed {
		return fmt.Sprintf("Changing spec.quota of the '%s' project needs the right to %s the '%s' ProjectQuota (projectquotas.deckhouse.io), "+
			"which the manage_quotas capability of multitenancy-manager grants", project.Name, verb, project.Name), nil, nil
	}
	if len(project.Spec.Quota) == 0 {
		return "", check.Warnings, nil
	}
	direct, err := v.hasDirectProjectQuota(ctx, project.Name)
	if err != nil {
		return "", nil, err
	}
	if direct {
		return fmt.Sprintf("The '%s' ProjectQuota was created directly, so spec.quota would not apply to it: "+
			"delete that ProjectQuota to let spec.quota make it, or leave spec.quota empty", project.Name), nil, nil
	}
	return "", check.Warnings, nil
}

// hasDirectProjectQuota reports a ProjectQuota of the name that the controller did not make from spec.quota.
func (v *validator) hasDirectProjectQuota(ctx context.Context, name string) (bool, error) {
	pq := new(v1alpha1.ProjectQuota)
	if err := v.apiReader().Get(ctx, client.ObjectKey{Name: name}, pq); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("get the '%s' ProjectQuota: %w", name, err)
	}
	return pq.Labels[v1alpha3.ResourceLabelManagedBy] != v1alpha3.ManagedByController, nil
}

// mayWriteProjectQuota asks the API server whether the user may make the change to the ProjectQuota.
func (v *validator) mayWriteProjectQuota(ctx context.Context, user authenticationv1.UserInfo, verb, name string) (bool, error) {
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
				Group:    v1alpha1.SchemeGroupVersion.Group,
				Resource: v1alpha1.ProjectQuotaResource,
				Verb:     verb,
				Name:     name,
			},
		},
	}
	if err := v.client.Create(ctx, review); err != nil {
		return false, fmt.Errorf("review the right to %s the '%s' ProjectQuota: %w", verb, name, err)
	}
	return review.Status.Allowed, nil
}
