/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authentication/user"

	"permission-browser-apiserver/pkg/authorizer/multitenancy"
)

// TestResolveAccessibleNamespaces_UnionProof reproduces the full real-world
// scenario:
//
//   - CAR "car0" for user alice: accessLevel Editor (materialized as a
//     cluster-wide CRB user-authz:car0:editor) + limitNamespaces [ns-a, ns-b, ns-c]
//   - a plain RoleBinding for alice in ns-d (get pods)
//   - AR "ar0" in ns-g (materialized as a RoleBinding in ns-g)
//   - ns-f exists but alice has no grants there
//
// Expected (union semantics): get accessiblenamespaces returns
// ns-a, ns-b, ns-c (CAR), ns-d (RoleBinding), ns-g (AR); NOT ns-f.
func TestResolveAccessibleNamespaces_UnionProof(t *testing.T) {
	deckhouseLabels := map[string]string{
		"heritage": "deckhouse",
		"module":   "user-authz",
	}

	objs := []runtime.Object{
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-a"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-b"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-c"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-d"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-f"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-g"}},

		// CAR accessLevel role (cluster-wide grant on namespaced resources)
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:editor"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}},
			},
		},
		// CAR-generated CRB (what cluster-role-bindings.yaml renders)
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "user-authz:car0:editor",
				Labels: deckhouseLabels,
			},
			Subjects: []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}},
			RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "user-authz:editor"},
		},

		// Plain RoleBinding in ns-d (created by hand, not by user-authz)
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

		// AR-generated RoleBinding in ns-g (what cluster-role-bindings.yaml renders for ARs)
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "user-authz:user"},
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get", "list", "watch"}},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "user-authz:ar0:user",
				Namespace: "ns-g",
				Labels:    deckhouseLabels,
			},
			Subjects: []rbacv1.Subject{{Kind: rbacv1.UserKind, Name: "alice"}},
			RoleRef:  rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "user-authz:user"},
		},
	}

	// MT config exactly as the configmap renders it: CAR with limitNamespaces + AR.
	config := `{
		"crds": [
			{
				"name": "car0",
				"spec": {
					"accessLevel": "Editor",
					"limitNamespaces": ["ns-a", "ns-b", "ns-c"],
					"subjects": [{"kind": "User", "name": "alice"}]
				}
			}
		],
		"ars": [
			{
				"name": "ar0",
				"namespace": "ns-g",
				"spec": {"subjects": [{"kind": "User", "name": "alice"}]}
			}
		]
	}`

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))

	mtEngine, err := multitenancy.NewEngine(configPath, nil, nil, nil)
	require.NoError(t, err)

	resolver := setupResolver(t, objs, mtEngine)

	namespaces, err := resolver.ResolveAccessibleNamespaces(&user.DefaultInfo{Name: "alice"})
	require.NoError(t, err)

	t.Logf("ACTUAL accessible namespaces: %v", namespaces)
	require.ElementsMatch(t, []string{"ns-a", "ns-b", "ns-c", "ns-d", "ns-g"}, namespaces,
		"union semantics: CAR namespaces + RoleBinding namespaces + AR namespaces, without ns-f")
}
