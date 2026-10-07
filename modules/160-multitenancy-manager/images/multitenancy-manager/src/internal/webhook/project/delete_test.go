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
	"errors"
	"net/http"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	authnv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"controller/apis/deckhouse.io/v1alpha3"
	rolebindingwebhook "controller/internal/webhook/rolebinding"
)

// namespaceReviewer answers the SubjectAccessReviews the webhook sends on a project delete: the
// requester may delete the namespaces in deletable and no other, and only when the review asks what
// the API server asks for a namespace delete. Every review is recorded; a set err fails them all.
type namespaceReviewer struct {
	deletable []string
	err       error
	reviews   []authorizationv1.SubjectAccessReviewSpec
}

func (r *namespaceReviewer) create(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
	review, ok := obj.(*authorizationv1.SubjectAccessReview)
	if !ok {
		return c.Create(ctx, obj, opts...)
	}
	if r.err != nil {
		return r.err
	}
	r.reviews = append(r.reviews, review.Spec)

	attributes := review.Spec.ResourceAttributes
	isNamespaceDelete := attributes != nil &&
		attributes.Group == "" &&
		attributes.Resource == "namespaces" &&
		attributes.Verb == "delete" &&
		attributes.Namespace == attributes.Name
	review.Status.Allowed = isNamespaceDelete && slices.Contains(r.deletable, attributes.Name)
	return nil
}

// newDeleteValidator reads objs from the API server. Its client, which stands for the cache and sends
// the reviews, holds none of them, so a check that read them through the client would see nothing.
func newDeleteValidator(t *testing.T, reviewer *namespaceReviewer, objs ...client.Object) *deleteValidator {
	t.Helper()
	return &deleteValidator{
		client: newClientBuilder(t).WithInterceptorFuncs(interceptor.Funcs{Create: reviewer.create}).Build(),
		reader: newClientBuilder(t, objs...).Build(),
	}
}

// deleteRequest is a project DELETE as the API server sends it: the name of the project and the
// stored object as the old one, with no new object.
func deleteRequest(t *testing.T, user authnv1.UserInfo, name string) admission.Request {
	t.Helper()
	raw, err := json.Marshal(&v1alpha3.Project{ObjectMeta: metav1.ObjectMeta{Name: name}})
	require.NoError(t, err)
	return admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: admissionv1.Delete,
		Name:      name,
		UserInfo:  user,
		OldObject: runtime.RawExtension{Raw: raw},
	}}
}

func namespaceOf(name, project string) *corev1.Namespace {
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	if project != "" {
		namespace.Labels = map[string]string{v1alpha3.ResourceLabelProject: project}
	}
	return namespace
}

