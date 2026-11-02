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
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	grantsv1alpha1 "controller/api/v1alpha1"
	grantsv1alpha2 "controller/api/v1alpha2"
	"controller/apis/deckhouse.io/v1alpha3"
	"controller/internal/controllers/projectgrants"
)

// grantReviewer answers the SubjectAccessReviews of the right to create grant policies and records
// them.
type grantReviewer struct {
	allowed bool
	reviews []authorizationv1.ResourceAttributes
}

func (r *grantReviewer) create(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
	review, ok := obj.(*authorizationv1.SubjectAccessReview)
	if !ok {
		return c.Create(ctx, obj, opts...)
	}
	if attributes := review.Spec.ResourceAttributes; attributes != nil && attributes.Group == "multitenancy.deckhouse.io" &&
		attributes.Resource == "clusterresourcegrantpolicies" {
		r.reviews = append(r.reviews, *attributes)
		review.Status.Allowed = r.allowed
	}
	return nil
}

func grantsProject(name string, grants *v1alpha3.ProjectGrants) *v1alpha3.Project {
	return &v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: v1alpha3.ProjectSpec{Grants: grants}}
}

func grantPolicy(name string, labels map[string]string, namespaceSelector *metav1.LabelSelector) *grantsv1alpha2.ClusterResourceGrantPolicy {
	return &grantsv1alpha2.ClusterResourceGrantPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		Spec: grantsv1alpha2.ClusterResourceGrantPolicySpec{NamespaceSelector: namespaceSelector}}
}

func grantRequest(operation admissionv1.Operation, version string) admission.Request {
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation:   operation,
		RequestKind: &metav1.GroupVersionKind{Group: "deckhouse.io", Version: version, Kind: "Project"},
		UserInfo:    authenticationv1.UserInfo{Username: "alice@example.com"},
	}}
}

