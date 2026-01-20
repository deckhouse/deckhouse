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
		if mapped, ok := legacyPreflightSkipAliases[skip]; ok {
			skip = mapped
		}
		PreflightSkipChecks = append(PreflightSkipChecks, skip)
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
}
