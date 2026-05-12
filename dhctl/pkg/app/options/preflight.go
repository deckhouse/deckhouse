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
)

// legacyPreflightSkipAliases maps deprecated --preflight-skip-... flag values
// to their canonical names.
var legacyPreflightSkipAliases = map[string]string{
	"preflight-skip-one-ssh-host": "static-single-ssh-host",
}

// PreflightOptions describes which preflight checks should be skipped.
type PreflightOptions struct {
	SkipAll    bool
	SkipChecks []string
}

// ApplySkips appends the given skip names to SkipChecks, normalizing legacy aliases.
func (o *PreflightOptions) ApplySkips(skipsList []string) {
	for _, skip := range skipsList {
		o.SkipChecks = append(o.SkipChecks, mapLegacyPreflightSkipAlias(skip))
	}
}

// DisabledChecks returns the full set of disabled checks. When SkipAll is set
// every known check is included.
func (o *PreflightOptions) DisabledChecks() []string {
	if o.SkipAll {
		return append(GeneratedChecks(), o.SkipChecks...)
	}
	return o.SkipChecks
}

// IsCheckDisabled reports whether the named check is currently disabled.
func (o *PreflightOptions) IsCheckDisabled(name string) bool {
	if o.SkipAll {
		return true
	}
	for _, skip := range o.SkipChecks {
		if skip == name {
			return true
		}
	}
	return false
}

// Validate ensures every entry in SkipChecks matches a known check name.
// Used as a kingpin PreAction in pkg/app.
func (o *PreflightOptions) Validate() error {
	if len(o.SkipChecks) == 0 {
		return nil
	}

	known := make(map[string]struct{}, len(generatedPreflightChecks))
	for _, name := range generatedPreflightChecks {
		known[name] = struct{}{}
	}

	for _, name := range o.SkipChecks {
		if _, ok := known[name]; !ok {
			return fmt.Errorf("unknown preflight check name: %s", name)
		}
	}

	return nil
}

func mapLegacyPreflightSkipAlias(name string) string {
	if mapped, ok := legacyPreflightSkipAliases[name]; ok {
		return mapped
	}
	return name
}
