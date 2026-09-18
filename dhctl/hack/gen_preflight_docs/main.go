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

// Command gen_preflight_docs prints the preflight checks as a Markdown table, for the list the
// installation documentation keeps. That list was maintained by hand and drifted: it named three
// checks that do not exist and omitted fifteen that do.
//
// Unlike hack/gen_preflight_checks, which reads the CheckName constants out of the source, this
// builds the suites and reads the checks as they are actually wired — so a check that exists but
// is in no suite does not appear here either.
package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
)

type row struct {
	name        string
	suite       string
	phase       string
	description string
	skippable   bool
}

func main() {
	rows := collect()

	var b strings.Builder
	fmt.Fprintln(&b, "| Check | Suite | Phase | Passes when | Skippable |")
	fmt.Fprintln(&b, "| --- | --- | --- | --- | --- |")
	for _, r := range rows {
		skippable := "yes"
		if !r.skippable {
			skippable = "no"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n", r.name, r.suite, r.phase, r.description, skippable)
	}

	if _, err := os.Stdout.WriteString(b.String()); err != nil {
		panic(err)
	}
}

func collect() []row {
	// Zero dependencies: the constructors only assemble Check values, and the bodies that would
	// dereference the dependencies are not called here.
	type suiteEntry struct {
		suite string
		s     preflight.Suite
	}

	built := make([]suiteEntry, 0, 5)
	built = append(built,
		suiteEntry{"global", suites.NewGlobalSuite(suites.GlobalDeps{})},
		suiteEntry{"cloud", suites.NewCloudSuite(suites.CloudDeps{})},
		suiteEntry{"cloud (post-infra)", suites.NewPostCloudSuite(suites.PostCloudDeps{})},
		suiteEntry{"immutable", suites.NewImmutableSuite(suites.ImmutableDeps{})},
		suiteEntry{"static", suites.NewStaticSuite(suites.StaticDeps{})},
	)

	seen := map[string]row{}
	for _, entry := range built {
		for _, check := range entry.s.Checks() {
			name := check.Name.String()
			if existing, ok := seen[name]; ok {
				// A check wired into more than one suite is listed once, with both suites named.
				existing.suite += ", " + entry.suite
				seen[name] = existing
				continue
			}
			seen[name] = row{
				name:        name,
				suite:       entry.suite,
				phase:       phaseLabel(check.Phase),
				description: check.Description,
				skippable:   !check.CannotBeSkipped,
			}
		}
	}

	rows := make([]row, 0, len(seen))
	for _, r := range seen {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
	return rows
}

func phaseLabel(p preflight.Phase) string {
	switch p {
	case preflight.PhasePreInfra:
		return "configuration"
	case preflight.PhasePostInfra:
		return "nodes"
	default:
		return string(p)
	}
}
