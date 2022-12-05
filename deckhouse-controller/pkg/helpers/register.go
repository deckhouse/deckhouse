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

package helpers

import (
	sh_app "github.com/flant/shell-operator/pkg/app"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers/jwt"
	dhctlapp "github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands"
)

func DefineHelperCommands(kpApp *kingpin.Application) {
	helpersCommand := sh_app.CommandWithDefaultUsageTemplate(kpApp, "helper", "Deckhouse helpers.")

	{
		genJWTCommand := helpersCommand.Command("gen-jwt", "Generate JWT token.")
		privateKeyPath := genJWTCommand.Flag("private-key-path", "Path to private RSA key in PEM format.").Required().ExistingFile()
		claims := genJWTCommand.Flag("claim", "Claims for token (ex --claim iss=deckhouse --claim sub=akakiy).").Required().StringMap()
		ttl := genJWTCommand.Flag("ttl", "TTL duration (ex. 10s).").Required().Duration()
		genJWTCommand.Action(func(c *kingpin.ParseContext) error {
			return jwt.GenJWT(*privateKeyPath, *claims, *ttl)
		})
	}

	// dhctl parser for ClusterConfiguration and <Provider-name>ClusterConfiguration secrets
	dhctlapp.DefineCommandParseClusterConfiguration(kpApp, helpersCommand)
	dhctlapp.DefineCommandParseCloudDiscoveryData(kpApp, helpersCommand)
}
