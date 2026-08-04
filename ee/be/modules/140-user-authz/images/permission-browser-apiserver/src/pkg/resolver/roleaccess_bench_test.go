/*
Copyright 2026 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package resolver

import (
	"context"
	"fmt"
	"testing"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

// The shape of a real cluster: a few dozen roles of the model, a few hundred
// capabilities under them, several hundred discovered resources, and a tail of
// ClusterRoles that belong to no model at all.
const (
	benchScopes          = 4
	benchLevelsPerScope  = 5
	benchCapsPerLevel    = 12
	benchRulesPerCap     = 6
	benchForeignRoles    = 400
	benchDiscoveryGroups = 40
	benchDiscoveryPerGrp = 10
)

func benchScopeNames() []string {
	return []string{"namespace", "project", "subsystem", "system"}[:benchScopes]
}

// benchRoleObjects builds the ClusterRoles of a cluster: the aggregating roles
// of the model, the capabilities they select, and unrelated roles the report
// must walk past.
func benchRoleObjects() []runtime.Object {
	var objs []runtime.Object

	for _, scope := range benchScopeNames() {
		for level := range benchLevelsPerScope {
			levelName := fmt.Sprintf("level%d", level)
			objs = append(objs, &rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name:   fmt.Sprintf("d8:%s:%s", scope, levelName),
					Labels: map[string]string{labelRoleKind: "role", labelRoleScope: scope},
				},
				AggregationRule: &rbacv1.AggregationRule{
					ClusterRoleSelectors: []metav1.LabelSelector{
						{MatchLabels: map[string]string{"rbac.deckhouse.io/aggregate-to-" + scope + "-as": levelName}},
					},
				},
			})

			for capability := range benchCapsPerLevel {
				objs = append(objs, &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("d8:%s-capability:module%d:cap%d", scope, capability, level),
						Labels: map[string]string{
							labelRoleKind:  "capability",
							labelRoleScope: scope,
							"rbac.deckhouse.io/aggregate-to-" + scope + "-as": levelName,
						},
						Annotations: map[string]string{"ru.meta.deckhouse.io/title": "заголовок"},
					},
					Rules: benchCapabilityRules(capability),
				})
			}
		}
	}

	for i := range benchForeignRoles {
		objs = append(objs, &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("foreign-%d", i)},
			Rules:      []rbacv1.PolicyRule{{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"get"}}},
		})
	}

	return objs
}

func benchCapabilityRules(seed int) []rbacv1.PolicyRule {
	rules := make([]rbacv1.PolicyRule, 0, benchRulesPerCap)
	for i := range benchRulesPerCap {
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{fmt.Sprintf("group%d.deckhouse.io", (seed+i)%benchDiscoveryGroups)},
			Resources: []string{fmt.Sprintf("resource%d", i)},
			Verbs:     []string{"get", "list", "watch", "create"},
		})
	}

	// One capability per level carries a wildcard: that is what makes the
	// report expand against the whole discovery snapshot.
	if seed%benchCapsPerLevel == 0 {
		rules = append(rules, rbacv1.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}})
	}

	return rules
}

func benchScopeCache() *ResourceScopeCache {
	cache := NewResourceScopeCache(nil)

	scopeMap := make(map[string]bool, benchDiscoveryGroups*benchDiscoveryPerGrp)
	details := make(map[string]resourceDetails, benchDiscoveryGroups*benchDiscoveryPerGrp)

	for group := range benchDiscoveryGroups {
		for resource := range benchDiscoveryPerGrp {
			key := fmt.Sprintf("group%d.deckhouse.io/resource%d", group, resource)
			scopeMap[key] = resource%3 != 0
			details[key] = resourceDetails{kind: "Kind", verbs: []string{"get", "list", "watch", "create", "update", "patch", "delete"}}
		}
	}

	cache.mu.Lock()
	cache.scopeMap = scopeMap
	cache.details = details
	cache.mu.Unlock()

	return cache
}

func newBenchRoleAccessResolver(b *testing.B) *RoleAccessResolver {
	b.Helper()

	client := fake.NewSimpleClientset(benchRoleObjects()...)
	informerFactory := informers.NewSharedInformerFactory(client, 0)
	lister := informerFactory.Rbac().V1().ClusterRoles().Lister()

	stopCh := make(chan struct{})
	b.Cleanup(func() { close(stopCh) })

	informerFactory.Start(stopCh)
	informerFactory.WaitForCacheSync(stopCh)

	resolver := NewRoleAccessResolver(lister, benchScopeCache())
	resolver.now = func() time.Time { return time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC) }

	return resolver
}

// The catalogue is built on request, so its cost is paid by whoever presses
// "build" -- but it walks every ClusterRole of the cluster for every role it
// reports, and that product is what has to stay in check.
func BenchmarkRoleAccessReport(b *testing.B) {
	resolver := newBenchRoleAccessResolver(b)

	cases := map[string]RoleAccessRequest{
		"matrix":             {ExpandWildcards: true},
		"detailed":           {ExpandWildcards: true, IncludeComposition: true},
		"detailed+inventory": {ExpandWildcards: true, IncludeComposition: true, IncludeInventory: true},
		"withoutWildcards":   {},
	}

	for name, request := range cases {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				status, err := resolver.Report(context.Background(), request)
				if err != nil {
					b.Fatal(err)
				}
				if len(status.Roles) == 0 {
					b.Fatal("the fixture reports roles")
				}
			}
		})
	}
}
