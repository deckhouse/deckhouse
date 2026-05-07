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

package hooks

import (
	"context"
	"errors"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

func userAuthzClusterAdminRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: userAuthzClusterAdminClusterRoleName},
	}
}

func TestSyncKubeadmClusterAdminsClusterRoleBinding(t *testing.T) {
	ctx := context.Background()
	logger := dlog.NewNop()

	t.Run("granular on rebinds cluster-admin CRB to user-authz:cluster-admin", func(t *testing.T) {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName, Labels: map[string]string{"heritage": "deckhouse"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(crb, userAuthzClusterAdminRole())
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, true); err != nil {
			t.Fatal(err)
		}
		got, err := cl.RbacV1().ClusterRoleBindings().Get(ctx, kubeadmClusterAdminsBindingName, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got.RoleRef.Name != userAuthzClusterAdminClusterRoleName {
			t.Fatalf("expected roleRef %q, got %q", userAuthzClusterAdminClusterRoleName, got.RoleRef.Name)
		}
	})

	t.Run("granular off rebinds user-authz CRB back to cluster-admin", func(t *testing.T) {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName, Labels: map[string]string{"heritage": "deckhouse"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: userAuthzClusterAdminClusterRoleName},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(crb)
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, false); err != nil {
			t.Fatal(err)
		}
		got, err := cl.RbacV1().ClusterRoleBindings().Get(ctx, kubeadmClusterAdminsBindingName, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got.RoleRef.Name != clusterAdminWildcardClusterRoleName {
			t.Fatalf("expected roleRef %q, got %q", clusterAdminWildcardClusterRoleName, got.RoleRef.Name)
		}
	})

	t.Run("already correct roleRef is no-op", func(t *testing.T) {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName, ResourceVersion: "1"},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: userAuthzClusterAdminClusterRoleName},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(crb, userAuthzClusterAdminRole())
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, true); err != nil {
			t.Fatal(err)
		}
		got, err := cl.RbacV1().ClusterRoleBindings().Get(ctx, kubeadmClusterAdminsBindingName, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got.ResourceVersion != "1" {
			t.Fatalf("expected CRB to be unchanged (same resourceVersion), got %q", got.ResourceVersion)
		}
	})

	t.Run("missing CRB creates user-authz binding when granular requested", func(t *testing.T) {
		cl := fake.NewSimpleClientset(userAuthzClusterAdminRole())
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, true); err != nil {
			t.Fatal(err)
		}
		got, err := cl.RbacV1().ClusterRoleBindings().Get(ctx, kubeadmClusterAdminsBindingName, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got.RoleRef.Name != userAuthzClusterAdminClusterRoleName {
			t.Fatalf("expected roleRef %q, got %q", userAuthzClusterAdminClusterRoleName, got.RoleRef.Name)
		}
	})

	t.Run("granular off on a fresh cluster preserves wildcard kubeadm-default", func(t *testing.T) {
		// fresh cluster (no CRB yet) with granular=false simulates the bootstrap window
		// (user-authz still off OR clusterIsBootstrapped still false): hook must create the
		// kubeadm-compatible cluster-admin binding so initial helm install does not fail later.
		cl := fake.NewSimpleClientset()
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, false); err != nil {
			t.Fatal(err)
		}
		got, err := cl.RbacV1().ClusterRoleBindings().Get(ctx, kubeadmClusterAdminsBindingName, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got.RoleRef.Name != clusterAdminWildcardClusterRoleName {
			t.Fatalf("expected roleRef %q, got %q", clusterAdminWildcardClusterRoleName, got.RoleRef.Name)
		}
	})

	t.Run("granular on but user-authz:cluster-admin role is missing keeps existing binding", func(t *testing.T) {
		// Race window: user-authz module is enabled but its templates have not been rendered yet.
		// We must NOT delete the existing kubeadm-default binding, otherwise the cluster would lose
		// admin.conf access (binding -> nonexistent role).
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName, ResourceVersion: "42"},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterAdminWildcardClusterRoleName},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(crb)
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, true); err != nil {
			t.Fatal(err)
		}
		got, err := cl.RbacV1().ClusterRoleBindings().Get(ctx, kubeadmClusterAdminsBindingName, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got.RoleRef.Name != clusterAdminWildcardClusterRoleName {
			t.Fatalf("expected roleRef to remain %q, got %q", clusterAdminWildcardClusterRoleName, got.RoleRef.Name)
		}
		if got.ResourceVersion != "42" {
			t.Fatalf("expected CRB to be untouched (resourceVersion 42), got %q", got.ResourceVersion)
		}
	})

	t.Run("failed create after delete rolls back to previous binding", func(t *testing.T) {
		// Simulate a cluster where the old binding exists, but Create on the new binding fails
		// (e.g. transient apiserver error). Hook must restore the previous binding so admin.conf
		// keeps working until the next reconciliation tick.
		existing := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   kubeadmClusterAdminsBindingName,
				Labels: map[string]string{"heritage": "deckhouse", "extra": "preserved"},
			},
			RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterAdminWildcardClusterRoleName},
			Subjects: []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(existing, userAuthzClusterAdminRole())

		// First Create call (the desired user-authz one) fails; subsequent Create (rollback) succeeds.
		var createCalls int
		cl.PrependReactor("create", "clusterrolebindings", func(action clienttesting.Action) (bool, runtime.Object, error) {
			createCalls++
			if createCalls == 1 {
				return true, nil, errors.New("simulated apiserver create failure")
			}
			return false, nil, nil
		})

		err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, true)
		if err == nil {
			t.Fatal("expected sync to return the original create error")
		}

		got, getErr := cl.RbacV1().ClusterRoleBindings().Get(ctx, kubeadmClusterAdminsBindingName, metav1.GetOptions{})
		if getErr != nil {
			t.Fatalf("rollback did not restore the binding: %v", getErr)
		}
		if got.RoleRef.Name != clusterAdminWildcardClusterRoleName {
			t.Fatalf("expected rollback to restore roleRef %q, got %q", clusterAdminWildcardClusterRoleName, got.RoleRef.Name)
		}
		if got.Labels["extra"] != "preserved" {
			t.Fatalf("expected rollback to preserve labels, got %v", got.Labels)
		}
	})

	t.Run("failed create with failed rollback returns the original error and logs the rollback failure", func(t *testing.T) {
		existing := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName, Labels: map[string]string{"heritage": "deckhouse"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterAdminWildcardClusterRoleName},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(existing, userAuthzClusterAdminRole())
		cl.PrependReactor("create", "clusterrolebindings", func(action clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("simulated total apiserver outage on create")
		})

		err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, true)
		if err == nil {
			t.Fatal("expected sync to return an error")
		}
	})

	t.Run("hook does not panic if Get returns an unexpected error", func(t *testing.T) {
		cl := fake.NewSimpleClientset(userAuthzClusterAdminRole())
		cl.PrependReactor("get", "clusterrolebindings", func(action clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("simulated apiserver get outage")
		})

		err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, true)
		if err == nil {
			t.Fatal("expected sync to surface the get error so addon-operator can retry")
		}
		if apierrors.IsNotFound(err) {
			t.Fatalf("expected non-NotFound error, got %v", err)
		}
	})
}

