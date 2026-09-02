/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package rbacadapter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

func snapshotWorld() []runtime.Object {
	deckhouseLabels := map[string]string{"heritage": "deckhouse", "module": "user-authz"}
	return []runtime.Object{
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:editor"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:car0:editor", Labels: deckhouseLabels},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "user-authz:editor"},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-reader", Namespace: "ns-d"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list"}},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "alice-pod-reader", Namespace: "ns-d"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "pod-reader"},
		},
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "secret-viewer"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get", "list"}},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "bob-secret-viewer"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "bob"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "secret-viewer"},
		},
	}
}

func TestSnapshot_MatchesPerCallAuthorize(t *testing.T) {
	auth := newTestRBACAuthorizer(t, snapshotWorld()...)

	cases := []*mockAttrs{
		{
			user:       &user.DefaultInfo{Name: "alice"},
			verb:       "list",
			resource:   "pods",
			isResource: true,
		},
		{
			user:       &user.DefaultInfo{Name: "alice"},
			verb:       "get",
			resource:   "pods",
			namespace:  "ns-d",
			isResource: true,
		},
		{
			user:       &user.DefaultInfo{Name: "alice"},
			verb:       "delete",
			resource:   "pods",
			namespace:  "ns-d",
			isResource: true,
		},
		{
			user:       &user.DefaultInfo{Name: "bob"},
			verb:       "list",
			resource:   "secrets",
			isResource: true,
		},
		{
			user:       &user.DefaultInfo{Name: "nobody"},
			verb:       "list",
			resource:   "pods",
			isResource: true,
		},
	}

	for _, attrs := range cases {
		unboundDecision, unboundReason, err := auth.Authorize(context.Background(), attrs)
		require.NoError(t, err)
		boundCtx := auth.BindSubject(context.Background(), attrs.user)
		boundDecision, boundReason, err := auth.Authorize(boundCtx, attrs)
		require.NoError(t, err)
		assert.Equal(t, unboundDecision, boundDecision, "decision for %+v", attrs)
		assert.Equal(t, unboundReason, boundReason, "reason for %+v", attrs)

		assert.Equal(t,
			auth.AllowsIndependently(context.Background(), attrs),
			auth.AllowsIndependently(boundCtx, attrs),
			"AllowsIndependently for %+v", attrs)
	}
}

func TestSnapshot_AllowsIndependentlySkipsCAR(t *testing.T) {
	auth := newTestRBACAuthorizer(t, snapshotWorld()...)
	alice := &user.DefaultInfo{Name: "alice"}
	ctx := auth.BindSubject(context.Background(), alice)

	carOnly := &mockAttrs{
		user:       alice,
		verb:       "get",
		resource:   "secrets",
		namespace:  "ns-x",
		isResource: true,
	}
	allow, reason, err := auth.Authorize(ctx, carOnly)
	require.NoError(t, err)
	assert.Equal(t, authorizer.DecisionAllow, allow)
	assert.Contains(t, reason, "ClusterRoleBinding")
	assert.False(t, auth.AllowsIndependently(ctx, carOnly),
		"CAR-managed CRB must not count as an independent grant")

	roleBinding := &mockAttrs{
		user:       alice,
		verb:       "get",
		resource:   "pods",
		namespace:  "ns-d",
		isResource: true,
	}
	assert.True(t, auth.AllowsIndependently(ctx, roleBinding))
}

func TestBindSubject_IgnoresNilUser(t *testing.T) {
	auth := newTestRBACAuthorizer(t)
	ctx := context.Background()
	assert.Equal(t, ctx, auth.BindSubject(ctx, nil))
}
