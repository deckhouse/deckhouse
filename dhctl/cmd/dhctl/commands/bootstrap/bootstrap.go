// Copyright 2021 Flant JSC
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

package bootstrap

import (
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/bootstrap"
)

func DefineBootstrapCommand(kpApp *kingpin.Application) *kingpin.CmdClause {
	cmd := kpApp.Command("bootstrap", "Bootstrap cluster.")
	app.DefineSSHFlags(cmd)
	app.DefineConfigFlags(cmd)
	app.DefineBecomeFlags(cmd)
	app.DefineCacheFlags(cmd)
	app.DefineDropCacheFlags(cmd)
	app.DefineResourcesFlags(cmd, false)
	app.DefineDeckhouseFlags(cmd)
	app.DefineDontUsePublicImagesFlags(cmd)
	app.DefinePostBootstrapScriptFlags(cmd)
	app.DefinePreflight(cmd)
	app.DefineSkipDebugFile(cmd)

	cmd.Action(func(c *kingpin.ParseContext) error {
		bootstraper := bootstrap.NewClusterBootstrapper(&bootstrap.Params{})
		return bootstraper.Bootstrap()
	})

	return cmd
}
