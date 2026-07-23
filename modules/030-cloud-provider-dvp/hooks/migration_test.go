/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"

	v1 "github.com/deckhouse/deckhouse/modules/030-cloud-provider-dvp/hooks/internal/v1"
)

// buildModuleConfigFromPCC and buildModuleConfigForHybrid are plain functions
// (no go_hook.HookInput, no Kubernetes access), so - unlike
// dvp_cluster_configuration_test.go - these tests need no envtest/fake-cluster
// setup at all and can run in any Go environment.

func strPtr(s string) *string       { return &s }
func slicePtr(s []string) *[]string { return &s }

func TestBuildModuleConfigFromPCC_PropagatesSSHCAKeys(t *testing.T) {
	cfg := &v1.DvpProviderClusterConfiguration{
		Provider: &v1.DvpProvider{
			Namespace:            strPtr("cloud-provider01"),
			KubeconfigDataBase64: strPtr("ZmFrZQ=="),
		},
		Layout:       strPtr("Standard"),
		SSHPublicKey: strPtr("ssh-rsa AAAAB3N"),
		SSHCAKeys:    slicePtr([]string{"ssh-rsa-cert-v01@openssh.com AAAACA", "ssh-rsa-cert-v01@openssh.com BBBBCB"}),
		Region:       strPtr("ru-msk-1"),
	}

	mc, err := buildModuleConfigFromPCC(cfg)
	require.NoError(t, err)

	nodesParameters := digMap(t, mc, "spec", "settings", "nodes", "parameters")
	require.Equal(t, "Standard", nodesParameters["layout"])
	require.Equal(t, "ssh-rsa AAAAB3N", nodesParameters["sshPublicKey"])
	require.Equal(t,
		[]any{"ssh-rsa-cert-v01@openssh.com AAAACA", "ssh-rsa-cert-v01@openssh.com BBBBCB"},
		nodesParameters["sshCAKeys"],
		"sshCAKeys must be propagated into the synthesized v2 ModuleConfig alongside sshPublicKey")
}

func TestBuildModuleConfigFromPCC_OmitsSSHCAKeysWhenAbsent(t *testing.T) {
	cfg := &v1.DvpProviderClusterConfiguration{
		Provider: &v1.DvpProvider{
			Namespace:            strPtr("cloud-provider01"),
			KubeconfigDataBase64: strPtr("ZmFrZQ=="),
		},
		Layout:       strPtr("Standard"),
		SSHPublicKey: strPtr("ssh-rsa AAAAB3N"),
		// SSHCAKeys intentionally nil - the common case, must not appear at all.
	}

	mc, err := buildModuleConfigFromPCC(cfg)
	require.NoError(t, err)

	nodesParameters := digMap(t, mc, "spec", "settings", "nodes", "parameters")
	_, present := nodesParameters["sshCAKeys"]
	require.False(t, present, "sshCAKeys must be entirely absent from the synthesized ModuleConfig when the source PCC has none")
}

func TestBuildModuleConfigFromPCC_OmitsSSHCAKeysWhenEmpty(t *testing.T) {
	cfg := &v1.DvpProviderClusterConfiguration{
		Provider: &v1.DvpProvider{
			Namespace:            strPtr("cloud-provider01"),
			KubeconfigDataBase64: strPtr("ZmFrZQ=="),
		},
		Layout:       strPtr("Standard"),
		SSHPublicKey: strPtr("ssh-rsa AAAAB3N"),
		SSHCAKeys:    slicePtr([]string{}), // explicitly present but empty
	}

	mc, err := buildModuleConfigFromPCC(cfg)
	require.NoError(t, err)

	nodesParameters := digMap(t, mc, "spec", "settings", "nodes", "parameters")
	_, present := nodesParameters["sshCAKeys"]
	require.False(t, present, "an empty (but non-nil) sshCAKeys list must not leave a stray empty key in the ModuleConfig")
}

// digMap walks a chain of map[string]any keys, failing the test immediately
// (with the full path) if any intermediate value is missing or not a map.
func digMap(t *testing.T, root map[string]any, path ...string) map[string]any {
	t.Helper()
	cur := root
	for i, key := range path {
		v, ok := cur[key]
		require.True(t, ok, "missing key %q at path %v", key, path[:i+1])
		next, ok := v.(map[string]any)
		require.True(t, ok, "value at path %v is not a map[string]any (got %T)", path[:i+1], v)
		cur = next
	}
	return cur
}