func TestUserAuthzClusterAdminClusterRoleExists(t *testing.T) {
	ctx := context.Background()

	t.Run("returns true when the role exists", func(t *testing.T) {
		cl := fake.NewSimpleClientset(userAuthzClusterAdminRole())
		got, err := userAuthzClusterAdminClusterRoleExists(ctx, cl)
		if err != nil {
			t.Fatal(err)
		}
		if !got {
			t.Fatal("expected true when the ClusterRole exists")
		}
	})

	t.Run("returns false (no error) when the role does not exist", func(t *testing.T) {
		cl := fake.NewSimpleClientset()
		got, err := userAuthzClusterAdminClusterRoleExists(ctx, cl)
		if err != nil {
			t.Fatalf("expected NotFound to be folded into (false, nil), got error: %v", err)
		}
		if got {
			t.Fatal("expected false when the ClusterRole does not exist")
		}
	})

	t.Run("returns the underlying error on transport/auth failures", func(t *testing.T) {
		cl := fake.NewSimpleClientset()
		cl.PrependReactor("get", "clusterroles", func(action clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("simulated apiserver outage")
		})
		_, err := userAuthzClusterAdminClusterRoleExists(ctx, cl)
		if err == nil {
			t.Fatal("expected non-nil error so addon-operator can retry the hook")
		}
	})
}