// Deleting a project uninstalls its release and deletes its namespaces with everything in them, so a
// project goes only when its requester may delete each of those namespaces. Otherwise a role that
// grants delete on projects and not on namespaces would remove any namespace through its project.
func TestDeleteValidator_RequiresNamespaceDelete(t *testing.T) {
	alice := authnv1.UserInfo{
		Username: "alice",
		UID:      "alice-uid",
		Groups:   []string{"platform-operators", "system:authenticated"},
		Extra:    map[string]authnv1.ExtraValue{"scopes": {"openid", "groups"}},
	}
	main := namespaceOf("foo", "foo")
	additional := namespaceOf("foo-cache", "foo")

	cases := []struct {
		name      string
		user      authnv1.UserInfo
		objects   []client.Object
		deletable []string
		// denial is a part of the refusal; empty means the delete is allowed
		denial string
		// reviewed are the namespaces the webhook asks about, in order
		reviewed []string
	}{
		{
			name:      "a requester who may delete every namespace deletes the project",
			user:      alice,
			objects:   []client.Object{main, additional},
			deletable: []string{"foo", "foo-cache"},
			reviewed:  []string{"foo", "foo-cache"},
		},
		{
			name:      "a requester who may not delete the main namespace is refused",
			user:      alice,
			objects:   []client.Object{main, additional},
			deletable: []string{"foo-cache"},
			denial:    `user "alice" may not delete the "foo" project: deleting it deletes the "foo" namespace`,
			reviewed:  []string{"foo"},
		},
		{
			name:      "a requester who may not delete an additional namespace is refused",
			user:      alice,
			objects:   []client.Object{main, additional},
			deletable: []string{"foo"},
			denial:    `deleting it deletes the "foo-cache" namespace`,
			reviewed:  []string{"foo", "foo-cache"},
		},
		{
			name:     "the main namespace counts without the project label",
			user:     alice,
			objects:  []client.Object{namespaceOf("foo", "")},
			denial:   `deleting it deletes the "foo" namespace`,
			reviewed: []string{"foo"},
		},
		{
			name:      "a missing namespace does not block",
			user:      alice,
			objects:   []client.Object{additional},
			deletable: []string{"foo-cache"},
			reviewed:  []string{"foo-cache"},
		},
		{
			name:    "the namespaces of another project are not asked about",
			user:    alice,
			objects: []client.Object{namespaceOf("bar", "bar"), namespaceOf("foobar", "foobar")},
		},
		{
			name:    "the controller deletes without a review",
			user:    authnv1.UserInfo{Username: rolebindingwebhook.ControllerServiceAccount},
			objects: []client.Object{main, additional},
		},
		{
			name:    "deckhouse deletes without a review",
			user:    authnv1.UserInfo{Username: rolebindingwebhook.DeckhouseServiceAccount},
			objects: []client.Object{main, additional},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reviewer := &namespaceReviewer{deletable: tc.deletable}
			v := newDeleteValidator(t, reviewer, tc.objects...)

			resp := v.Handle(context.Background(), deleteRequest(t, tc.user, "foo"))

			if tc.denial == "" {
				assert.True(t, resp.Allowed, resp.Result)
			} else {
				assert.False(t, resp.Allowed)
				require.NotNil(t, resp.Result)
				assert.Contains(t, resp.Result.Message, tc.denial)
			}

			expected := make([]authorizationv1.SubjectAccessReviewSpec, 0, len(tc.reviewed))
			for _, namespace := range tc.reviewed {
				expected = append(expected, authorizationv1.SubjectAccessReviewSpec{
					User:   "alice",
					UID:    "alice-uid",
					Groups: []string{"platform-operators", "system:authenticated"},
					Extra:  map[string]authorizationv1.ExtraValue{"scopes": {"openid", "groups"}},
					ResourceAttributes: &authorizationv1.ResourceAttributes{
						Namespace: namespace,
						Verb:      "delete",
						Resource:  "namespaces",
						Name:      namespace,
					},
				})
			}
			assert.Equal(t, expected, append([]authorizationv1.SubjectAccessReviewSpec{}, reviewer.reviews...))
		})
	}
}

// A review the API server does not answer refuses the delete: the namespaces would otherwise go
// with the project unchecked.
func TestDeleteValidator_FailsClosedWithoutAReview(t *testing.T) {
	reviewer := &namespaceReviewer{err: errors.New("the API server is unavailable")}
	v := newDeleteValidator(t, reviewer, namespaceOf("foo", "foo"))

	resp := v.Handle(context.Background(), deleteRequest(t, authnv1.UserInfo{Username: "alice"}, "foo"))

	assert.False(t, resp.Allowed)
	require.NotNil(t, resp.Result)
	assert.Equal(t, int32(http.StatusInternalServerError), resp.Result.Code)
	assert.Contains(t, resp.Result.Message, "the API server is unavailable")
}

// The namespaces are read from the API server. A namespace labelled into the project just before the
// delete, which the cache has not seen yet, is checked like any other.
func TestDeleteValidator_ReadsNamespacesFromTheAPIServer(t *testing.T) {
	reviewer := &namespaceReviewer{deletable: []string{"foo"}}
	v := &deleteValidator{
		client: newClientBuilder(t, namespaceOf("foo", "foo")).
			WithInterceptorFuncs(interceptor.Funcs{Create: reviewer.create}).Build(),
		reader: newClientBuilder(t, namespaceOf("foo", "foo"), namespaceOf("foo-cache", "foo")).Build(),
	}

	resp := v.Handle(context.Background(), deleteRequest(t, authnv1.UserInfo{Username: "alice"}, "foo"))

	assert.False(t, resp.Allowed)
	require.NotNil(t, resp.Result)
	assert.Contains(t, resp.Result.Message, `deleting it deletes the "foo-cache" namespace`)
}
