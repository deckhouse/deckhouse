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

package client

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTailOutputKeepsTheEndOfTheOutput(t *testing.T) {
	lines := make([]string, 0, maxLoggedOutputLines+10)
	for i := 1; i <= maxLoggedOutputLines+10; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}

	tail := tailOutput(strings.Join(lines, "\n"))

	require.Equal(t, maxLoggedOutputLines, len(strings.Split(tail, "\n")))
	require.True(t, strings.HasSuffix(tail, fmt.Sprintf("line %d", maxLoggedOutputLines+10)),
		"the last lines are where a failed command reports the reason")
	require.NotContains(t, tail, "line 1\n")
}

func TestTailOutputCapsBytes(t *testing.T) {
	tail := tailOutput(strings.Repeat("a", maxLoggedOutputBytes*2))

	require.LessOrEqual(t, len(tail), maxLoggedOutputBytes)
}

func TestTailOutputHandlesEmptyOutput(t *testing.T) {
	require.Empty(t, tailOutput(""))
	require.Empty(t, tailOutput("\n\n"))
}

func TestCombinedOutputKeepsStdoutAndStderr(t *testing.T) {
	require.Equal(t, "out\nerr", combinedOutput("out", "err"))
	require.Equal(t, "out", combinedOutput("out", ""))
	require.Equal(t, "err", combinedOutput("", "err"))
	require.Empty(t, combinedOutput("", ""))
}

func TestTruncateForEventCollapsesOutputIntoOneLine(t *testing.T) {
	message := truncateForEvent("first line\n\nsecond\tline with spaces")

	require.Equal(t, "first line second line with spaces", message)
	require.LessOrEqual(t, len(message), maxEventMessageBytes)
}

func TestRemoteCommandErrorUnwrapsTheOriginalError(t *testing.T) {
	cause := errors.New("exit status 1")
	err := fmt.Errorf("failed to exec ssh command: %w", &remoteCommandError{
		err:    cause,
		output: "bashible failed",
	})

	require.ErrorIs(t, err, cause)

	var cmdErr *remoteCommandError
	require.ErrorAs(t, err, &cmdErr)
	require.Equal(t, "bashible failed", cmdErr.output)
	require.Equal(t, "bashible failed", cmdErr.EventMessage())
}

func TestRemoteCommandErrorEventMessageFallsBackToTheError(t *testing.T) {
	err := &remoteCommandError{err: errors.New("connection reset by peer")}

	require.Equal(t, "connection reset by peer", err.EventMessage(),
		"a dropped ssh session leaves no output, but the event still needs a message")
}
