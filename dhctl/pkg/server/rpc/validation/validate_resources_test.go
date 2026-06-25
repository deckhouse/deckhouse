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

package validation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	pb "github.com/deckhouse/deckhouse/dhctl/pkg/server/pb/dhctl"
)

func TestValidateResources_StaticCluster_NoCNIError(t *testing.T) {
	// A static cluster config (no provider, no resources) must produce no err.
	// Pre-refactor, the old handler would FAIL: it tried to validate
	// ClusterConfiguration as a "resource" — wrong kind.
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
	resp, err := s.ValidateResources(context.Background(), &pb.ValidateResourcesRequest{
		Config: cfg,
		Opts:   &pb.ValidateOptions{CommanderMode: true},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Err, "static cluster config must not produce validation errors")
}

func TestValidateResources_UserResourceMissingApiVersion(t *testing.T) {
	cfg := `
kind: Secret
metadata:
  name: my-secret
`
	s := New(config.NewSchemaStore(nil), nil)
	resp, err := s.ValidateResources(context.Background(), &pb.ValidateResourcesRequest{
		Config: cfg,
		Opts:   &pb.ValidateOptions{CommanderMode: true},
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Err)

	var ve config.ValidationError
	require.NoError(t, json.Unmarshal([]byte(resp.Err), &ve))
	require.Equal(t, config.ErrKindValidationFailed, ve.Kind, "top-level Kind must be legacy ValidationFailed")
	require.Len(t, ve.Errors, 1)
	require.Equal(t, config.ErrKindValidationFailed, ve.Errors[0].Reason)
	require.Contains(t, strings.Join(ve.Errors[0].Messages, " "), ".apiVersion is required")
}

func TestValidateResources_EmptyPayload_NoError(t *testing.T) {
	s := New(config.NewSchemaStore(nil), nil)
	resp, err := s.ValidateResources(context.Background(), &pb.ValidateResourcesRequest{
		Config: "",
		Opts:   &pb.ValidateOptions{CommanderMode: true},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Err)
}
