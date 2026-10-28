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
	"strings"

	"github.com/spf13/cobra"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefinePreflight registers the preflight flags into o and normalizes the skip names in a
// hook, so an unknown one is refused before the command starts rather than ignored.
func DefinePreflight(cmd *cobra.Command, o *options.PreflightOptions) {
	flags := cmd.Flags()
	BoolFlag(flags, Flag{
		Name: "preflight-skip-all-checks",
		Env:  "DHCTL_CLI_PREFLIGHT_SKIP_ALL_CHECKS",
		Help: "Skip all preflight checks",
	}, &o.SkipAll)

	// The names, and where to find out what they mean. Grouping them by phase here would be
	// better still, but the phase lives on the check and pkg/app cannot reach the suites without
	// an import cycle — `dhctl preflight list` prints that table, from the suites themselves.
	desc := "Disable specific preflight checks by `name` (repeatable, or comma-separated). " +
		"Run `dhctl preflight list` for the names and what each one asserts."
	StringsFlag(flags, Flag{
		Name: "preflight-skip-check",
		Env:  "DHCTL_CLI_PREFLIGHT_SKIP_CHECKS",
		Help: desc,
	}, &o.SkipChecks)

	BoolFlag(flags, Flag{
		Name: "preflight-fail-fast",
		Env:  "DHCTL_CLI_PREFLIGHT_FAIL_FAST",
		Help: "Stop at the first failed preflight check instead of reporting every check of the phase",
	}, &o.FailFast)
	BoolFlag(flags, Flag{
		Name: "preflight-no-cache",
		Env:  "DHCTL_CLI_PREFLIGHT_NO_CACHE",
		Help: "Run every preflight check for real, ignoring results remembered from a previous run",
	}, &o.NoCache)

	AfterApply(cmd, func(cmd *cobra.Command) error {
		if err := o.Normalize(); err != nil {
			return err
		}
		for _, retired := range o.Retired {
			// Not an error: the flag is still accepted so an existing pipeline keeps working.
			// It is said once, so the operator learns the check moved.
			name, hint, _ := strings.Cut(retired, ":")
			cmd.PrintErrf("WARNING --preflight-skip-check=%s no longer skips anything. %s\n", name, strings.TrimSpace(hint))
		}
		return nil
	})
}
