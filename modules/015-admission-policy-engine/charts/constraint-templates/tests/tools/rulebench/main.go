// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// rulebench: time AND memory cost benchmark for Gatekeeper ConstraintTemplate
// Rego rules, using OPA's own Go SDK.
//
// Unlike ../bench_rules.py, which shells out to `opa eval --count N --metrics`
// (time only, one process per sample), this builds a rego.PreparedEvalQuery
// once per constraint and drives it with Go's testing.Benchmark + ReportAllocs(),
// which additionally reports real allocation cost per evaluation: bytes/op and
// allocs/op - the actual heap pressure a rule puts on Gatekeeper's GC, not just
// wall-clock time.
//
// This is a standalone Go module (its own go.mod/go.sum) precisely so it can
// depend on github.com/open-policy-agent/opa without touching the deckhouse
// monorepo's root go.mod - `go build ./...` at the repo root does not descend
// into directories that declare their own module.
//
// Usage: since this directory is its own Go module, `go run` on a relative
// path to it fails from outside that module (e.g. from tests/test_cases/
// constraints, which belongs to the repo's root module) with "main module
// ... does not contain package ...". Use the ../rulebench.sh wrapper
// instead - it resolves the constraints-root argument to an absolute path
// and cd's into this directory before invoking `go run .`:
//
//	cd tests/test_cases/constraints   # after rendered/ fixtures exist, see ../../README.md
//	../../tools/rulebench.sh .                  # all constraints
//	../../tools/rulebench.sh . allow-privileged  # one constraint
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
	"gopkg.in/yaml.v3"
)

type ctSource struct {
	Version string   `yaml:"version"`
	Libs    []string `yaml:"libs"`
	Rego    string   `yaml:"rego"`
}

type ctTarget struct {
	Rego string `yaml:"rego"` // legacy form
	Code []struct {
		Source ctSource `yaml:"source"`
	} `yaml:"code"`
}

type constraintTemplate struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Targets []ctTarget `yaml:"targets"`
	} `yaml:"spec"`
}

var pkgRe = regexp.MustCompile(`(?m)^package\s+(\S+)`)

func extractRego(rendered string) ([]string, string, string, bool, error) {
	data, err := os.ReadFile(filepath.Join(rendered, "constraint-template.yaml"))
	if err != nil {
		return nil, "", "", false, err
	}
	var ct constraintTemplate
	if err := yaml.Unmarshal(data, &ct); err != nil {
		return nil, "", "", false, err
	}
	if len(ct.Spec.Targets) == 0 {
		return nil, "", "", false, fmt.Errorf("no targets")
	}
	t := ct.Spec.Targets[0]
	var libs []string
	var regoSrc string
	var v0 bool
	if len(t.Code) > 0 {
		src := t.Code[0].Source
		libs = src.Libs
		regoSrc = src.Rego
		v0 = src.Version != "v1"
	} else {
		regoSrc = t.Rego
		v0 = true
	}
	m := pkgRe.FindStringSubmatch(regoSrc)
	if m == nil {
		return nil, "", "", false, fmt.Errorf("no package clause")
	}
	pkg := m[1]
	return libs, regoSrc, pkg, v0, nil
}

func containsAny(s string, keys ...string) bool {
	for _, k := range keys {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}

// findSamples mirrors bench_rules.py's find_samples(): best-effort (allowed, disallowed)
// pair by filename convention, falling back to the first two distinct files.
func findSamples(rendered string) (string, string) {
	samplesDir := filepath.Join(rendered, "test_samples")
	var all []string
	var allowed, disallowed string
	_ = filepath.Walk(samplesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}
		all = append(all, path)
		return nil
	})
	sort.Strings(all)
	for _, p := range all {
		name := strings.ToLower(filepath.Base(p))
		if disallowed == "" && containsAny(name, "disallowed", "violat", "forbidden", "denied") {
			disallowed = p
		} else if allowed == "" && containsAny(name, "allowed", "compliant") {
			allowed = p
		}
		if allowed != "" && disallowed != "" {
			break
		}
	}
	if allowed == "" && len(all) > 0 {
		allowed = all[0]
	}
	if disallowed == "" && len(all) > 1 {
		disallowed = all[1]
	}
	return allowed, disallowed
}

func loadInput(sample string) (map[string]any, error) {
	data, err := os.ReadFile(sample)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := yaml.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return map[string]any{
		"review": map[string]any{
			"object":    obj,
			"operation": "CREATE",
		},
	}, nil
}

