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
	"slices"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr string
	}{
		{
			name:  "nothing to normalize",
			input: nil,
			want:  []string{},
		},
		{
			name:  "a comma-separated list is one flag value, not one name",
			input: []string{"sudo-allowed,time-drift"},
			want:  []string{"sudo-allowed", "time-drift"},
		},
		{
			name:  "surrounding whitespace is not part of the name",
			input: []string{" sudo-allowed ", "\ttime-drift"},
			want:  []string{"sudo-allowed", "time-drift"},
		},
		{
			// The name documented for DVP. The alias existed but the CLI never applied it, so
			// following the documentation made dhctl refuse to start.
			name:  "a retired name maps to the current one",
			input: []string{"preflight-skip-one-ssh-host"},
			want:  []string{"static-single-ssh-host"},
		},
		{
			name:  "duplicates collapse",
			input: []string{"sudo-allowed", "sudo-allowed,sudo-allowed"},
			want:  []string{"sudo-allowed"},
		},
		{
			name:    "a near miss is refused with the name that was meant",
			input:   []string{"sudo"},
			wantErr: `did you mean "sudo-allowed"`,
		},
		{
			name:    "an unknown name is refused",
			input:   []string{"not-a-check-at-all"},
			wantErr: `unknown preflight check name "not-a-check-at-all"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := &PreflightOptions{SkipChecks: tc.input}
			err := o.Normalize()

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("want an error containing %q, got none", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want an error containing %q, got %q", tc.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(o.SkipChecks, tc.want) {
				t.Errorf("got %v, want %v", o.SkipChecks, tc.want)
			}
		})
	}
}

// TestDisabledChecksUnderSkipAll: the flag covers every name the operator could have passed one
// by one — which is every check except the ones that cannot be skipped at all.
func TestDisabledChecksUnderSkipAll(t *testing.T) {
	o := &PreflightOptions{SkipAll: true}
	disabled := o.DisabledChecks()

	if got, want := len(disabled), len(GeneratedChecks())-len(unskippablePreflightChecks); got != want {
		t.Errorf("got %d disabled checks, want %d", got, want)
	}
	for name := range unskippablePreflightChecks {
		if slices.Contains(disabled, name) {
			t.Errorf("--preflight-skip-all-checks must not disable %q", name)
		}
	}
}

// TestEveryGeneratedNameNormalizes guards the two lists against each other: a name dhctl
// advertises in --help must be one --preflight-skip-check accepts — unless it is one of the few
// that cannot be skipped, which is refused on purpose and with a reason.
func TestEveryGeneratedNameNormalizes(t *testing.T) {
	skippable := make([]string, 0, len(GeneratedChecks()))
	for _, name := range GeneratedChecks() {
		if _, unskippable := unskippablePreflightChecks[name]; unskippable {
			o := &PreflightOptions{SkipChecks: []string{name}}
			err := o.Normalize()
			if err == nil {
				t.Errorf("--preflight-skip-check=%s must be refused", name)
				continue
			}
			if !strings.Contains(err.Error(), "cannot be skipped") {
				t.Errorf("the refusal must say why, got %q", err)
			}
			continue
		}
		skippable = append(skippable, name)
	}

	o := &PreflightOptions{SkipChecks: skippable}
	if err := o.Normalize(); err != nil {
		t.Fatalf("a generated name was refused: %v", err)
	}
	if !slices.Equal(o.SkipChecks, skippable) {
		t.Error("normalizing the generated list must leave it unchanged")
	}
}

// TestSkippingASplitCheckSkipsWhatItUsedToDo: the old registry-credentials reached the registry
// and then authenticated to it. An operator who skipped it skipped both, and a pipeline carrying
// that flag must not start failing on the half that kept the name.
func TestSkippingASplitCheckSkipsWhatItUsedToDo(t *testing.T) {
	tests := []struct {
		skipped string
		want    []string
	}{
		{"registry-credentials", []string{"registry-credentials", "registry-reachable"}},
		{"dhctl-edition", []string{"dhctl-edition", "deckhouse-image-available"}},
		{"sudo-allowed", []string{"sudo-allowed", "sudo-installed"}},
		{"static-ssh-credential", []string{"static-ssh-credential", "static-ssh-connectivity"}},
		{"time-drift", []string{"time-drift"}},
	}

	for _, tt := range tests {
		t.Run(tt.skipped, func(t *testing.T) {
			o := &PreflightOptions{SkipChecks: []string{tt.skipped}}
			if err := o.Normalize(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(o.DisabledChecks(), tt.want) {
				t.Errorf("got %v, want %v", o.DisabledChecks(), tt.want)
			}
		})
	}
}

// TestEverySplitExpansionNamesAKnownCheck guards the table against the names drifting.
func TestEverySplitExpansionNamesAKnownCheck(t *testing.T) {
	known := GeneratedChecks()
	for original, expansions := range splitPreflightChecks {
		if !slices.Contains(known, original) {
			t.Errorf("splitPreflightChecks names %q, which is not a check", original)
		}
		for _, expanded := range expansions {
			if !slices.Contains(known, expanded) {
				t.Errorf("splitPreflightChecks[%q] names %q, which is not a check", original, expanded)
			}
		}
	}
}

// TestRetiredCheckNamesAreAcceptedNotRefused: cidr-intersection and public-domain-template moved
// into configuration loading, where no flag skips them. A pipeline that still carries
// --preflight-skip-check=cidr-intersection must not start failing at argument parsing over it.
func TestRetiredCheckNamesAreAcceptedNotRefused(t *testing.T) {
	o := &PreflightOptions{SkipChecks: []string{"cidr-intersection", "sudo-allowed", "public-domain-template"}}

	if err := o.Normalize(); err != nil {
		t.Fatalf("a retired name must be accepted, got %v", err)
	}
	if !slices.Equal(o.SkipChecks, []string{"sudo-allowed"}) {
		t.Errorf("the retired names must be dropped, got %v", o.SkipChecks)
	}
	if len(o.Retired) != 2 {
		t.Fatalf("both retired names must be reported, got %v", o.Retired)
	}
	for _, retired := range o.Retired {
		if !strings.Contains(retired, "while the configuration is loaded") {
			t.Errorf("a retired name must say where the check went, got %q", retired)
		}
	}
}

// TestRetiredNamesAreNotAlsoLiveChecks guards the two tables against each other.
func TestRetiredNamesAreNotAlsoLiveChecks(t *testing.T) {
	known := GeneratedChecks()
	for name := range retiredPreflightChecks {
		if slices.Contains(known, name) {
			t.Errorf("%q is listed as retired but is still a check", name)
		}
	}
}

// TestRenamedNodeChecksKeepTheirOldNames: the checks that ask about the machine are asked of a
// cloud master too now, so their names no longer say static-. An operator's pipeline that carries
// the old spelling must not start failing at argument parsing over that.
func TestRenamedNodeChecksKeepTheirOldNames(t *testing.T) {
	tests := map[string]string{
		"static-system-requirements":   "node-system-requirements",
		"static-hostname":              "node-hostname",
		"static-disk-space":            "node-disk-space",
		"static-node-leftovers":        "node-leftovers",
		"static-node-cri-requirements": "node-cri-requirements",
		"static-node-internal-network": "node-internal-network",
	}

	for old, current := range tests {
		t.Run(old, func(t *testing.T) {
			o := &PreflightOptions{SkipChecks: []string{old}}
			if err := o.Normalize(); err != nil {
				t.Fatalf("the old name must still be accepted, got %v", err)
			}
			if !slices.Equal(o.SkipChecks, []string{current}) {
				t.Errorf("got %v, want the check it was renamed to, %q", o.SkipChecks, current)
			}
		})
	}
}

// TestNoAliasShadowsALiveCheck: an alias that is also a check name would rewrite a name to
// something else behind the operator's back.
func TestNoAliasShadowsALiveCheck(t *testing.T) {
	known := GeneratedChecks()
	for alias, target := range legacyPreflightSkipAliases {
		if slices.Contains(known, alias) {
			t.Errorf("%q is an alias and a check at the same time", alias)
		}
		if !slices.Contains(known, target) {
			t.Errorf("alias %q points at %q, which is not a check", alias, target)
		}
	}
}
