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
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/server"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/server/singlethreaded"
)

func DefineServerCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	// cmd = parent.Command(cmd.Model().Name, cmd.Model().Help)
	app.DefineServerFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		runtime.SetMutexProfileFraction(5)

		go func() {
			fmt.Println("booting on localhost:7550")
			err := http.ListenAndServe(":7550", nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error starting server: %v\n", err)
			}
		}()

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
