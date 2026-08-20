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

// TestDefineBootstrapCommand_DefinesTheKubeFlags is the flag half of bootstrapping a cluster whose
// control plane dhctl did not create. Such a cluster has no master to tunnel to, so the API server
// can only be reached the way install-deckhouse and create-resources reach it - and until these
// flags existed the walk got as far as InstallDeckhouse and refused there for want of a kube
// provider, naming --kubeconfig, a flag the command did not define.
func TestDefineBootstrapCommand_DefinesTheKubeFlags(t *testing.T) {
	t.Parallel()

	opts := options.New()
	cmd := DefineBootstrapCommand(kingpin.New("dhctl", "").Command("bootstrap", ""), opts)

	defined := make([]string, 0)
	for _, flag := range cmd.Model().Flags {
		defined = append(defined, flag.Name)
	}

	require.Subset(t, defined, []string{"kubeconfig", "kubeconfig-context", "kube-client-from-cluster"})
}
