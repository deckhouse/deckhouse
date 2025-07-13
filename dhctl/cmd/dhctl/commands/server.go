// Copyright 2024 Flant JSC
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
	"fmt"
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/server"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/server/singlethreaded"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"runtime"

	"github.com/linkdata/deadlock"
)

func DefineServerCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	// cmd = parent.Command(cmd.Model().Name, cmd.Model().Help)
	app.DefineServerFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {

		if deadlock.Enabled {
			fmt.Fprintf(os.Stderr, "Deadlock detect enabled\n")
		} else {
			fmt.Fprintf(os.Stderr, "Deadlock detect disabled\n")
		}

		deadlock.Opts.OnPotentialDeadlock = func() {
			fmt.Fprintf(os.Stderr, "Deadlock detected\n")
			fmt.Fprintln(os.Stderr, "---Potential gorutines deadlock---")
			// 10 mb
			buf := make([]byte, 10485760)  // Allocate a buffer for the stack trace
			nn := runtime.Stack(buf, true) // Pass 'true' to get all goroutine stack traces
			fmt.Fprintf(os.Stderr, "%s\n", string(buf[:nn]))
			fmt.Fprintln(os.Stderr, "---")

			buf = nil
		}

		deadlock.Opts.LogBuf = os.Stderr

		return server.Serve(app.ServerNetwork, app.ServerAddress, app.ServerParallelTasksLimit, app.ServerRequestsCounterMaxDuration)
	})
	return cmd
}

func DefineSingleThreadedServerCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	// cmd = parent.Command(cmd.Model().Name, cmd.Model().Help)
	app.DefineServerFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		return singlethreaded.Serve(app.ServerNetwork, app.ServerAddress)
	})
	return cmd
}
