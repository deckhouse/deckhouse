/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package rbacadapter

import (
	"context"
	"sync"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

// TestRBACAuthorizer_ConcurrentAccess tests that concurrent Authorize calls don't race
func TestRBACAuthorizer_ConcurrentAccess(t *testing.T) {
	// Create fake clientset with some RBAC rules
	objs := []runtime.Object{
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "admin"},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"*"},
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
		},
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "admin-binding"},
			Subjects: []rbacv1.Subject{
				{Kind: "User", Name: "admin-user"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "admin",
			},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: "developer", Namespace: "dev"},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods"},
					Verbs:     []string{"get", "list"},
				},
			},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "developer-binding", Namespace: "dev"},
			Subjects: []rbacv1.Subject{
				{Kind: "User", Name: "dev-user"},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "developer",
			},
		},
	}

	client := fake.NewSimpleClientset(objs...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)

	// Start informers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	rbacAuth := NewRBACAuthorizer(informerFactory)

	const goroutines = 100
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				var attrs authorizer.Attributes

				switch id % 3 {
				case 0:
					// Admin user
					attrs = &mockAttrs{
						user:       &mockUser{name: "admin-user"},
						verb:       "get",
						resource:   "pods",
						namespace:  "default",
						isResource: true,
					}
				case 1:
					// Developer user
					attrs = &mockAttrs{
						user:       &mockUser{name: "dev-user"},
						verb:       "list",
						resource:   "pods",
						namespace:  "dev",
						isResource: true,
					}
				case 2:
					// Unknown user
					attrs = &mockAttrs{
						user:       &mockUser{name: "unknown"},
						verb:       "get",
						resource:   "secrets",
						namespace:  "kube-system",
						isResource: true,
					}
				}

				_, _, _ = rbacAuth.Authorize(ctx, attrs)
			}
		}(i)
	}

	wg.Wait()
}

// Mock types are defined in rbac_test.go
