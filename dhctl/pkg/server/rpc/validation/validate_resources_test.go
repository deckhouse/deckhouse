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
	resp, err := s.ValidateResources(t.Context(), &pb.ValidateResourcesRequest{
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
	resp, err := s.ValidateResources(t.Context(), &pb.ValidateResourcesRequest{
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
	resp, err := s.ValidateResources(t.Context(), &pb.ValidateResourcesRequest{
		Config: "",
		Opts:   &pb.ValidateOptions{CommanderMode: true},
	})
	require.NoError(t, err)
	require.Empty(t, resp.Err)
}

// TestValidateResources_PreviousRegistryModesRefused pins that this RPC refuses the previous registry
// implementation's modes like every operation does. The request names no operation, and commander
// sends here the resources of an existing cluster too, but its converge, check and destroy refuse
// these modes as well, so resources that still name them have to be corrected either way.
func TestValidateResources_PreviousRegistryModesRefused(t *testing.T) {
	for mode, cfg := range previousRegistryModeConfigs() {
		t.Run(mode, func(t *testing.T) {
			s := New(config.NewSchemaStore(nil), nil)
			resp, err := s.ValidateResources(t.Context(), &pb.ValidateResourcesRequest{
				Config: cfg,
				Opts:   &pb.ValidateOptions{CommanderMode: true},
			})
			require.NoError(t, err)
			require.Contains(t, resp.Err, "belongs to the previous registry implementation")
		})
	}
}

// previousRegistryModeConfigs returns, by mode, a static cluster whose deckhouse ModuleConfig names a
// mode of the previous registry implementation.
func previousRegistryModeConfigs() map[string]string {
	const staticCluster = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: Automatic
clusterDomain: cluster.local
`
	modes := map[string]string{
		"Proxy": `      mode: Proxy
      proxy:
        imagesRepo: r.example.com/test
        scheme: HTTPS
`,
		"Local": "      mode: Local\n",
	}

	configs := make(map[string]string, len(modes))
	for mode, registry := range modes {
		configs[mode] = staticCluster + `---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
    registry:
` + registry
	}
	return configs
}

// Commander validates the resources of an installation on their own, without a ClusterConfiguration.
// A deckhouse ModuleConfig there that names Proxy or Local of the previous registry implementation is
// refused at this point, in any copy, rather than when the bootstrap starts.
func TestValidateResources_PreviousRegistryImplementation(t *testing.T) {
	deckhouseMC := func(settings string) string {
		return `
---
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: deckhouse
spec:
  enabled: true
  version: 1
  settings:
` + settings
	}
	withoutRegistry := deckhouseMC("    bundle: Default\n")
	secret := `
---
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: default
`

	validate := func(t *testing.T, cfg string) string {
		t.Helper()

		s := New(config.NewSchemaStore(nil), nil)
		resp, err := s.ValidateResources(t.Context(), &pb.ValidateResourcesRequest{
			Config: cfg,
			Opts:   &pb.ValidateOptions{CommanderMode: true},
		})
		require.NoError(t, err)
		return resp.Err
	}

	for mode, settings := range map[string]string{
		"Proxy": "    registry:\n      mode: Proxy\n      proxy:\n        imagesRepo: r.example.com/test\n        scheme: HTTPS\n",
		"Local": "    registry:\n      mode: Local\n",
	} {
		t.Run(mode+" -> refused", func(t *testing.T) {
			respErr := validate(t, secret+withoutRegistry+deckhouseMC(settings))
			require.NotEmpty(t, respErr)

			var ve config.ValidationError
			require.NoError(t, json.Unmarshal([]byte(respErr), &ve))
			require.Equal(t, config.ErrKindValidationFailed, ve.Kind)
			require.Len(t, ve.Errors, 1, "reported once, for the copy that names the mode")
			require.Equal(t, config.ModuleConfigKind, ve.Errors[0].Kind)
			require.Equal(t, "deckhouse", ve.Errors[0].Name)
			require.NotNil(t, ve.Errors[0].Index)
			// Counted as validateUserResources counts them: the payload opens with a separator, so
			// the empty document before it is index 0.
			require.Equal(t, 3, *ve.Errors[0].Index)
			require.Contains(t, strings.Join(ve.Errors[0].Messages, " "), `registry.mode "`+mode+`"`)
			require.Contains(t, strings.Join(ve.Errors[0].Messages, " "), "belongs to the previous registry implementation")
		})
	}

	// With a ClusterConfiguration the payload is parsed whole, and that parse refuses the mode in
	// every copy: the refusal is reported once, not once per validator.
	t.Run("with a ClusterConfiguration -> refused once", func(t *testing.T) {
		const staticCluster = `
apiVersion: deckhouse.io/v1
kind: ClusterConfiguration
clusterType: Static
podSubnetCIDR: 10.111.0.0/16
serviceSubnetCIDR: 10.222.0.0/16
kubernetesVersion: "1.33"
clusterDomain: "cluster.local"
`
		respErr := validate(t, staticCluster+withoutRegistry+deckhouseMC("    registry:\n      mode: Local\n"))
		require.Equal(t, 1, strings.Count(respErr, "belongs to the previous registry implementation"), respErr)
	})

	t.Run("Direct and no registry section -> accepted", func(t *testing.T) {
		require.Empty(t, validate(t, secret+withoutRegistry+deckhouseMC(`    registry:
      mode: Direct
      direct:
        imagesRepo: r.example.com/test
        scheme: HTTPS
`)))
	})
}
