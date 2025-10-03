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
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/server"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/server/settings"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/server/singlethreaded"
)

func DefineServerCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	// cmd = parent.Command(cmd.Model().Name, cmd.Model().Help)
	app.DefineServerFlags(cmd)
	app.DoNotWriteDebugLogFile = true

	cmd.Action(func(c *kingpin.ParseContext) error {
		return server.Serve(settings.ServerParams{
			ServerGeneralParams: settings.ServerGeneralParams{
				Network: app.ServerNetwork,
				Address: app.ServerAddress,
				TmpDir:  app.TmpDirName,
			},
			ParallelTasksLimit:         app.ServerParallelTasksLimit,
			RequestsCounterMaxDuration: app.ServerRequestsCounterMaxDuration,
		})
	})
	return cmd
}

func DefineSingleThreadedServerCommand(cmd *kingpin.CmdClause) *kingpin.CmdClause {
	// cmd = parent.Command(cmd.Model().Name, cmd.Model().Help)
	app.DefineServerFlags(cmd)
	app.DoNotWriteDebugLogFile = true

	cmd.Action(func(c *kingpin.ParseContext) error {
		return singlethreaded.Serve(settings.ServerSingleshotParams{
			ServerGeneralParams: settings.ServerGeneralParams{
				Network: app.ServerNetwork,
				Address: app.ServerAddress,
				TmpDir:  app.TmpDirName,
			},
		})
	})
	return cmd
}
