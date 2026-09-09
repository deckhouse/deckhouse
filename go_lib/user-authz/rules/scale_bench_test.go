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

package rules

import (
	"fmt"
	"testing"
)

// The scale these benchmarks use is the one the module is expected to meet: 500 rules in a large
// cluster, 2000 as the pessimistic ceiling. A rebuild happens at most once per debounce window, a
// lookup happens on every request the API server sends to the webhook, which has a 3s fail-closed
// deadline.
//
//	go test -bench 'Scale' -benchmem ./rules/

func benchRules(count int) []Rule {
	out := make([]Rule, 0, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("rule-%d", i)
		out = append(out, Rule{
			Name:            name,
			ResourceVersion: fmt.Sprintf("%d", i+1),
			Subjects: []Subject{
				{Kind: "User", Name: fmt.Sprintf("user-%d@example.com", i)},
				{Kind: "Group", Name: fmt.Sprintf("team-%d", i%50)},
				{Kind: "ServiceAccount", Name: "bot", Namespace: fmt.Sprintf("ns-%d", i)},
			},
			LimitNamespaces: []string{fmt.Sprintf("ns-%d", i), fmt.Sprintf("team-%d-.*", i%50)},
		})
	}
	return out
}

func benchmarkBuild(b *testing.B, count int) {
	b.Helper()
	rs := benchRules(count)
	builder := NewBuilder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.Build(rs)
	}
}

func BenchmarkScaleBuild500(b *testing.B)  { benchmarkBuild(b, 500) }
func BenchmarkScaleBuild2000(b *testing.B) { benchmarkBuild(b, 2000) }

// The request path: find every entry that names the subject and fold them into one.
func benchmarkLookup(b *testing.B, count int) {
	b.Helper()
	dir, _ := NewBuilder().Build(benchRules(count))
	user := fmt.Sprintf("user-%d@example.com", count/2)
	groups := []string{fmt.Sprintf("team-%d", (count/2)%50), "system:authenticated"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		entries := dir.Lookup(user, groups)
		combined := Combine(entries)
		if !combined.HasAnyFilters() {
			b.Fatal("the fixture subject is limited, the benchmark measures the wrong path")
		}
	}
}

func BenchmarkScaleLookup500(b *testing.B)  { benchmarkLookup(b, 500) }
func BenchmarkScaleLookup2000(b *testing.B) { benchmarkLookup(b, 2000) }

// The namespace decision itself. Two cases, because they cost very different things and only one
// of them is the interesting one.
//
// The subject below is in a group that aggregates forty rules, so its combined entry carries about
// eighty patterns. Asking about a namespace its own rule opens returns on the first matcher and
// says nothing about scale - that is Hit. Asking about a namespace no pattern opens walks all of
// them, which is what a denial costs, and denials are the common answer on the authorization path
// for a subject a rule limits. That is Miss.
func benchmarkNamespaceAllowed(b *testing.B, namespace string, want bool) {
	b.Helper()
	dir, _ := NewBuilder().Build(benchRules(2000))
	entry := Combine(dir.Lookup("user-1000@example.com", []string{"team-0"}))
	if len(entry.LimitNamespaces) < 50 {
		b.Fatalf("the fixture entry carries %d patterns; the benchmark is meant to walk a large one", len(entry.LimitNamespaces))
	}
	allowed, err := NamespaceAllowed(&entry, namespace, nil)
	if err != nil {
		b.Fatal(err)
	}
	if allowed != want {
		b.Fatalf("NamespaceAllowed(%q) = %v, want %v: the benchmark measures the wrong path", namespace, allowed, want)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := NamespaceAllowed(&entry, namespace, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScaleNamespaceAllowedHit(b *testing.B) {
	benchmarkNamespaceAllowed(b, "ns-1000", true)
}

func BenchmarkScaleNamespaceAllowedMiss(b *testing.B) {
	benchmarkNamespaceAllowed(b, "ns-does-not-match-anything", false)
}

// The ordering guard asks this for every rule a binding claims binds the subject.
func BenchmarkScaleRuleCovers(b *testing.B) {
	dir, _ := NewBuilder().Build(benchRules(2000))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !dir.RuleCovers("rule-1000", "user-1000@example.com", nil) {
			b.Fatal("the fixture rule names the subject")
		}
	}
}
