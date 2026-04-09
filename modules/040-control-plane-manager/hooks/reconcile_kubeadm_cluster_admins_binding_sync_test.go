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
	clusterYAML := func(version string) []byte {
		return []byte("kubernetesVersion: \"" + version + "\"\n")
	}

	t.Run("below 1.29 does not touch CRB", func(t *testing.T) {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(crb)
		err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, clusterYAML("1.28.5"), true)
		if err != nil {
			t.Fatalf("sync: %v", err)
		}
		got, err := cl.RbacV1().ClusterRoleBindings().Get(ctx, kubeadmClusterAdminsBindingName, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if got.RoleRef.Name != "cluster-admin" {
			t.Fatalf("expected roleRef cluster-admin, got %q", got.RoleRef.Name)
		}
	})

	t.Run("1.30 user-authz on rebinds cluster-admin CRB to user-authz:cluster-admin", func(t *testing.T) {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName, Labels: map[string]string{"heritage": "deckhouse"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(crb)
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, clusterYAML("1.30.0"), true); err != nil {
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

	t.Run("1.30 user-authz off rebinds user-authz CRB to cluster-admin", func(t *testing.T) {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName, Labels: map[string]string{"heritage": "deckhouse"}},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: userAuthzClusterAdminClusterRoleName},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(crb)
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, clusterYAML("1.30.0"), false); err != nil {
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

	t.Run("1.30 already correct roleRef is no-op", func(t *testing.T) {
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: kubeadmClusterAdminsBindingName, ResourceVersion: "1"},
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: userAuthzClusterAdminClusterRoleName},
			Subjects:   []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: kubeadmClusterAdminsBindingName}},
		}
		cl := fake.NewSimpleClientset(crb)
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, clusterYAML("1.30.0"), true); err != nil {
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

	t.Run("1.30 missing CRB creates user-authz binding when module enabled", func(t *testing.T) {
		cl := fake.NewSimpleClientset()
		if err := syncKubeadmClusterAdminsClusterRoleBinding(ctx, logger, cl, clusterYAML("1.30.0"), true); err != nil {
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
