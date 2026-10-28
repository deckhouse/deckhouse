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

package configcmd

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// A lone dash as --file reads stdin, as kingpin read it: an empty value.
func TestParseFileDashReadsStdin(t *testing.T) {
	commands := map[string]func(*cobra.Command, *options.Options){
		"cluster-configuration": DefineCommandParseClusterConfiguration,
		"cloud-discovery-data":  DefineCommandParseCloudDiscoveryData,
	}
	for name, define := range commands {
		for _, args := range [][]string{{"-f", "-"}, {"--file", "-"}} {
			t.Run(name+" "+strings.Join(args, " "), func(t *testing.T) {
				cmd := &cobra.Command{Use: name, SilenceUsage: true, SilenceErrors: true}
				define(cmd, options.New())

				stdin := strings.NewReader("apiVersion: deckhouse.io/v1\nkind: Unknown\n")
				cmd.SetIn(stdin)
				cmd.SetOut(io.Discard)
				cmd.SetArgs(args)

				// The document is of no known kind: whether that fails is up to the command.
				if err := cmd.Execute(); err != nil {
					require.NotContains(t, err.Error(), "open -")
				}

				require.Zero(t, stdin.Len(), "stdin is read")
			})
		}
	}
}
