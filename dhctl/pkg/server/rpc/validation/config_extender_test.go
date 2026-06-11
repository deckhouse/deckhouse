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

package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

func TestConfigExtender_StaticCluster_EmptyConfig(t *testing.T) {
	cfg := `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: Automatic
clusterDomain: cluster.local
`
	s := New(config.NewSchemaStore(nil), nil)
	resp, err := s.ConfigExtender(context.Background(), &pb.ConfigExtenderRequest{
		Config: cfg,
		Kind:   pb.ConfigExtensionKind_CONFIG_EXTENSION_KIND_CNI,
	})
	require.NoError(t, err)
	require.Empty(t, resp.Config)
}

func TestConfigExtender_UnspecifiedKind_EmptyConfig(t *testing.T) {
	s := New(config.NewSchemaStore(nil), nil)
	resp, err := s.ConfigExtender(context.Background(), &pb.ConfigExtenderRequest{
		Config: "",
		Kind:   pb.ConfigExtensionKind_CONFIG_EXTENSION_KIND_UNSPECIFIED,
	})
	require.NoError(t, err)
	require.Empty(t, resp.Config)
}

// Parse failures surface in the response's Err field rather than as a gRPC
// error, matching the convention used by other validation methods.
func TestConfigExtender_InvalidConfig_ErrorInResponse(t *testing.T) {
	s := New(config.NewSchemaStore(nil), nil)
	resp, err := s.ConfigExtender(context.Background(), &pb.ConfigExtenderRequest{
		Config: "not a yaml: : :",
		Kind:   pb.ConfigExtensionKind_CONFIG_EXTENSION_KIND_CNI,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Err)
	require.Empty(t, resp.Config)
}
