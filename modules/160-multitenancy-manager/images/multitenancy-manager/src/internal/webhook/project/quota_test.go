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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"controller/apis/deckhouse.io/v1alpha1"
	"controller/apis/deckhouse.io/v1alpha3"
)

// quotaReviewer answers the SubjectAccessReviews of the right to set the quota of a project, the update
// verb on projects/quota, and records them; other reviews are refused.
type quotaReviewer struct {
	allowed bool
	reviews []authorizationv1.ResourceAttributes
}

func (r *quotaReviewer) create(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
	review, ok := obj.(*authorizationv1.SubjectAccessReview)
	if !ok {
		return c.Create(ctx, obj, opts...)
	}
	attributes := review.Spec.ResourceAttributes
	if attributes != nil && attributes.Group == "deckhouse.io" && attributes.Resource == "projects" && attributes.Subresource == "quota" {
		r.reviews = append(r.reviews, *attributes)
		review.Status.Allowed = r.allowed
	}
	return nil
}

func quotaProject(quota corev1.ResourceList) *v1alpha3.Project {
	return &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team"}, Spec: v1alpha3.ProjectSpec{Quota: quota}}
}

func TestCheckQuota(t *testing.T) {
	user := authenticationv1.UserInfo{Username: "alice@example.com"}
	pods := corev1.ResourceList{"pods": resource.MustParse("10")}
	tests := []struct {
		name       string
		stored     *v1alpha3.Project
		project    *v1alpha3.Project
		existing   *v1alpha1.ProjectQuotaStatus
		privileged bool
		allowed    bool
		denied     string
		verb       string
		warnings   []string
	}{
		{name: "a new quota needs the right to set the quota", project: quotaProject(pods), allowed: true, verb: "update"},
		{name: "without the right", project: quotaProject(pods), denied: "needs the right to update projects/quota", verb: "update"},
		{name: "a changed quota", stored: quotaProject(corev1.ResourceList{"pods": resource.MustParse("5")}), project: quotaProject(pods), allowed: true, verb: "update"},
		{name: "a cleared quota", stored: quotaProject(pods), project: quotaProject(nil), allowed: true, verb: "update"},
		{name: "an unchanged quota is not checked", stored: quotaProject(pods), project: quotaProject(pods)},
		{name: "the controller is not asked about", project: quotaProject(pods), privileged: true},
		{name: "a ProjectQuotaStatus the controller made", stored: quotaProject(corev1.ResourceList{"pods": resource.MustParse("5")}), project: quotaProject(pods),
			existing: &v1alpha1.ProjectQuotaStatus{ObjectMeta: metav1.ObjectMeta{Name: "team", Labels: map[string]string{v1alpha3.ResourceLabelManagedBy: v1alpha3.ManagedByController}}},
			allowed:  true, verb: "update"},
		{name: "a limit lowered below the usage is allowed with a warning", stored: quotaProject(corev1.ResourceList{"pods": resource.MustParse("20"), "requests.cpu": resource.MustParse("8")}),
			project: quotaProject(corev1.ResourceList{"pods": resource.MustParse("5"), "requests.cpu": resource.MustParse("8")}),
			existing: &v1alpha1.ProjectQuotaStatus{ObjectMeta: metav1.ObjectMeta{Name: "team"},
				Status: v1alpha1.ProjectQuotaStatusStatus{Used: corev1.ResourceList{"pods": resource.MustParse("12"), "requests.cpu": resource.MustParse("9")}}},
			allowed: true, verb: "update",
			warnings: []string{"pods: the limit 5 is below what the project uses, 12; nothing running is evicted, and creating more is denied until the usage is back within the limit"}},
		{name: "the usage of another project", project: quotaProject(pods),
			existing: &v1alpha1.ProjectQuotaStatus{ObjectMeta: metav1.ObjectMeta{Name: "other"}, Status: v1alpha1.ProjectQuotaStatusStatus{Used: corev1.ResourceList{"pods": resource.MustParse("50")}}},
			allowed:  true, verb: "update"},
		{name: "a refused key", project: quotaProject(corev1.ResourceList{"resourcequotas": resource.MustParse("1")}), allowed: true, denied: "quota of the 'team' project is invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewer := &quotaReviewer{allowed: tt.allowed}
			builder := newClientBuilder(t)
			if tt.existing != nil {
				builder = builder.WithObjects(tt.existing)
			}
			v := &validator{client: builder.WithInterceptorFuncs(interceptor.Funcs{Create: reviewer.create}).Build()}
			denied, warnings, err := v.checkQuota(context.Background(), user, tt.project, tt.stored, tt.privileged)
			require.NoError(t, err)
			assert.Equal(t, tt.warnings, warnings)
			if tt.denied == "" {
				assert.Empty(t, denied)
			} else {
				assert.Contains(t, denied, tt.denied)
			}
			if tt.verb == "" {
				assert.Empty(t, reviewer.reviews)
				return
			}
			require.Len(t, reviewer.reviews, 1)
			assert.Equal(t, tt.verb, reviewer.reviews[0].Verb)
			assert.Equal(t, "team", reviewer.reviews[0].Name)
		})
	}
}