func benchSample(name, label string, pkg string, v0 bool, libs []string, regoSrc string, sample string) string {
	input, err := loadInput(sample)
	if err != nil {
		return fmt.Sprintf("%-40s %-11s ERROR input: %v", name, label, err)
	}

	opts := []func(*rego.Rego){
		rego.Query(fmt.Sprintf("data.%s.violation", pkg)),
	}
	if v0 {
		opts = append(opts, rego.SetRegoVersion(ast.RegoV0))
	}
	for i, lib := range libs {
		opts = append(opts, rego.Module(fmt.Sprintf("lib_%d.rego", i), lib))
	}
	opts = append(opts, rego.Module("policy.rego", regoSrc))

	ctx := context.Background()
	r := rego.New(opts...)
	pq, err := r.PrepareForEval(ctx)
	if err != nil {
		return fmt.Sprintf("%-40s %-11s ERROR prepare: %v", name, label, err)
	}

	// Sanity eval once (surface eval-time errors early, outside the benchmark loop).
	if _, err := pq.Eval(ctx, rego.EvalInput(input)); err != nil {
		return fmt.Sprintf("%-40s %-11s ERROR eval: %v", name, label, err)
	}

	result := testing.Benchmark(func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = pq.Eval(ctx, rego.EvalInput(input))
		}
	})

	return fmt.Sprintf("%-40s %-11s %10.2f ns/op  %8d B/op  %6d allocs/op   (%s)",
		name, label, float64(result.NsPerOp()), result.AllocedBytesPerOp(), result.AllocsPerOp(), filepath.Base(sample))
}

func benchOne(name string, conDir string) []string {
	rendered := filepath.Join(conDir, "rendered")
	libs, regoSrc, pkg, v0, err := extractRego(rendered)
	if err != nil {
		return []string{fmt.Sprintf("%-40s ERROR extract: %v", name, err)}
	}
	allowed, disallowed := findSamples(rendered)
	if allowed == "" && disallowed == "" {
		return []string{fmt.Sprintf("%-40s ERROR: no test samples found", name)}
	}
	var lines []string
	if allowed != "" {
		lines = append(lines, benchSample(name, "allowed", pkg, v0, libs, regoSrc, allowed))
	}
	if disallowed != "" {
		lines = append(lines, benchSample(name, "disallowed", pkg, v0, libs, regoSrc, disallowed))
	}
	return lines
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: rulebench <constraints_root> [only-name]")
		fmt.Fprintln(os.Stderr, "  (normally invoked via ../rulebench.sh, not `go run .` directly - see that script)")
		os.Exit(2)
	}
	root := os.Args[1]
	only := ""
	if len(os.Args) > 2 {
		only = os.Args[2]
	}

	type entry struct{ group, name, dir string }
	var entries []entry
	// Mirror bench_rules.py exactly: scan every directory under root (not a
	// hardcoded {"operation", "security"} list) skipping "_"/"."-prefixed
	// ones, so a new group is picked up by both tools instead of silently
	// skipped by whichever one still hardcodes the old set.
	groups, err := os.ReadDir(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot read constraints root %s: %v\n", root, err)
		os.Exit(1)
	}
	for _, g := range groups {
		if !g.IsDir() || strings.HasPrefix(g.Name(), "_") || strings.HasPrefix(g.Name(), ".") {
			continue
		}
		group := g.Name()
		groupDir := filepath.Join(root, group)
		dirs, err := os.ReadDir(groupDir)
		if err != nil {
			continue
		}
		for _, d := range dirs {
			if !d.IsDir() || strings.HasPrefix(d.Name(), ".") {
				continue
			}
			// Substring match, same as bench_rules.py's --only, so the two
			// tools' "same arguments" really are the same.
			if only != "" && !strings.Contains(d.Name(), only) {
				continue
			}
			entries = append(entries, entry{group, d.Name(), filepath.Join(groupDir, d.Name())})
		}
	}

	// Deliberately sequential: testing.Benchmark's timed run is serialized
	// process-wide by the testing package's own benchmarkLock
	// ($GOROOT/src/testing/benchmark.go), so running these concurrently
	// would not shorten the timed portion at all - only let unrelated
	// goroutines' PrepareForEval/compile work overlap with someone else's
	// timed window and leak a few stray allocations into that window's
	// ReportAllocs() numbers. Not worth the complexity for a diagnostic tool
	// whose whole point is trustworthy B/op and allocs/op numbers.
	fmt.Printf("%-40s %-11s %10s  %10s  %13s   %s\n", "constraint", "sample", "ns/op", "B/op", "allocs/op", "file")
	for _, e := range entries {
		for _, line := range benchOne(e.group+"/"+e.name, e.dir) {
			fmt.Println(line)
		}
	}
}
