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

package bootstrap

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	preflight "github.com/deckhouse/deckhouse/dhctl/pkg/preflight"
	"github.com/deckhouse/deckhouse/dhctl/pkg/preflight/suites"
)

// DefinePreflightListCommand prints the preflight checks: their names, when each applies and what
// it asserts.
//
// The names were discoverable only from a single --help line listing thirty-odd of them with no
// description, and from documentation that had drifted — it named three checks that do not exist
// and omitted fifteen that do. A failure report now prints the flag that skips it, and this
// prints the rest of them.
func DefinePreflightListCommand(cmd *kingpin.CmdClause, _ *options.Options) *kingpin.CmdClause {
	return cmd.Action(func(_ *kingpin.ParseContext) error {
		return printPreflightChecks(os.Stdout)
	})
}

func printPreflightChecks(out io.Writer) error {
	rows := preflightCheckRows()

	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "PHASE\tNAME\tAPPLIES TO\tPASSES WHEN")
	for _, row := range rows {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", row.phase, row.name, row.suite, row.description)
	}
	if err := writer.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(out, "\nSkip one with --preflight-skip-check=<name>, or all of them with --preflight-skip-all-checks.\n")
	return nil
}

type preflightCheckRow struct {
	phase       string
	name        string
	suite       string
	description string
}

// preflightCheckRows reads the suites as they are actually built. Every dependency is nil: the
// constructors only assemble Check values, and the bodies that would use the dependencies are not
// called here.
func preflightCheckRows() []preflightCheckRow {
	built := []struct {
		suite string
		s     preflight.Suite
	}{
		{"every cluster", suites.NewGlobalSuite(suites.GlobalDeps{})},
		{"cloud", suites.NewCloudSuite(suites.CloudDeps{})},
		{"cloud", suites.NewPostCloudSuite(suites.PostCloudDeps{})},
		{"static", suites.NewStaticSuite(suites.StaticDeps{})},
		{"immutable master", suites.NewImmutableSuite(suites.ImmutableDeps{})},
	}

	seen := map[string]preflightCheckRow{}
	for _, entry := range built {
		for _, check := range entry.s.Checks() {
			name := check.Name.String()
			if existing, ok := seen[name]; ok {
				if !strings.Contains(existing.suite, entry.suite) {
					existing.suite += ", " + entry.suite
					seen[name] = existing
				}
				continue
			}
			seen[name] = preflightCheckRow{
				phase:       preflightPhaseLabel(check.Phase),
				name:        name,
				suite:       entry.suite,
				description: check.Description,
			}
		}
	}

	rows := make([]preflightCheckRow, 0, len(seen))
	for _, row := range seen {
		rows = append(rows, row)
	}
	// Configuration checks first, then node checks, and alphabetically inside each: that is the
	// order they run in, and the order the report prints them in.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].phase != rows[j].phase {
			return rows[i].phase == "configuration"
		}
		return rows[i].name < rows[j].name
	})
	return rows
}

func preflightPhaseLabel(phase preflight.Phase) string {
	switch phase {
	case preflight.PhasePreInfra:
		return "configuration"
	case preflight.PhasePostInfra:
		return "nodes"
	default:
		return string(phase)
	}
}
