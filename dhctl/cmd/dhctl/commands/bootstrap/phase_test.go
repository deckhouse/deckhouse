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

package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

// A bootstrap of an immutable master that has to be aborted leaves VMs and disks
// behind unless abort can reach the cluster, and the only way in is the collected
// admin kubeconfig: the node answers no sshd. The other phase commands that build
// a Kubernetes client take the same flags.
func TestBootstrapPhaseCommandsTakeKubeFlags(t *testing.T) {
	tests := []struct {
		name   string
		define func(*kingpin.CmdClause, *options.Options) *kingpin.CmdClause
	}{
		{name: "abort", define: DefineBootstrapAbortCommand},
		{name: "install-deckhouse", define: DefineBootstrapInstallDeckhouseCommand},
		{name: "create-resources", define: DefineCreateResourcesCommand},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := kingpin.New("dhctl", "")
			cmd := tt.define(app.Command(tt.name, ""), options.New())

			for _, flag := range []string{"kubeconfig", "kubeconfig-context", "kube-client-from-cluster"} {
				require.NotNil(t, cmd.GetFlag(flag), "the %s command takes no --%s", tt.name, flag)
			}
		})
	}
}
