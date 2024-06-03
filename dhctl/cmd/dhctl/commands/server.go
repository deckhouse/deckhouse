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
	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/proxy"
	"github.com/deckhouse/deckhouse/dhctl/pkg/server/server"
	"gopkg.in/alecthomas/kingpin.v2"
)

func DefineServerCommand(parent *kingpin.Application) *kingpin.CmdClause {
	cmd := parent.Command("server", "Start dhctl as GRPC server.")
	app.DefineServerFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		return proxy.Serve(app.ServerNetwork, app.ServerAddress, app.ServerParallelTasksLimit)
	})
	return cmd
}

func DefineSingleThreadedServerCommand(parent *kingpin.Application) *kingpin.CmdClause {
	cmd := parent.Command("_server", "Start dhctl as GRPC server. Single threaded version.")
	app.DefineServerFlags(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		return server.Serve(app.ServerNetwork, app.ServerAddress)
	})
	return cmd
}
