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

package options

import (
	"fmt"
	"slices"
	"strings"

	otattribute "go.opentelemetry.io/otel/attribute"
)

// legacyPreflightSkipAliases maps deprecated --preflight-skip-... flag values
// to their canonical names.
var legacyPreflightSkipAliases = map[string]string{
	"preflight-skip-one-ssh-host": "static-single-ssh-host",

	// The checks that ask about the machine itself are asked of a cloud master as well as of a
	// static node, so their names no longer say static-. The old spellings keep working: a
	// pipeline that carries one must not start failing at argument parsing over a rename.
	"static-system-requirements":   "node-system-requirements",
	"static-hostname":              "node-hostname",
	"static-disk-space":            "node-disk-space",
	"static-node-leftovers":        "node-leftovers",
	"static-node-cri-requirements": "node-cri-requirements",
	"static-node-internal-network": "node-internal-network",
}

// unskippablePreflightChecks are the checks --preflight-skip-check must refuse to turn off,
// mapped to why.
//
// These do not ask whether the operator's cluster is in good shape — they ask whether dhctl has
// any code for the shape it is being pointed at. Skipping one does not let the run proceed; it
// lets the run proceed into a state the path behind it cannot handle, and fail later and worse.
// It is why ValidateClusterType had to be taken out of preflight: everything there was skippable,
// including the guards.
//
// The list is kept in step with the CannotBeSkipped fields of the checks themselves by
// TestUnskippableChecksMatchTheSuites in pkg/preflight/suites.
var unskippablePreflightChecks = map[string]string{
	"immutable-supported-provider": "an immutable master is only implemented for some platforms; skipping the check does not implement it for the others",
	"immutable-registry-mode":      "an immutable master pulls from the registry directly, and there is no code path for the other registry modes",
}

// retiredPreflightChecks are names that no longer name a check, mapped to what became of them.
//
// A name is retired, not forgotten: a pipeline that carries --preflight-skip-check=cidr-intersection
// must not start failing at argument parsing because the check moved. The flag is accepted, has no
// effect, and says so once.
var retiredPreflightChecks = map[string]string{
	"cidr-intersection":        "the cluster CIDRs are now compared while the configuration is loaded, which no flag skips",
	"static-cidr-intersection": "the cluster CIDRs are now compared with internalNetworkCIDRs while the configuration is loaded, which no flag skips",
	"public-domain-template":   "publicDomainTemplate is now compared with clusterDomain while the configuration is loaded, which no flag skips",
}

// splitPreflightChecks keeps a skip flag meaning what it meant before the check behind it was
// split in two. The old registry-credentials reached the registry and then authenticated to it;
// an operator who skipped it skipped both, and a pipeline carrying that flag must not start
// failing on the half that kept the name.
//
// Each entry names the checks the original check used to perform, including itself.
var splitPreflightChecks = map[string][]string{
	"registry-credentials":  {"registry-reachable"},
	"dhctl-edition":         {"deckhouse-image-available"},
	"sudo-allowed":          {"sudo-installed"},
	"static-ssh-credential": {"static-ssh-connectivity"},
}

// PreflightOptions describes which preflight checks should be skipped.
type PreflightOptions struct {
	SkipAll    bool
	SkipChecks []string
	// FailFast stops the phase at its first failed check instead of reporting every one of
	// them. The default is to report them all: three mistakes in one configuration used to
	// cost three runs, and on a cloud cluster each of those runs paid for infrastructure.
	FailFast bool
	// NoCache runs every check for real, ignoring the remembered passes of previous runs.
	NoCache bool
	// Retired holds one line per skip name that no longer names a check, filled in by Normalize.
	// The name is accepted so an existing pipeline keeps working; the flag layer prints these
	// once so the operator learns it no longer does anything.
	Retired []string
}

// ApplySkips appends the given skip names to SkipChecks. Aliases, comma-separated values and
// unknown names are dealt with by Normalize, which every caller runs afterwards.
func (o *PreflightOptions) ApplySkips(skipsList []string) {
	o.SkipChecks = append(o.SkipChecks, skipsList...)
}

