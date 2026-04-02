// Copyright 2025 Flant JSC
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

package destroy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/lib-connection/pkg/kube"
	"github.com/deckhouse/lib-connection/pkg/provider"
	"github.com/deckhouse/lib-connection/pkg/settings"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	libdhctl_log "github.com/deckhouse/lib-dhctl/pkg/log"

	"github.com/deckhouse/deckhouse/dhctl/pkg/app"
)

func TestCleanupsDoesNotPanic(t *testing.T) {
	logger := libdhctl_log.NewDummyLogger(false)
	loggerProvider := libdhctl_log.SimpleLoggerProvider(logger)
	params := settings.ProviderParams{LoggerProvider: loggerProvider, IsDebug: app.IsDebug, NodeTmpPath: app.DeckhouseNodeTmpPath, NodeBinPath: app.DeckhouseNodeBinPath, TmpDir: app.GetDefaultTmpDir()}
	sett := settings.NewBaseProviders(params)

	sshProvider := testCreateDefaultTestSSHProvider(session.Host{Host: "host"}, false)
	providerInitializer := provider.NewSimpleSSHProviderInitializer(sshProvider)
	cfg := &kube.Config{}
	runnerInterface, err := provider.GetRunnerInterface(context.Background(), cfg, sett, providerInitializer)
	require.NoError(t, err)
	kubeProvider := provider.NewDefaultKubeProvider(sett, cfg, runnerInterface)
	require.NoError(t, err)

	provider := newKubeClientProvider(kubeProvider)

	cleanupTest := func() {
		provider.Cleanup(true)
	}
	require.NotPanics(t, cleanupTest)
	// double call not panic
	require.NotPanics(t, cleanupTest)
}
