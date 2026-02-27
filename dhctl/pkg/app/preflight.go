// Copyright 2023 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"fmt"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
)

var PreflightSkipAll = false

var PreflightSkipChecks []string

var legacyPreflightSkipAliases = map[string]string{
	"preflight-skip-one-ssh-host": "static-single-ssh-host",
}

func ApplyPreflightSkips(skipsList []string) {
	for _, skip := range skipsList {
		PreflightSkipChecks = append(PreflightSkipChecks, mapLegacyPreflightSkipAlias(skip))
	}
}

func DisabledPreflightChecks() []string {
	if PreflightSkipAll {
		return append(generatedChecks(), PreflightSkipChecks...)
	}
	return PreflightSkipChecks
}

func IsPreflightCheckDisabled(name string) bool {
	if PreflightSkipAll {
		return true
	}
	for _, skip := range PreflightSkipChecks {
		if skip == name {
			return true
		}
	}
	return false
}

func DefinePreflight(cmd *kingpin.CmdClause) {
	cmd.Flag("preflight-skip-all-checks", "Skip all preflight checks").
		Envar(configEnvName("PREFLIGHT_SKIP_ALL_CHECKS")).
		BoolVar(&PreflightSkipAll)

	desc := fmt.Sprintf("Disable specific preflight checks by name (repeatable). Known checks: %s", strings.Join(generatedChecks(), ", "))
	cmd.Flag("preflight-skip-check", desc).
		Envar(configEnvName("PREFLIGHT_SKIP_CHECKS")).
		PlaceHolder("name").
		StringsVar(&PreflightSkipChecks)

	cmd.PreAction(func(_ *kingpin.ParseContext) error {
		return validatePreflightSkipChecks()
	})
}

func mapLegacyPreflightSkipAlias(name string) string {
	if mapped, ok := legacyPreflightSkipAliases[name]; ok {
		return mapped
	}
	return name
}

func validatePreflightSkipChecks() error {
	if len(PreflightSkipChecks) == 0 {
		return nil
	}

	known := make(map[string]struct{}, len(generatedPreflightChecks))
	for _, name := range generatedPreflightChecks {
		known[name] = struct{}{}
	}

	for _, name := range PreflightSkipChecks {
		if _, ok := known[name]; !ok {
			return fmt.Errorf("unknown preflight check name: %s", name)
		}
	}

	return nil
}