func TestCheckGrants(t *testing.T) {
	storage := &v1alpha3.ProjectGrants{Resources: []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses", Allowed: []string{"fast"}}}}
	web := &v1alpha3.ProjectGrants{GrantPolicies: []string{"web"}}
	selected := &metav1.LabelSelector{MatchLabels: map[string]string{"team": "a"}}
	// valid as the name of a library policy, but "project-team-" in front of it makes 258 characters
	longName := strings.Repeat("a", 245)
	objects := []client.Object{
		grantPolicy(longName, nil, nil),
		grantPolicy("web", nil, nil),
		grantPolicy("bound", nil, selected),
		grantPolicy("project-team-taken", nil, nil),
		grantPolicy("taken", nil, nil),
		grantPolicy("b-c", nil, nil),
		grantsProject("a-b", &v1alpha3.ProjectGrants{GrantPolicies: []string{"c"}}),
		grantPolicy("c", nil, nil),
	}
	tests := []struct {
		name       string
		req        admission.Request
		stored     *v1alpha3.Project
		project    *v1alpha3.Project
		privileged bool
		allowed    bool
		denied     string
		reviewed   bool
	}{
		{name: "new grants need the right to create grant policies", req: grantRequest(admissionv1.Create, "v1alpha3"), project: grantsProject("team", storage),
			allowed: true, reviewed: true},
		{name: "without the right", req: grantRequest(admissionv1.Create, "v1alpha3"), project: grantsProject("team", storage), denied: "needs the right to create", reviewed: true},
		{name: "cleared grants need the right too", req: grantRequest(admissionv1.Update, "v1alpha3"), stored: grantsProject("team", storage),
			project: grantsProject("team", nil), denied: "needs the right to create", reviewed: true},
		{name: "unchanged grants are not checked", req: grantRequest(admissionv1.Update, "v1alpha3"), stored: grantsProject("team", storage), project: grantsProject("team", storage)},
		{name: "empty grants are no grants", req: grantRequest(admissionv1.Update, "v1alpha3"), stored: grantsProject("team", nil), project: grantsProject("team", &v1alpha3.ProjectGrants{})},
		{name: "the controller is not asked about", req: grantRequest(admissionv1.Create, "v1alpha3"), project: grantsProject("team", storage), privileged: true},
		{name: "a library policy", req: grantRequest(admissionv1.Create, "v1alpha3"), project: grantsProject("team", web), allowed: true, reviewed: true},
		{name: "without the right, the policies are not looked at", req: grantRequest(admissionv1.Create, "v1alpha3"),
			project: grantsProject("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"none"}}), denied: "needs the right to create", reviewed: true},
		{name: "a missing policy", req: grantRequest(admissionv1.Create, "v1alpha3"), project: grantsProject("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"none"}}),
			allowed: true, denied: "does not exist", reviewed: true},
		{name: "a policy with a selector", req: grantRequest(admissionv1.Create, "v1alpha3"), project: grantsProject("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"bound"}}),
			allowed: true, denied: "library policies", reviewed: true},
		{name: "the reserved inline name", req: grantRequest(admissionv1.Create, "v1alpha3"), project: grantsProject("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"inline"}}),
			allowed: true, denied: "reserved", reviewed: true},
		{name: "an empty selector", req: grantRequest(admissionv1.Create, "v1alpha3"), project: grantsProject("team", &v1alpha3.ProjectGrants{
			Resources: []grantsv1alpha1.GrantResource{{ResourceName: "storageclasses", AllowedSelector: &metav1.LabelSelector{}}}}), allowed: true, denied: "empty selector", reviewed: true},
		{name: "a managed name over 253 characters", req: grantRequest(admissionv1.Create, "v1alpha3"),
			project: grantsProject("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{longName}}), allowed: true, denied: "not a valid object name", reviewed: true},
		{name: "a managed name of a policy the project does not manage", req: grantRequest(admissionv1.Create, "v1alpha3"),
			project: grantsProject("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"taken"}}), allowed: true, denied: "exists and is not the project's", reviewed: true},
		{name: "a managed name of another project", req: grantRequest(admissionv1.Create, "v1alpha3"),
			project: grantsProject("a", &v1alpha3.ProjectGrants{GrantPolicies: []string{"b-c"}}), allowed: true, denied: "which the grants of the 'a-b' project produce", reviewed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reviewer := &grantReviewer{allowed: tt.allowed}
			v := &validator{client: newClientBuilder(t, objects...).WithInterceptorFuncs(interceptor.Funcs{Create: reviewer.create}).Build()}
			denied, err := v.checkGrants(context.Background(), tt.req, tt.project, tt.stored, tt.privileged)
			require.NoError(t, err)
			if tt.denied == "" {
				assert.Empty(t, denied)
			} else {
				assert.Contains(t, denied, tt.denied)
			}
			if tt.reviewed {
				require.Len(t, reviewer.reviews, 1)
				assert.Equal(t, "create", reviewer.reviews[0].Verb)
			} else {
				assert.Empty(t, reviewer.reviews)
			}
		})
	}
}

// The projects whose grants may produce the same managed name are listed through the API reader: the
// cache may not have seen a project created just before this one.
func TestCheckGrantsListsProjectsThroughTheAPIReader(t *testing.T) {
	reviewer := &grantReviewer{allowed: true}
	v := &validator{
		client: newClientBuilder(t, grantPolicy("b-c", nil, nil)).WithInterceptorFuncs(interceptor.Funcs{Create: reviewer.create}).Build(),
		reader: newClientBuilder(t, grantPolicy("b-c", nil, nil), grantsProject("a-b", &v1alpha3.ProjectGrants{GrantPolicies: []string{"c"}})).Build(),
	}
	denied, err := v.checkGrants(context.Background(), grantRequest(admissionv1.Create, "v1alpha3"),
		grantsProject("a", &v1alpha3.ProjectGrants{GrantPolicies: []string{"b-c"}}), nil, false)
	require.NoError(t, err)
	assert.Contains(t, denied, "which the grants of the 'a-b' project produce")
}

// A policy the project manages already keeps its name.
func TestCheckGrantsKeepsItsOwnManagedPolicy(t *testing.T) {
	owned := grantPolicy("project-team-web", map[string]string{projectgrants.LabelManagedByProject: "team"}, &metav1.LabelSelector{})
	reviewer := &grantReviewer{allowed: true}
	v := &validator{client: newClientBuilder(t, owned, grantPolicy("web", nil, nil)).WithInterceptorFuncs(interceptor.Funcs{Create: reviewer.create}).Build()}
	denied, err := v.checkGrants(context.Background(), grantRequest(admissionv1.Update, "v1alpha3"), grantsProject("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"web"}}),
		grantsProject("team", nil), false)
	require.NoError(t, err)
	assert.Empty(t, denied)
}

// A write through v1alpha2 reaches the webhook with neither the old nor the new object carrying the
// grants: the API server applies it to the stored project converted to v1alpha2. The stored project
// is what tells the write would erase them, whoever makes it.
func TestV1alpha2GrantsErasure(t *testing.T) {
	stored := grantsProject("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"web"}})
	stored.ResourceVersion = "5"
	withoutGrants := func(rv string) []byte {
		raw, err := json.Marshal(&v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team", ResourceVersion: rv}})
		require.NoError(t, err)
		return raw
	}
	request := func(version, rv string) admission.Request {
		req := grantRequest(admissionv1.Update, version)
		req.UserInfo = authenticationv1.UserInfo{Username: "q-editor@example.com"}
		req.Object.Raw = withoutGrants(rv)
		req.OldObject.Raw = withoutGrants(rv)
		return req
	}
	v := &validator{client: newClientBuilder(t, stored).Build()}
	project := grantsProject("team", nil)

	resp := v.v1alpha2GrantsErasure(context.Background(), request("v1alpha2", "5"), project)
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "would erase them")

	resp = v.v1alpha2GrantsErasure(context.Background(), request("v1alpha2", "4"), project)
	assert.False(t, resp.Allowed)
	assert.Equal(t, int32(http.StatusConflict), resp.Result.Code, "a project changed meanwhile is a conflict, which clients retry")

	assert.True(t, v.v1alpha2GrantsErasure(context.Background(), request("v1alpha3", "5"), project).Allowed, "v1alpha3 carries the grants")

	plain := &validator{client: newClientBuilder(t, grantsProject("team", nil)).Build()}
	current := new(v1alpha3.Project)
	require.NoError(t, plain.client.Get(context.Background(), client.ObjectKey{Name: "team"}, current))
	assert.True(t, plain.v1alpha2GrantsErasure(context.Background(), request("v1alpha2", current.ResourceVersion), project).Allowed,
		"a project without grants stays writable through v1alpha2")
}

// A write of projects/status through v1alpha2 erases the grants as a write of the project does, so the
// status webhook runs the same check, and only that one.
func TestStatusValidatorRefusesAV1alpha2WriteThatErasesGrants(t *testing.T) {
	stored := grantsProject("team", &v1alpha3.ProjectGrants{GrantPolicies: []string{"web"}})
	stored.ResourceVersion = "5"
	raw, err := json.Marshal(&v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: "team", ResourceVersion: "5"}})
	require.NoError(t, err)
	request := func(version string) admission.Request {
		req := grantRequest(admissionv1.Update, version)
		req.SubResource = "status"
		req.UserInfo = authenticationv1.UserInfo{Username: "legacy-client@example.com"}
		req.Object.Raw = raw
		req.OldObject.Raw = raw
		return req
	}
	s := &statusValidator{validator: &validator{client: newClientBuilder(t, stored).Build()}}

	resp := s.Handle(context.Background(), request("v1alpha2"))
	assert.False(t, resp.Allowed)
	assert.Contains(t, resp.Result.Message, "would erase them")

	assert.True(t, s.Handle(context.Background(), request("v1alpha3")).Allowed, "v1alpha3 carries the grants")

	bad := request("v1alpha2")
	bad.Object.Raw = []byte("{")
	resp = s.Handle(context.Background(), bad)
	assert.False(t, resp.Allowed)
	assert.Equal(t, int32(http.StatusBadRequest), resp.Result.Code)
}
