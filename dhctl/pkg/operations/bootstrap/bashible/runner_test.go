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

package bashible

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"
)

const validChecksum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func newTestNode(t *testing.T, commandStdout []byte, commandErr error, captured *[]string) libcon.Interface {
	t.Helper()

	host := "127.0.0.1"
	cl := testssh.NewClient(session.NewSession(session.Input{
		AvailableHosts: []session.Host{{Host: host, Name: "localhost"}},
	}), nil)

	cl.AddCommandProvider(host, func(_ testssh.Bastion, name string, args ...string) *testssh.Command {
		if captured != nil {
			*captured = append(*captured, strings.Join(append([]string{name}, args...), " "))
		}
		return testssh.NewCommand(commandStdout).WithErr(commandErr)
	})

	require.NoError(t, cl.Start(t.Context()))

	return cl
}

func TestParseStepsStatus(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   map[string]string
	}{
		{
			name:   "empty output",
			output: "",
			want:   map[string]string{},
		},
		{
			name:   "single entry",
			output: "000_step_one " + validChecksum + "\n",
			want:   map[string]string{"000_step_one": validChecksum},
		},
		{
			name: "multiple entries",
			output: "000_step_one " + validChecksum + "\n" +
				"001_step_two " + strings.Repeat("b", 64) + "\n",
			want: map[string]string{
				"000_step_one": validChecksum,
				"001_step_two": strings.Repeat("b", 64),
			},
		},
		{
			name:   "malformed line is skipped",
			output: "not-enough-fields\n000_step_one " + validChecksum + "\n",
			want:   map[string]string{"000_step_one": validChecksum},
		},
		{
			name:   "invalid checksum is skipped",
			output: "000_step_one not-a-checksum\n",
			want:   map[string]string{},
		},
		{
			name:   "invalid name is skipped",
			output: "../etc/passwd " + validChecksum + "\n",
			want:   map[string]string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, parseStepsStatus(tc.output))
		})
	}
}

func TestRunner_FetchStepsStatus(t *testing.T) {
	stdout := []byte("000_step_one " + validChecksum + "\n")
	node := newTestNode(t, stdout, nil, nil)

	r := NewRunner(node, slog.Default())

	statuses, err := r.FetchStepsStatus(t.Context())
	require.NoError(t, err)
	require.Equal(t, map[string]string{"000_step_one": validChecksum}, statuses)
}

func TestRunner_PushStepsStatus_Empty(t *testing.T) {
	var captured []string
	node := newTestNode(t, nil, nil, &captured)

	r := NewRunner(node, slog.Default())

	require.NoError(t, r.PushStepsStatus(t.Context(), nil))
	require.Empty(t, captured, "no command should be issued for an empty statuses map")
}

func TestRunner_PushStepsStatus_InvalidEntry(t *testing.T) {
	var captured []string
	node := newTestNode(t, nil, nil, &captured)

	r := NewRunner(node, slog.Default())

	err := r.PushStepsStatus(t.Context(), map[string]string{"000_step_one": "not-a-checksum"})
	require.Error(t, err)
	require.Empty(t, captured, "no command should be issued when an entry fails validation")
}

func TestRunner_PushStepsStatus_Success(t *testing.T) {
	var captured []string
	node := newTestNode(t, nil, nil, &captured)

	r := NewRunner(node, slog.Default())

	err := r.PushStepsStatus(t.Context(), map[string]string{"000_step_one": validChecksum})
	require.NoError(t, err)
	require.Len(t, captured, 1)
	require.Contains(t, captured[0], bundleStepsStatusDir)
	require.Contains(t, captured[0], validChecksum)
	require.Contains(t, captured[0], "000_step_one")
}

func TestRunner_ClearBundleStepsDir(t *testing.T) {
	var captured []string
	node := newTestNode(t, nil, nil, &captured)

	r := NewRunner(node, slog.Default())

	require.NoError(t, r.clearBundleStepsDir(t.Context()))
	require.Len(t, captured, 1)
	require.Equal(t, "rm -rf "+bundleStepsDir+" "+bundleStepsConflictFile, captured[0])
}

func TestRunner_CheckStepsConflict_None(t *testing.T) {
	node := newTestNode(t, nil, nil, nil)

	r := NewRunner(node, slog.Default())

	steps, err := r.checkStepsConflict(t.Context())
	require.NoError(t, err)
	require.Empty(t, steps)
}

func TestRunner_CheckStepsConflict_Found(t *testing.T) {
	node := newTestNode(t, []byte("075_add_failure.sh\n"), nil, nil)

	r := NewRunner(node, slog.Default())

	steps, err := r.checkStepsConflict(t.Context())
	require.NoError(t, err)
	require.Equal(t, []string{"075_add_failure.sh"}, steps)
}

func TestStepsConflictBreakPredicate(t *testing.T) {
	require.True(t, stepsConflictBreakPredicate(newStepsConflictError([]string{"075_add_failure.sh"})))
	require.False(t, stepsConflictBreakPredicate(errors.New("some transient error")))
}
