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

package static

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	libcon "github.com/deckhouse/lib-connection/pkg"
	"github.com/deckhouse/lib-connection/pkg/ssh/session"
	"github.com/deckhouse/lib-connection/pkg/ssh/testssh"
	dhlog "github.com/deckhouse/lib-dhctl/pkg/logger"

	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

const connectedHost = "10.0.0.1"

func destroyerWithCommand(t *testing.T, command *testssh.Command, attempts int) (*Destroyer, libcon.SSHClient, session.Host) {
	t.Helper()

	host := session.Host{Host: connectedHost}
	sshProvider := testssh.NewSSHProvider(session.NewSession(session.Input{
		User:           "user",
		Port:           "22",
		AvailableHosts: []session.Host{host},
	}), true)
	sshProvider.AddCommandProvider(host.Host, func(_ testssh.Bastion, _ string, _ ...string) *testssh.Command {
		return command
	})

	sshClient, err := sshProvider.Client(context.Background())
	require.NoError(t, err)

	destroyer := NewDestroyer(&DestroyerParams{
		Logger: dhlog.Discard(),
		Loops: LoopsParams{
			DestroyMaster: retry.NewEmptyParams(
				retry.WithAttempts(attempts),
				retry.WithWait(time.Millisecond),
			),
		},
	})

	return destroyer, sshClient, host
}

// ssh exits 255 on its own failures: no connection, failed auth, or a dropped session. The cleanup
// script schedules its reboot and exits 0, so a 255 means that the host was not cleaned up.
func TestDestroyerFailsWhenSSHExits255(t *testing.T) {
	sshFailure := exec.Command("sh", "-c", "exit 255").Run()
	var exitErr *exec.ExitError
	require.ErrorAs(t, sshFailure, &exitErr)
	require.Equal(t, 255, exitErr.ExitCode())

	command := testssh.NewCommand(nil).
		WithStdErr([]byte(strings.Join([]string{
			"channel 0: open failed: connect failed: Connection refused",
			"stdio forwarding failed",
			"kex_exchange_identification: Connection closed by remote host",
			"Connection closed by UNKNOWN port 65535",
			"debug1: Exit status 255",
		}, "\n"))).
		WithErr(sshFailure)
	destroyer, sshClient, host := destroyerWithCommand(t, command, 1)

	err := destroyer.processStaticHost(context.Background(), sshClient, host, func(string) {}, "cleanup")
	require.ErrorContains(t, err, "exit status 255, last output: channel 0: open failed: connect failed: Connection refused | stdio forwarding failed")
	require.NotContains(t, err.Error(), "debug1")
}

// The executor reports a command killed by its timeout as a clean exit. Checking the attempt context
// catches that case, and the cleanup is not retried on a host that may already be partly cleaned.
func TestDestroyerFailsWhenCleanupOutlivesTimeout(t *testing.T) {
	defaultTimeout := cleanupAttemptTimeout
	cleanupAttemptTimeout = 10 * time.Millisecond
	t.Cleanup(func() { cleanupAttemptTimeout = defaultTimeout })

	attempts := 0
	command := testssh.NewCommand(nil).WithRun(func() {
		attempts++
		time.Sleep(100 * time.Millisecond)
	})
	destroyer, sshClient, host := destroyerWithCommand(t, command, 3)

	err := destroyer.processStaticHost(context.Background(), sshClient, host, func(string) {}, "cleanup")
	require.ErrorContains(t, err, "context deadline exceeded")
	require.Equal(t, 1, attempts)
}
