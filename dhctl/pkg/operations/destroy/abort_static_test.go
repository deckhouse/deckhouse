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
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/static"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	dhctlstate "github.com/deckhouse/deckhouse/dhctl/pkg/state"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/testssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	rootTmpDirStaticAbort = path.Join(os.TempDir(), "dhctl-test-static-abort")
)

func TestStaticAbort(t *testing.T) {
	defer func() {
		logger := log.GetDefaultLogger()
		if err := os.RemoveAll(rootTmpDirStaticAbort); err != nil {
			logger.LogErrorF("Couldn't remove temp dir '%s': %v\n", rootTmpDirStaticAbort, err)
			return
		}
		logger.LogInfoF("Tmp dir '%s' removed\n", rootTmpDirStaticAbort)
	}()

	host := session.Host{
		Host: "127.0.0.2",
	}

	tests := []testAbortStaticTestParams{
		{
			name:                     "success",
			host:                     host,
			cleanOut:                 "ok",
			destroyShouldReturnError: false,
			overBastion:              true,
		},

		{
			name:                     "without bastion",
			host:                     host,
			cleanOut:                 "ok",
			destroyShouldReturnError: false,
			overBastion:              false,
		},

		{
			name:                     "error",
			host:                     host,
			cleanOut:                 "error",
			cleanErr:                 errors.New("error"),
			destroyShouldReturnError: true,
			overBastion:              true,
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			ts := testCreateAbortStaticProviderTest(t, tst)
			ctx := context.TODO()

			destroyer, err := GetAbortDestroyer(ctx, ts.abortParams)
			require.NoError(t, err, "GetAbortDestroyer should return destroyer")

			err = destroyer.DestroyCluster(ctx, true)
			createAssertError(tst.destroyShouldReturnError, "should cleaned", "should not cleaned")(t, err)

			require.Equal(t, 1, ts.sshProvider.cleanCommandCalled, "should clean command ran once")
			ts.assertStateCacheIsEmpty(t)

			assertOverDefaultBastion(t, tst.overBastion, ts.sshProvider.bastion, "clean script")
		})
	}
}

type testAbortStaticTestParams struct {
	name string

	host        session.Host
	cleanOut    string
	cleanErr    error
	overBastion bool

	destroyShouldReturnError bool
}

type testAbortStaticTest struct {
	*baseTest

	params testAbortStaticTestParams

	abortParams *GetAbortDestroyerParams
	sshProvider *testAbortSSHProvider
}

func (ts *testAbortStaticTest) getStateCache() dhctlstate.Cache {
	return ts.abortParams.StateCache
}

func testCreateAbortStaticProviderTest(t *testing.T, params testAbortStaticTestParams) *testAbortStaticTest {
	require.NotEmpty(t, params.host.Host)

	metaConfig, err := config.ParseConfigFromData(context.TODO(), staticClusterGeneralConfigYAML, config.DummyPreparatorProvider())
	require.NoError(t, err, "parsing config from data")
	metaConfig.UUID = uuid.Must(uuid.NewRandom()).String()

	i := rand.New(rand.NewSource(time.Now().UnixNano()))
	tmpDir, err := fs.RandomTmpDirWithNRunes(rootTmpDirStaticAbort, fmt.Sprintf("%d", i), 15)
	require.NoError(t, err, "create test directory")

	logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())
	logger.LogInfoF("Tmp dir: '%s'\n", tmpDir)

	loggerProvider := log.SimpleLoggerProvider(logger)

	sshProvider := testCreateAbortSSHProvider(params, logger)

	stateCache := cache.NewTestCache()

	pec := phases.NewDefaultPhasedExecutionContext(phases.OperationBootstrap, nil, nil)
	require.NoError(t, pec.InitPipeline(stateCache))

	abortParams := &GetAbortDestroyerParams{
		MetaConfig:             metaConfig,
		StateCache:             stateCache,
		PhasedExecutionContext: pec,
		LoggerProvider:         loggerProvider,
		TmpDir:                 tmpDir,
		SSHClientProvider:      sshProvider.provider,

		overridePhaseProvider: phases.NewDefaultPhaseActionProviderWithStateCache(pec, stateCache),
		staticLoopsParams: static.LoopsParams{
			DestroyMaster: retry.NewEmptyParams(),
		},
	}

	tst := &testAbortStaticTest{
		baseTest: &baseTest{
			stateCache:   stateCache,
			tmpDir:       tmpDir,
			logger:       logger,
			kubeProvider: newKubeClientErrorProvider("kube api does not use in abort"),
		},

		params: params,

		abortParams: abortParams,
		sshProvider: sshProvider,
	}

	return tst
}

type testAbortSSHProvider struct {
	provider *testssh.SSHProvider
	logger   log.Logger

	cleanCommandCalled int
	bastion            testssh.Bastion
}

func (t *testAbortSSHProvider) runCommand(bastion testssh.Bastion, msg string) {
	t.bastion = bastion
	t.cleanCommandCalled++

	t.logger.LogInfoLn(msg)
}

func testCreateAbortSSHProvider(params testAbortStaticTestParams, logger log.Logger) *testAbortSSHProvider {
	result := &testAbortSSHProvider{
		provider: testCreateDefaultTestSSHProvider(params.host, params.overBastion),
		logger:   logger,
	}

	result.provider.AddCommandProvider(params.host.Host, func(bastion testssh.Bastion, scriptPath string, args ...string) *testssh.Command {
		if !testIsCleanCommand(scriptPath) {
			return nil
		}

		cmd := testssh.NewCommand([]byte(params.cleanOut))
		if params.cleanErr != nil {
			cmd.WithErr(params.cleanErr).WithRun(func() {
				result.runCommand(bastion, "Clean command failed")
			})

			return cmd
		}

		return cmd.WithErr(nil).WithRun(func() {
			result.runCommand(bastion, "Clean command success")
		})
	})

	return result
}
