// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package providerinitializer

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"

	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app/options"
)

func restConfigTestSettings() *settings.BaseProviders {
	return settings.NewBaseProviders(settings.ProviderParams{
		Logger: dhlog.Discard(),
		TmpDir: options.DefaultTmpDir(),
	})
}

// TestWithKubeRestConfigNeedsNoSSH pins the contract Deckhouse Commander relies on:
// once an API server endpoint is supplied, the Kubernetes connection is served
// without any SSH provider at all - no session to a master, no kubectl proxy there.
// Passing a nil SSH initializer is the assertion: over SSH, lib-connection refuses.
func TestWithKubeRestConfigNeedsNoSSH(t *testing.T) {
	sett := restConfigTestSettings()

	cfg, err := resolveKubeConfig(t.Context(), sett, newProviderOptions(
		WithConnectionConfig(connectionConfigWithoutHosts),
		WithKubeRestConfig(&rest.Config{Host: "https://ampg-api-server.example:6443", BearerToken: "token"}),
	))
	require.NoError(t, err)
	require.True(t, cfg.IsRest())
	require.False(t, cfg.OverSSH())

	runner, err := provider.GetRunnerInterface(t.Context(), cfg, sett, nil)
	require.NoError(t, err)
	require.IsType(t, &provider.RunnerInterfaceNoAction{}, runner)
}

// TestWithKubeRestConfigIsNotImpersonated: a kubeconfig is rewritten to act as
// the impersonated user, an endpoint the caller already authenticated is not -
// the token Commander sends is the identity, as it was up to v1.76.
func TestWithKubeRestConfigIsNotImpersonated(t *testing.T) {
	restConfig := &rest.Config{Host: "https://ampg-api-server.example:6443", BearerToken: "token"}

	cfg, err := resolveKubeConfig(t.Context(), restConfigTestSettings(), newProviderOptions(
		WithKubeRestConfig(restConfig),
	))
	require.NoError(t, err)
	require.Same(t, restConfig, cfg.RestConfig)
	require.Empty(t, cfg.RestConfig.Impersonate.UserName)
}

// TestWithoutKubeRestConfigGoesOverSSH is the other half of the contract: a request
// that carries no API server endpoint keeps the SSH path it has always had.
func TestWithoutKubeRestConfigGoesOverSSH(t *testing.T) {
	sett := restConfigTestSettings()

	cfg, err := resolveKubeConfig(t.Context(), sett, newProviderOptions(
		WithConnectionConfig(connectionConfigWithoutHosts),
	))
	require.NoError(t, err)
	require.False(t, cfg.IsRest())
	require.True(t, cfg.OverSSH())

	_, err = provider.GetRunnerInterface(t.Context(), cfg, sett, nil)
	require.Error(t, err)
}
