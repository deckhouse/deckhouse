// Copyright 2023 Flant JSC
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
	"github.com/google/go-containerregistry/pkg/authn"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations"
)

func DefineMirrorModulesCommand(parent *kingpin.Application) *kingpin.CmdClause {
	cmd := parent.Command("mirror-modules", "Copy Deckhouse modules images from ModuleSource's to local filesystem and to third-party registries.")
	app.DefineMirrorModulesFlags(cmd)

	cmd.Action(func(context *kingpin.ParseContext) error {
		if app.MirrorRegistry != "" {
			var authProvider authn.Authenticator = nil
			if app.MirrorRegistryUsername != "" {
				authProvider = authn.FromConfig(authn.AuthConfig{
					Username: app.MirrorRegistryUsername,
					Password: app.MirrorRegistryPassword,
				})
			}

			return log.Process("mirror", "Push Modules to registry", func() error {
				return operations.PushModulesToRegistry(app.MirrorModuleDirectory, app.MirrorRegistry, authProvider, app.MirrorInsecure, app.MirrorTLSSkipVerify)
			})
		}

		return log.Process("mirror", "Pull Modules to local filesystem", func() error {
			return operations.PullExternalModulesToLocalFS(app.MirrorModuleSourcePath, app.MirrorModuleDirectory, app.MirrorTLSSkipVerify)
		})
	})

	return cmd
}
