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

package controllersuite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/testclient"
)

var defaultLogOutput = os.Stderr

type Suite struct {
	suite.Suite
	sync.Mutex

	ctx        context.Context
	stopNotify context.CancelFunc
	logger     *log.Logger
	client     *testclient.Client
	logOutput  io.Writer
	tmpDir     string
}

func (suite *Suite) Setup(initObjects []client.Object, opts ...SuiteOption) error {
	suite.Lock()
	defer suite.Unlock()

	return suite.SetupNoLock(initObjects, opts...)
}

func (suite *Suite) SetupNoLock(initObjects []client.Object, opts ...SuiteOption) error {
	suite.withDefaults()
	for _, opt := range opts {
		opt(suite)
	}

	logger := log.NewLogger(
		log.WithLevel(slog.LevelWarn),
		log.WithOutput(suite.logOutput),
		log.WithTimeFunc(func(_ time.Time) time.Time {
			return dependency.TestDC.GetClock().Now()
		}),
	)
	suite.logger = logger.Named("suite")

	var err error
	suite.client, err = testclient.New(logger.Named("test k8s client"), initObjects)
	if err != nil {
		return fmt.Errorf("new: %w", err)
	}
	return nil
}

func (suite *Suite) Check(err error) {
	suite.Require().NoError(err)
}

func (suite *Suite) Logger() *log.Logger {
	suite.Lock()
	defer suite.Unlock()

	if suite.logger == nil {
		suite.T().Fatal("missing controllersuite.(*Suite).Setup() call")
	}

	return suite.logger
}

func (suite *Suite) Client() *testclient.Client {
	suite.Lock()
	defer suite.Unlock()

	if suite.client == nil {
		suite.T().Fatal("missing controllersuite.(*Suite).Setup() call")
	}

	return suite.client
}

func (suite *Suite) TmpDir() string {
	suite.Lock()
	defer suite.Unlock()

	return suite.tmpDir
}

func (suite *Suite) Context() context.Context {
	suite.Lock()
	defer suite.Unlock()

	if suite.ctx == nil {
		suite.T().Fatal("missing controllersuite.(*Suite).SetupSuite() call")
	}

	return suite.ctx
}

func (suite *Suite) withDefaults() {
	suite.logOutput = defaultLogOutput
}

func (suite *Suite) SetupSuite() {
	suite.T().Setenv("D8_IS_TESTS_ENVIRONMENT", "true")
	suite.ctx, suite.stopNotify = signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
}

func (suite *Suite) TearDownSuite() {
	if suite.stopNotify != nil {
		suite.stopNotify()
	}
}

func (suite *Suite) SetupSubTest() {
	err := suite.Context().Err()
	if err != nil {
		suite.T().Fatal(err.Error())
	}

	suite.tmpDir = suite.T().TempDir()
}

func (suite *Suite) TearDownSubTest() {
	tmpDir := suite.TmpDir()
	if tmpDir != "" {
		err := os.RemoveAll(tmpDir)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}

		suite.Check(err)
	}
}

var (
	_ suite.SetupAllSuite    = (*Suite)(nil)
	_ suite.TearDownAllSuite = (*Suite)(nil)
	_ suite.SetupSubTest     = (*Suite)(nil)
	_ suite.TearDownSubTest  = (*Suite)(nil)
)