// DisabledChecks returns the full set of disabled checks. When SkipAll is set
// every known check is included.
func (o *PreflightOptions) DisabledChecks() []string {
	if o.SkipAll {
		// --preflight-skip-all-checks means every check the operator could have named one by
		// one, and the unskippable ones are exactly the ones they could not have.
		skippable := make([]string, 0, len(GeneratedChecks()))
		for _, name := range GeneratedChecks() {
			if _, unskippable := unskippablePreflightChecks[name]; !unskippable {
				skippable = append(skippable, name)
			}
		}
		return append(skippable, o.SkipChecks...)
	}

	disabled := make([]string, 0, len(o.SkipChecks))
	seen := make(map[string]struct{}, len(o.SkipChecks))
	for _, name := range o.SkipChecks {
		for _, expanded := range append([]string{name}, splitPreflightChecks[name]...) {
			if _, dup := seen[expanded]; dup {
				continue
			}
			seen[expanded] = struct{}{}
			disabled = append(disabled, expanded)
		}
	}
	return disabled
}

// Normalize is the one place a skip name is turned from what the user typed into what the runner
// matches: comma-separated lists are split, whitespace is trimmed, retired names are mapped to
// current ones, duplicates are dropped, and anything left over is refused with the closest known
// name. It is called from the kingpin PreAction and from the gRPC request path, so a typo is
// caught wherever it arrives — the server used to accept one silently and run every check.
func (o *PreflightOptions) Normalize() error {
	known := GeneratedChecks()
	seen := make(map[string]struct{}, len(o.SkipChecks))
	normalized := make([]string, 0, len(o.SkipChecks))
	o.Retired = nil

	for _, raw := range o.SkipChecks {
		for _, name := range strings.Split(raw, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if mapped, ok := legacyPreflightSkipAliases[name]; ok {
				name = mapped
			}
			if became, retired := retiredPreflightChecks[name]; retired {
				o.Retired = append(o.Retired, fmt.Sprintf("%s: %s", name, became))
				continue
			}
			if why, unskippable := unskippablePreflightChecks[name]; unskippable {
				return fmt.Errorf("preflight check %q cannot be skipped: %s", name, why)
			}
			if !slices.Contains(known, name) {
				return unknownCheckError(name, known)
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			normalized = append(normalized, name)
		}
	}

	o.SkipChecks = normalized
	return nil
}

// Validate is Normalize under the name the flag layer used to call.
//
// Deprecated: call Normalize; it also rewrites SkipChecks into what the runner matches.
func (o *PreflightOptions) Validate() error {
	return o.Normalize()
}

func unknownCheckError(name string, known []string) error {
	if suggestion, ok := closestName(name, known); ok {
		return fmt.Errorf("unknown preflight check name %q; did you mean %q? Run `dhctl bootstrap --help` for all names", name, suggestion)
	}
	return fmt.Errorf("unknown preflight check name %q; run `dhctl bootstrap --help` for all names", name)
}

// closestName picks the known name nearest to what was typed. The common mistake is a fragment
// of a name rather than a misspelling of one — "sudo" for "sudo-allowed", "registry" for
// "registry-credentials" — so a name that contains what was typed is preferred outright, and
// edit distance is the fallback for actual typos. Nothing is offered when nothing is near: a
// suggestion further than a third of the typed name away is noise, not help.
func closestName(name string, known []string) (string, bool) {
	shortestContaining := ""
	for _, candidate := range known {
		if !strings.Contains(candidate, name) {
			continue
		}
		if shortestContaining == "" || len(candidate) < len(shortestContaining) {
			shortestContaining = candidate
		}
	}
	if shortestContaining != "" {
		return shortestContaining, true
	}

	best, bestDistance := "", -1
	for _, candidate := range known {
		d := editDistance(name, candidate)
		if bestDistance < 0 || d < bestDistance {
			best, bestDistance = candidate, d
		}
	}
	if bestDistance < 0 || bestDistance > max(2, len(name)/3) {
		return "", false
	}
	return best, true
}

// editDistance is Levenshtein over two rows, enough for a "did you mean" on a list of 30 names.
func editDistance(a, b string) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func (o *PreflightOptions) ToSpanAttributes() []otattribute.KeyValue {
	return []otattribute.KeyValue{
		otattribute.Bool("preflight.skipAll", o.SkipAll),
		otattribute.StringSlice("preflight.skipChecks", o.SkipChecks),
		otattribute.Bool("preflight.failFast", o.FailFast),
		otattribute.Bool("preflight.noCache", o.NoCache),
	}
}
