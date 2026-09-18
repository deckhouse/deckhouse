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

package hook

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"
	"github.com/deckhouse/lib-dhctl/pkg/retry"
)

type fakeChecker struct {
	name  string
	ready bool
	err   error
	calls int
}

func (f *fakeChecker) IsReady(context.Context, string) (bool, error) {
	f.calls++
	return f.ready, f.err
}

func (f *fakeChecker) Name() string { return f.name }

// ctxWithStreamLogger renders through the plain sink, which is what the terminal sees; a buffer
// logger would hand back raw slog text and prove nothing about the rendered output.
func ctxWithStreamLogger(t *testing.T) (context.Context, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	return dhlog.ToContext(context.Background(), dhlog.NewStreamLogger(&buf)), &buf
}

// TestIsNodeReadyOpensNoBlockPerChecker is the regression test for the converge output: the
// readiness check runs inside a retry loop, so a process block opened per checker opened and
// failed once per attempt, and the live UI restates every failed block as a persistent FAILED
// milestone. A node that took forty seconds to come up left forty identical rows on screen.
func TestIsNodeReadyOpensNoBlockPerChecker(t *testing.T) {
	retry.InTestEnvironment = true
	t.Cleanup(func() { retry.InTestEnvironment = false })

	ctx, buf := ctxWithStreamLogger(t)
	check := &fakeChecker{name: "Control plane readiness", ready: false}

	ok, err := IsNodeReady(ctx, []NodeChecker{check}, "master-0", "converge")

	require.False(t, ok)
	require.Error(t, err)
	require.Equal(t, 1, check.calls)

	// One attempt must open exactly one block - the retry loop's own. A second "┌" here is a
	// block opened per checker, which on the live UI becomes one pinned FAILED milestone per
	// attempt. (InTestEnvironment collapses the loop to a single attempt; in production the
	// same body runs up to 300 times.)
	out := buf.String()
	require.Equal(t, 1, strings.Count(out, "┌"),
		"exactly one process block per attempt, the retry loop's own: %q", out)
	require.NotContains(t, out, check.name,
		"the checker must not be framed as a block of its own: %q", out)
	require.Contains(t, out, "Node master-0 readiness check",
		"the retry loop's own block is the one that frames the wait: %q", out)
}

// The checker's name moved from the block title into the error, which is the only place left that
// says which of several checks was the one still failing.
func TestIsNodeReadyErrorNamesTheFailingChecker(t *testing.T) {
	retry.InTestEnvironment = true
	t.Cleanup(func() { retry.InTestEnvironment = false })

	ctx, _ := ctxWithStreamLogger(t)

	t.Run("not ready", func(t *testing.T) {
		check := &fakeChecker{name: "Control plane readiness", ready: false}
		_, err := IsNodeReady(ctx, []NodeChecker{check}, "master-0", "converge")
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "Control plane readiness"),
			"error must name the checker: %v", err)
	})

	t.Run("checker error", func(t *testing.T) {
		boom := errors.New("connection refused")
		check := &fakeChecker{name: "Kube node is ready", err: boom}
		_, err := IsNodeReady(ctx, []NodeChecker{check}, "master-0", "converge")
		require.Error(t, err)
		require.True(t, strings.Contains(err.Error(), "Kube node is ready"),
			"error must name the checker: %v", err)
		require.True(t, strings.Contains(err.Error(), "connection refused"),
			"error must keep the underlying cause: %v", err)
	})
}

// A checker that passes must not short-circuit the ones after it.
func TestIsNodeReadyRunsEveryChecker(t *testing.T) {
	retry.InTestEnvironment = true
	t.Cleanup(func() { retry.InTestEnvironment = false })

	ctx, _ := ctxWithStreamLogger(t)
	first := &fakeChecker{name: "Kube node is ready", ready: true}
	second := &fakeChecker{name: "Control plane readiness", ready: true}

	ok, err := IsNodeReady(ctx, []NodeChecker{first, second}, "master-0", "converge")

	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, 1, first.calls)
	require.Equal(t, 1, second.calls)
}
