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

package binding

import (
	"fmt"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The scale here is the expected ceiling of a large cluster: 5000 to 20000 RoleBindings and
// ClusterRoleBindings. RulesFor runs on every request the API server sends to the webhook.
//
//	go test -bench 'Scale' -benchmem ./binding/

func benchIndex(count int) *Index {
	idx := NewIndex()
	for i := 0; i < count; i++ {
		rule := fmt.Sprintf("rule-%d", i%2000)
		crb := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%s%s:level-%d", NamePrefix, rule, i),
				Labels: map[string]string{
					LabelHeritage:  HeritageValue,
					LabelModule:    ModuleName,
					LabelManagedBy: ManagedByValue,
				},
			},
			Subjects: []rbacv1.Subject{
				{Kind: rbacv1.UserKind, Name: fmt.Sprintf("user-%d@example.com", i%2000)},
				{Kind: rbacv1.GroupKind, Name: fmt.Sprintf("team-%d", i%50)},
			},
		}
		idx.Upsert(crb)
	}
	return idx
}

func benchmarkRulesFor(b *testing.B, count int) {
	b.Helper()
	idx := benchIndex(count)
	groups := []string{"team-7", "system:authenticated"}
	b.ReportAllocs()
	for b.Loop() {
		if len(idx.RulesFor("user-7@example.com", groups)) == 0 {
			b.Fatal("the fixture subject is bound, the benchmark measures the wrong path")
		}
	}
}

func BenchmarkScaleRulesFor5k(b *testing.B)  { benchmarkRulesFor(b, 5000) }
func BenchmarkScaleRulesFor20k(b *testing.B) { benchmarkRulesFor(b, 20000) }

// Building the index is what the initial list of the informer costs.
func benchmarkIndexBuild(b *testing.B, count int) {
	b.Helper()
	b.ReportAllocs()
	for b.Loop() {
		benchIndex(count)
	}
}

func BenchmarkScaleIndexBuild5k(b *testing.B)  { benchmarkIndexBuild(b, 5000) }
func BenchmarkScaleIndexBuild20k(b *testing.B) { benchmarkIndexBuild(b, 20000) }
