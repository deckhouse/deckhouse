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

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefinePreflight registers preflight skip flags into o and validates them in a PreAction.
func DefinePreflight(cmd *kingpin.CmdClause, o *options.PreflightOptions) {
	cmd.Flag("preflight-skip-all-checks", "Skip all preflight checks").
		Envar(configEnvName("PREFLIGHT_SKIP_ALL_CHECKS")).
		BoolVar(&o.SkipAll)

	desc := fmt.Sprintf("Disable specific preflight checks by name (repeatable). Known checks: %s", strings.Join(options.GeneratedChecks(), ", "))
	cmd.Flag("preflight-skip-check", desc).
		Envar(configEnvName("PREFLIGHT_SKIP_CHECKS")).
		PlaceHolder("name").
		StringsVar(&o.SkipChecks)

	cmd.PreAction(func(_ *kingpin.ParseContext) error {
		return o.Validate()
	})
}
