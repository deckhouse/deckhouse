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

package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
)

// DefinePhaseCatalogCommand defines `dhctl phase-catalog`, which prints the
// full phase/subphase title catalog (both namespaces, all supported locales)
// as JSON. installer uses it to render localized phase trees from a progress
// file; it is also handy for debugging. The output is produced by the same
// loader the gRPC GetPhaseCatalog handler uses, so the CLI and gRPC transports
// cannot diverge.
func DefinePhaseCatalogCommand(cmd *kingpin.CmdClause, _ *options.Options) *kingpin.CmdClause {
	return cmd.Action(func(_ *kingpin.ParseContext) error {
		titles, err := phases.LoadTitles()
		if err != nil {
			return fmt.Errorf("loading phase titles: %w", err)
		}

		data, err := json.MarshalIndent(titles.ToCatalog(), "", "  ")
		if err != nil {
			return fmt.Errorf("marshalling phase catalog: %w", err)
		}

		fmt.Fprintln(os.Stdout, string(data))
		return nil
	})
}
