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

package immutable

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Every payload this package built used to be a master's, and the group name was
// written in as a constant. A CloudPermanent group now takes the same path, and a
// node that registers into the master group instead of its own gets the master's
// labels and taints - and the NodeGroup it was created for never becomes Ready.
func TestJoinPayloadPutsTheNodeInItsOwnGroup(t *testing.T) {
	nodeConfig, err := buildNodeConfig(t.Context(), nodeConfigInput{
		NodeName:      "front-0",
		NodeGroupName: "front",
		MetaConfig:    testMetaConfig(t),
		Join: &joinInput{
			CACert:             "Y2E=",
			BootstrapToken:     "abcdef.0123456789abcdef",
			APIServerEndpoints: []string{"https://10.0.0.1:6443"},
		},
	})
	require.NoError(t, err)

	require.Equal(t, "front", nodeConfig.Metadata.Labels["node.deckhouse.io/group"],
		"the NodeConfig names the group the node belongs to")
	require.Equal(t, "front", nodeConfig.Spec.Kubelet.NodeLabels["node.deckhouse.io/group"],
		"kubelet registers the node into its own group")
}

// A caller that names no group is building the first master, and the master group
// is the only group it can be in.
func TestJoinPayloadDefaultsToTheMasterGroup(t *testing.T) {
	nodeConfig, err := buildNodeConfig(t.Context(), nodeConfigInput{
		NodeName:   "master-0",
		MetaConfig: testMetaConfig(t),
	})
	require.NoError(t, err)

	require.Equal(t, "master", nodeConfig.Metadata.Labels["node.deckhouse.io/group"])
	require.True(t, strings.HasPrefix(nodeConfig.Metadata.Name, "master-"))
}
