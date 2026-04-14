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
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	dlog "github.com/deckhouse/deckhouse/pkg/log"
)

func TestSyncKubeadmClusterAdminsClusterRoleBinding(t *testing.T) {
	ctx := context.Background()
	logger := dlog.NewNop()

	t.Run("user-authz on rebinds cluster-admin CRB to user-authz:cluster-admin", func(t *testing.T) {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName, Labels: map[string]string{"heritage": "deckhouse"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
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
		if got.RoleRef.Name != userAuthzClusterAdminClusterRoleName {
			t.Fatalf("expected roleRef %q, got %q", userAuthzClusterAdminClusterRoleName, got.RoleRef.Name)
		}
	})

	t.Run("user-authz off rebinds user-authz CRB to cluster-admin", func(t *testing.T) {
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
		cl := fake.NewSimpleClientset(crb)
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

	t.Run("missing CRB creates user-authz binding when module enabled", func(t *testing.T) {
		cl := fake.NewSimpleClientset()
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
}
