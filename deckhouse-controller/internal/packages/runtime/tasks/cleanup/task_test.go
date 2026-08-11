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

package cleanup_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/packages/runtime/tasks/cleanup"
	"github.com/deckhouse/deckhouse/deckhouse-controller/internal/queue"
	"github.com/deckhouse/deckhouse/pkg/log"
)

type CleanupSuite struct {
	suite.Suite

	task queue.Task
}

func TestCleanupSuite(t *testing.T) {
	suite.Run(t, new(CleanupSuite))
}

func (s *CleanupSuite) SetupTest() {
	s.task = cleanup.NewTask("test-module", log.NewNop())
}

// The task exists to be the last thing in a removal's queue, so its whole contract is that it
// completes: only a completed task fires the OnDone the teardown rides on.
func (s *CleanupSuite) TestExecuteSucceeds() {
	s.Require().NoError(s.task.Execute(context.Background()))
}

// A removal superseded by a recreate cancels the barrier's context. It must still not report a
// failure, because a returned error retries forever and would hold the head of the queue.
func (s *CleanupSuite) TestExecuteSucceedsOnCanceledContext() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.Require().NoError(s.task.Execute(ctx))
}

// Execute does no work, so a retry cannot observe anything the first run left behind.
func (s *CleanupSuite) TestExecuteIsIdempotent() {
	s.Require().NoError(s.task.Execute(context.Background()))
	s.Require().NoError(s.task.Execute(context.Background()))
}

// String is the identity in queue dumps and the dedup key under WithUnique.
func (s *CleanupSuite) TestStringNamesTheStage() {
	s.Equal("Cleanup", s.task.String())
}
