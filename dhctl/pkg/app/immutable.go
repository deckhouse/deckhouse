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

package app

import (
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// DefineImmutableHostsFlags declares where the machines of a static cluster of
// immutable nodes are. Repeatable, like --ssh-host, and for the same reason:
// one flag per machine reads better than one flag with a list in it.
func DefineImmutableHostsFlags(cmd *kingpin.CmdClause, opts *options.BootstrapOptions) {
	cmd.Flag("master-host", "Control-plane machine of a static cluster as <node-name>=<address>, can be specified multiple times").
		Envar(configEnvName("MASTER_HOSTS")).
		StringsVar(&opts.MasterHostsRaw)
}
