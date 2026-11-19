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

// checkQuota checks a change of spec.quota. spec.quota is the one place the quota of a project is set:
// the controller turns it into the ProjectQuotaStatus of the project, which nobody else writes. Its keys
// follow the rules of the project quota, and the requester needs the right to set the quota of the
// project: whoever may change a project may not give it a quota of their own choosing. A write that
// leaves spec.quota as it was is not checked, and neither are the controller and Deckhouse. A limit set
// below what the project uses already is allowed with a warning.
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

	allowed, err := v.maySetQuota(ctx, user, project.Name)
	if err != nil {
		return "", nil, err
	}
	if !allowed {
		return fmt.Sprintf("Changing spec.quota of the '%s' project needs the right to update projects/quota of deckhouse.io for it, "+
			"which the manage_quotas capability of multitenancy-manager grants", project.Name), nil, nil
	}
	if len(project.Spec.Quota) == 0 {
		return "", check.Warnings, nil
	}
	used, err := v.projectUsage(ctx, project.Name)
	if err != nil {
		return "", nil, err
	}
	return "", append(check.Warnings, quota.BelowUsage(project.Spec.Quota, before, used)...), nil
}

// projectUsage reads the usage of the project from its ProjectQuotaStatus: nil when the project has no
// quota yet.
func (v *validator) projectUsage(ctx context.Context, name string) (corev1.ResourceList, error) {
	pq := new(v1alpha1.ProjectQuotaStatus)
	if err := v.apiReader().Get(ctx, client.ObjectKey{Name: name}, pq); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("get the '%s' ProjectQuotaStatus: %w", name, err)
	}
	return pq.Status.Used, nil
}

// maySetQuota asks the API server whether the user may set the quota of the project: the update verb
// on its quota subresource, projects/quota. The subresource is not served; the right is only asked about,
// so that it is granted apart from the right to change the project.
func (v *validator) maySetQuota(ctx context.Context, user authenticationv1.UserInfo, name string) (bool, error) {
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
				Group:       v1alpha3.SchemeGroupVersion.Group,
				Resource:    v1alpha3.ProjectResource,
				Subresource: v1alpha3.ProjectQuotaSubresource,
				Verb:        "update",
				Name:        name,
			},
		},
	}
	if err := v.client.Create(ctx, review); err != nil {
		return false, fmt.Errorf("review the right to set the quota of the '%s' project: %w", name, err)
	}
	return review.Status.Allowed, nil
}
