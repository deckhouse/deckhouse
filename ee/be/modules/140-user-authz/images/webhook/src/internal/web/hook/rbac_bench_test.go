/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hook

import (
	"fmt"
	"io"
	"log"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

// A Deckhouse cluster carries hundreds of ClusterRoleBindings before a single
// user is created: every module ships its own. These numbers are the shape of
// the problem, not a worst case.
const (
	benchClusterRoleBindings = 600
	benchRulesPerRole        = 40
	benchSubjectsPerBinding  = 3
)

func benchRBACObjects() []runtime.Object {
	objs := []runtime.Object{
		&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: "wide"},
			Rules:      benchRules(),
		},
	}

	for i := range benchClusterRoleBindings {
		objs = append(objs, &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("binding-%d", i)},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "wide"},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: fmt.Sprintf("user-%d@example.com", i)},
				{Kind: rbacv1.GroupKind, Name: fmt.Sprintf("group-%d", i)},
				{Kind: rbacv1.ServiceAccountKind, Name: fmt.Sprintf("sa-%d", i), Namespace: "d8-system"},
			},
		})
	}

	return objs
}

func benchRules() []rbacv1.PolicyRule {
	rules := make([]rbacv1.PolicyRule, 0, benchRulesPerRole)
	for i := range benchRulesPerRole {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{fmt.Sprintf("group-%d.example.com", i)},
			Resources: []string{fmt.Sprintf("resource-%d", i)},
			Verbs:     []string{"get", "list", "watch"},
		})
	}

	return rules
}

func newBenchRBACEvaluator(b *testing.B) *RBACEvaluator {
	b.Helper()

	client := fake.NewSimpleClientset(benchRBACObjects()...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	evaluator := NewRBACEvaluator(log.New(io.Discard, "", 0), informerFactory)

	stopCh := make(chan struct{})
	b.Cleanup(func() { close(stopCh) })
	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	return evaluator
}

// The deny path of a cluster-scoped request runs this check, so it is on the
// authorization path of every "kubectl get pods -A" a tenant user makes.
func BenchmarkAllowsIndependently(b *testing.B) {
	evaluator := newBenchRBACEvaluator(b)

	// The worst case is also the common one: nothing matches, so every binding
	// of the cluster is inspected before the answer is "no".
	miss := &WebhookResourceSpec{
		User:               "alice@example.com",
		Group:              []string{"Everyone", "tenant-a"},
		ResourceAttributes: WebhookResourceAttributes{Verb: "list", Resource: "pods", Namespace: ""},
	}

	// A hit still has to walk the bindings until it finds the one that matches.
	hit := &WebhookResourceSpec{
		User:               fmt.Sprintf("user-%d@example.com", benchClusterRoleBindings-1),
		Group:              []string{"Everyone"},
		ResourceAttributes: WebhookResourceAttributes{Verb: "get", Resource: "resource-0", Group: "group-0.example.com"},
	}

	b.Run("miss", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if evaluator.AllowsIndependently(miss) {
				b.Fatal("the fixture grants nothing to this subject")
			}
		}
	})

	b.Run("hit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if !evaluator.AllowsIndependently(hit) {
				b.Fatal("the fixture grants this subject the resource")
			}
		}
	})
}
